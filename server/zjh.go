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
		return zjhHand{Type: 5, Score: 5000 + vs[0], Cards: cards}, true
	case straight && flush:
		return zjhHand{Type: 4, Score: 4000 + high, Cards: cards}, true
	case flush:
		return zjhHand{Type: 3, Score: 3000 + enc(vs[2], vs[1], vs[0]), Cards: cards}, true
	case straight:
		return zjhHand{Type: 2, Score: 2000 + high, Cards: cards}, true
	case pair:
		var pv, kicker int
		if vs[0] == vs[1] {
			pv, kicker = vs[0], vs[2]
		} else {
			pv, kicker = vs[1], vs[0]
		}
		return zjhHand{Type: 1, Score: 1000 + pv*15 + kicker, Cards: cards}, true
	default:
		return zjhHand{Type: 0, Score: enc(vs[2], vs[1], vs[0]), Cards: cards}, true
	}
}

func zjhTypeName(t int) string {
	return [...]string{"单张", "对子", "顺子", "金花", "顺金", "豹子"}[t]
}

func zjhCompare(a, b zjhHand) int {
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
}

func (e *zjhEngine) Name() string    { return "zjh" }
func (e *zjhEngine) Label() string   { return "炸金花" }
func (e *zjhEngine) MinPlayers() int { return 2 }
func (e *zjhEngine) MaxPlayers() int { return 6 }

func (e *zjhEngine) reset() {
	*e = zjhEngine{baseBet: 2, currentBet: 2, cap: 32}
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
		if !r.Seats[s].IsFolded {
			out = append(out, s)
		}
	}
	return out
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
	if !seat.IsLooked {
		actions = append([]string{"look"}, actions...)
	}
	if seat.IsLooked && len(e.activeSeats(r)) >= 2 {
		actions = append(actions, "compare")
	}
	return Event{Type: "turn", Data: ActionData{
		"seat": seatIdx, "phase": "betting", "actions": actions,
		"currentBet": e.currentBet, "pot": e.pot,
		"callCost": e.callCost(seat),
	}, Target: -1}
}

func (e *zjhEngine) Start(r *Room) []Event {
	e.reset()
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
		evs = append(evs, Event{Type: "deal", Data: ActionData{"cards": r.Seats[seatIdx].Hand}, Target: seatIdx})
	}
	e.currentSeat = r.rnd.Intn(len(e.occupied))
	e.phase = "betting"
	evs = append(evs, Event{Type: "phase", Data: ActionData{
		"phase": "betting", "message": "下注阶段：看牌/跟注/加注/弃牌/比牌",
		"currentBet": e.currentBet, "pot": e.pot,
	}, Target: -1})
	evs = append(evs, e.turnEvent(r))
	return evs
}

func (e *zjhEngine) nextActive(r *Room) {
	n := len(e.occupied)
	for i := 0; i < n; i++ {
		e.currentSeat = (e.currentSeat + 1) % n
		if !r.Seats[e.occupied[e.currentSeat]].IsFolded {
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
		if s.IsLooked {
			return []Event{{Type: "error", Data: ActionData{"msg": "已经看牌"}, Target: seat}}
		}
		s.IsLooked = true
		evs := []Event{
			{Type: "phase", Data: ActionData{"event": "look", "seat": seat, "name": s.Name}, Target: -1},
		}
		// 看牌不消耗轮次？规则上仍轮到下一家。这里看牌后直接轮转
		e.nextActive(r)
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
		e.currentBet = newBet
		cost := e.callCost(s)
		if s.Chips < cost {
			return []Event{{Type: "error", Data: ActionData{"msg": "筹码不足，请弃牌"}, Target: seat}}
		}
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
