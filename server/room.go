package main

import (
	"math/rand"
	"strings"
	"sync"
	"time"
)

const startChips = 1000

// Seat 表示房间中的一个座位。Hand 仅存在于服务端内存。
type Seat struct {
	Index         int
	Client        *Client // nil 表示空座
	Name          string
	PlayerID      string
	Chips         int
	Ready         bool
	Hand          []Card // 仅服务端可见，永不下发
	IsLandlord    bool
	IsDealer      bool
	IsFolded      bool
	IsLooked      bool
	CurrentBet    int
	HasNiu        bool
	NiuValue      int
	NiuCards      []Card // 凑牛的 3 张
	SettledDelta  int
	LookedIndices []bool // 蒙牌模式下已查看的牌索引
	IsRevealed    bool   // 蒙牌模式下是否已开牌(向所有人展示)
}

func (s *Seat) occupied() bool { return s.Client != nil }

// GameEngine 三款游戏的统一接口
type GameEngine interface {
	Name() string
	Label() string
	MinPlayers() int
	MaxPlayers() int
	Start(r *Room) []Event
	HandleAction(r *Room, seat int, action string, data ActionData) []Event
	PublicArea(r *Room) PublicAreaView
	// PlayerHand 返回该玩家当前可看到的手牌（蒙牌模式下仅为已查看的牌）
	PlayerHand(s *Seat) []Card
}

// Room 一个游戏房间
type Room struct {
	Code       string
	Game       string
	HostID     string
	Phase      string // waiting / playing / settled
	Seats      []*Seat
	Spectators []*Client
	Engine     GameEngine
	BlindMode  bool // 炸金花蒙牌模式开关（房主开局前设置）
	rnd        *rand.Rand
	mu         sync.Mutex
	createdAt  time.Time
}

func newRoom(code, game, hostID string) *Room {
	r := &Room{
		Code:      code,
		Game:      game,
		HostID:    hostID,
		Phase:     "waiting",
		rnd:       rand.New(rand.NewSource(time.Now().UnixNano())),
		createdAt: time.Now(),
	}
	r.Engine = newEngine(game)
	max := r.Engine.MaxPlayers()
	r.Seats = make([]*Seat, max)
	for i := 0; i < max; i++ {
		r.Seats[i] = &Seat{Index: i, Chips: startChips}
	}
	return r
}

func (r *Room) seatCount() int {
	n := 0
	for _, s := range r.Seats {
		if s.occupied() {
			n++
		}
	}
	return n
}

func (r *Room) findSeat(playerID string) int {
	for i, s := range r.Seats {
		if s.occupied() && s.PlayerID == playerID {
			return i
		}
	}
	return -1
}

func (r *Room) firstEmptySeat() int {
	for i, s := range r.Seats {
		if !s.occupied() {
			return i
		}
	}
	return -1
}

func (r *Room) broadcast(ev Event) {
	switch ev.Target {
	case -1:
		for _, s := range r.Seats {
			if s.occupied() {
				s.Client.sendMsg(Message{Type: ev.Type, Data: ev.Data})
			}
		}
		for _, c := range r.Spectators {
			c.sendMsg(Message{Type: ev.Type, Data: ev.Data})
		}
	default:
		if ev.Target >= 0 && ev.Target < len(r.Seats) && r.Seats[ev.Target].occupied() {
			r.Seats[ev.Target].Client.sendMsg(Message{Type: ev.Type, Data: ev.Data})
		}
	}
}

// broadcastState 向房间所有人发送裁剪后的房间状态；对在座且对局中的玩家补发自己的手牌
func (r *Room) broadcastState() {
	r.mu.Lock()
	defer r.mu.Unlock()
	// 旁观者与在座玩家都收到公开状态
	for _, s := range r.Seats {
		if !s.occupied() {
			continue
		}
		s.Client.sendMsg(Message{Type: "roomState", Data: r.viewFor(s.Client)})
		if r.Phase == "playing" {
			// 蒙牌模式下由引擎按可见性返回；其他模式即返回完整手牌
			hand := r.Engine.PlayerHand(s)
			dealData := ActionData{"cards": hand}
			if r.BlindMode {
				dealData["blindMode"] = true
				dealData["lookedIndices"] = s.LookedIndices
			}
			s.Client.sendMsg(Message{Type: "deal", Data: dealData})
		}
	}
	for _, c := range r.Spectators {
		c.sendMsg(Message{Type: "roomState", Data: r.viewFor(c)})
	}
}

// viewFor 构建给指定客户端的视角（绝不包含他人手牌）
func (r *Room) viewFor(c *Client) RoomStateView {
	mySeat := r.findSeat(c.playerID)
	seats := make([]SeatView, 0, len(r.Seats))
	for _, s := range r.Seats {
		sv := SeatView{
			Seat:         s.Index,
			Chips:        s.Chips,
			Ready:        s.Ready,
			IsLandlord:   s.IsLandlord,
			IsDealer:     s.IsDealer,
			IsFolded:     s.IsFolded,
			IsLooked:     s.IsLooked,
			CurrentBet:   s.CurrentBet,
			HasNiu:       s.HasNiu,
			NiuValue:     s.NiuValue,
			SettledDelta: s.SettledDelta,
			IsRevealed:   s.IsRevealed,
		}
		// 蒙牌模式下，LookedIndices 总是下发，让其他玩家看到谁在看第几张
		if s.LookedIndices != nil {
			sv.LookedIndices = append([]bool{}, s.LookedIndices...)
		}
		// 已开牌：在 SeatView 中包含 RevealedCards（该座位的 Hand），所有人可见
		if s.IsRevealed && len(s.Hand) > 0 {
			sv.RevealedCards = append([]Card{}, s.Hand...)
		}
		// 已分配座位（含临时断线）均展示信息；断线时 online=false
		if s.PlayerID != "" {
			sv.PlayerID = s.PlayerID
			sv.Name = s.Name
			sv.Avatar = avatarFor(s.Name)
			sv.Online = s.occupied()
			sv.CardCount = len(s.Hand)
			if s.PlayerID == r.HostID {
				sv.IsOwner = true
			}
		}
		seats = append(seats, sv)
	}
	pub := PublicAreaView{}
	if r.Engine != nil {
		pub = r.Engine.PublicArea(r)
	}
	return RoomStateView{
		Code:       r.Code,
		Game:       r.Game,
		HostID:     r.HostID,
		Phase:      r.Phase,
		Seats:      seats,
		MySeat:     mySeat,
		PublicArea: pub,
		MinPlayers: r.Engine.MinPlayers(),
		MaxPlayers: r.Engine.MaxPlayers(),
		GameLabel:  r.Engine.Label(),
		BlindMode:  r.BlindMode,
	}
}

func (r *Room) handleDisconnect(c *Client) {
	r.mu.Lock()
	leaveName := c.name
	// 旁观者移除
	for i, sp := range r.Spectators {
		if sp == c {
			r.Spectators = append(r.Spectators[:i], r.Spectators[i+1:]...)
			break
		}
	}
	// 座位：对局中仅标记离线保留座位可重连；等待阶段直接释放
	for _, s := range r.Seats {
		if s.Client == c {
			if r.Phase == "playing" {
				s.Client = nil
			} else {
				r.standLocked(s.Index)
			}
			break
		}
	}
	r.broadcastStateAsync()
	r.mu.Unlock()
	if leaveName != "" {
		r.systemChat(leaveName + " 离开了房间")
	}
}

// 由于 handleDisconnect 已持有锁，这里用无锁版本广播
func (r *Room) broadcastStateAsync() {
	for _, s := range r.Seats {
		if s.occupied() {
			s.Client.sendMsg(Message{Type: "roomState", Data: r.viewForLocked(s.Client)})
		}
	}
	for _, c := range r.Spectators {
		c.sendMsg(Message{Type: "roomState", Data: r.viewForLocked(c)})
	}
}

func (r *Room) viewForLocked(c *Client) RoomStateView {
	// 复用 viewFor（viewFor 不持锁，仅读字段）
	return r.viewFor(c)
}

func (r *Room) emitEvents(evs []Event) {
	for _, e := range evs {
		r.broadcast(e)
	}
}

// 执行一个动作并广播状态
func (r *Room) applyAction(c *Client, action string, data ActionData) {
	r.mu.Lock()
	seat := r.findSeat(c.playerID)

	// 全局动作（不需要座位）
	switch action {
	case "ready", "stand":
		if seat < 0 {
			r.mu.Unlock()
			c.emitError("请先入座")
			return
		}
	case "sit":
		r.mu.Unlock()
		r.handleSit(c, data)
		return
	case "chat":
		r.mu.Unlock()
		r.handleChat(c, data)
		return
	case "rename":
		r.mu.Unlock()
		r.handleRename(c, data)
		return
	case "setBlindMode":
		r.mu.Unlock()
		r.handleSetBlindMode(c, data)
		return
	}

	if r.Phase != "playing" {
		// 非对局中只允许准备/开局/离座
		switch action {
		case "ready":
			r.Seats[seat].Ready = !r.Seats[seat].Ready
			r.mu.Unlock()
			r.broadcastState()
			return
		case "stand":
			standName := r.Seats[seat].Name
			r.standLocked(seat)
			r.mu.Unlock()
			r.systemChat(standName + " 离座旁观")
			r.broadcastState()
			return
		case "start":
			r.mu.Unlock()
			r.handleStart(c, data)
			return
		default:
			r.mu.Unlock()
			c.emitError("当前不能进行此操作")
			return
		}
	}

	// 对局中：交给引擎处理
	if seat < 0 {
		r.mu.Unlock()
		c.emitError("你未入座，无法操作")
		return
	}
	evs := r.Engine.HandleAction(r, seat, action, data)
	r.mu.Unlock()
	r.emitEvents(evs)
	r.broadcastState()
}

func (r *Room) standLocked(seat int) {
	s := r.Seats[seat]
	s.Client = nil
	s.PlayerID = ""
	s.Name = ""
	s.Ready = false
	s.Hand = nil
	s.IsLandlord = false
	s.IsDealer = false
	s.IsFolded = false
	s.IsLooked = false
	s.CurrentBet = 0
	s.HasNiu = false
	s.NiuValue = 0
	s.NiuCards = nil
	s.SettledDelta = 0
	s.LookedIndices = nil
	s.IsRevealed = false
}

func (r *Room) handleSit(c *Client, data ActionData) {
	r.mu.Lock()
	if r.Phase == "playing" {
		r.mu.Unlock()
		c.emitError("对局进行中，无法入座")
		return
	}
	// 已在座则换座
	cur := r.findSeat(c.playerID)
	if cur >= 0 {
		r.standLocked(cur)
	}
	seat := -1
	if v, ok := data["seat"].(float64); ok {
		seat = int(v)
	}
	if seat < 0 || seat >= len(r.Seats) || r.Seats[seat].occupied() {
		seat = r.firstEmptySeat()
	}
	if seat < 0 {
		r.mu.Unlock()
		c.emitError("座位已满")
		return
	}
	s := r.Seats[seat]
	s.Client = c
	s.PlayerID = c.playerID
	s.Name = c.name
	// 移出旁观者
	for i, sp := range r.Spectators {
		if sp == c {
			r.Spectators = append(r.Spectators[:i], r.Spectators[i+1:]...)
			break
		}
	}
	seatName := c.name
	r.mu.Unlock()
	r.systemChat(seatName + " 入座")
	r.broadcastState()
}

// systemChat 广播一条系统消息到聊天框
func (r *Room) systemChat(text string) {
	r.broadcast(Event{Type: "chat", Data: ActionData{"player": "系统", "text": text, "system": true}, Target: -1})
}

func (r *Room) handleChat(c *Client, data ActionData) {
	text, _ := data["text"].(string)
	if text == "" {
		return
	}
	name := c.name
	if seat := r.findSeat(c.playerID); seat >= 0 {
		name = r.Seats[seat].Name
	}
	r.broadcast(Event{Type: "chat", Data: ActionData{"player": name, "text": text}, Target: -1})
}

// handleSetBlindMode 房主设置炸金花蒙牌模式（仅等待阶段）
func (r *Room) handleSetBlindMode(c *Client, data ActionData) {
	if c.playerID != r.HostID {
		c.emitError("仅房主可设置")
		return
	}
	if r.Phase != "waiting" {
		c.emitError("仅等待阶段可设置")
		return
	}
	if r.Game != "zjh" {
		c.emitError("仅炸金花支持蒙牌模式")
		return
	}
	if v, ok := data["blindMode"].(bool); ok {
		r.BlindMode = v
	}
	r.broadcastState()
}

// handleRename 修改指定座位的玩家昵称（可修改自己或他人）
func (r *Room) handleRename(c *Client, data ActionData) {
	seatNum := -1
	if v, ok := data["seat"].(float64); ok {
		seatNum = int(v)
	}
	newName, _ := data["name"].(string)
	newName = strings.TrimSpace(newName)
	if newName == "" || len(newName) > 16 {
		c.emitError("昵称无效（1-16字）")
		return
	}
	r.mu.Lock()
	if seatNum < 0 || seatNum >= len(r.Seats) || !r.Seats[seatNum].occupied() {
		r.mu.Unlock()
		c.emitError("目标座位无效")
		return
	}
	oldName := r.Seats[seatNum].Name
	r.Seats[seatNum].Name = newName
	// 同步更新 Client.name 以便后续聊天/断线消息使用
	if r.Seats[seatNum].Client != nil {
		r.Seats[seatNum].Client.name = newName
	}
	r.mu.Unlock()
	r.systemChat(oldName + " 改名为 " + newName)
	r.broadcastState()
}

func (r *Room) handleStart(c *Client, data ActionData) {
	r.mu.Lock()
	if c.playerID != r.HostID {
		r.mu.Unlock()
		c.emitError("仅房主可开局")
		return
	}
	if r.Phase == "playing" {
		r.mu.Unlock()
		c.emitError("对局已在进行")
		return
	}
	n := 0
	for _, s := range r.Seats {
		if s.occupied() && s.Ready {
			n++
		}
	}
	if n < r.Engine.MinPlayers() {
		r.mu.Unlock()
		c.emitError("入座且准备的人数不足")
		return
	}
	// 房主可在开局数据中指定蒙牌模式（仅炸金花生效）
	if v, ok := data["blindMode"].(bool); ok {
		r.BlindMode = v
	} else {
		r.BlindMode = false
	}
	// 重置座位对局状态
	for _, s := range r.Seats {
		if !s.occupied() {
			continue
		}
		s.Hand = nil
		s.IsLandlord = false
		s.IsDealer = false
		s.IsFolded = false
		s.IsLooked = false
		s.CurrentBet = 0
		s.HasNiu = false
		s.NiuValue = 0
		s.NiuCards = nil
		s.SettledDelta = 0
		s.LookedIndices = nil
		s.IsRevealed = false
	}
	r.Phase = "playing"
	evs := r.Engine.Start(r)
	r.mu.Unlock()
	r.emitEvents(evs)
	r.broadcastState()
}

// handleReconnect 处理玩家重连：若该玩家原座位 Client 为空且 playerID 匹配则恢复
func (r *Room) tryReclaim(c *Client) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.Seats {
		if s.PlayerID == c.playerID && s.Client == nil {
			s.Client = c
			return true
		}
	}
	return false
}
