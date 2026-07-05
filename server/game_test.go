package main

import "testing"

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
	_ = e.settle(r)
	total := r.Seats[0].Chips + r.Seats[1].Chips
	if total != 5+1000 {
		t.Errorf("筹码不守恒: total=%d", total)
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
