// 斗地主牌型识别（仅用于前端显示提示，实际校验由服务端完成）
// value 映射（与服务端 cards.go 一致）：3-14=3~A, 15=2, 16=小王, 17=大王
import type { Card } from '@/types'

export interface PlayTypeInfo {
  name: string
  valid: boolean
}

// 识别选中牌的牌型，返回名称与是否合法
export function identifyDDZPlay(cards: Card[]): PlayTypeInfo {
  const n = cards.length
  if (n === 0) return { name: '', valid: false }

  // 按 value 排序
  const sorted = [...cards].sort((a, b) => a.value - b.value)
  const values = sorted.map((c) => c.value)
  const counts: Record<number, number> = {}
  for (const v of values) counts[v] = (counts[v] || 0) + 1
  const groups = Object.entries(counts).map(([v, c]) => ({ v: Number(v), c }))
  // 按 c 降序排，相同 c 按 v 降序
  groups.sort((a, b) => b.c - a.c || b.v - a.v)

  const rankName = (v: number): string => {
    if (v <= 10) return String(v)
    if (v === 11) return 'J'
    if (v === 12) return 'Q'
    if (v === 13) return 'K'
    if (v === 14) return 'A'
    if (v === 15) return '2'
    if (v === 16) return '小王'
    if (v === 17) return '大王'
    return '?'
  }

  // 王炸
  if (n === 2 && values.includes(16) && values.includes(17)) {
    return { name: '王炸', valid: true }
  }
  // 炸弹（4 张相同）
  if (n === 4 && groups[0].c === 4) {
    return { name: `炸弹 ${rankName(groups[0].v)}`, valid: true }
  }
  // 单张
  if (n === 1) return { name: `单 ${rankName(values[0])}`, valid: true }
  // 对子
  if (n === 2 && groups[0].c === 2) return { name: `对 ${rankName(groups[0].v)}`, valid: true }
  // 三张
  if (n === 3 && groups[0].c === 3) return { name: `三张 ${rankName(groups[0].v)}`, valid: true }
  // 三带一
  if (n === 4 && groups[0].c === 3 && groups[1].c === 1) {
    return { name: `三带一 ${rankName(groups[0].v)}`, valid: true }
  }
  // 三带对
  if (n === 5 && groups[0].c === 3 && groups[1].c === 2) {
    return { name: `三带对 ${rankName(groups[0].v)}`, valid: true }
  }
  // 顺子（5+ 连续单牌，不含 2 和王）
  if (n >= 5 && groups.every((g) => g.c === 1)) {
    const vs = [...values].sort((a, b) => a - b)
    let ok = true
    for (let i = 1; i < vs.length; i++) {
      if (vs[i] !== vs[i - 1] + 1) { ok = false; break }
    }
    if (ok && vs[vs.length - 1] < 15) return { name: `顺子 ${n}张`, valid: true }
  }
  // 连对（3+ 连续对子，不含 2 和王）
  if (n >= 6 && n % 2 === 0 && groups.every((g) => g.c === 2)) {
    const vs = groups.map((g) => g.v).sort((a, b) => a - b)
    let ok = true
    for (let i = 1; i < vs.length; i++) {
      if (vs[i] !== vs[i - 1] + 1) { ok = false; break }
    }
    if (ok && vs[vs.length - 1] < 15) return { name: `连对 ${n / 2}对`, valid: true }
  }
  // 飞机（2+ 连续三张，不含 2 和王）
  if (n >= 6 && n % 3 === 0 && groups.every((g) => g.c === 3)) {
    const vs = groups.map((g) => g.v).sort((a, b) => a - b)
    let ok = true
    for (let i = 1; i < vs.length; i++) {
      if (vs[i] !== vs[i - 1] + 1) { ok = false; break }
    }
    if (ok && vs[vs.length - 1] < 15) return { name: `飞机 ${n / 3}组`, valid: true }
  }
  // 飞机带翅膀（N 组连续三张 + N 个翅膀，翅膀为单牌或对牌，不含 2 和王）
  // 标准：2组三张带2个翅膀（8张：2单 或 2对），3组三张带3个翅膀（12张：3单 或 3对）
  const tripleGroups = groups.filter((g) => g.c === 3)
  if (tripleGroups.length >= 2) {
    const tps = tripleGroups.map((g) => g.v).sort((a, b) => a - b)
    // 找最长连续段
    let maxConsec = 1, curConsec = 1
    for (let i = 1; i < tps.length; i++) {
      if (tps[i] === tps[i - 1] + 1) { curConsec++; maxConsec = Math.max(maxConsec, curConsec) }
      else curConsec = 1
    }
    if (maxConsec >= 2) {
      // 取最长连续段的三张数 k，剩余必须正好是 k 个翅膀（每个为单牌或对牌，且不含 2/王）
      // 简化：用 maxConsec 作为三张组数，校验剩余牌数
      const k = maxConsec
      // 取连续段的三张
      const usedTriples = new Set<number>()
      let segStart = 0, segLen = 1
      for (let i = 1; i < tps.length; i++) {
        if (tps[i] === tps[i - 1] + 1) { segLen++; if (segLen >= k) segStart = i - segLen + 1 }
        else segLen = 1
      }
      for (let i = segStart; i < segStart + k; i++) usedTriples.add(tps[i])
      const wings = groups.filter((g) => !usedTriples.has(g.v))
      // 三张部分若有非连续三张（c===3 但不在段内），则整体不合法
      if (wings.some((g) => g.c === 3)) return { name: '无法识别', valid: false }
      // 翅膀数必须等于 k，每个翅膀为单牌(c===1)或对牌(c===2)
      if (wings.length === k && wings.every((g) => g.c === 1 || g.c === 2)) {
        // 翅膀不含 2 和王
        if (wings.every((g) => g.v < 15)) {
          const allPairs = wings.every((g) => g.c === 2)
          return { name: `飞机带${allPairs ? '对' : '翅膀'} ${k}组`, valid: true }
        }
      }
    }
  }
  // 四带二
  if (n === 6 && groups[0].c === 4) return { name: `四带二 ${rankName(groups[0].v)}`, valid: true }
  if (n === 8 && groups[0].c === 4 && groups[1].c === 2 && groups[2].c === 2) {
    return { name: `四带两对 ${rankName(groups[0].v)}`, valid: true }
  }

  return { name: '无法识别', valid: false }
}
