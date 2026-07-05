package main

import "math/rand"

// Card 表示一张扑克牌。Value 用于比较大小（各游戏自定义排序）。
type Card struct {
	Suit  string `json:"suit"`  // ♠ ♥ ♦ ♣，大小王为空
	Rank  string `json:"rank"`  // "3".."2","J","Q","K","A","小王","大王"
	Value int    `json:"value"` // 比较用的数值
}

// 标准 52 张（不含王），用于炸金花 / 牛牛
func newStandardDeck() []Card {
	suits := []string{"♠", "♥", "♦", "♣"}
	ranks := []struct {
		r string
		v int
	}{
		{"2", 2}, {"3", 3}, {"4", 4}, {"5", 5}, {"6", 6}, {"7", 7}, {"8", 8},
		{"9", 9}, {"10", 10}, {"J", 11}, {"Q", 12}, {"K", 13}, {"A", 14},
	}
	deck := make([]Card, 0, 52)
	for _, s := range suits {
		for _, r := range ranks {
			deck = append(deck, Card{Suit: s, Rank: r.r, Value: r.v})
		}
	}
	return deck
}

// 54 张（含大小王），用于斗地主。Value: 3..15(2) 小王16 大王17
func newDDZDeck() []Card {
	deck := newStandardDeck()
	// 斗地主排序：3<4<...<K<A<2<小王<大王，重新赋 Value
	order := map[string]int{"3": 3, "4": 4, "5": 5, "6": 6, "7": 7, "8": 8, "9": 9,
		"10": 10, "J": 11, "Q": 12, "K": 13, "A": 14, "2": 15}
	for i := range deck {
		deck[i].Value = order[deck[i].Rank]
	}
	deck = append(deck, Card{Suit: "", Rank: "小王", Value: 16})
	deck = append(deck, Card{Suit: "", Rank: "大王", Value: 17})
	return deck
}

func shuffle(deck []Card, r *rand.Rand) []Card {
	out := make([]Card, len(deck))
	copy(out, deck)
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// 按 Value 升序排列（炸金花/牛牛手牌展示用）
func sortByValue(cards []Card) {
	for i := 1; i < len(cards); i++ {
		for j := i; j > 0 && cards[j].Value < cards[j-1].Value; j-- {
			cards[j], cards[j-1] = cards[j-1], cards[j]
		}
	}
}

// 斗地主手牌排序：升序（与 sortByValue 实现相同，保留独立函数便于后续按花色扩展）
func sortByValueDDZ(cards []Card) {
	for i := 1; i < len(cards); i++ {
		for j := i; j > 0 && cards[j].Value < cards[j-1].Value; j-- {
			cards[j], cards[j-1] = cards[j-1], cards[j]
		}
	}
}

func cardEquals(a, b Card) bool {
	return a.Suit == b.Suit && a.Rank == b.Rank && a.Value == b.Value
}

// 从手牌中移除指定牌（按引用匹配），返回是否成功
func removeCards(hand []Card, target []Card) ([]Card, bool) {
	out := make([]Card, 0, len(hand))
	used := make([]bool, len(target))
	for _, h := range hand {
		matched := false
		for i, t := range target {
			if !used[i] && cardEquals(h, t) {
				used[i] = true
				matched = true
				break
			}
		}
		if !matched {
			out = append(out, h)
		}
	}
	for _, u := range used {
		if !u {
			return nil, false
		}
	}
	return out, true
}
