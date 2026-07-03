package main

type ddzEngine struct {
	occupied        []int   // 参与的座位序号（按顺序）
	bottomCards     []Card
	bottomRevealed  bool
	landlordSeat    int
	currentSeat     int // 在 occupied 中的索引
	lastPlay        *PlayInfo
	lastPlayAnalysis *ddzPlay
	lastPlayerIdx   int // 上次实际出牌者在 occupied 中的索引
	passCount       int
	phase           string // callLandlord / playing / settled
	callIdx         int
	baseScore       int
	multiplier      int
	winnerTeam      int // 0=地主胜 1=农民胜
}

func (e *ddzEngine) Name() string  { return "ddz" }
func (e *ddzEngine) Label() string { return "斗地主" }
func (e *ddzEngine) MinPlayers() int { return 3 }
func (e *ddzEngine) MaxPlayers() int { return 3 }

// PlayerHand 斗地主无蒙牌概念，始终返回完整手牌
func (e *ddzEngine) PlayerHand(s *Seat) []Card {
	return s.Hand
}

// OnSeatVacated 斗地主为固定 3 人游戏，座位被腾空后无法继续；直接中止本局回到等待阶段
func (e *ddzEngine) OnSeatVacated(r *Room, seat int) []Event {
	if e.phase != "playing" && e.phase != "callLandlord" {
		return nil
	}
	e.phase = "settled"
	// 统一为 settled 状态，由房主重新准备开局；避免 phase 字段矛盾导致前端困惑
	r.Phase = "settled"
	for _, s := range r.Seats {
		s.Ready = false
		s.Hand = nil
	}
	// 生成结算事件（delta=0 平局），让前端能正常显示结算面板并重启
	results := []ActionData{}
	for _, seatIdx := range e.occupied {
		s := r.Seats[seatIdx]
		s.SettledDelta = 0
		results = append(results, ActionData{"seat": seatIdx, "name": s.Name, "delta": 0, "chips": s.Chips, "win": false})
	}
	return []Event{
		{Type: "settle", Data: ActionData{"results": results, "game": "ddz", "landlordWin": false, "multiplier": 1, "aborted": true}, Target: -1},
		{Type: "phase", Data: ActionData{"phase": "settled", "message": "玩家离场，本局中止"}, Target: -1},
	}
}

func (e *ddzEngine) reset() {
	*e = ddzEngine{baseScore: 2, multiplier: 1}
}

func (e *ddzEngine) Start(r *Room) []Event {
	e.reset()
	e.occupied = nil
	for _, s := range r.Seats {
		if s.occupied() {
			e.occupied = append(e.occupied, s.Index)
		}
	}
	evs := e.dealAndBid(r)
	return evs
}

func (e *ddzEngine) dealAndBid(r *Room) []Event {
	deck := shuffle(newDDZDeck(), r.rnd)
	// 3 人各 17 张，留 3 张底牌
	per := 17
	for i, seatIdx := range e.occupied {
		hand := deck[i*per : (i+1)*per]
		r.Seats[seatIdx].Hand = append([]Card{}, hand...)
		sortByValueDescDDZ(r.Seats[seatIdx].Hand)
	}
	e.bottomCards = append([]Card{}, deck[3*per:3*per+3]...)
	e.bottomRevealed = false
	e.phase = "callLandlord"
	e.callIdx = r.rnd.Intn(len(e.occupied))
	e.currentSeat = e.callIdx

	evs := []Event{}
	// 各玩家收到自己的手牌（deal 由 broadcastState 在 playing 阶段补发，但叫地主阶段也需手牌）
	for _, seatIdx := range e.occupied {
		evs = append(evs, Event{Type: "deal", Data: ActionData{"cards": r.Seats[seatIdx].Hand}, Target: seatIdx})
	}
	evs = append(evs, Event{Type: "phase", Data: ActionData{
		"phase": "callLandlord",
		"message": "叫地主阶段：选择是否当地主",
		"callerSeat": e.occupied[e.callIdx],
	}, Target: -1})
	evs = append(evs, Event{Type: "turn", Data: ActionData{
		"seat": e.occupied[e.callIdx],
		"phase": "callLandlord",
		"actions": []string{"callLandlord"},
	}, Target: -1})
	return evs
}

func (e *ddzEngine) seatIdxInOccupied(seat int) int {
	for i, s := range e.occupied {
		if s == seat {
			return i
		}
	}
	return -1
}

func (e *ddzEngine) HandleAction(r *Room, seat int, action string, data ActionData) []Event {
	if e.phase == "callLandlord" {
		return e.handleCall(r, seat, action, data)
	}
	if e.phase == "playing" {
		return e.handlePlay(r, seat, action, data)
	}
	return nil
}

func (e *ddzEngine) handleCall(r *Room, seat int, action string, data ActionData) []Event {
	if action != "callLandlord" {
		return []Event{{Type: "error", Data: ActionData{"msg": "当前为叫地主阶段"}, Target: seat}}
	}
	if e.seatIdxInOccupied(seat) != e.currentSeat {
		return []Event{{Type: "error", Data: ActionData{"msg": "还没轮到你叫地主"}, Target: seat}}
	}
	call, _ := data["call"].(bool)
	if call {
		// 成为地主
		e.landlordSeat = seat
		r.Seats[seat].IsLandlord = true
		r.Seats[seat].Hand = append(r.Seats[seat].Hand, e.bottomCards...)
		sortByValueDescDDZ(r.Seats[seat].Hand)
		e.bottomRevealed = true
		e.phase = "playing"
		e.currentSeat = e.seatIdxInOccupied(seat)
		e.lastPlayerIdx = e.currentSeat
		e.lastPlay = nil
		e.lastPlayAnalysis = nil
		evs := []Event{
			{Type: "phase", Data: ActionData{"phase": "playing", "message": "地主已确定，开始出牌"}, Target: -1},
			{Type: "reveal", Data: ActionData{"seat": seat, "cards": e.bottomCards, "note": "底牌"}, Target: -1},
			// 地主手牌更新（含底牌）单独下发
			{Type: "deal", Data: ActionData{"cards": r.Seats[seat].Hand}, Target: seat},
			{Type: "turn", Data: ActionData{"seat": seat, "phase": "playing", "actions": []string{"play"}}, Target: -1},
		}
		return evs
	}
	// pass，下一位
	e.callIdx = (e.callIdx + 1) % len(e.occupied)
	// 全部 pass → 重新发牌
	if e.callIdx == e.currentSeat {
		evs := []Event{{Type: "phase", Data: ActionData{"phase": "callLandlord", "message": "全部不叫，重新发牌"}, Target: -1}}
		evs = append(evs, e.dealAndBid(r)...)
		return evs
	}
	e.currentSeat = e.callIdx
	return []Event{{Type: "turn", Data: ActionData{
		"seat": e.occupied[e.callIdx], "phase": "callLandlord", "actions": []string{"callLandlord"},
	}, Target: -1}}
}

func (e *ddzEngine) handlePlay(r *Room, seat int, action string, data ActionData) []Event {
	idx := e.seatIdxInOccupied(seat)
	if idx != e.currentSeat {
		return []Event{{Type: "error", Data: ActionData{"msg": "还没轮到你出牌"}, Target: seat}}
	}
	if action == "pass" {
		if e.lastPlay == nil {
			return []Event{{Type: "error", Data: ActionData{"msg": "你是自由出牌，不能要不起"}, Target: seat}}
		}
		e.passCount++
		evs := []Event{{Type: "played", Data: ActionData{"seat": seat, "pass": true}, Target: -1}}
		e.advanceTurn(r)
		return evs
	}
	if action != "play" {
		return []Event{{Type: "error", Data: ActionData{"msg": "操作不支持"}, Target: seat}}
	}
	cardMaps, _ := data["cards"].([]interface{})
	if len(cardMaps) == 0 {
		return []Event{{Type: "error", Data: ActionData{"msg": "请选择要出的牌"}, Target: seat}}
	}
	playCards := extractCards(cardMaps)
	if playCards == nil {
		return []Event{{Type: "error", Data: ActionData{"msg": "牌数据格式错误"}, Target: seat}}
	}
	// 校验手牌中是否含这些牌
	newHand, ok := removeCards(r.Seats[seat].Hand, playCards)
	if !ok {
		return []Event{{Type: "error", Data: ActionData{"msg": "你没有这些牌"}, Target: seat}}
	}
	analysis, valid := analyzeDDZ(playCards)
	if !valid {
		return []Event{{Type: "error", Data: ActionData{"msg": "牌型不合法"}, Target: seat}}
	}
	if !ddzCanBeat(analysis, e.lastPlayAnalysis) {
		return []Event{{Type: "error", Data: ActionData{"msg": "管不上上家的牌"}, Target: seat}}
	}
	// 出牌
	r.Seats[seat].Hand = newHand
	e.lastPlay = &PlayInfo{Player: r.Seats[seat].PlayerID, Seat: seat, Cards: playCards}
	e.lastPlayAnalysis = analysis
	e.lastPlayerIdx = idx
	e.passCount = 0
	// 炸弹/火箭翻倍
	if analysis.Type == "bomb" || analysis.Type == "rocket" {
		e.multiplier *= 2
	}
	evs := []Event{{
		Type: "played",
		Data: ActionData{"seat": seat, "cards": playCards, "type": analysis.Type},
		Target: -1,
	}}
	// 判断是否出完
	if len(newHand) == 0 {
		return append(evs, e.settle(r, seat)...)
	}
	e.advanceTurn(r)
	return evs
}

func (e *ddzEngine) advanceTurn(r *Room) {
	// 若连续 2 次 pass，回到上次出牌者自由出牌
	if e.passCount >= 2 {
		e.lastPlay = nil
		e.lastPlayAnalysis = nil
		e.passCount = 0
		e.currentSeat = e.lastPlayerIdx
	} else {
		e.currentSeat = (e.currentSeat + 1) % len(e.occupied)
	}
}

func (e *ddzEngine) settle(r *Room, winnerSeat int) []Event {
	e.phase = "settled"
	// 地主赢 or 农民赢
	landlordWin := winnerSeat == e.landlordSeat
	delta := e.baseScore * e.multiplier
	evs := []Event{}
	results := []ActionData{}
	for _, seatIdx := range e.occupied {
		s := r.Seats[seatIdx]
		var d int
		if seatIdx == e.landlordSeat {
			if landlordWin {
				d = 2 * delta
			} else {
				d = -2 * delta
			}
		} else {
			if landlordWin {
				d = -delta
			} else {
				d = delta
			}
		}
		s.Chips += d
		s.SettledDelta = d
		results = append(results, ActionData{"seat": seatIdx, "name": s.Name, "delta": d, "chips": s.Chips,
			"isLandlord": seatIdx == e.landlordSeat, "win": d > 0})
	}
	e.winnerTeam = map[bool]int{true: 0, false: 1}[landlordWin]
	evs = append(evs, Event{Type: "phase", Data: ActionData{"phase": "settled", "message": "对局结束", "winnerSeat": winnerSeat, "landlordWin": landlordWin}, Target: -1})
	evs = append(evs, Event{Type: "settle", Data: ActionData{"results": results, "game": "ddz", "landlordWin": landlordWin, "multiplier": e.multiplier}, Target: -1})
	r.Phase = "settled"
	// 结束后重置准备状态以便再来一局
	for _, s := range r.Seats {
		s.Ready = false
		if s.Chips < 0 {
			s.Chips = 0 // 筹码下界封 0
		}
	}
	return evs
}

func (e *ddzEngine) PublicArea(r *Room) PublicAreaView {
	v := PublicAreaView{Phase: e.phase}
	if len(e.occupied) > 0 && e.currentSeat >= 0 && e.currentSeat < len(e.occupied) {
		v.CurrentSeat = e.occupied[e.currentSeat]
	}
	if e.bottomRevealed {
		v.BottomCards = e.bottomCards
	}
	if e.lastPlay != nil {
		v.LastPlay = e.lastPlay
	}
	return v
}

// extractCards 从前端传来的 []interface{} 解析出 Card 列表
func extractCards(raw []interface{}) []Card {
	out := make([]Card, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil
		}
		c := Card{}
		if s, ok := m["suit"].(string); ok {
			c.Suit = s
		}
		if r, ok := m["rank"].(string); ok {
			c.Rank = r
		}
		if v, ok := m["value"].(float64); ok {
			c.Value = int(v)
		}
		if c.Rank == "" {
			return nil
		}
		out = append(out, c)
	}
	return out
}
