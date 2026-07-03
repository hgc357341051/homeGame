import { defineStore } from 'pinia'
import { ref, computed, reactive } from 'vue'
import type { RoomState, Card, ChatMsg, ServerMessage } from '@/types'
import router from '@/router'

const LS_PID = 'fc_playerId'
const LS_NAME = 'fc_name'

let socket: WebSocket | null = null
let connectPromise: Promise<void> | null = null

export const useGameStore = defineStore('game', () => {
  const playerId = ref(localStorage.getItem(LS_PID) || '')
  const name = ref(localStorage.getItem(LS_NAME) || '')
  const connected = ref(false)
  const connecting = ref(false)

  const room = ref<RoomState | null>(null)
  const myHand = ref<Card[]>([])
  const chat = ref<ChatMsg[]>([])
  const turn = ref<{ seat: number; phase: string; actions: string[]; currentBet?: number; pot?: number; callCost?: number; blindMode?: boolean } | null>(null)
  const phaseMsg = ref<string>('')
  const log = ref<{ id: number; text: string; kind: string }[]>([])
  const reveal = ref<any>(null)
  const settle = ref<any>(null)
  const errorToast = ref<string>('')
  let logId = 0
  let errorTimer: any = null

  const mySeatView = computed(() => {
    if (!room.value || room.value.mySeat < 0) return null
    return room.value.seats[room.value.mySeat] || null
  })
  const isOwner = computed(() => room.value?.hostId === playerId.value)
  const isMyTurn = computed(() => turn.value?.seat === room.value?.mySeat)

  function pushLog(text: string, kind = 'info') {
    log.value.push({ id: ++logId, text, kind })
    if (log.value.length > 60) log.value.shift()
  }

  function showError(msg: string) {
    errorToast.value = msg
    if (errorTimer) clearTimeout(errorTimer)
    errorTimer = setTimeout(() => (errorToast.value = ''), 2600)
  }

  function connect(): Promise<void> {
    if (socket && socket.readyState === WebSocket.OPEN) {
      connected.value = true
      return Promise.resolve()
    }
    if (connectPromise) return connectPromise
    connecting.value = true
    connectPromise = new Promise<void>((resolve, reject) => {
      const proto = location.protocol === 'https:' ? 'wss' : 'ws'
      const url = `${proto}://${location.host}/ws`
      const ws = new WebSocket(url)
      socket = ws
      ws.onopen = () => {
        connected.value = true
        connecting.value = false
        // 上报 enter（携带本地 playerId 以便重连夺回座位）
        ws.send(JSON.stringify({ type: 'enter', data: { name: name.value, playerId: playerId.value } }))
        resolve()
      }
      ws.onmessage = (ev) => {
        try {
          const msg: ServerMessage = JSON.parse(ev.data)
          handle(msg)
        } catch (e) {
          /* ignore */
        }
      }
      ws.onerror = () => {
        connecting.value = false
        showError('网络连接异常')
      }
      ws.onclose = () => {
        connected.value = false
        connecting.value = false
        socket = null
        connectPromise = null
      }
      // 超时保护
      setTimeout(() => {
        if (!connected.value) reject(new Error('连接超时'))
      }, 8000)
    })
    return connectPromise
  }

  function send(type: string, data: any = {}) {
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({ type, data }))
    }
  }

  function setName(n: string) {
    name.value = n
    localStorage.setItem(LS_NAME, n)
  }

  function handle(msg: ServerMessage) {
    const d = msg.data || {}
    switch (msg.type) {
      case 'entered':
        playerId.value = d.playerId
        localStorage.setItem(LS_PID, d.playerId)
        if (d.name) name.value = d.name
        break
      case 'roomCreated':
        router.push(`/room/${d.code}`)
        break
      case 'joined':
        if (d.code) router.push(`/room/${d.code}`)
        break
      case 'roomState':
        room.value = d
        break
      case 'deal':
        if (d.blindMode && d.cardCount && !d.cards) {
          // 蒙牌模式：初始化占位手牌（牌面朝下）
          myHand.value = Array.from({ length: d.cardCount }, () => ({ suit: '', rank: '?', value: 0 }))
        } else if (d.blindMode && d.cards && d.index !== undefined) {
          // 蒙牌模式：逐张看牌，更新指定位置
          const idx = d.index as number
          if (myHand.value[idx]) {
            myHand.value[idx] = (d.cards as Card[])[0]
            myHand.value = [...myHand.value]
          }
        } else if (d.blindMode && d.cards && d.lookedIndices) {
          // 蒙牌模式：状态刷新（重连），已查看位置有牌值，未查看为零值占位
          const cards = d.cards as Card[]
          const looked = d.lookedIndices as boolean[]
          myHand.value = cards.map((c, i) => looked[i] ? c : { suit: '', rank: '?', value: 0 })
        } else {
          myHand.value = d.cards || []
        }
        break
      case 'turn':
        turn.value = d
        break
      case 'phase':
        if (d.message) {
          phaseMsg.value = d.message
          pushLog(d.message, d.phase || 'info')
        }
        if (d.event === 'look') pushLog(`${d.name} 看牌`, 'event')
        if (d.event === 'lookCard') pushLog(`${d.name} 查看第${(d.index ?? 0) + 1}张牌`, 'event')
        if (d.event === 'fold') pushLog(`${d.name} 弃牌`, 'event')
        if (d.event === 'call') pushLog(`${d.name} 跟注 ${d.amount}`, 'event')
        if (d.event === 'raise') pushLog(`${d.name} 加注到 ${d.currentBet} (付 ${d.amount})`, 'event')
        if (d.event === 'niuniuSet') pushLog(`${d.name} 确认 ${d.name2}`, 'event')
        break
      case 'played':
        if (d.pass) pushLog(`${seatName(d.seat)} 不要`, 'event')
        else pushLog(`${seatName(d.seat)} 出牌`, 'event')
        break
      case 'reveal':
        reveal.value = { ...d, ts: Date.now() }
        break
      case 'settle':
        settle.value = { ...d, ts: Date.now() }
        break
      case 'chat':
        chat.value.push({ player: d.player, text: d.text, ts: Date.now(), system: d.system })
        if (chat.value.length > 100) chat.value.shift()
        break
      case 'error':
        showError(d.msg || '操作失败')
        break
    }
  }

  function seatName(seat: number): string {
    const s = room.value?.seats[seat]
    return s ? s.name : `座位${seat}`
  }

  function clearReveal() {
    reveal.value = null
  }
  function clearSettle() {
    settle.value = null
  }

  function leaveRoom() {
    send('leave', {})
    room.value = null
    myHand.value = []
    chat.value = []
    turn.value = null
    reveal.value = null
    settle.value = null
    router.push('/')
  }

  return {
    playerId,
    name,
    connected,
    connecting,
    room,
    myHand,
    chat,
    turn,
    phaseMsg,
    log,
    reveal,
    settle,
    errorToast,
    mySeatView,
    isOwner,
    isMyTurn,
    connect,
    send,
    setName,
    clearReveal,
    clearSettle,
    leaveRoom,
    seatName,
  }
})
