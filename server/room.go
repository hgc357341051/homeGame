package main

import (
	"math/rand"
	"strings"
	"sync"
	"time"
)

const startChips = 1000

// offlineTimeout 掉线后座位保留时长，超时后释放供他人入座
const offlineTimeout = 3 * time.Minute

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
	NiuName       string // 牛型名称（五小牛/炸弹牛/五花牛/牛牛/牛N/没牛），前端直接展示
	NiuCards      []Card // 凑牛的 3 张
	SettledDelta  int
	LookedIndices []bool // 蒙牌模式下已查看的牌索引
	IsRevealed    bool   // 蒙牌模式下是否已开牌(向所有人展示)
	DisconnectedAt time.Time // 掉线时间（对局中保留座位，超时释放）
}

// isOffline 掉线但座位保留中（对局内）
func (s *Seat) isOffline() bool {
	return s.PlayerID != "" && s.Client == nil && !s.DisconnectedAt.IsZero()
}

// offlineLeftSec 掉线座位剩余秒数
func (s *Seat) offlineLeftSec() int {
	if !s.isOffline() {
		return 0
	}
	left := offlineTimeout - time.Since(s.DisconnectedAt)
	if left < 0 {
		return 0
	}
	return int(left.Seconds())
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
	// OnSeatVacated 当座位被腾空（踢人/超时）时调用，引擎推进回合或结算
	OnSeatVacated(r *Room, seat int) []Event
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
	// 懒清理：释放已超时的掉线座位
	r.cleanupOfflineSeatsLocked()
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
			NiuName:      s.NiuName,
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
			// 掉线保留座位信息
			if s.isOffline() {
				sv.Offline = true
				sv.OfflineLeft = s.offlineLeftSec()
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
	found := false
	// 旁观者移除
	for i, sp := range r.Spectators {
		if sp == c {
			r.Spectators = append(r.Spectators[:i], r.Spectators[i+1:]...)
			found = true
			break
		}
	}
	// 座位：对局中仅标记离线保留座位可重连（记录掉线时间）；其余阶段直接释放
	for _, s := range r.Seats {
		if s.Client == c {
			if r.Phase == "playing" {
				s.Client = nil
				s.DisconnectedAt = time.Now()
			} else {
				// waiting / settled 阶段直接释放座位，避免离线座位残留阻塞新玩家入座
				r.standLocked(s.Index)
			}
			found = true
			break
		}
	}
	// 快照 phase，避免 Unlock 后并发读取 r.Phase 产生 data race
	phase := r.Phase
	r.broadcastStateAsync()
	r.mu.Unlock()
	// 仅当该连接确实在房间内时才提示（避免僵尸连接替换后误报）
	if found && leaveName != "" {
		if phase == "playing" {
			r.systemChat(leaveName + " 掉线了（座位保留 3 分钟）")
		} else {
			r.systemChat(leaveName + " 离开了房间")
		}
	}
}

// cleanupOfflineSeatsLocked 释放已超时的掉线座位（懒清理，调用方需持有 r.mu）
func (r *Room) cleanupOfflineSeatsLocked() {
	hostVacated := false
	for _, s := range r.Seats {
		if s.isOffline() && time.Since(s.DisconnectedAt) > offlineTimeout {
			name := s.Name
			seatIdx := s.Index
			wasHost := s.PlayerID == r.HostID
			r.standLocked(seatIdx)
			r.systemChat(name + " 掉线超时，座位已释放")
			// 对局中腾空座位：通知引擎推进回合或结算
			if r.Phase == "playing" {
				r.emitEvents(r.Engine.OnSeatVacated(r, seatIdx))
			}
			if wasHost {
				hostVacated = true
			}
		}
	}
	// 仅当房主座位因超时被腾空且对局已结束时，才转移房主权
	// （不影响房主作为旁观者未入座的正常场景）
	if hostVacated && r.Phase != "playing" {
		r.transferHostLocked()
	}
}

// handleLeave 主动离开：永久移除（不等同于掉线，不保留座位）
func (r *Room) handleLeave(c *Client) {
	r.mu.Lock()
	leaveName := c.name
	// 旁观者移除
	for i, sp := range r.Spectators {
		if sp == c {
			r.Spectators = append(r.Spectators[:i], r.Spectators[i+1:]...)
			break
		}
	}
	// 座位：无论阶段直接释放（主动离开=永久移除）
	vacatedSeat := -1
	for _, s := range r.Seats {
		if s.Client == c {
			vacatedSeat = s.Index
			r.standLocked(s.Index)
			break
		}
	}
	c.room = nil
	var evs []Event
	if vacatedSeat >= 0 && r.Phase == "playing" {
		evs = r.Engine.OnSeatVacated(r, vacatedSeat)
	}
	r.mu.Unlock()
	if leaveName != "" {
		r.systemChat(leaveName + " 离开了房间")
	}
	r.emitEvents(evs)
	r.broadcastState()
}

// handleKick 房主踢出掉线保留中的座位
func (r *Room) handleKick(c *Client, data ActionData) {
	if c.playerID != r.HostID {
		c.emitError("仅房主可踢人")
		return
	}
	seatNum := -1
	if v, ok := data["seat"].(float64); ok {
		seatNum = int(v)
	}
	r.mu.Lock()
	if seatNum < 0 || seatNum >= len(r.Seats) {
		r.mu.Unlock()
		c.emitError("座位无效")
		return
	}
	s := r.Seats[seatNum]
	if !s.isOffline() {
		r.mu.Unlock()
		c.emitError("该座位未掉线")
		return
	}
	kickName := s.Name
	r.standLocked(seatNum)
	var evs []Event
	if r.Phase == "playing" {
		evs = r.Engine.OnSeatVacated(r, seatNum)
	}
	r.mu.Unlock()
	r.systemChat("房主踢出了掉线的 " + kickName)
	r.emitEvents(evs)
	r.broadcastState()
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
	// 防御僵尸连接越权：若该座位已被新连接接管，旧连接的动作一律拒绝
	if seat >= 0 && r.Seats[seat].Client != c {
		r.mu.Unlock()
		c.emitError("会话已失效，请刷新页面")
		return
	}

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
	case "leave":
		r.mu.Unlock()
		r.handleLeave(c)
		return
	case "kick":
		r.mu.Unlock()
		r.handleKick(c, data)
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
	wasHost := s.PlayerID == r.HostID
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
	s.NiuName = ""
	s.NiuCards = nil
	s.SettledDelta = 0
	s.LookedIndices = nil
	s.IsRevealed = false
	s.DisconnectedAt = time.Time{}
	s.Chips = startChips // 重置筹码，避免新入座者继承前任筹码
	// 房主离开：在 waiting/settled 阶段将房主转移给最早入座的在线玩家，避免房间无法继续操作
	if wasHost && (r.Phase == "waiting" || r.Phase == "settled") {
		r.transferHostLocked()
	}
}

// transferHostLocked 将房主权转移给最早入座的在线玩家（调用方需持有 r.mu）
func (r *Room) transferHostLocked() {
	for _, s := range r.Seats {
		if s.occupied() && s.PlayerID != "" {
			r.HostID = s.PlayerID
			return
		}
	}
	// 无人可转移时保留原 HostID（房间将等 reaper 回收）
}

func (r *Room) handleSit(c *Client, data ActionData) {
	r.mu.Lock()
	// 已在座则换座（仅等待阶段）
	cur := r.findSeat(c.playerID)
	if cur >= 0 && r.Phase != "playing" {
		r.standLocked(cur)
	}
	seat := -1
	if v, ok := data["seat"].(float64); ok {
		seat = int(v)
	}
	// 选定座位校验
	if seat >= 0 && seat < len(r.Seats) {
		target := r.Seats[seat]
		if target.occupied() {
			// 已有在线玩家占用
			seat = r.firstEmptySeat()
		} else if target.isOffline() {
			// 掉线保留中：超时则可入座（继承手牌）；未超时仅原玩家可夺回
			if time.Since(target.DisconnectedAt) > offlineTimeout {
				// 超时释放，可继承手牌入座
			} else if target.PlayerID == c.playerID {
				// 原玩家夺回（走 tryReclaim 更合适，这里允许）
			} else {
				r.mu.Unlock()
				c.emitError("该座位掉线保留中，请等待超时或房主踢人")
				return
			}
		}
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
	inheritHand := r.Phase == "playing" && len(s.Hand) > 0 && s.isOffline()
	s.Client = c
	s.PlayerID = c.playerID
	s.Name = c.name
	s.DisconnectedAt = time.Time{}
	// 移出旁观者
	for i, sp := range r.Spectators {
		if sp == c {
			r.Spectators = append(r.Spectators[:i], r.Spectators[i+1:]...)
			break
		}
	}
	seatName := c.name
	// 在锁内计算继承手牌的可视图（蒙牌模式下不能暴露未查看的牌）
	var inheritCards []Card
	var inheritBlind bool
	var inheritLooked []bool
	if inheritHand {
		inheritCards = r.Engine.PlayerHand(s)
		inheritBlind = r.BlindMode
		if r.BlindMode && s.LookedIndices != nil {
			inheritLooked = append([]bool{}, s.LookedIndices...)
		}
	}
	r.mu.Unlock()
	if inheritHand {
		r.systemChat(seatName + " 接管了掉线座位（继承手牌）")
	} else {
		r.systemChat(seatName + " 入座")
	}
	r.broadcastState()
	// 继承手牌时补发 deal 给新入座者（用引擎可见性，避免蒙牌模式下暴露未查看的牌）
	if inheritHand {
		dealData := ActionData{"cards": inheritCards}
		if inheritBlind {
			dealData["blindMode"] = true
			if inheritLooked != nil {
				dealData["lookedIndices"] = inheritLooked
			}
		}
		c.sendMsg(Message{Type: "deal", Data: dealData})
	}
}

// systemChat 广播一条系统消息到聊天框
func (r *Room) systemChat(text string) {
	r.broadcast(Event{Type: "chat", Data: ActionData{"player": "系统", "text": text, "system": true}, Target: -1})
}

func (r *Room) handleChat(c *Client, data ActionData) {
	text, _ := data["text"].(string)
	text = strings.TrimSpace(text)
	// 限制单条聊天长度，避免刷屏
	if text == "" || len([]rune(text)) > 200 {
		c.emitError("聊天内容无效（1-200字）")
		return
	}
	// 在锁内读取名字，避免与 standLocked/handleRename 竞争
	r.mu.Lock()
	name := c.name
	if seat := r.findSeat(c.playerID); seat >= 0 {
		name = r.Seats[seat].Name
	}
	r.mu.Unlock()
	r.broadcast(Event{Type: "chat", Data: ActionData{"player": name, "text": text}, Target: -1})
}

// handleSetBlindMode 房主设置炸金花蒙牌模式（仅等待阶段）
func (r *Room) handleSetBlindMode(c *Client, data ActionData) {
	r.mu.Lock()
	if c.playerID != r.HostID {
		r.mu.Unlock()
		c.emitError("仅房主可设置")
		return
	}
	if r.Phase != "waiting" {
		r.mu.Unlock()
		c.emitError("仅等待阶段可设置")
		return
	}
	if r.Game != "zjh" {
		r.mu.Unlock()
		c.emitError("仅炸金花支持蒙牌模式")
		return
	}
	if v, ok := data["blindMode"].(bool); ok {
		r.BlindMode = v
	}
	r.mu.Unlock()
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
	// 房主可在开局数据中指定蒙牌模式（仅炸金花生效）；
	// 未传 blindMode 字段时保留 r.BlindMode 现有值（避免覆盖 setBlindMode 已设置的开关）
	if v, ok := data["blindMode"].(bool); ok && r.Game == "zjh" {
		r.BlindMode = v
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
		s.NiuName = ""
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

// tryReclaim 处理玩家重连：若该玩家原座位 playerID 匹配则恢复（含夺回掉线座位与替换僵尸连接）
func (r *Room) tryReclaim(c *Client) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.Seats {
		if s.PlayerID != c.playerID {
			continue
		}
		// 已超时释放的座位不能夺回（视为空座，需走 sit 流程）
		if s.isOffline() && time.Since(s.DisconnectedAt) > offlineTimeout {
			return false
		}
		if s.Client == nil {
			// 掉线座位夺回
			s.Client = c
			s.DisconnectedAt = time.Time{}
			return true
		}
		if s.Client != c {
			// 僵尸连接替换：旧连接尚未感知断开，新连接直接接管
			// 旧连接的 readPump 终止时会触发 unregister→handleDisconnect，但找不到该 client 已不处理
			s.Client = c
			s.DisconnectedAt = time.Time{}
			return true
		}
	}
	return false
}
