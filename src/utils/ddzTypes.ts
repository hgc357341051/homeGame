// 斗地主牌型识别（仅用于前端显示提示，实际校验由服务端完成）
// value: 3-15(3-A), 16=2, 17=小王, 18=大王
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
  // 飞机带翅膀（飞机 + 等量单牌/对子）— 简化识别
  const tripleGroups = groups.filter((g) => g.c >= 3)
  if (tripleGroups.length >= 2 && n >= 8) {
    const tps = tripleGroups.map((g) => g.v).sort((a, b) => a - b)
    let consecutive = 1
    for (let i = 1; i < tps.length; i++) {
      if (tps[i] === tps[i - 1] + 1) consecutive++
    }
    if (consecutive >= 2) return { name: `飞机带翅膀`, valid: true }
  }
  // 四带二
  if (n === 6 && groups[0].c === 4) return { name: `四带二 ${rankName(groups[0].v)}`, valid: true }
  if (n === 8 && groups[0].c === 4 && groups[1].c === 2 && groups[2].c === 2) {
    return { name: `四带两对 ${rankName(groups[0].v)}`, valid: true }
  }

  return { name: '无法识别', valid: false }
}
