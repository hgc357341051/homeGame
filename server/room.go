package main

import (
	"math/rand"
	"sync"
	"time"
)

const startChips = 1000

// Seat 表示房间中的一个座位。Hand 仅存在于服务端内存。
type Seat struct {
	Index       int
	Client      *Client // nil 表示空座
	Name        string
	PlayerID    string
	Chips       int
	Ready       bool
	Hand        []Card // 仅服务端可见，永不下发
	IsLandlord  bool
	IsDealer    bool
	IsFolded    bool
	IsLooked    bool
	CurrentBet  int
	HasNiu      bool
	NiuValue    int
	NiuCards    []Card // 凑牛的 3 张
	SettledDelta int
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
			s.Client.sendMsg(Message{Type: "deal", Data: ActionData{"cards": s.Hand}})
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
	}
}

func (r *Room) handleDisconnect(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
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
			r.standLocked(seat)
			r.mu.Unlock()
			r.broadcastState()
			return
		case "start":
			r.mu.Unlock()
			r.handleStart(c)
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
	r.mu.Unlock()
	r.broadcastState()
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

func (r *Room) handleStart(c *Client) {
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
