// 通用与对局相关类型定义，与服务端 protocol.go 对齐

export type GameCode = 'ddz' | 'zjh' | 'nn'

export interface Card {
  suit: string // ♠ ♥ ♦ ♣，大小王为空
  rank: string
  value: number
}

export interface SeatView {
  seat: number
  playerId: string
  name: string
  avatar: string
  chips: number
  ready: boolean
  cardCount: number
  isLandlord?: boolean
  isDealer?: boolean
  isFolded?: boolean
  isLooked?: boolean
  isOwner?: boolean
  online: boolean
  currentBet?: number
  hasNiu?: boolean
  niuValue?: number
  settledDelta?: number
}

export interface PlayInfo {
  player: string
  seat: number
  cards: Card[]
  pass?: boolean
}

export interface PublicAreaView {
  lastPlay?: PlayInfo
  lastPlays?: PlayInfo[]
  bottomCards?: Card[]
  pot?: number
  currentSeat?: number
  baseBet?: number
  currentBet?: number
  lookedCount?: number
  activeCount?: number
  phase?: string
  message?: string
  dealerSeat?: number
  winnerSeat?: number
}

export interface RoomState {
  code: string
  game: GameCode
  hostId: string
  phase: 'waiting' | 'playing' | 'settled'
  seats: SeatView[]
  mySeat: number
  publicArea: PublicAreaView
  minPlayers: number
  maxPlayers: number
  gameLabel: string
}

export interface ChatMsg {
  player: string
  text: string
  ts: number
}

// 服务端 -> 客户端 消息
export interface ServerMessage {
  type: string
  data: any
}

export const GAME_META: Record<GameCode, { label: string; desc: string; icon: string; players: string; accent: string }> = {
  ddz: {
    label: '斗地主',
    desc: '三人对抗，地主单挑双农民，炸弹翻倍',
    icon: '👑',
    players: '3 人',
    accent: '#8B2635',
  },
  zjh: {
    label: '炸金花',
    desc: '三张比大小，闷牌看牌斗智斗勇',
    icon: '🌸',
    players: '2-6 人',
    accent: '#D4AF37',
  },
  nn: {
    label: '牛牛',
    desc: '五张凑牛，庄闲对决倍数结算',
    icon: '🐂',
    players: '2-6 人',
    accent: '#2E7D5B',
  },
}

export function cardKey(c: Card): string {
  return `${c.suit}-${c.rank}-${c.value}`
}

export function isRedSuit(suit: string): boolean {
  return suit === '♥' || suit === '♦'
}
