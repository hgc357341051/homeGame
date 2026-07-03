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
