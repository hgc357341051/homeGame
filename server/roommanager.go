package main

import (
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// RoomManager 管理所有房间
type RoomManager struct {
	rooms map[string]*Room
	mu    sync.Mutex
}

func newRoomManager() *RoomManager {
	return &RoomManager{rooms: make(map[string]*Room)}
}

// roomIdleTimeout 房间空置多久后自动删除（无在座玩家也无旁观者）
const roomIdleTimeout = 30 * time.Minute

// reaper 周期性清理：空房间回收 + 超时掉线座位推进/释放（即使没人广播也能解死锁）
func (rm *RoomManager) reaper() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		type pendingAction struct {
			room      *Room
			sysMsg    string
			evs       []Event
			needBcast bool
		}
		var actions []pendingAction
		toDelete := []string{}
		rm.mu.Lock()
		now := time.Now()
		for code, room := range rm.rooms {
			room.mu.Lock()
			pa := pendingAction{room: room}
			// 1) 释放已超时的掉线座位，并触发引擎推进/结算
			if room.Phase == "playing" {
				for _, s := range room.Seats {
					if s.isOffline() && time.Since(s.DisconnectedAt) > offlineTimeout {
						name := s.Name
						seatIdx := s.Index
						room.standLocked(seatIdx)
						evs := room.Engine.OnSeatVacated(room, seatIdx)
						pa.sysMsg = name + " 掉线超时，座位已释放"
						pa.evs = evs
						pa.needBcast = true
						break // 一轮处理一个座位即可，下一轮再处理下一个
					}
				}
			}
			// 2) 判定房间是否空闲（无在座、无旁观）
			hasPlayer := false
			for _, s := range room.Seats {
				if s.PlayerID != "" {
					hasPlayer = true
					break
				}
			}
			empty := !hasPlayer && len(room.Spectators) == 0
			room.mu.Unlock()
			if empty && now.Sub(room.createdAt) > roomIdleTimeout {
				toDelete = append(toDelete, code)
				continue
			}
			if pa.needBcast {
				actions = append(actions, pa)
			}
		}
		for _, code := range toDelete {
			delete(rm.rooms, code)
		}
		rm.mu.Unlock()
		// 解锁后发送系统消息、事件、广播状态
		for _, pa := range actions {
			if pa.sysMsg != "" {
				pa.room.systemChat(pa.sysMsg)
			}
			pa.room.emitEvents(pa.evs)
			pa.room.broadcastState()
		}
		for _, code := range toDelete {
			log.Printf("房间 %s 已空置超时，已回收", code)
		}
	}
}

const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func genRoomCode(rm *RoomManager) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		b := make([]byte, 6)
		for i := range b {
			b[i] = codeAlphabet[r.Intn(len(codeAlphabet))]
		}
		code := string(b)
		if _, ok := rm.rooms[code]; !ok {
			return code
		}
	}
	return strings.ToUpper(genID())[:6]
}

var idAlphabet = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

// genUniqueID 生成唯一玩家 ID，最多重试 100 次以避免碰撞。
func (h *Hub) genUniqueID() string {
	for i := 0; i < 100; i++ {
		id := genID()
		h.mu.Lock()
		_, exists := h.clients[id]
		h.mu.Unlock()
		if !exists {
			return id
		}
	}
	return genID() // 极端情况下兜底返回
}

func genID() string {
	b := make([]rune, 12)
	for i := range b {
		b[i] = idAlphabet[rand.Intn(len(idAlphabet))]
	}
	return string(b)
}

func newEngine(game string) GameEngine {
	switch game {
	case "ddz":
		return &ddzEngine{}
	case "zjh":
		return &zjhEngine{}
	case "nn":
		return &nnEngine{}
	}
	return &ddzEngine{}
}

var avatars = []string{"🦊", "🐼", "🦁", "🐯", "🐰", "🐻", "🐲", "🦄", "🐧", "🦉", "🐺", "🐸"}

func avatarFor(name string) string {
	if name == "" {
		return avatars[0]
	}
	sum := 0
	for _, c := range name {
		sum += int(c)
	}
	return avatars[sum%len(avatars)]
}

func (rm *RoomManager) handleAction(c *Client, action string, data ActionData) {
	switch action {
	case "enter":
		if name, ok := data["name"].(string); ok && name != "" {
			c.name = name
		}
		// 支持重连：客户端携带本地存储的 playerId 时，恢复使用以便夺回原座位
		if pid, ok := data["playerId"].(string); ok && pid != "" {
			c.hub.mu.Lock()
			if existing, exists := c.hub.clients[pid]; exists && existing != c {
				// 该 pid 已被另一个活跃连接占用：拒绝覆盖，避免旧引用泄漏与座位竞争
				// 旧连接仍在线，新连接应使用服务端分配的新 pid
				c.hub.mu.Unlock()
				c.sendMsg(Message{Type: "error", Data: ActionData{"msg": "该玩家标识已在线，已为你分配新身份"}})
			} else {
				delete(c.hub.clients, c.playerID)
				c.playerID = pid
				c.hub.clients[pid] = c
				c.hub.mu.Unlock()
			}
		}
		if c.name == "" {
			c.name = "玩家" + c.playerID[:4]
		}
		c.sendMsg(Message{Type: "entered", Data: ActionData{"playerId": c.playerID, "name": c.name}})
		return
	case "createRoom":
		rm.createRoom(c, data)
		return
	case "joinRoom":
		rm.joinRoom(c, data)
		return
	case "ping":
		// 应用层心跳：回复 pong，不进入房间逻辑
		c.sendMsg(Message{Type: "pong", Data: ActionData{}})
		return
	}

	// 其余动作需在房间内
	if c.room == nil {
		c.emitError("请先创建或加入房间")
		return
	}
	c.room.applyAction(c, action, data)
}

// maxSpectators 每个房间旁观者上限，防止旁观连接耗尽资源
const maxSpectators = 20

func (rm *RoomManager) createRoom(c *Client, data ActionData) {
	game, _ := data["game"].(string)
	if game != "ddz" && game != "zjh" && game != "nn" {
		game = "ddz"
	}
	if c.name == "" {
		c.name = "玩家" + c.playerID[:4]
	}
	rm.mu.Lock()
	code := genRoomCode(rm)
	room := newRoom(code, game, c.playerID)
	rm.rooms[code] = room
	rm.mu.Unlock()
	c.room = room
	// 房主自动作为旁观者，需手动入座（持房间锁 append，避免与并发 joinRoom 竞争）
	room.mu.Lock()
	room.Spectators = append(room.Spectators, c)
	room.mu.Unlock()
	c.sendMsg(Message{Type: "roomCreated", Data: ActionData{"code": code}})
	room.broadcastState()
}

func (rm *RoomManager) joinRoom(c *Client, data ActionData) {
	code, _ := data["code"].(string)
	code = strings.ToUpper(strings.TrimSpace(code))
	rm.mu.Lock()
	room, ok := rm.rooms[code]
	rm.mu.Unlock()
	if !ok {
		c.emitError("房间不存在，请检查配对码")
		return
	}
	if c.name == "" {
		if name, ok := data["name"].(string); ok && name != "" {
			c.name = name
		}
	}
	if c.name == "" {
		c.name = "玩家" + c.playerID[:4]
	}
	// 重连恢复座位（夺回掉线座位或替换僵尸连接）
	if room.tryReclaim(c) {
		c.room = room
		// 防御性：确保不在旁观者列表中重复
		room.mu.Lock()
		for i, sp := range room.Spectators {
			if sp == c {
				room.Spectators = append(room.Spectators[:i], room.Spectators[i+1:]...)
				break
			}
		}
		// 重连补发：若对局中且轮到该玩家，补发 turn 事件，否则客户端 turn 状态为陈旧值
		// （broadcastState 只发 roomState + deal，不发 turn）
		var turnEv *Event
		seatIdx := room.findSeat(c.playerID)
		if seatIdx >= 0 && room.Phase == "playing" && room.Engine != nil {
			turnEv = room.Engine.CurrentTurnEvent(room, seatIdx)
		}
		room.mu.Unlock()
		c.sendMsg(Message{Type: "joined", Data: ActionData{"code": code, "reclaimed": true}})
		room.broadcastState()
		if turnEv != nil {
			c.sendMsg(Message{Type: turnEv.Type, Data: turnEv.Data})
		}
		return
	}
	// 已在该房间旁观则不重复加入（防止重复 joinRoom 导致消息重复）
	room.mu.Lock()
	alreadySpectator := false
	for _, sp := range room.Spectators {
		if sp == c {
			alreadySpectator = true
			break
		}
	}
	// 若已在某座位（在线），也不重复加入
	for _, s := range room.Seats {
		if s.Client == c {
			alreadySpectator = true
			break
		}
	}
	if !alreadySpectator {
		if len(room.Spectators) >= maxSpectators {
			room.mu.Unlock()
			c.emitError("房间旁观人数已满")
			return
		}
		c.room = room
		room.Spectators = append(room.Spectators, c)
	}
	room.mu.Unlock()
	c.sendMsg(Message{Type: "joined", Data: ActionData{"code": code}})
	if !alreadySpectator {
		room.systemChat(c.name + " 加入了房间")
	}
	room.broadcastState()
}
