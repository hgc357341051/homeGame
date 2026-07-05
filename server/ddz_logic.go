package main

import "sort"

// ===== 斗地主牌型分析 =====

type ddzPlay struct {
	Type  string // single/pair/triple/tripleSingle/triplePair/straight/pairStraight/plane/planeSingle/planePair/fourTwo/fourTwoPair/bomb/rocket
	Main  int    // 主牌点数用于比较
	Len   int    // 顺子/连对/飞机的长度（组数或张数）
	Cards []Card
}

func ddzValueCounts(cards []Card) map[int]int {
	m := map[int]int{}
	for _, c := range cards {
		m[c.Value]++
	}
	return m
}

func ddzSortedValues(cards []Card) []int {
	vs := make([]int, 0, len(cards))
	for _, c := range cards {
		vs = append(vs, c.Value)
	}
	sort.Ints(vs)
	return vs
}

// analyzeDDZ 判定牌型，返回是否合法
func analyzeDDZ(cards []Card) (*ddzPlay, bool) {
	n := len(cards)
	if n == 0 {
		return nil, false
	}
	counts := ddzValueCounts(cards)
	// rocket
	if n == 2 {
		v := ddzSortedValues(cards)
		if v[0] == 16 && v[1] == 17 {
			return &ddzPlay{Type: "rocket", Main: 17, Len: 2, Cards: cards}, true
		}
	}
	// 按出现次数分组
	var fours, threes, twos, ones []int
	for val, cnt := range counts {
		switch cnt {
		case 4:
			fours = append(fours, val)
		case 3:
			threes = append(threes, val)
		case 2:
			twos = append(twos, val)
		case 1:
			ones = append(ones, val)
		}
	}
	sort.Ints(fours)
	sort.Ints(threes)
	sort.Ints(twos)
	sort.Ints(ones)

	// bomb
	if n == 4 && len(fours) == 1 {
		return &ddzPlay{Type: "bomb", Main: fours[0], Len: 4, Cards: cards}, true
	}
	// single
	if n == 1 {
		return &ddzPlay{Type: "single", Main: ones[0], Len: 1, Cards: cards}, true
	}
	// pair
	if n == 2 && len(twos) == 1 {
		return &ddzPlay{Type: "pair", Main: twos[0], Len: 1, Cards: cards}, true
	}
	// triple
	if n == 3 && len(threes) == 1 {
		return &ddzPlay{Type: "triple", Main: threes[0], Len: 1, Cards: cards}, true
	}
	// triple + single
	if n == 4 && len(threes) == 1 && len(ones) == 1 {
		return &ddzPlay{Type: "tripleSingle", Main: threes[0], Len: 1, Cards: cards}, true
	}
	// triple + pair
	if n == 5 && len(threes) == 1 && len(twos) == 1 {
		return &ddzPlay{Type: "triplePair", Main: threes[0], Len: 1, Cards: cards}, true
	}
	// four + two singles (允许两张单牌点数相同，即一个对子拆成两张单牌)
	if n == 6 && len(fours) == 1 && len(ones)+2*len(twos) == 2 && len(threes) == 0 {
		return &ddzPlay{Type: "fourTwo", Main: fours[0], Len: 1, Cards: cards}, true
	}
	// four + two pairs
	if n == 8 && len(fours) == 1 && len(twos) == 2 {
		return &ddzPlay{Type: "fourTwoPair", Main: fours[0], Len: 1, Cards: cards}, true
	}
	// straight: 5+ singles, consecutive, all <= 14 (A)
	if n >= 5 && len(ones) == n && isConsecutive(ones) && ones[len(ones)-1] <= 14 {
		return &ddzPlay{Type: "straight", Main: ones[0], Len: n, Cards: cards}, true
	}
	// pair straight: 3+ pairs, consecutive, all <= 14
	if n >= 6 && n%2 == 0 && len(twos) == n/2 && isConsecutive(twos) && twos[len(twos)-1] <= 14 {
		return &ddzPlay{Type: "pairStraight", Main: twos[0], Len: len(twos), Cards: cards}, true
	}
	// plane (pure consecutive triples)
	if n >= 6 && n%3 == 0 && len(threes) == n/3 && isConsecutive(threes) && threes[len(threes)-1] <= 14 {
		return &ddzPlay{Type: "plane", Main: threes[0], Len: len(threes), Cards: cards}, true
	}
	// plane + singles: k triples consecutive + k singles (允许翼含同点对子拆分)
	if len(threes) >= 2 && isConsecutive(threes) && threes[len(threes)-1] <= 14 {
		k := len(threes)
		// 翼总张数 = k，可由单牌 + 对子拆分组成；不能含三张/四张
		if len(ones)+2*len(twos) == k && len(fours) == 0 && n == 4*k {
			return &ddzPlay{Type: "planeSingle", Main: threes[0], Len: k, Cards: cards}, true
		}
		// plane + pairs: k triples + k pairs, total = 3k + 2k
		if len(twos) == k && n == 5*k {
			return &ddzPlay{Type: "planePair", Main: threes[0], Len: k, Cards: cards}, true
		}
	}
	return nil, false
}

func isConsecutive(vals []int) bool {
	if len(vals) < 2 {
		return false
	}
	for i := 1; i < len(vals); i++ {
		if vals[i] != vals[i-1]+1 {
			return false
		}
	}
	return true
}

func ddzCanBeat(np, lp *ddzPlay) bool {
	if np == nil {
		return false
	}
	if lp == nil {
		return true // 自由出牌
	}
	if np.Type == "rocket" {
		return true
	}
	if np.Type == "bomb" {
		if lp.Type == "rocket" {
			return false
		}
		if lp.Type == "bomb" {
			return np.Main > lp.Main
		}
		return true
	}
	if lp.Type == "bomb" || lp.Type == "rocket" {
		return false
	}
	if np.Type != lp.Type || np.Len != lp.Len {
		return false
	}
	return np.Main > lp.Main
}
