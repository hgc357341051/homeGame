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
	occupied   []int
	dealerIdx  int // occupied 索引，跨局保留
	baseBet    int
	phase      string // setNiu / settled
	setCount   int
	results    map[int]nnResult // seat -> 结果
	totalSeats int
}

func (e *nnEngine) Name() string    { return "nn" }
func (e *nnEngine) Label() string   { return "牛牛" }
func (e *nnEngine) MinPlayers() int { return 2 }
func (e *nnEngine) MaxPlayers() int { return 6 }

func (e *nnEngine) Start(r *Room) []Event {
	// 保留 dealerIdx 跨局轮转
	prevDealer := e.dealerIdx
	e.baseBet = 2
	e.phase = "setNiu"
	e.setCount = 0
	e.results = map[int]nnResult{}
	e.occupied = nil
	for _, s := range r.Seats {
		if s.occupied() {
			e.occupied = append(e.occupied, s.Index)
		}
	}
	e.totalSeats = len(e.occupied)
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
	}
	dealerSeat := e.occupied[e.dealerIdx]
	r.Seats[dealerSeat].IsDealer = true

	deck := shuffle(newStandardDeck(), r.rnd)
	evs := []Event{}
	for i, seatIdx := range e.occupied {
		hand := deck[i*5 : (i+1)*5]
		r.Seats[seatIdx].Hand = append([]Card{}, hand...)
		sortByValue(r.Seats[seatIdx].Hand)
		evs = append(evs, Event{Type: "deal", Data: ActionData{"cards": r.Seats[seatIdx].Hand}, Target: seatIdx})
	}
	evs = append(evs, Event{Type: "phase", Data: ActionData{
		"phase": "setNiu", "message": "选择 3 张凑牛（或自动）",
		"dealerSeat": dealerSeat,
	}, Target: -1})
	// 提示所有玩家出 niuniuSet
	for _, seatIdx := range e.occupied {
		evs = append(evs, Event{Type: "turn", Data: ActionData{
			"seat": seatIdx, "phase": "setNiu", "actions": []string{"niuniuSet"},
		}, Target: -1})
	}
	return evs
}

func (e *nnEngine) HandleAction(r *Room, seat int, action string, data ActionData) []Event {
	if e.phase != "setNiu" {
		return []Event{{Type: "error", Data: ActionData{"msg": "当前不能操作"}, Target: seat}}
	}
	if action != "niuniuSet" {
		return []Event{{Type: "error", Data: ActionData{"msg": "请先凑牛"}, Target: seat}}
	}
	if _, done := e.results[seat]; done {
		return []Event{{Type: "error", Data: ActionData{"msg": "已确认"}, Target: seat}}
	}
	s := r.Seats[seat]
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
	if e.setCount >= e.totalSeats {
		return append(evs, e.settle(r)...)
	}
	return evs
}

func (e *nnEngine) settle(r *Room) []Event {
	e.phase = "settled"
	dealerSeat := e.occupied[e.dealerIdx]
	dealerRes := e.results[dealerSeat]
	base := e.baseBet
	evs := []Event{}
	// 公开所有人手牌与牛型
	for _, seatIdx := range e.occupied {
		res := e.results[seatIdx]
		evs = append(evs, Event{Type: "reveal", Data: ActionData{
			"seat": seatIdx, "cards": r.Seats[seatIdx].Hand,
			"niuCards": res.NiuCards, "niuName": nnName(res), "multiplier": res.Multiplier,
		}, Target: -1})
	}
	results := []ActionData{}
	for _, seatIdx := range e.occupied {
		s := r.Seats[seatIdx]
		var d int
		if seatIdx == dealerSeat {
			// 庄家与所有闲家结算
			for _, other := range e.occupied {
				if other == dealerSeat {
					continue
				}
				cmp := nnCompare(dealerRes, e.results[other])
				if cmp > 0 {
					d += dealerRes.Multiplier * base
				} else if cmp < 0 {
					d -= e.results[other].Multiplier * base
				}
			}
		} else {
			cmp := nnCompare(e.results[seatIdx], dealerRes)
			if cmp > 0 {
				d = e.results[seatIdx].Multiplier * base
			} else if cmp < 0 {
				d = -dealerRes.Multiplier * base
			}
		}
		s.Chips += d
		s.SettledDelta = d
		results = append(results, ActionData{"seat": seatIdx, "name": s.Name, "delta": d, "chips": s.Chips,
			"niuName": nnName(e.results[seatIdx]), "isDealer": seatIdx == dealerSeat, "win": d > 0})
	}
	evs = append(evs, Event{Type: "settle", Data: ActionData{"results": results, "game": "nn", "dealerSeat": dealerSeat}, Target: -1})
	evs = append(evs, Event{Type: "phase", Data: ActionData{"phase": "settled", "message": "对局结束"}, Target: -1})
	r.Phase = "settled"
	for _, s := range r.Seats {
		s.Ready = false
	}
	return evs
}

func (e *nnEngine) PublicArea(r *Room) PublicAreaView {
	v := PublicAreaView{Phase: e.phase}
	if e.totalSeats > 0 && len(e.occupied) > 0 && e.dealerIdx >= 0 && e.dealerIdx < len(e.occupied) {
		v.DealerSeat = e.occupied[e.dealerIdx]
	}
	return v
}
