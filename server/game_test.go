package main

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"
)

func tc(suit, rank string, v int) Card { return Card{Suit: suit, Rank: rank, Value: v} }

func TestDDZAnalyze(t *testing.T) {
	cases := []struct {
		name  string
		cards []Card
		typ   string
	}{
		{"rocket", []Card{tc("", "小王", 16), tc("", "大王", 17)}, "rocket"},
		{"bomb", []Card{tc("♠", "3", 3), tc("♥", "3", 3), tc("♦", "3", 3), tc("♣", "3", 3)}, "bomb"},
		{"single", []Card{tc("♠", "3", 3)}, "single"},
		{"pair", []Card{tc("♠", "5", 5), tc("♥", "5", 5)}, "pair"},
		{"triple", []Card{tc("♠", "7", 7), tc("♥", "7", 7), tc("♦", "7", 7)}, "triple"},
		{"tripleSingle", []Card{tc("♠", "7", 7), tc("♥", "7", 7), tc("♦", "7", 7), tc("♣", "8", 8)}, "tripleSingle"},
		{"triplePair", []Card{tc("♠", "7", 7), tc("♥", "7", 7), tc("♦", "7", 7), tc("♣", "8", 8), tc("♠", "8", 8)}, "triplePair"},
		{"straight5", []Card{tc("♠", "3", 3), tc("♥", "4", 4), tc("♦", "5", 5), tc("♣", "6", 6), tc("♠", "7", 7)}, "straight"},
		{"pairStraight", []Card{tc("♠", "5", 5), tc("♥", "5", 5), tc("♦", "6", 6), tc("♣", "6", 6), tc("♠", "7", 7), tc("♥", "7", 7)}, "pairStraight"},
		{"plane", []Card{tc("♠", "5", 5), tc("♥", "5", 5), tc("♦", "5", 5), tc("♣", "6", 6), tc("♠", "6", 6), tc("♥", "6", 6)}, "plane"},
		{"planeSingle", []Card{tc("♠", "5", 5), tc("♥", "5", 5), tc("♦", "5", 5), tc("♣", "6", 6), tc("♠", "6", 6), tc("♥", "6", 6), tc("♦", "2", 15), tc("♣", "A", 14)}, "planeSingle"},
		{"fourTwo", []Card{tc("♠", "9", 9), tc("♥", "9", 9), tc("♦", "9", 9), tc("♣", "9", 9), tc("♠", "3", 3), tc("♥", "4", 4)}, "fourTwo"},
	}
	for _, cc := range cases {
		p, ok := analyzeDDZ(cc.cards)
		if !ok {
			t.Errorf("%s: expected valid, got invalid", cc.name)
			continue
		}
		if p.Type != cc.typ {
			t.Errorf("%s: expected %s, got %s", cc.name, cc.typ, p.Type)
		}
	}
	// 不应识别含2的顺子
	if _, ok := analyzeDDZ([]Card{tc("♠", "2", 15), tc("♥", "3", 3), tc("♦", "4", 4), tc("♣", "5", 5), tc("♠", "6", 6)}); ok {
		t.Error("含2的顺子不应合法")
	}
}

func TestDDZBeat(t *testing.T) {
	bomb, _ := analyzeDDZ([]Card{tc("♠", "3", 3), tc("♥", "3", 3), tc("♦", "3", 3), tc("♣", "3", 3)})
	single, _ := analyzeDDZ([]Card{tc("♠", "A", 14)})
	rocket, _ := analyzeDDZ([]Card{tc("", "小王", 16), tc("", "大王", 17)})
	if !ddzCanBeat(bomb, single) {
		t.Error("炸弹应能打单张")
	}
	if !ddzCanBeat(rocket, bomb) {
		t.Error("王炸应能打炸弹")
	}
	s1, _ := analyzeDDZ([]Card{tc("♠", "3", 3), tc("♥", "4", 4), tc("♦", "5", 5), tc("♣", "6", 6), tc("♠", "7", 7)})
	s2, _ := analyzeDDZ([]Card{tc("♠", "4", 4), tc("♥", "5", 5), tc("♦", "6", 6), tc("♣", "7", 7), tc("♠", "8", 8)})
	if !ddzCanBeat(s2, s1) {
		t.Error("更大的顺子应能打")
	}
	if ddzCanBeat(s1, s2) {
		t.Error("更小的顺子不应能打")
	}
}

func TestZJHEval(t *testing.T) {
	triple, _ := evalZJH([]Card{tc("♠", "A", 14), tc("♥", "A", 14), tc("♦", "A", 14)})
	if triple.Type != 5 {
		t.Errorf("豹子 Type=%d", triple.Type)
	}
	straight, _ := evalZJH([]Card{tc("♠", "A", 14), tc("♥", "2", 2), tc("♦", "3", 3)})
	if straight.Type != 2 {
		t.Errorf("A-2-3 应为顺子 Type=%d", straight.Type)
	}
	sf, _ := evalZJH([]Card{tc("♠", "5", 5), tc("♠", "6", 6), tc("♠", "7", 7)})
	if sf.Type != 4 {
		t.Errorf("顺金 Type=%d", sf.Type)
	}
	pair, _ := evalZJH([]Card{tc("♠", "K", 13), tc("♥", "K", 13), tc("♦", "2", 2)})
	if zjhCompare(straight, pair) <= 0 {
		t.Error("顺子应大于对子")
	}
}

func TestNNEval(t *testing.T) {
	res := evalNN([]Card{tc("♠", "3", 3), tc("♥", "3", 3), tc("♦", "4", 4), tc("♣", "5", 5), tc("♠", "5", 5)})
	if res.Value != 10 {
		t.Errorf("应为牛牛(10), got %d", res.Value)
	}
	if res.Multiplier != 4 {
		t.Errorf("牛牛倍数应为4, got %d", res.Multiplier)
	}
	flower := evalNN([]Card{tc("♠", "J", 11), tc("♥", "J", 11), tc("♦", "Q", 12), tc("♣", "K", 13), tc("♠", "Q", 12)})
	if flower.Level != 3 {
		t.Errorf("五花牛 Level=%d", flower.Level)
	}
	bomb := evalNN([]Card{tc("♠", "7", 7), tc("♥", "7", 7), tc("♦", "7", 7), tc("♣", "7", 7), tc("♠", "3", 3)})
	if bomb.Level != 4 {
		t.Errorf("炸弹 Level=%d", bomb.Level)
	}
	if nnCompare(bomb, flower) <= 0 {
		t.Error("炸弹应大于五花牛")
	}
}

func TestRemoveCards(t *testing.T) {
	hand := []Card{tc("♠", "3", 3), tc("♥", "5", 5), tc("♦", "5", 5), tc("♣", "7", 7)}
	out, ok := removeCards(hand, []Card{tc("♥", "5", 5), tc("♣", "7", 7)})
	if !ok {
		t.Fatal("remove failed")
	}
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
}

// 验证 H1 修复：飞机带翼允许"翼为同点对子拆分"
func TestDDZPlaneSinglePairWing(t *testing.T) {
	// 555+666+77（两张7作为两单翼）应识别为 planeSingle
	cards := []Card{
		tc("♠", "5", 5), tc("♥", "5", 5), tc("♦", "5", 5),
		tc("♠", "6", 6), tc("♥", "6", 6), tc("♦", "6", 6),
		tc("♠", "7", 7), tc("♣", "7", 7),
	}
	p, ok := analyzeDDZ(cards)
	if !ok {
		t.Fatal("555+666+77 应识别为 planeSingle")
	}
	if p.Type != "planeSingle" {
		t.Errorf("期望 planeSingle, got %s", p.Type)
	}
	// 9999+33（对子3拆成两单翼）应识别为 fourTwo
	cards2 := []Card{
		tc("♠", "9", 9), tc("♥", "9", 9), tc("♦", "9", 9), tc("♣", "9", 9),
		tc("♠", "3", 3), tc("♥", "3", 3),
	}
	p2, ok := analyzeDDZ(cards2)
	if !ok {
		t.Fatal("9999+33 应识别为 fourTwo")
	}
	if p2.Type != "fourTwo" {
		t.Errorf("期望 fourTwo, got %s", p2.Type)
	}
}

// 验证 M3 修复：DDZ 结算筹码守恒（输方筹码不足时按比例折扣）
func TestDDZSettleConservation(t *testing.T) {
	// 构造一个极简 Room：地主赢，但农民筹码不足以赔付
	r := &Room{Seats: []*Seat{
		{Index: 0, PlayerID: "P0", Name: "P0", Chips: 10}, // 农民，筹码不足
		{Index: 1, PlayerID: "P1", Name: "P1", Chips: 1000},
		{Index: 2, PlayerID: "P2", Name: "P2", Chips: 1000},
	}}
	e := &ddzEngine{
		occupied:     []int{0, 1, 2},
		landlordSeat: 1,
		baseScore:    2,
		multiplier:   1,
	}
	r.Seats[0].Chips = 10
	r.Seats[1].Chips = 1000
	r.Seats[2].Chips = 1000
	_ = e.settle(r, 1) // 地主赢
	// 守恒：所有人筹码之和不变
	total := r.Seats[0].Chips + r.Seats[1].Chips + r.Seats[2].Chips
	if total != 10+1000+1000 {
		t.Errorf("筹码不守恒: total=%d", total)
	}
	// 没有人筹码为负
	for _, s := range r.Seats {
		if s.Chips < 0 {
			t.Errorf("座位 %d 筹码为负: %d", s.Index, s.Chips)
		}
	}
}

// 验证 M3 修复：牛牛结算筹码守恒
// 结算前总财富 = sum(Chips) + pot（pot 是押注阶段已扣除的筹码）
// 结算后总财富 = sum(Chips)（pot 已分配给赢家，清零）
func TestNNSettleConservation(t *testing.T) {
	r := &Room{Seats: []*Seat{
		{Index: 0, PlayerID: "P0", Name: "P0", Chips: 5}, // 闲家筹码不足
		{Index: 1, PlayerID: "P1", Name: "P1", Chips: 1000},
	}}
	e := &nnEngine{
		occupied:    []int{0, 1},
		dealerIdx:   1, // P1 是庄家
		currentBet:  10,
		pot:         20,
		baseBet:     10,
		results:     map[int]nnResult{},
	}
	e.results[0] = nnResult{Level: 2, Value: 10, Multiplier: 4} // 闲家牛牛
	e.results[1] = nnResult{Level: 2, Value: 0, Multiplier: 1}  // 庄家没牛
	initialTotal := r.Seats[0].Chips + r.Seats[1].Chips + e.pot // 含底池的总财富
	_ = e.settle(r)
	total := r.Seats[0].Chips + r.Seats[1].Chips
	if total != initialTotal {
		t.Errorf("筹码不守恒: total=%d, expected=%d", total, initialTotal)
	}
	for _, s := range r.Seats {
		if s.Chips < 0 {
			t.Errorf("座位 %d 筹码为负: %d", s.Index, s.Chips)
		}
	}
}

// 验证 235 杀豹子特殊规则（ZJH）
func TestZJH235KillsTriple(t *testing.T) {
	aaa := []Card{tc("♠", "A", 14), tc("♥", "A", 14), tc("♦", "A", 14)}
	two35 := []Card{tc("♠", "2", 2), tc("♥", "3", 3), tc("♦", "5", 5)}
	aHand, _ := evalZJH(aaa)
	bHand, _ := evalZJH(two35)
	if zjhCompare(bHand, aHand) != 1 {
		t.Error("235 应杀豹子 AAA")
	}
	if zjhCompare(aHand, bHand) != -1 {
		t.Error("豹子 AAA 应输给 235")
	}
}

// 验证 235 不杀其他牌型（只杀豹子）
func TestZJH235OnlyKillsTriple(t *testing.T) {
	two35 := []Card{tc("♠", "2", 2), tc("♥", "3", 3), tc("♦", "5", 5)}
	pairKK := []Card{tc("♠", "K", 13), tc("♥", "K", 13), tc("♦", "2", 2)}
	a, _ := evalZJH(two35)
	b, _ := evalZJH(pairKK)
	// 235 是单张(0)，对子(1) 应赢
	if zjhCompare(a, b) >= 0 {
		t.Error("235 不应赢对子")
	}
}

// 验证 235 同花（金花）不杀豹子：仅散牌 235 才适用特殊规则
func TestZJH235FlushDoesNotKillTriple(t *testing.T) {
	aaa := []Card{tc("♠", "A", 14), tc("♥", "A", 14), tc("♦", "A", 14)}
	two35Flush := []Card{tc("♠", "2", 2), tc("♠", "3", 3), tc("♠", "5", 5)}
	aHand, _ := evalZJH(aaa)
	bHand, _ := evalZJH(two35Flush)
	if bHand.Type != 3 {
		t.Fatalf("2♠3♠5♠ 应为金花(Type=3), got Type=%d", bHand.Type)
	}
	// 235 金花不应杀豹子，豹子应赢
	if zjhCompare(bHand, aHand) >= 0 {
		t.Error("235 金花不应杀豹子")
	}
	if zjhCompare(aHand, bHand) <= 0 {
		t.Error("豹子应赢 235 金花")
	}
}

// 验证 ZJH Score 编码范围不重叠
func TestZJHScoreRange(t *testing.T) {
	// 单张最大 A-K-J（非连续，避免误判为顺子）: enc(14,13,11)=14*225+13*15+11=3150+195+11=3356
	highSingle := []Card{tc("♠", "A", 14), tc("♥", "K", 13), tc("♦", "J", 11)}
	// 对子最小 2-2-3: 10000+2*15+3=10033
	lowPair := []Card{tc("♠", "2", 2), tc("♥", "2", 2), tc("♦", "3", 3)}
	sHand, _ := evalZJH(highSingle)
	pHand, _ := evalZJH(lowPair)
	if sHand.Type != 0 {
		t.Errorf("A-K-J 应为单张(0), got Type=%d", sHand.Type)
	}
	if sHand.Score >= pHand.Score {
		t.Errorf("单张最大 Score=%d 应 < 对子最小 Score=%d", sHand.Score, pHand.Score)
	}
	if zjhCompare(sHand, pHand) >= 0 {
		t.Error("单张不应赢对子（即使最大单张 vs 最小对子）")
	}
}

// 验证 ensureHostLocked：房主离线后转移给最早在线在座玩家
func TestEnsureHostLockedTransfersWhenHostGone(t *testing.T) {
	r := &Room{
		HostID: "HOST",
		Phase:  "settled",
		Seats: []*Seat{
			{Index: 0, PlayerID: "HOST", Name: "H", Chips: 1000, Client: nil, DisconnectedAt: time.Time{}}, // 已腾空
			{Index: 1, PlayerID: "P1", Name: "P1", Chips: 1000, Client: &Client{playerID: "P1"}},
			{Index: 2, PlayerID: "P2", Name: "P2", Chips: 1000, Client: &Client{playerID: "P2"}},
		},
	}
	r.ensureHostLocked()
	if r.HostID != "P1" {
		t.Errorf("房主应转移给 P1, got HostID=%s", r.HostID)
	}
}

// 验证 ensureHostLocked：房主仍在线在座时不转移
func TestEnsureHostLockedKeepsOnlineHost(t *testing.T) {
	r := &Room{
		HostID: "HOST",
		Phase:  "settled",
		Seats: []*Seat{
			{Index: 0, PlayerID: "HOST", Name: "H", Chips: 1000, Client: &Client{playerID: "HOST"}},
			{Index: 1, PlayerID: "P1", Name: "P1", Chips: 1000, Client: &Client{playerID: "P1"}},
		},
	}
	r.ensureHostLocked()
	if r.HostID != "HOST" {
		t.Errorf("房主仍在线不应转移, got HostID=%s", r.HostID)
	}
}

// 验证 ensureHostLocked：房主在线旁观时不转移
func TestEnsureHostLockedKeepsSpectatorHost(t *testing.T) {
	r := &Room{
		HostID:     "HOST",
		Phase:      "settled",
		Spectators: []*Client{{playerID: "HOST"}},
		Seats: []*Seat{
			{Index: 0, PlayerID: "P1", Name: "P1", Chips: 1000, Client: &Client{playerID: "P1"}},
		},
	}
	r.ensureHostLocked()
	if r.HostID != "HOST" {
		t.Errorf("房主在线旁观不应转移, got HostID=%s", r.HostID)
	}
}

// 验证 ensureHostLocked：房主掉线（座位保留但 Client=nil）时转移
func TestEnsureHostLockedTransfersWhenHostOffline(t *testing.T) {
	r := &Room{
		HostID: "HOST",
		Phase:  "settled",
		Seats: []*Seat{
			{Index: 0, PlayerID: "HOST", Name: "H", Chips: 1000, Client: nil, DisconnectedAt: time.Now()},
			{Index: 1, PlayerID: "P1", Name: "P1", Chips: 1000, Client: &Client{playerID: "P1"}},
		},
	}
	r.ensureHostLocked()
	if r.HostID != "P1" {
		t.Errorf("房主掉线应转移给 P1, got HostID=%s", r.HostID)
	}
}

// 验证 handleLeave 对局中房主离场后房主转移（集成测试）
func TestHandleLeaveHostTransferDuringPlaying(t *testing.T) {
	hostClient := &Client{playerID: "HOST", name: "房主"}
	p1Client := &Client{playerID: "P1", name: "玩家1"}
	r := &Room{
		Code:   "TEST",
		Game:   "zjh",
		HostID: "HOST",
		Phase:  "playing",
		Engine: &zjhEngine{},
		Seats: []*Seat{
			{Index: 0, PlayerID: "HOST", Name: "房主", Chips: 1000, Client: hostClient},
			{Index: 1, PlayerID: "P1", Name: "玩家1", Chips: 1000, Client: p1Client},
		},
	}
	hostClient.room = r
	p1Client.room = r
	// 初始化引擎状态：2人在座
	e := r.Engine.(*zjhEngine)
	e.occupied = []int{0, 1}
	e.phase = "betting"
	e.activeCount = 2
	e.baseBet = 2
	e.currentBet = 2
	e.pot = 4
	e.currentSeat = 0
	r.Seats[0].CurrentBet = 2
	r.Seats[1].CurrentBet = 2
	// 房主离开
	r.handleLeave(hostClient)
	if r.Phase != "settled" {
		t.Errorf("房主离开后应结算, got Phase=%s", r.Phase)
	}
	if r.HostID != "P1" {
		t.Errorf("房主应转移给 P1, got HostID=%s", r.HostID)
	}
}

// recvMsg 从客户端 send channel 读取一条消息；无消息时返回 nil
func recvMsg(ch chan []byte) *Message {
	select {
	case b := <-ch:
		var msg Message
		if err := json.Unmarshal(b, &msg); err != nil {
			return nil
		}
		return &msg
	default:
		return nil
	}
}

func TestZJHResendTurn(t *testing.T) {
	// 重连玩家在 betting 阶段应收到 turn 事件
	client := &Client{playerID: "P1", name: "玩家1", send: make(chan []byte, 8)}
	r := &Room{
		Code:   "TEST",
		Game:   "zjh",
		HostID: "P1",
		Phase:  "playing",
		Engine: &zjhEngine{},
		Seats: []*Seat{
			{Index: 0, PlayerID: "P1", Name: "玩家1", Chips: 1000, Client: client},
			{Index: 1, PlayerID: "P2", Name: "玩家2", Chips: 1000, Client: &Client{playerID: "P2"}},
		},
	}
	client.room = r
	e := r.Engine.(*zjhEngine)
	e.occupied = []int{0, 1}
	e.phase = "betting"
	e.currentSeat = 0 // P1 的回合
	e.baseBet = 2
	e.currentBet = 2
	e.activeCount = 2
	r.Seats[0].CurrentBet = 2
	r.Seats[1].CurrentBet = 2

	r.Engine.ResendTurn(r, client)
	msg := recvMsg(client.send)
	if msg == nil {
		t.Fatal("ResendTurn 应发送 turn 消息，但未收到")
	}
	if msg.Type != "turn" {
		t.Errorf("应收到 turn 事件, got %s", msg.Type)
	}

	// 非 betting 阶段不应发送
	e.phase = "settled"
	r.Engine.ResendTurn(r, client)
	if msg := recvMsg(client.send); msg != nil {
		t.Errorf("settled 阶段不应发送消息, got %v", msg.Type)
	}
}

func TestNNResendTurnSetNiu(t *testing.T) {
	// 凑牛阶段：未确认凑牛的玩家重连应收到 turn 事件
	client := &Client{playerID: "P1", name: "玩家1", send: make(chan []byte, 8)}
	r := &Room{
		Code:   "TEST",
		Game:   "nn",
		HostID: "P1",
		Phase:  "playing",
		Engine: &nnEngine{},
		Seats: []*Seat{
			{Index: 0, PlayerID: "P1", Name: "玩家1", Chips: 1000, Client: client, Hand: []Card{tc("♠", "A", 1), tc("♥", "5", 5), tc("♦", "5", 5), tc("♣", "J", 10), tc("♠", "K", 10)}},
		},
	}
	client.room = r
	e := r.Engine.(*nnEngine)
	e.phase = "setNiu"
	e.results = map[int]nnResult{} // P1 未确认

	r.Engine.ResendTurn(r, client)
	msg := recvMsg(client.send)
	if msg == nil {
		t.Fatal("setNiu 阶段未确认的玩家应收到 turn 消息")
	}
	if msg.Type != "turn" {
		t.Errorf("应收到 turn 事件, got %s", msg.Type)
	}

	// 已确认凑牛的玩家不应再收到
	e.results[0] = nnResult{}
	r.Engine.ResendTurn(r, client)
	if msg := recvMsg(client.send); msg != nil {
		t.Errorf("已确认凑牛的玩家不应再收到 turn, got %v", msg.Type)
	}
}

func TestNNSetNiuVacateAfterConfirmNoPrematureSettle(t *testing.T) {
	// 3 人在座，A 已凑牛后离场：不应因 setCount 与 expected 不符而提前结算
	cA := &Client{playerID: "A", name: "A", send: make(chan []byte, 8)}
	cB := &Client{playerID: "B", name: "B"}
	cC := &Client{playerID: "C", name: "C"}
	r := &Room{
		Code: "TEST", Game: "nn", HostID: "A", Phase: "playing",
		Engine: &nnEngine{},
		Seats: []*Seat{
			{Index: 0, PlayerID: "A", Name: "A", Chips: 1000, Client: cA},
			{Index: 1, PlayerID: "B", Name: "B", Chips: 1000, Client: cB},
			{Index: 2, PlayerID: "C", Name: "C", Chips: 1000, Client: cC},
		},
	}
	cA.room = r
	e := r.Engine.(*nnEngine)
	e.occupied = []int{0, 1, 2}
	e.phase = "setNiu"
	e.results = map[int]nnResult{}
	// A 已凑牛
	e.results[0] = nnResult{Level: 1, Value: 5, Cards: r.Seats[0].Hand}
	// A 离场：OnSeatVacated 标记弃牌并删除结果
	evs := r.Engine.OnSeatVacated(r, 0)
	for _, ev := range evs {
		if ev.Type == "settle" {
			t.Fatal("A 离场后不应触发结算（B/C 尚未凑牛）")
		}
	}
	if e.phase != "setNiu" {
		t.Errorf("应仍在 setNiu 阶段, got %s", e.phase)
	}
	// B 凑牛后仍不应结算（C 还没凑牛）
	e.results[1] = nnResult{Level: 1, Value: 3, Cards: r.Seats[1].Hand}
	if e.allActiveSetNiu(r) {
		t.Fatal("B 凑牛后不应判定为全部完成（C 尚未凑牛）")
	}
	// C 也凑牛后才应结算
	e.results[2] = nnResult{Level: 1, Value: 7, Cards: r.Seats[2].Hand}
	if !e.allActiveSetNiu(r) {
		t.Fatal("B/C 都凑牛后应判定为全部完成")
	}
}

// 验证房主换座时保留房主身份（standLocked 会误转移给其他在座玩家）
func TestHostSwitchSeatKeepsHost(t *testing.T) {
	hostClient := &Client{playerID: "HOST", name: "房主", send: make(chan []byte, 8)}
	p1Client := &Client{playerID: "P1", name: "玩家1", send: make(chan []byte, 8)}
	r := &Room{
		Code:   "TEST",
		Game:   "zjh",
		HostID: "HOST",
		Phase:  "waiting",
		Engine: &zjhEngine{},
		Seats: []*Seat{
			{Index: 0, PlayerID: "HOST", Name: "房主", Chips: 1000, Client: hostClient},
			{Index: 1, PlayerID: "P1", Name: "玩家1", Chips: 1000, Client: p1Client},
			{Index: 2, PlayerID: "", Name: "", Chips: 1000, Client: nil},
		},
	}
	hostClient.room = r
	p1Client.room = r
	// 房主从座位 0 换到座位 2
	r.handleSit(hostClient, ActionData{"seat": float64(2)})
	if r.HostID != "HOST" {
		t.Errorf("房主换座后应保留房主身份, got HostID=%s", r.HostID)
	}
	if r.Seats[0].PlayerID != "" {
		t.Errorf("原座位应已腾空, got PlayerID=%s", r.Seats[0].PlayerID)
	}
	if r.Seats[2].PlayerID != "HOST" {
		t.Errorf("应已入座座位 2, got PlayerID=%s", r.Seats[2].PlayerID)
	}
}

// 验证离座旁观后加入旁观者列表且保留房主身份
func TestStandAddsToSpectatorsAndKeepsHost(t *testing.T) {
	hostClient := &Client{playerID: "HOST", name: "房主", send: make(chan []byte, 8)}
	p1Client := &Client{playerID: "P1", name: "玩家1", send: make(chan []byte, 8)}
	r := &Room{
		Code:   "TEST",
		Game:   "zjh",
		HostID: "HOST",
		Phase:  "waiting",
		Engine: &zjhEngine{},
		Seats: []*Seat{
			{Index: 0, PlayerID: "HOST", Name: "房主", Chips: 1000, Client: hostClient},
			{Index: 1, PlayerID: "P1", Name: "玩家1", Chips: 1000, Client: p1Client},
		},
	}
	hostClient.room = r
	p1Client.room = r
	// 房主离座旁观
	r.applyAction(hostClient, "stand", nil)
	if r.HostID != "HOST" {
		t.Errorf("房主离座旁观后应保留房主身份, got HostID=%s", r.HostID)
	}
	found := false
	for _, sp := range r.Spectators {
		if sp == hostClient {
			found = true
			break
		}
	}
	if !found {
		t.Error("房主离座旁观后应在旁观者列表中")
	}
	if r.Seats[0].PlayerID != "" {
		t.Errorf("原座位应已腾空, got PlayerID=%s", r.Seats[0].PlayerID)
	}
}

// 验证房主以旁观者身份断线时房主转移给在座玩家
func TestHostDisconnectAsSpectatorTransfersHost(t *testing.T) {
	hostClient := &Client{playerID: "HOST", name: "房主", send: make(chan []byte, 8)}
	p1Client := &Client{playerID: "P1", name: "玩家1", send: make(chan []byte, 8)}
	r := &Room{
		Code:   "TEST",
		Game:   "zjh",
		HostID: "HOST",
		Phase:  "waiting",
		Engine: &zjhEngine{},
		Seats: []*Seat{
			{Index: 0, PlayerID: "P1", Name: "玩家1", Chips: 1000, Client: p1Client},
		},
		Spectators: []*Client{hostClient},
	}
	hostClient.room = r
	p1Client.room = r
	// 房主（旁观者）断线
	r.handleDisconnect(hostClient)
	if r.HostID != "P1" {
		t.Errorf("房主旁观断线后应转移给 P1, got HostID=%s", r.HostID)
	}
	// 旁观者列表应已移除
	for _, sp := range r.Spectators {
		if sp == hostClient {
			t.Error("断线的旁观房主应已从旁观者列表移除")
			break
		}
	}
}

// 验证 rename 权限：仅允许改自己或房主改任意座位
func TestRenamePermission(t *testing.T) {
	hostClient := &Client{playerID: "HOST", name: "房主", send: make(chan []byte, 8)}
	p1Client := &Client{playerID: "P1", name: "玩家1", send: make(chan []byte, 8)}
	r := &Room{
		Code:   "TEST",
		Game:   "zjh",
		HostID: "HOST",
		Phase:  "waiting",
		Engine: &zjhEngine{},
		Seats: []*Seat{
			{Index: 0, PlayerID: "HOST", Name: "房主", Chips: 1000, Client: hostClient},
			{Index: 1, PlayerID: "P1", Name: "玩家1", Chips: 1000, Client: p1Client},
		},
	}
	hostClient.room = r
	p1Client.room = r

	// P1 改自己的昵称：应成功
	r.handleRename(p1Client, ActionData{"seat": float64(1), "name": "新名字"})
	if r.Seats[1].Name != "新名字" {
		t.Errorf("改自己昵称应成功, got %s", r.Seats[1].Name)
	}

	// P1 改房主的昵称：应被拒
	r.handleRename(p1Client, ActionData{"seat": float64(0), "name": "被改了"})
	if r.Seats[0].Name != "房主" {
		t.Errorf("非房主不应能改他人昵称, got %s", r.Seats[0].Name)
	}

	// 房主改 P1 的昵称：应成功（房主权限）
	r.handleRename(hostClient, ActionData{"seat": float64(1), "name": "房主改的"})
	if r.Seats[1].Name != "房主改的" {
		t.Errorf("房主应能改任意座位昵称, got %s", r.Seats[1].Name)
	}
}

// 验证重连后补发手牌（掉线期间手牌保留在 Seat.Hand，重连需重新发送 deal）
func TestReclaimResendsHand(t *testing.T) {
	oldClient := &Client{playerID: "P1", name: "玩家1", send: make(chan []byte, 8)}
	r := &Room{
		Code:   "TEST",
		Game:   "zjh",
		HostID: "P1",
		Phase:  "playing",
		Engine: &zjhEngine{},
		Seats: []*Seat{
			{Index: 0, PlayerID: "P1", Name: "玩家1", Chips: 1000, Client: oldClient, Hand: []Card{tc("♠", "A", 14), tc("♥", "K", 13), tc("♦", "Q", 12)}},
		},
	}
	oldClient.room = r
	// 模拟掉线
	r.Seats[0].Client = nil
	r.Seats[0].DisconnectedAt = time.Now()

	// 新连接重连
	newClient := &Client{playerID: "P1", name: "玩家1", send: make(chan []byte, 8)}
	newClient.room = r
	reclaimed := r.tryReclaim(newClient)
	if !reclaimed {
		t.Fatal("应成功夺回座位")
	}
	// 应收到 deal 事件包含手牌
	msg := recvMsg(newClient.send)
	if msg == nil {
		t.Fatal("重连后应收到 deal 消息")
	}
	if msg.Type != "deal" {
		t.Errorf("应收到 deal 事件, got %s", msg.Type)
	}
	data, _ := msg.Data.(map[string]interface{})
	if data == nil {
		t.Fatal("deal 消息 Data 应为 map")
	}
	cards, _ := data["cards"].([]interface{})
	if len(cards) != 3 {
		t.Errorf("应收到 3 张手牌, got %d", len(cards))
	}
}

// 验证蒙牌模式重连补发手牌时携带 blindMode 和 lookedIndices
func TestReclaimBlindModeHandFlags(t *testing.T) {
	oldClient := &Client{playerID: "P1", name: "玩家1", send: make(chan []byte, 8)}
	e := &zjhEngine{blindMode: true}
	r := &Room{
		Code:      "TEST",
		Game:      "zjh",
		HostID:    "P1",
		Phase:     "playing",
		Engine:    e,
		BlindMode: true,
		Seats: []*Seat{
			{Index: 0, PlayerID: "P1", Name: "玩家1", Chips: 1000, Client: oldClient,
				Hand:           []Card{tc("♠", "A", 14), tc("♥", "K", 13), tc("♦", "Q", 12)},
				LookedIndices:  []bool{true, false, false},
				IsRevealed:     false},
		},
	}
	oldClient.room = r
	// 模拟掉线
	r.Seats[0].Client = nil
	r.Seats[0].DisconnectedAt = time.Now()

	newClient := &Client{playerID: "P1", name: "玩家1", send: make(chan []byte, 8)}
	newClient.room = r
	r.tryReclaim(newClient)

	msg := recvMsg(newClient.send)
	if msg == nil || msg.Type != "deal" {
		t.Fatal("应收到 deal 消息")
	}
	data, _ := msg.Data.(map[string]interface{})
	if bm, ok := data["blindMode"]; !ok || bm != true {
		t.Error("蒙牌模式重连应携带 blindMode=true")
	}
	if _, ok := data["lookedIndices"]; !ok {
		t.Error("蒙牌模式重连应携带 lookedIndices")
	}
}

// 验证房主在对局中掉线（座位保留）时房主身份不立即转移，等待重连
func TestHostInGameDisconnectKeepsHostForReconnect(t *testing.T) {
	hostClient := &Client{playerID: "HOST", name: "房主", send: make(chan []byte, 8)}
	p1Client := &Client{playerID: "P1", name: "玩家1", send: make(chan []byte, 8)}
	r := &Room{
		Code:   "TEST",
		Game:   "zjh",
		HostID: "HOST",
		Phase:  "playing",
		Engine: &zjhEngine{},
		Seats: []*Seat{
			{Index: 0, PlayerID: "HOST", Name: "房主", Chips: 1000, Client: hostClient},
			{Index: 1, PlayerID: "P1", Name: "玩家1", Chips: 1000, Client: p1Client},
		},
	}
	hostClient.room = r
	p1Client.room = r
	// 房主在对局中掉线
	r.handleDisconnect(hostClient)
	if r.HostID != "HOST" {
		t.Errorf("对局中房主掉线应保留房主身份等待重连, got HostID=%s", r.HostID)
	}
	if r.Seats[0].PlayerID != "HOST" {
		t.Errorf("座位应保留给房主, got PlayerID=%s", r.Seats[0].PlayerID)
	}
	if r.Seats[0].Client != nil {
		t.Error("掉线后座位 Client 应为 nil")
	}
	if r.Seats[0].DisconnectedAt.IsZero() {
		t.Error("应记录掉线时间")
	}
}

// 验证开局时释放上一局遗留的掉线座位（避免重连拿到旧手牌）
func TestStartReleasesOfflineSeatsWithStaleHand(t *testing.T) {
	hostClient := &Client{playerID: "HOST", name: "房主", send: make(chan []byte, 8)}
	p1Client := &Client{playerID: "P1", name: "玩家1", send: make(chan []byte, 8)}
	// P2 已掉线但座位保留中（上一局的旧手牌仍在）
	staleHand := []Card{tc("♠", "A", 14), tc("♥", "K", 13), tc("♦", "Q", 12)}
	r := &Room{
		Code:   "TEST",
		Game:   "zjh",
		HostID: "HOST",
		Phase:  "settled",
		Engine: &zjhEngine{},
		rnd:    rand.New(rand.NewSource(1)),
		Seats: []*Seat{
			{Index: 0, PlayerID: "HOST", Name: "房主", Chips: 1000, Client: hostClient, Ready: true},
			{Index: 1, PlayerID: "P1", Name: "玩家1", Chips: 1000, Client: p1Client, Ready: true},
			{Index: 2, PlayerID: "P2", Name: "玩家2", Chips: 800, Client: nil, DisconnectedAt: time.Now(), Hand: staleHand, IsFolded: true},
		},
	}
	hostClient.room = r
	p1Client.room = r
	// 房主开局
	r.handleStart(hostClient, ActionData{})
	if r.Phase != "playing" {
		t.Fatalf("应进入 playing, got %s", r.Phase)
	}
	// P2 的掉线座位应已释放：PlayerID 清空、手牌清空
	if r.Seats[2].PlayerID != "" {
		t.Errorf("掉线座位应已释放, got PlayerID=%s", r.Seats[2].PlayerID)
	}
	if len(r.Seats[2].Hand) != 0 {
		t.Errorf("掉线座位的旧手牌应已清空, got %d 张", len(r.Seats[2].Hand))
	}
}

// 验证 handleKick 筹码守恒：对局中踢出掉线玩家时，必须保留 CurrentBet 供引擎结算。
// 修复前 standLocked 会重置 CurrentBet=0/Chips=startChips，导致被踢玩家 delta=0
// 且 Chips 被重置为初始值，但底注仍在 pot 中归赢家，筹码凭空增加。
// 此外，OnSeatVacated 可能把 phase 从 playing 改为 settled，若用改后的 phase 判断
// 会误触发 standLocked，重置已结算的 Chips/SettledDelta。
func TestHandleKickChipConservation(t *testing.T) {
	hostClient := &Client{playerID: "P0", name: "房主", send: make(chan []byte, 16)}
	r := &Room{
		Code:   "TEST",
		Game:   "zjh",
		HostID: "P0",
		Phase:  "playing",
		Seats: []*Seat{
			{Index: 0, PlayerID: "P0", Name: "房主", Chips: 990, Client: hostClient, CurrentBet: 10},
			{Index: 1, PlayerID: "P1", Name: "玩家1", Chips: 990, Client: nil, CurrentBet: 10, DisconnectedAt: time.Now()},
		},
	}
	hostClient.room = r
	e := &zjhEngine{
		occupied:    []int{0, 1},
		currentSeat: 0,
		baseBet:     2,
		currentBet:  2,
		cap:         32,
		pot:         20,
		phase:       "betting",
		activeCount: 2,
	}
	r.Engine = e

	// 初始总财富 = 玩家筹码 + 底池（底池是已从玩家扣除的注码）
	initialTotal := r.Seats[0].Chips + r.Seats[1].Chips + e.pot // 990+990+20=2000

	// 房主踢出掉线的 P1
	r.handleKick(hostClient, ActionData{"seat": float64(1)})

	// 对局应已结算（仅剩 P0 一人活跃）
	if r.Phase != "settled" {
		t.Fatalf("踢出后应结算, got Phase=%s", r.Phase)
	}
	// P1 的底注应被正确计入盈亏（delta = -CurrentBet = -10），而非 0
	if r.Seats[1].SettledDelta != -10 {
		t.Errorf("被踢玩家 delta 应为 -10, got %d（CurrentBet 可能被 standLocked 重置）", r.Seats[1].SettledDelta)
	}
	// P1 的 Chips 不应被 standLocked 重置为 startChips(1000)
	if r.Seats[1].Chips != 990 {
		t.Errorf("被踢玩家 Chips 应保持 990, got %d（可能被 standLocked 重置为初始值）", r.Seats[1].Chips)
	}
	// 筹码守恒：结算后总财富 = 玩家筹码之和（pot 已分配给赢家）
	finalTotal := r.Seats[0].Chips + r.Seats[1].Chips
	if finalTotal != initialTotal {
		t.Errorf("筹码不守恒: final=%d, expected=%d（凭空增减 %d）", finalTotal, initialTotal, finalTotal-initialTotal)
	}
	// P0 应赢得底池
	if r.Seats[0].SettledDelta != 10 {
		t.Errorf("赢家 delta 应为 +10, got %d", r.Seats[0].SettledDelta)
	}
	if r.Seats[0].Chips != 1010 {
		t.Errorf("赢家 Chips 应为 1010, got %d", r.Seats[0].Chips)
	}
}

// 验证 handleLeave 筹码守恒：对局中主动离开时，必须保留 CurrentBet 供引擎结算。
// 修复前 standLocked 在 OnSeatVacated 之前调用，重置 Chips/CurrentBet，
// 导致离场玩家 delta=0 且 Chips 被重置为初始值，但底注仍在 pot 中归赢家，
// 筹码凭空增加。与 handleKick 同类 bug。
func TestHandleLeaveChipConservation(t *testing.T) {
	p0Client := &Client{playerID: "P0", name: "玩家0", send: make(chan []byte, 16)}
	p1Client := &Client{playerID: "P1", name: "玩家1", send: make(chan []byte, 16)}
	r := &Room{
		Code:   "TEST",
		Game:   "zjh",
		HostID: "P0",
		Phase:  "playing",
		Seats: []*Seat{
			{Index: 0, PlayerID: "P0", Name: "玩家0", Chips: 990, Client: p0Client, CurrentBet: 10},
			{Index: 1, PlayerID: "P1", Name: "玩家1", Chips: 990, Client: p1Client, CurrentBet: 10},
		},
	}
	p0Client.room = r
	p1Client.room = r
	e := &zjhEngine{
		occupied:    []int{0, 1},
		currentSeat: 0,
		baseBet:     2,
		currentBet:  2,
		cap:         32,
		pot:         20,
		phase:       "betting",
		activeCount: 2,
	}
	r.Engine = e

	initialTotal := r.Seats[0].Chips + r.Seats[1].Chips + e.pot // 2000

	// P1 对局中主动离开
	r.handleLeave(p1Client)

	// 对局应已结算（仅剩 P0 一人活跃）
	if r.Phase != "settled" {
		t.Fatalf("离场后应结算, got Phase=%s", r.Phase)
	}
	// P1 的底注应被正确计入盈亏（delta = -10），而非 0
	if r.Seats[1].SettledDelta != -10 {
		t.Errorf("离场玩家 delta 应为 -10, got %d（CurrentBet 可能被 standLocked 重置）", r.Seats[1].SettledDelta)
	}
	// P1 的 Chips 不应被 standLocked 重置为 startChips(1000)
	if r.Seats[1].Chips != 990 {
		t.Errorf("离场玩家 Chips 应保持 990, got %d（可能被 standLocked 重置为初始值）", r.Seats[1].Chips)
	}
	// 筹码守恒
	finalTotal := r.Seats[0].Chips + r.Seats[1].Chips
	if finalTotal != initialTotal {
		t.Errorf("筹码不守恒: final=%d, expected=%d（凭空增减 %d）", finalTotal, initialTotal, finalTotal-initialTotal)
	}
	// P0 应赢得底池
	if r.Seats[0].Chips != 1010 {
		t.Errorf("赢家 Chips 应为 1010, got %d", r.Seats[0].Chips)
	}
	// P1 应标记为弃牌（避免被引擎误认为活跃）
	if !r.Seats[1].IsFolded {
		t.Error("离场玩家应标记为弃牌")
	}
}

// 验证 DDZ 结算截断误差修正：输方筹码不足时，int(scale*gain) 截断会导致筹码凭空消失
// 场景：地主输，筹码仅 3（不足以赔付 2*delta=4），两个农民各应得 delta=2
// scale=3/4=0.75，int(2*0.75)=1，两个农民各得 1，地主赔 3，总和 -1（1 筹码凭空消失）
func TestDDZSettleTruncationFix(t *testing.T) {
	r := &Room{Seats: []*Seat{
		{Index: 0, PlayerID: "P0", Name: "P0", Chips: 1000}, // 农民
		{Index: 1, PlayerID: "P1", Name: "P1", Chips: 3},     // 地主，筹码不足
		{Index: 2, PlayerID: "P2", Name: "P2", Chips: 1000}, // 农民
	}}
	e := &ddzEngine{
		occupied:     []int{0, 1, 2},
		landlordSeat: 1,
		baseScore:    2,
		multiplier:   1,
	}
	initialTotal := r.Seats[0].Chips + r.Seats[1].Chips + r.Seats[2].Chips // 2003
	_ = e.settle(r, 0) // 农民 P0 赢（地主输）
	finalTotal := r.Seats[0].Chips + r.Seats[1].Chips + r.Seats[2].Chips
	if finalTotal != initialTotal {
		t.Errorf("截断误差导致筹码不守恒: final=%d, expected=%d（消失 %d）", finalTotal, initialTotal, initialTotal-finalTotal)
	}
}

// 验证 NN 结算截断误差修正：庄家筹码不足时，int(scale*gain) 截断会导致筹码凭空消失
func TestNNSettleTruncationFix(t *testing.T) {
	r := &Room{Seats: []*Seat{
		{Index: 0, PlayerID: "P0", Name: "P0", Chips: 5},   // 庄家，筹码不足
		{Index: 1, PlayerID: "P1", Name: "P1", Chips: 1000}, // 闲家1
		{Index: 2, PlayerID: "P2", Name: "P2", Chips: 1000}, // 闲家2
	}}
	e := &nnEngine{
		occupied:    []int{0, 1, 2},
		dealerIdx:   0,
		currentBet:  2,
		baseBet:     2,
		pot:         0, // 无底池，仅测试庄闲结算
		results:     map[int]nnResult{},
	}
	// P1、P2 都牛牛(倍数4)，庄家没牛 → 庄家输给每个闲家 4*2=8，总输 16
	e.results[0] = nnResult{Level: 0, Value: 0, Multiplier: 1}
	e.results[1] = nnResult{Level: 2, Value: 10, Multiplier: 4}
	e.results[2] = nnResult{Level: 2, Value: 10, Multiplier: 4}
	initialTotal := r.Seats[0].Chips + r.Seats[1].Chips + r.Seats[2].Chips // 2005
	_ = e.settle(r)
	finalTotal := r.Seats[0].Chips + r.Seats[1].Chips + r.Seats[2].Chips
	if finalTotal != initialTotal {
		t.Errorf("截断误差导致筹码不守恒: final=%d, expected=%d（消失 %d）", finalTotal, initialTotal, initialTotal-finalTotal)
	}
}
