package main

import "sort"

// ===== 炸金花牌型 =====
// 类型: 5豹子 4顺金 3金花 2顺子 1对子 0单张

type zjhHand struct {
	Type  int
	Score int
	Cards []Card
}

func zjhSortedVals(cards []Card) []int {
	vs := make([]int, 0, 3)
	for _, c := range cards {
		vs = append(vs, c.Value)
	}
	sort.Ints(vs)
	return vs
}

func evalZJH(cards []Card) (zjhHand, bool) {
	if len(cards) != 3 {
		return zjhHand{}, false
	}
	vs := zjhSortedVals(cards)
	suit := cards[0].Suit
	flush := cards[1].Suit == suit && cards[2].Suit == suit
	straight := false
	high := vs[2]
	if vs[1] == vs[0]+1 && vs[2] == vs[1]+1 {
		straight = true
	}
	if vs[0] == 2 && vs[1] == 3 && vs[2] == 14 { // A-2-3 最小顺子
		straight = true
		high = 3
	}
	triple := vs[0] == vs[1] && vs[1] == vs[2]
	pair := vs[0] == vs[1] || vs[1] == vs[2]
	enc := func(hi, mid, lo int) int { return hi*225 + mid*15 + lo }

	switch {
	case triple:
		return zjhHand{Type: 5, Score: 50000 + vs[0], Cards: cards}, true
	case straight && flush:
		return zjhHand{Type: 4, Score: 40000 + high, Cards: cards}, true
	case flush:
		return zjhHand{Type: 3, Score: 30000 + enc(vs[2], vs[1], vs[0]), Cards: cards}, true
	case straight:
		return zjhHand{Type: 2, Score: 20000 + high, Cards: cards}, true
	case pair:
		var pv, kicker int
		if vs[0] == vs[1] {
			pv, kicker = vs[0], vs[2]
		} else {
			pv, kicker = vs[1], vs[0]
		}
		return zjhHand{Type: 1, Score: 10000 + pv*15 + kicker, Cards: cards}, true
	default:
		return zjhHand{Type: 0, Score: enc(vs[2], vs[1], vs[0]), Cards: cards}, true
	}
}

func zjhTypeName(t int) string {
	return [...]string{"单张", "对子", "顺子", "金花", "顺金", "豹子"}[t]
}

// is235 判断是否为 2,3,5（不论花色）的散牌。部分地区玩法：235 可击败豹子。
func is235(cards []Card) bool {
	if len(cards) != 3 {
		return false
	}
	vs := zjhSortedVals(cards)
	return vs[0] == 2 && vs[1] == 3 && vs[2] == 5
}

func zjhCompare(a, b zjhHand) int {
	// 特殊规则：235 散牌杀豹子（仅当一方为 235 且为散牌、另一方为豹子时生效）
	a235, b235 := is235(a.Cards), is235(b.Cards)
	if a235 && a.Type == 0 && b.Type == 5 {
		return 1
	}
	if b235 && b.Type == 0 && a.Type == 5 {
		return -1
	}
	if a.Score > b.Score {
		return 1
	}
	if a.Score < b.Score {
		return -1
	}
	return 0
}

// ===== 炸金花引擎 =====

type zjhEngine struct {
	occupied    []int
	currentSeat int // occupied 索引
	baseBet     int
	currentBet  int // 闷牌单注
	cap         int
	pot         int
	phase       string // betting / settled
	activeCount int
	blindMode   bool // 蒙牌模式开关
}

func (e *zjhEngine) Name() string    { return "zjh" }
func (e *zjhEngine) Label() string   { return "炸金花" }
func (e *zjhEngine) MinPlayers() int { return 2 }
func (e *zjhEngine) MaxPlayers() int { return 6 }

func (e *zjhEngine) reset() {
	*e = zjhEngine{baseBet: 2, currentBet: 2, cap: 32}
	// blindMode 默认 false（不蒙牌）；由 Start 从 Room 读取覆盖
}

// PlayerHand 返回该玩家当前可看到的手牌
// 蒙牌模式下未开牌时返回完整长度数组，未查看的位置为零值占位
func (e *zjhEngine) PlayerHand(s *Seat) []Card {
	if !e.blindMode || s.IsRevealed {
		return s.Hand
	}
	out := make([]Card, len(s.Hand))
	for i, looked := range s.LookedIndices {
		if looked && i < len(s.Hand) {
			out[i] = s.Hand[i]
		}
	}
	return out
}

func (e *zjhEngine) idxOf(seat int) int {
	for i, s := range e.occupied {
		if s == seat {
			return i
		}
	}
	return -1
}

func (e *zjhEngine) activeSeats(r *Room) []int {
	out := []int{}
	for _, s := range e.occupied {
		seat := r.Seats[s]
		// 跳过已弃牌或已腾空（PlayerID 被清空）的座位
		if !seat.IsFolded && seat.PlayerID != "" {
			out = append(out, s)
		}
	}
	return out
}

// OnSeatVacated 座位被腾空（踢人/超时）：若轮到该座位则推进，仅剩一人则结算
func (e *zjhEngine) OnSeatVacated(r *Room, seat int) []Event {
	if e.phase != "betting" {
		return nil
	}
	actives := e.activeSeats(r)
	// 关键：更新 activeCount，否则后续 fold/compare 的 activeCount-- 与 <=1 判断全部失真
	e.activeCount = len(actives)
	if len(actives) <= 1 {
		// 仅剩一人或无人，直接结算
		return e.settleByFold(r)
	}
	// 若当前轮到的是被腾空座位，推进到下一活跃玩家
	if len(e.occupied) > 0 && e.occupied[e.currentSeat] == seat {
		e.nextActive(r)
	}
	return []Event{e.turnEvent(r)}
}

func (e *zjhEngine) callCost(seat *Seat) int {
	if seat.IsLooked {
		return e.currentBet * 2
	}
	return e.currentBet
}

func (e *zjhEngine) turnEvent(r *Room) Event {
	seatIdx := e.occupied[e.currentSeat]
	seat := r.Seats[seatIdx]
	actions := []string{"call", "raise", "fold"}
	if e.blindMode {
		// 蒙牌模式：提供 lookCard（仍有未看的牌时）和 reveal
		hasUnlooked := false
		for _, looked := range seat.LookedIndices {
			if !looked {
				hasUnlooked = true
				break
			}
		}
		if hasUnlooked {
			actions = append([]string{"lookCard"}, actions...)
		}
		actions = append(actions, "reveal")
		// 全部牌都已查看视为已看牌（用于 callCost 翻倍与 compare）
		if !hasUnlooked && len(seat.LookedIndices) > 0 {
			seat.IsLooked = true
		}
		if seat.IsLooked && len(e.activeSeats(r)) >= 2 {
			actions = append(actions, "compare")
		}
	} else {
		if !seat.IsLooked {
			actions = append([]string{"look"}, actions...)
		}
		if seat.IsLooked && len(e.activeSeats(r)) >= 2 {
			actions = append(actions, "compare")
		}
	}
	return Event{Type: "turn", Data: ActionData{
		"seat": seatIdx, "phase": "betting", "actions": actions,
		"currentBet": e.currentBet, "pot": e.pot,
		"callCost":  e.callCost(seat),
		"blindMode": e.blindMode,
	}, Target: -1}
}

func (e *zjhEngine) Start(r *Room) []Event {
	e.reset()
	e.blindMode = r.BlindMode
	e.occupied = nil
	for _, s := range r.Seats {
		if s.occupied() {
			e.occupied = append(e.occupied, s.Index)
		}
	}
	e.activeCount = len(e.occupied)
	deck := shuffle(newStandardDeck(), r.rnd)
	evs := []Event{}
	for i, seatIdx := range e.occupied {
		hand := deck[i*3 : (i+1)*3]
		r.Seats[seatIdx].Hand = append([]Card{}, hand...)
		sortByValue(r.Seats[seatIdx].Hand)
		r.Seats[seatIdx].Chips -= e.baseBet
		r.Seats[seatIdx].CurrentBet = e.baseBet
		e.pot += e.baseBet
		if e.blindMode {
			// 蒙牌模式：初始化 LookedIndices，发牌事件不发送实际牌值
			r.Seats[seatIdx].LookedIndices = []bool{false, false, false}
			r.Seats[seatIdx].IsRevealed = false
			evs = append(evs, Event{Type: "deal", Data: ActionData{"cardCount": 3, "blindMode": true}, Target: seatIdx})
		} else {
			evs = append(evs, Event{Type: "deal", Data: ActionData{"cards": r.Seats[seatIdx].Hand}, Target: seatIdx})
		}
	}
	e.currentSeat = r.rnd.Intn(len(e.occupied))
	e.phase = "betting"
	msg := "下注阶段：看牌/跟注/加注/弃牌/比牌"
	if e.blindMode {
		msg = "蒙牌模式：逐张看牌/开牌/跟注/加注/弃牌/比牌"
	}
	evs = append(evs, Event{Type: "phase", Data: ActionData{
		"phase": "betting", "message": msg,
		"currentBet": e.currentBet, "pot": e.pot,
		"blindMode": e.blindMode,
	}, Target: -1})
	evs = append(evs, e.turnEvent(r))
	return evs
}

func (e *zjhEngine) nextActive(r *Room) {
	n := len(e.occupied)
	for i := 0; i < n; i++ {
		e.currentSeat = (e.currentSeat + 1) % n
		seat := r.Seats[e.occupied[e.currentSeat]]
		// 跳过已弃牌、已腾空、掉线（Client==nil）的座位，避免轮到离线玩家卡死
		if !seat.IsFolded && seat.PlayerID != "" && seat.Client != nil {
			return
		}
	}
}

func (e *zjhEngine) HandleAction(r *Room, seat int, action string, data ActionData) []Event {
	if e.phase != "betting" {
		return []Event{{Type: "error", Data: ActionData{"msg": "当前不能操作"}, Target: seat}}
	}
	if e.occupied[e.currentSeat] != seat {
		return []Event{{Type: "error", Data: ActionData{"msg": "还没轮到你"}, Target: seat}}
	}
	s := r.Seats[seat]
	switch action {
	case "look":
		if e.blindMode {
			return []Event{{Type: "error", Data: ActionData{"msg": "蒙牌模式请使用 lookCard"}, Target: seat}}
		}
		if s.IsLooked {
			return []Event{{Type: "error", Data: ActionData{"msg": "已经看牌"}, Target: seat}}
		}
		s.IsLooked = true
		evs := []Event{
			{Type: "phase", Data: ActionData{"event": "look", "seat": seat, "name": s.Name}, Target: -1},
		}
		// 看牌不消耗轮次（与 lookCard/reveal 保持一致）：玩家仍需 call/raise/fold/compare
		return append(evs, e.turnEvent(r))
	case "lookCard":
		// 蒙牌模式：逐张看牌。不消耗轮次，玩家可连续看多张后再 call/raise/fold/reveal
		if !e.blindMode {
			return []Event{{Type: "error", Data: ActionData{"msg": "非蒙牌模式"}, Target: seat}}
		}
		index := -1
		if v, ok := data["index"].(float64); ok {
			index = int(v)
		}
		if index < 0 || index >= len(s.LookedIndices) || index >= len(s.Hand) {
			return []Event{{Type: "error", Data: ActionData{"msg": "无效的牌索引"}, Target: seat}}
		}
		if s.LookedIndices[index] {
			return []Event{{Type: "error", Data: ActionData{"msg": "已查看该牌"}, Target: seat}}
		}
		s.LookedIndices[index] = true
		// 全部牌都已查看视为已看牌
		allLooked := true
		for _, looked := range s.LookedIndices {
			if !looked {
				allLooked = false
				break
			}
		}
		if allLooked {
			s.IsLooked = true
		}
		// 定向发送该张牌给该玩家；广播通知（不带牌面）
		evs := []Event{
			{Type: "deal", Data: ActionData{"cards": []Card{s.Hand[index]}, "index": index, "blindMode": true}, Target: seat},
			{Type: "phase", Data: ActionData{"event": "lookCard", "seat": seat, "name": s.Name, "index": index}, Target: -1},
		}
		// 不消耗轮次：重新发 turn 让该玩家继续操作
		return append(evs, e.turnEvent(r))
	case "reveal":
		// 蒙牌模式：开牌，向所有人展示该玩家全部牌。不消耗轮次
		if !e.blindMode {
			return []Event{{Type: "error", Data: ActionData{"msg": "非蒙牌模式"}, Target: seat}}
		}
		if s.IsRevealed {
			return []Event{{Type: "error", Data: ActionData{"msg": "已开牌"}, Target: seat}}
		}
		s.IsRevealed = true
		s.IsLooked = true // 开牌后视为已看牌
		evs := []Event{{
			Type: "reveal",
			Data: ActionData{
				"event": "reveal", "seat": seat, "name": s.Name, "cards": s.Hand,
			},
			Target: -1,
		}}
		// 不消耗轮次：重新发 turn 让该玩家继续操作
		return append(evs, e.turnEvent(r))
	case "fold":
		s.IsFolded = true
		e.activeCount--
		evs := []Event{{Type: "phase", Data: ActionData{"event": "fold", "seat": seat, "name": s.Name}, Target: -1}}
		if e.activeCount <= 1 {
			return append(evs, e.settleByFold(r)...)
		}
		e.nextActive(r)
		return append(evs, e.turnEvent(r))
	case "call":
		cost := e.callCost(s)
		if s.Chips < cost {
			return []Event{{Type: "error", Data: ActionData{"msg": "筹码不足，请弃牌"}, Target: seat}}
		}
		s.Chips -= cost
		s.CurrentBet += cost
		e.pot += cost
		evs := []Event{{Type: "phase", Data: ActionData{
			"event": "call", "seat": seat, "name": s.Name,
			"amount": cost, "pot": e.pot, "looked": s.IsLooked,
		}, Target: -1}}
		e.nextActive(r)
		return append(evs, e.turnEvent(r))
	case "raise":
		newBet := e.currentBet * 2
		if newBet > e.cap {
			newBet = e.cap
		}
		// 已达上限不能再加注
		if newBet == e.currentBet {
			return []Event{{Type: "error", Data: ActionData{"msg": "已达加注上限"}, Target: seat}}
		}
		cost := e.callCost(s)
		if s.Chips < cost {
			return []Event{{Type: "error", Data: ActionData{"msg": "筹码不足，请弃牌"}, Target: seat}}
		}
		// 先校验筹码再提交 currentBet，避免失败后全局 currentBet 被错误翻倍
		e.currentBet = newBet
		s.Chips -= cost
		s.CurrentBet += cost
		e.pot += cost
		evs := []Event{{Type: "phase", Data: ActionData{
			"event": "raise", "seat": seat, "name": s.Name,
			"amount": cost, "pot": e.pot, "currentBet": e.currentBet, "looked": s.IsLooked,
		}, Target: -1}}
		e.nextActive(r)
		return append(evs, e.turnEvent(r))
	case "compare":
		if !s.IsLooked {
			return []Event{{Type: "error", Data: ActionData{"msg": "需先看牌才能比牌"}, Target: seat}}
		}
		actives := e.activeSeats(r)
		if len(actives) < 2 {
			return []Event{{Type: "error", Data: ActionData{"msg": "活跃人数不足"}, Target: seat}}
		}
		targetSeat := -1
		if t, ok := data["target"].(float64); ok {
			ts := int(t)
			for _, a := range actives {
				if a == ts && a != seat {
					targetSeat = a
					break
				}
			}
		}
		if targetSeat == -1 {
			// 自动选一个非己活跃
			for _, a := range actives {
				if a != seat {
					targetSeat = a
					break
				}
			}
		}
		if targetSeat == -1 {
			return []Event{{Type: "error", Data: ActionData{"msg": "无可比牌对象"}, Target: seat}}
		}
		cost := e.currentBet * 2 // 比牌费用
		if s.Chips < cost {
			return []Event{{Type: "error", Data: ActionData{"msg": "筹码不足"}, Target: seat}}
		}
		s.Chips -= cost
		s.CurrentBet += cost
		e.pot += cost
		myHand, _ := evalZJH(s.Hand)
		opp := r.Seats[targetSeat]
		oppHand, _ := evalZJH(opp.Hand)
		cmp := zjhCompare(myHand, oppHand)
		var loser, winner int
		if cmp >= 0 { // 我方不小于对方，我赢
			winner, loser = seat, targetSeat
		} else {
			winner, loser = targetSeat, seat
		}
		r.Seats[loser].IsFolded = true
		e.activeCount--
		evs := []Event{{
			Type: "reveal",
			Data: ActionData{
				"seat": seat, "cards": s.Hand, "type": zjhTypeName(myHand.Type),
				"seat2": targetSeat, "cards2": opp.Hand, "type2": zjhTypeName(oppHand.Type),
				"winner": winner, "loser": loser, "cost": cost, "pot": e.pot,
			},
			Target: -1,
		}}
		if e.activeCount <= 1 {
			return append(evs, e.settleByFold(r)...)
		}
		e.nextActive(r)
		return append(evs, e.turnEvent(r))
	}
	return []Event{{Type: "error", Data: ActionData{"msg": "未知操作"}, Target: seat}}
}

func (e *zjhEngine) settleByFold(r *Room) []Event {
	e.phase = "settled"
	actives := e.activeSeats(r)
	// 防御：活跃玩家为空时（全员弃牌/腾空），退还各自已下注码，避免底池筹码凭空消失
	if len(actives) == 0 {
		results := []ActionData{}
		for _, seatIdx := range e.occupied {
			s := r.Seats[seatIdx]
			// 退还已扣除的注码，本局净盈亏为 0
			s.Chips += s.CurrentBet
			s.SettledDelta = 0
			results = append(results, ActionData{"seat": seatIdx, "name": s.Name, "delta": 0, "chips": s.Chips, "win": false})
		}
		e.pot = 0
		r.Phase = "settled"
		for _, s := range r.Seats {
			s.Ready = false
			s.CurrentBet = 0
		}
		return []Event{
			{Type: "settle", Data: ActionData{"results": results, "game": "zjh", "winnerSeat": -1, "aborted": true}, Target: -1},
			{Type: "phase", Data: ActionData{"phase": "settled", "message": "全员离场，本局中止（注码退还）"}, Target: -1},
		}
	}
	winner := actives[0]
	w := r.Seats[winner]
	evs := []Event{{
		Type: "reveal",
		Data: ActionData{"seat": winner, "cards": w.Hand, "note": "获胜手牌"},
		Target: -1,
	}}
	results := []ActionData{}
	for _, seatIdx := range e.occupied {
		s := r.Seats[seatIdx]
		var d int
		if seatIdx == winner {
			d = e.pot - s.CurrentBet
		} else {
			d = -s.CurrentBet
		}
		s.Chips += map[bool]int{true: e.pot, false: 0}[seatIdx == winner]
		s.SettledDelta = d
		results = append(results, ActionData{"seat": seatIdx, "name": s.Name, "delta": d, "chips": s.Chips, "win": seatIdx == winner})
	}
	e.pot = 0
	evs = append(evs, Event{Type: "settle", Data: ActionData{"results": results, "game": "zjh", "winnerSeat": winner}, Target: -1})
	evs = append(evs, Event{Type: "phase", Data: ActionData{"phase": "settled", "message": w.Name + " 获胜"}, Target: -1})
	r.Phase = "settled"
	for _, s := range r.Seats {
		s.Ready = false
		s.CurrentBet = 0
	}
	return evs
}

func (e *zjhEngine) PublicArea(r *Room) PublicAreaView {
	v := PublicAreaView{
		Phase:       e.phase,
		Pot:         e.pot,
		BaseBet:     e.baseBet,
		CurrentBet:  e.currentBet,
		ActiveCount: e.activeCount,
	}
	if len(e.occupied) > 0 && e.currentSeat >= 0 && e.currentSeat < len(e.occupied) {
		v.CurrentSeat = e.occupied[e.currentSeat]
	}
	looked := 0
	for _, s := range e.occupied {
		if r.Seats[s].IsLooked {
			looked++
		}
	}
	v.LookedCount = looked
	return v
}

// ResendTurn 重连后补发当前轮次信息
func (e *zjhEngine) ResendTurn(r *Room, c *Client) {
	if e.phase != "betting" {
		return
	}
	ev := e.turnEvent(r)
	c.sendMsg(Message{Type: ev.Type, Data: ev.Data})
}
