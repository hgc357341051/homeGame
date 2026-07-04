package main

// ===== 牛牛牌型 =====
// 点数: A=1, 2-10=面值, J/Q/K=10；比较点数: A=1,2..10,J=11,Q=12,K=13

func nnPoint(c Card) int {
	switch c.Rank {
	case "A":
		return 1
	case "J", "Q", "K":
		return 10
	default:
		return c.Value // 2..10 的 Value 即点数
	}
}

func nnRank(c Card) int {
	// 用于比较大小：A最小(1)，K最大(13)
	switch c.Rank {
	case "A":
		return 1
	case "J", "Q", "K":
		return c.Value // 11,12,13
	default:
		return c.Value
	}
}

type nnResult struct {
	Level      int // 5五小 4炸弹 3五花 2普通牛(0..10)
	Value      int // 普通:0没牛..10牛牛；炸弹:点数
	Multiplier int
	NiuCards   []Card
	MaxCard    Card
	Cards      []Card
}

func maxCardOf(cards []Card) Card {
	mx := cards[0]
	for _, c := range cards[1:] {
		if nnRank(c) > nnRank(mx) || (nnRank(c) == nnRank(mx) && suitOrder(c.Suit) > suitOrder(mx.Suit)) {
			mx = c
		}
	}
	return mx
}

func suitOrder(s string) int {
	switch s {
	case "♠":
		return 4
	case "♥":
		return 3
	case "♣":
		return 2
	case "♦":
		return 1
	}
	return 0
}

func evalNN(cards []Card) nnResult {
	if len(cards) != 5 {
		return nnResult{Cards: cards}
	}
	res := nnResult{Cards: cards, MaxCard: maxCardOf(cards), Multiplier: 1}
	// 五小牛：5 张点数均 <=4 且总和 <=10
	sumAll := 0
	small := true
	for _, c := range cards {
		p := nnPoint(c)
		sumAll += p
		if p > 4 {
			small = false
		}
	}
	if small && sumAll <= 10 {
		res.Level = 5
		res.Multiplier = 7
		return res
	}
	// 炸弹：4 张同点
	cnt := map[int]int{}
	for _, c := range cards {
		cnt[nnRank(c)]++
	}
	for v, c := range cnt {
		if c == 4 {
			res.Level = 4
			res.Value = v
			res.Multiplier = 6
			return res
		}
	}
	// 五花牛：5 张均为 J/Q/K
	allFace := true
	for _, c := range cards {
		if c.Rank != "J" && c.Rank != "Q" && c.Rank != "K" {
			allFace = false
			break
		}
	}
	if allFace {
		res.Level = 3
		res.Multiplier = 5
		return res
	}
	// 普通牛：找 3 张点数和为 10 的倍数
	bestNiu := -1
	var bestCombo []Card
	for i := 0; i < 5; i++ {
		for j := i + 1; j < 5; j++ {
			for k := j + 1; k < 5; k++ {
				s := nnPoint(cards[i]) + nnPoint(cards[j]) + nnPoint(cards[k])
				if s%10 == 0 {
					// 其余两张
					rest := []Card{}
					for x := 0; x < 5; x++ {
						if x != i && x != j && x != k {
							rest = append(rest, cards[x])
						}
					}
					niu := (nnPoint(rest[0]) + nnPoint(rest[1])) % 10
					if niu == 0 {
						niu = 10 // 牛牛
					}
					if niu > bestNiu {
						bestNiu = niu
						bestCombo = []Card{cards[i], cards[j], cards[k]}
					}
				}
			}
		}
	}
	res.Level = 2
	if bestNiu < 0 {
		res.Value = 0 // 没牛
		res.Multiplier = 1
	} else {
		res.Value = bestNiu
		res.NiuCards = bestCombo
		switch {
		case bestNiu == 10:
			res.Multiplier = 4
		case bestNiu == 9:
			res.Multiplier = 3
		case bestNiu >= 7:
			res.Multiplier = 2
		default:
			res.Multiplier = 1
		}
	}
	return res
}

func nnCompare(a, b nnResult) int {
	if a.Level != b.Level {
		if a.Level > b.Level {
			return 1
		}
		return -1
	}
	if a.Value != b.Value {
		if a.Value > b.Value {
			return 1
		}
		return -1
	}
	// 同级同值比最大牌
	if nnRank(a.MaxCard) != nnRank(b.MaxCard) {
		if nnRank(a.MaxCard) > nnRank(b.MaxCard) {
			return 1
		}
		return -1
	}
	return 0
}

func nnName(r nnResult) string {
	switch r.Level {
	case 5:
		return "五小牛"
	case 4:
		return "炸弹牛"
	case 3:
		return "五花牛"
	case 2:
		if r.Value == 0 {
			return "没牛"
		}
		if r.Value == 10 {
			return "牛牛"
		}
		return []string{"", "牛一", "牛二", "牛三", "牛四", "牛五", "牛六", "牛七", "牛八", "牛九"}[r.Value]
	}
	return "没牛"
}

// ===== 牛牛引擎 =====

type nnEngine struct {
	occupied       []int
	dealerIdx      int // occupied 索引，跨局保留
	baseBet        int
	currentBet     int // 当前注码（押注阶段会随 raise 增长）
	pot            int
	cap            int
	activeCount    int
	currentSeat    int // occupied 索引，押注轮到的玩家
	actedThisRound []bool // 本轮押注中各 occupied 索引是否已行动
	phase          string // betting / setNiu / settled
	setCount       int
	results        map[int]nnResult // seat -> 结果
	totalSeats     int
}

func (e *nnEngine) Name() string    { return "nn" }
func (e *nnEngine) Label() string   { return "牛牛" }
func (e *nnEngine) MinPlayers() int { return 2 }
func (e *nnEngine) MaxPlayers() int { return 6 }

// PlayerHand 牛牛无蒙牌概念，始终返回完整手牌
func (e *nnEngine) PlayerHand(s *Seat) []Card {
	return s.Hand
}

// OnSeatVacated 座位被腾空（踢人/超时）：押注阶段推进回合或结算，凑牛阶段检查是否可直接结算
func (e *nnEngine) OnSeatVacated(r *Room, seat int) []Event {
	if e.phase != "betting" && e.phase != "setNiu" {
		return nil
	}
	// 标记为弃牌，使 nextActive / bettingRoundComplete / setNiu 统计自然跳过
	if seat >= 0 && seat < len(r.Seats) {
		r.Seats[seat].IsFolded = true
	}
	// 删除该座位的凑牛结果，避免庄家离场后其旧结果仍参与结算（M2）
	delete(e.results, seat)
	if e.phase == "betting" {
		// 重新统计活跃玩家数（跳过已弃牌与已腾空座位）
		actives := 0
		for _, seatIdx := range e.occupied {
			s := r.Seats[seatIdx]
			if !s.IsFolded && s.PlayerID != "" {
				actives++
			}
		}
		e.activeCount = actives
		if actives <= 1 {
			return e.settleByFold(r)
		}
		// 若当前轮到的是被腾空座位，推进到下一活跃玩家
		if len(e.occupied) > 0 && e.occupied[e.currentSeat] == seat {
			e.nextActive(r)
		}
		return []Event{e.turnEvent(r)}
	}
	// setNiu 阶段：若所有非弃牌玩家都已凑牛，则直接结算
	expected := 0
	for _, seatIdx := range e.occupied {
		if !r.Seats[seatIdx].IsFolded {
			expected++
		}
	}
	if e.setCount >= expected {
		return e.settle(r)
	}
	return nil
}

func (e *nnEngine) Start(r *Room) []Event {
	// 保留 dealerIdx 跨局轮转
	prevDealer := e.dealerIdx
	e.baseBet = 2
	e.currentBet = e.baseBet
	e.cap = 32
	e.pot = 0
	e.phase = "betting"
	e.setCount = 0
	e.results = map[int]nnResult{}
	e.occupied = nil
	for _, s := range r.Seats {
		if s.occupied() {
			e.occupied = append(e.occupied, s.Index)
		}
	}
	e.totalSeats = len(e.occupied)
	e.activeCount = e.totalSeats
	// 庄家轮转
	if e.totalSeats > 0 {
		if prevDealer < 0 || prevDealer >= e.totalSeats {
			e.dealerIdx = r.rnd.Intn(e.totalSeats)
		} else {
			e.dealerIdx = (prevDealer + 1) % e.totalSeats
		}
	}
	for _, seatIdx := range e.occupied {
		r.Seats[seatIdx].IsDealer = false
		r.Seats[seatIdx].HasNiu = false
		r.Seats[seatIdx].NiuValue = 0
		r.Seats[seatIdx].NiuCards = nil
		r.Seats[seatIdx].IsFolded = false
	}
	dealerSeat := e.occupied[e.dealerIdx]
	r.Seats[dealerSeat].IsDealer = true

	deck := shuffle(newStandardDeck(), r.rnd)
	evs := []Event{}
	for i, seatIdx := range e.occupied {
		hand := deck[i*5 : (i+1)*5]
		r.Seats[seatIdx].Hand = append([]Card{}, hand...)
		sortByValue(r.Seats[seatIdx].Hand)
		// 扣底注进 pot
		r.Seats[seatIdx].Chips -= e.baseBet
		r.Seats[seatIdx].CurrentBet = e.baseBet
		e.pot += e.baseBet
		evs = append(evs, Event{Type: "deal", Data: ActionData{"cards": r.Seats[seatIdx].Hand}, Target: seatIdx})
	}
	// 押注从庄家下一家开始
	if e.totalSeats > 0 {
		e.currentSeat = (e.dealerIdx + 1) % e.totalSeats
	}
	e.actedThisRound = make([]bool, e.totalSeats)
	evs = append(evs, Event{Type: "phase", Data: ActionData{
		"phase": "betting", "message": "押注阶段：跟注/加注/弃牌",
		"dealerSeat": dealerSeat,
		"currentBet": e.currentBet, "pot": e.pot,
	}, Target: -1})
	evs = append(evs, e.turnEvent(r))
	return evs
}

// turnEvent 押注阶段发送当前玩家的可操作动作
func (e *nnEngine) turnEvent(r *Room) Event {
	seatIdx := e.occupied[e.currentSeat]
	actions := []string{"call", "raise", "fold"}
	return Event{Type: "turn", Data: ActionData{
		"seat": seatIdx, "phase": "betting", "actions": actions,
		"currentBet": e.currentBet, "pot": e.pot,
	}, Target: -1}
}

func (e *nnEngine) nextActive(r *Room) {
	n := len(e.occupied)
	for i := 0; i < n; i++ {
		e.currentSeat = (e.currentSeat + 1) % n
		s := r.Seats[e.occupied[e.currentSeat]]
		// 跳过已弃牌、已腾空、掉线的座位
		if !s.IsFolded && s.PlayerID != "" && s.Client != nil {
			return
		}
	}
}

// bettingRoundComplete 判断本轮押注是否所有活跃玩家都已行动
func (e *nnEngine) bettingRoundComplete(r *Room) bool {
	for i := 0; i < e.totalSeats; i++ {
		s := r.Seats[e.occupied[i]]
		if !s.IsFolded && !e.actedThisRound[i] {
			return false
		}
	}
	return true
}

// enterSetNiu 押注结束，进入凑牛阶段
func (e *nnEngine) enterSetNiu(r *Room) []Event {
	e.phase = "setNiu"
	e.setCount = 0
	dealerSeat := e.occupied[e.dealerIdx]
	evs := []Event{Event{Type: "phase", Data: ActionData{
		"phase": "setNiu", "message": "选择 3 张凑牛（或自动）",
		"dealerSeat": dealerSeat,
		"pot": e.pot, "currentBet": e.currentBet,
	}, Target: -1}}
	// 提示所有非弃牌玩家出 niuniuSet
	for _, seatIdx := range e.occupied {
		if r.Seats[seatIdx].IsFolded {
			continue
		}
		evs = append(evs, Event{Type: "turn", Data: ActionData{
			"seat": seatIdx, "phase": "setNiu", "actions": []string{"niuniuSet"},
		}, Target: -1})
	}
	return evs
}

// advanceBetting 押注行动后推进：若本轮完成则进入 setNiu，否则轮转下一家
func (e *nnEngine) advanceBetting(r *Room) []Event {
	if e.bettingRoundComplete(r) {
		return e.enterSetNiu(r)
	}
	e.nextActive(r)
	return []Event{e.turnEvent(r)}
}

// settleByFold 仅剩一人时直接结算：剩余玩家赢得底池
func (e *nnEngine) settleByFold(r *Room) []Event {
	e.phase = "settled"
	winner := -1
	for _, seatIdx := range e.occupied {
		if !r.Seats[seatIdx].IsFolded {
			winner = seatIdx
			break
		}
	}
	evs := []Event{}
	results := []ActionData{}
	for _, seatIdx := range e.occupied {
		s := r.Seats[seatIdx]
		var d int
		if seatIdx == winner {
			d = e.pot - s.CurrentBet
			s.Chips += e.pot
		} else {
			d = -s.CurrentBet
		}
		s.SettledDelta = d
		results = append(results, ActionData{
			"seat": seatIdx, "name": s.Name, "delta": d, "chips": s.Chips,
			"isFolded": s.IsFolded, "win": seatIdx == winner,
		})
	}
	e.pot = 0
	msg := "对局结束"
	if winner >= 0 {
		msg = "其他玩家弃牌，" + r.Seats[winner].Name + " 获胜"
	}
	evs = append(evs, Event{Type: "settle", Data: ActionData{
		"results": results, "game": "nn", "winnerSeat": winner,
	}, Target: -1})
	evs = append(evs, Event{Type: "phase", Data: ActionData{
		"phase": "settled", "message": msg,
	}, Target: -1})
	r.Phase = "settled"
	for _, s := range r.Seats {
		s.Ready = false
		s.CurrentBet = 0
		if s.Chips < 0 {
			s.Chips = 0 // 筹码下界封 0
		}
	}
	return evs
}

func (e *nnEngine) HandleAction(r *Room, seat int, action string, data ActionData) []Event {
	// 出牌超时：押注阶段筹码够则跟注，不够则弃牌；凑牛阶段自动选最佳
	if action == "timeout" {
		if e.phase == "betting" {
			s := r.Seats[seat]
			if s.Chips >= e.currentBet {
				action = "call"
			} else {
				action = "fold"
			}
		} else if e.phase == "setNiu" {
			// 凑牛超时：自动选最佳，传空 cards 走自动路径
			return e.handleSetNiu(r, seat, "niuniuSet", ActionData{})
		}
	}
	if e.phase == "betting" {
		return e.handleBetting(r, seat, action, data)
	}
	if e.phase == "setNiu" {
		return e.handleSetNiu(r, seat, action, data)
	}
	return []Event{{Type: "error", Data: ActionData{"msg": "当前不能操作"}, Target: seat}}
}

// handleBetting 处理押注阶段动作：call/raise/fold
func (e *nnEngine) handleBetting(r *Room, seat int, action string, data ActionData) []Event {
	if e.occupied[e.currentSeat] != seat {
		return []Event{{Type: "error", Data: ActionData{"msg": "还没轮到你"}, Target: seat}}
	}
	s := r.Seats[seat]
	switch action {
	case "call":
		cost := e.currentBet
		if s.Chips < cost {
			return []Event{{Type: "error", Data: ActionData{"msg": "筹码不足，请弃牌"}, Target: seat}}
		}
		s.Chips -= cost
		s.CurrentBet += cost
		e.pot += cost
		e.actedThisRound[e.currentSeat] = true
		evs := []Event{{Type: "phase", Data: ActionData{
			"event": "call", "seat": seat, "name": s.Name,
			"amount": cost, "pot": e.pot, "currentBet": e.currentBet,
		}, Target: -1}}
		return append(evs, e.advanceBetting(r)...)
	case "raise":
		// 加注：currentBet 翻倍（不超过 cap），加注者需付新注码
		newBet := e.currentBet * 2
		if newBet > e.cap {
			newBet = e.cap
		}
		if newBet == e.currentBet {
			return []Event{{Type: "error", Data: ActionData{"msg": "已达上限，无法加注"}, Target: seat}}
		}
		cost := newBet
		if s.Chips < cost {
			// 筹码不足：不提交 currentBet，避免回滚算法在 cap 截断时算错
			return []Event{{Type: "error", Data: ActionData{"msg": "筹码不足，请弃牌"}, Target: seat}}
		}
		e.currentBet = newBet
		s.Chips -= cost
		s.CurrentBet += cost
		e.pot += cost
		// 加注后重置已行动标记，其他活跃玩家需重新行动
		for i := range e.actedThisRound {
			e.actedThisRound[i] = false
		}
		e.actedThisRound[e.currentSeat] = true
		evs := []Event{{Type: "phase", Data: ActionData{
			"event": "raise", "seat": seat, "name": s.Name,
			"amount": cost, "pot": e.pot, "currentBet": e.currentBet,
		}, Target: -1}}
		return append(evs, e.advanceBetting(r)...)
	case "fold":
		s.IsFolded = true
		e.activeCount--
		e.actedThisRound[e.currentSeat] = true
		evs := []Event{{Type: "phase", Data: ActionData{
			"event": "fold", "seat": seat, "name": s.Name,
		}, Target: -1}}
		if e.activeCount <= 1 {
			return append(evs, e.settleByFold(r)...)
		}
		return append(evs, e.advanceBetting(r)...)
	}
	return []Event{{Type: "error", Data: ActionData{"msg": "未知操作"}, Target: seat}}
}

// handleSetNiu 处理凑牛阶段：仅非弃牌玩家可参与
func (e *nnEngine) handleSetNiu(r *Room, seat int, action string, data ActionData) []Event {
	if action != "niuniuSet" {
		return []Event{{Type: "error", Data: ActionData{"msg": "请先凑牛"}, Target: seat}}
	}
	s := r.Seats[seat]
	if s.IsFolded {
		return []Event{{Type: "error", Data: ActionData{"msg": "已弃牌"}, Target: seat}}
	}
	if _, done := e.results[seat]; done {
		return []Event{{Type: "error", Data: ActionData{"msg": "已确认"}, Target: seat}}
	}
	var chosen []Card
	if arr, ok := data["cards"].([]interface{}); ok && len(arr) == 3 {
		chosen = extractCards(arr)
	}
	// 校验 chosen 属于手牌
	valid := false
	if len(chosen) == 3 {
		_, ok2 := removeCards(s.Hand, chosen)
		valid = ok2
	}
	if !valid {
		chosen = nil // 自动选最佳
	}
	// 计算：若玩家选了 3 张，校验其和是否为 10 倍数；否则自动最佳
	allCards := s.Hand
	res := evalNN(allCards)
	if valid {
		// 用玩家选择的 3 张作为牛牌，重新算 value
		rest := []Card{}
		_, _ = removeCards(s.Hand, chosen) // 已校验
		for _, c := range s.Hand {
			inside := false
			for _, cc := range chosen {
				if cardEquals(c, cc) {
					inside = true
					break
				}
			}
			if !inside {
				rest = append(rest, c)
			}
		}
		sum3 := nnPoint(chosen[0]) + nnPoint(chosen[1]) + nnPoint(chosen[2])
		if sum3%10 == 0 {
			niu := (nnPoint(rest[0]) + nnPoint(rest[1])) % 10
			if niu == 0 {
				niu = 10
			}
			res = nnResult{Level: 2, Value: niu, NiuCards: chosen, MaxCard: maxCardOf(s.Hand), Cards: s.Hand}
			switch {
			case niu == 10:
				res.Multiplier = 4
			case niu == 9:
				res.Multiplier = 3
			case niu >= 7:
				res.Multiplier = 2
			default:
				res.Multiplier = 1
			}
		} else {
			res = nnResult{Level: 2, Value: 0, NiuCards: nil, MaxCard: maxCardOf(s.Hand), Cards: s.Hand, Multiplier: 1}
		}
	}
	e.results[seat] = res
	s.HasNiu = res.Value > 0 || res.Level > 2
	s.NiuValue = res.Value
	if res.NiuCards != nil {
		s.NiuCards = res.NiuCards
	}
	e.setCount++
	evs := []Event{{Type: "phase", Data: ActionData{"event": "niuniuSet", "seat": seat, "name": s.Name, "name2": nnName(res)}, Target: -1}}
	// 仅统计非弃牌玩家是否都已凑牛
	expected := 0
	for _, seatIdx := range e.occupied {
		if !r.Seats[seatIdx].IsFolded {
			expected++
		}
	}
	if e.setCount >= expected {
		return append(evs, e.settle(r)...)
	}
	return evs
}

func (e *nnEngine) settle(r *Room) []Event {
	e.phase = "settled"
	dealerSeat := e.occupied[e.dealerIdx]
	dealerRes, dealerInResults := e.results[dealerSeat]
	base := e.currentBet // 庄闲结算用当前注码（考虑底池）
	// 找出非弃牌玩家中的最佳手牌（底池归属）
	bestSeat := -1
	var bestRes nnResult
	for _, seatIdx := range e.occupied {
		s := r.Seats[seatIdx]
		if s.IsFolded {
			continue
		}
		res := e.results[seatIdx]
		if bestSeat == -1 || nnCompare(res, bestRes) > 0 {
			bestSeat = seatIdx
			bestRes = res
		}
	}
	evs := []Event{}
	// 公开所有非弃牌玩家手牌与牛型
	for _, seatIdx := range e.occupied {
		s := r.Seats[seatIdx]
		if s.IsFolded {
			continue
		}
		res := e.results[seatIdx]
		evs = append(evs, Event{Type: "reveal", Data: ActionData{
			"seat": seatIdx, "cards": s.Hand,
			"niuCards": res.NiuCards, "niuName": nnName(res), "multiplier": res.Multiplier,
		}, Target: -1})
	}
	// deltaMap: 庄闲结算 + 底池分配（押注阶段已扣注码，故底池直接补给赢家）
	deltaMap := map[int]int{}
	if dealerInResults {
		for _, seatIdx := range e.occupied {
			s := r.Seats[seatIdx]
			if s.IsFolded || seatIdx == dealerSeat {
				continue
			}
			cmp := nnCompare(e.results[seatIdx], dealerRes)
			if cmp > 0 {
				deltaMap[seatIdx] += e.results[seatIdx].Multiplier * base
				deltaMap[dealerSeat] -= e.results[seatIdx].Multiplier * base
			} else if cmp < 0 {
				deltaMap[seatIdx] -= dealerRes.Multiplier * base
				deltaMap[dealerSeat] += dealerRes.Multiplier * base
			}
		}
	}
	// 底池分配给最佳手牌（同分平分）
	potWinners := []int{}
	for _, seatIdx := range e.occupied {
		s := r.Seats[seatIdx]
		if s.IsFolded {
			continue
		}
		if nnCompare(e.results[seatIdx], bestRes) == 0 {
			potWinners = append(potWinners, seatIdx)
		}
	}
	share := 0
	if len(potWinners) > 0 {
		share = e.pot / len(potWinners)
	}
	for _, w := range potWinners {
		deltaMap[w] += share
	}
	results := []ActionData{}
	for _, seatIdx := range e.occupied {
		s := r.Seats[seatIdx]
		gain := deltaMap[seatIdx] // 庄闲结算 + 底池收益
		s.Chips += gain
		// SettledDelta 为本局总盈亏 = 收益 - 自己已下的注（注码已在押注阶段扣除）
		s.SettledDelta = gain - s.CurrentBet
		niuName := ""
		if !s.IsFolded {
			niuName = nnName(e.results[seatIdx])
		}
		results = append(results, ActionData{
			"seat": seatIdx, "name": s.Name, "delta": s.SettledDelta, "chips": s.Chips,
			"niuName": niuName, "isDealer": seatIdx == dealerSeat,
			"isFolded": s.IsFolded, "win": s.SettledDelta > 0,
		})
	}
	e.pot = 0
	evs = append(evs, Event{Type: "settle", Data: ActionData{
		"results": results, "game": "nn", "dealerSeat": dealerSeat, "potWinner": bestSeat,
	}, Target: -1})
	evs = append(evs, Event{Type: "phase", Data: ActionData{"phase": "settled", "message": "对局结束"}, Target: -1})
	r.Phase = "settled"
	for _, s := range r.Seats {
		s.Ready = false
		s.CurrentBet = 0
		if s.Chips < 0 {
			s.Chips = 0 // 筹码下界封 0，避免负数
		}
	}
	return evs
}

func (e *nnEngine) PublicArea(r *Room) PublicAreaView {
	v := PublicAreaView{
		Phase:       e.phase,
		Pot:         e.pot,
		BaseBet:     e.baseBet,
		CurrentBet:  e.currentBet,
		ActiveCount: e.activeCount,
	}
	if e.totalSeats > 0 && len(e.occupied) > 0 && e.dealerIdx >= 0 && e.dealerIdx < len(e.occupied) {
		v.DealerSeat = e.occupied[e.dealerIdx]
	}
	if e.phase == "betting" && len(e.occupied) > 0 && e.currentSeat >= 0 && e.currentSeat < len(e.occupied) {
		v.CurrentSeat = e.occupied[e.currentSeat]
	}
	return v
}
