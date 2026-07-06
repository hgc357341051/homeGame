import { defineStore } from 'pinia'
import { ref, computed, reactive } from 'vue'
import type { RoomState, Card, ChatMsg, ServerMessage } from '@/types'
import router from '@/router'

const LS_PID = 'fc_playerId'
const LS_NAME = 'fc_name'

let socket: WebSocket | null = null
let connectPromise: Promise<void> | null = null
// 已加入的房间号（用于断线重连后重新 joinRoom）
let joinedCode: string | null = null
// 心跳与重连定时器
let heartbeatTimer: any = null
let reconnectTimer: any = null
let reconnectAttempts = 0
// 主动离开/清理标记：为 true 时 onclose 不触发自动重连
// 用独立标志而非复用 reconnectAttempts，避免污染计数器导致新房间无法重连
let intentionalClose = false
const HEARTBEAT_INTERVAL = 15000 // 15s 应用层心跳
const MAX_RECONNECT = 8

export const useGameStore = defineStore('game', () => {
  const playerId = ref(localStorage.getItem(LS_PID) || '')
  const name = ref(localStorage.getItem(LS_NAME) || '')
  const connected = ref(false)
  const connecting = ref(false)
  const reconnecting = ref(false)
  const failed = ref(false) // 重连彻底失败，需用户手动刷新

  const room = ref<RoomState | null>(null)
  const myHand = ref<Card[]>([])
  const chat = ref<ChatMsg[]>([])
  const turn = ref<{ seat: number; phase: string; actions: string[]; currentBet?: number; pot?: number; callCost?: number; blindMode?: boolean } | null>(null)
  const phaseMsg = ref<string>('')
  const log = ref<{ id: number; ts: number; text: string; kind: string }[]>([])
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
    log.value.push({ id: ++logId, ts: Date.now(), text, kind })
    if (log.value.length > 60) log.value.shift()
  }

  function showError(msg: string) {
    errorToast.value = msg
    if (errorTimer) clearTimeout(errorTimer)
    errorTimer = setTimeout(() => (errorToast.value = ''), 2600)
  }

  function startHeartbeat() {
    stopHeartbeat()
    heartbeatTimer = setInterval(() => {
      // 应用层心跳：发 ping，服务端可记录；同时检测连接是否健康
      if (socket && socket.readyState === WebSocket.OPEN) {
        send('ping', {})
      }
    }, HEARTBEAT_INTERVAL)
  }
  function stopHeartbeat() {
    if (heartbeatTimer) { clearInterval(heartbeatTimer); heartbeatTimer = null }
  }

  // 断线自动重连：递增退避，重连成功后自动重新 joinRoom 夺回座位
  function scheduleReconnect() {
    if (reconnectTimer) return
    if (reconnectAttempts >= MAX_RECONNECT) {
      reconnecting.value = false
      failed.value = true
      showError('重连失败，请刷新页面')
      return
    }
    reconnecting.value = true
    reconnectAttempts++
    const delay = Math.min(1000 * reconnectAttempts, 5000)
    reconnectTimer = setTimeout(async () => {
      reconnectTimer = null
      try {
        await connect(true)
        // 重连成功：重新进入房间
        if (joinedCode) {
          send('joinRoom', { code: joinedCode })
        }
      } catch {
        scheduleReconnect()
      }
    }, delay)
  }

  function connect(isReconnect = false): Promise<void> {
    if (socket && socket.readyState === WebSocket.OPEN) {
      connected.value = true
      // 重置重连状态（可能从其他页面切回，socket 仍在线）
      intentionalClose = false
      failed.value = false
      reconnectAttempts = 0
      return Promise.resolve()
    }
    if (connectPromise) return connectPromise
    connecting.value = true
    // 开始新连接前清除主动关闭标记
    intentionalClose = false
    connectPromise = new Promise<void>((resolve, reject) => {
      const proto = location.protocol === 'https:' ? 'wss' : 'ws'
      const url = `${proto}://${location.host}/ws`
      const ws = new WebSocket(url)
      socket = ws
      let settled = false
      ws.onopen = () => {
        connected.value = true
        connecting.value = false
        reconnecting.value = false
        failed.value = false
        reconnectAttempts = 0
        intentionalClose = false
        // 上报 enter（携带本地 playerId 以便重连夺回座位）
        ws.send(JSON.stringify({ type: 'enter', data: { name: name.value, playerId: playerId.value } }))
        startHeartbeat()
        settled = true
        resolve()
      }
      ws.onmessage = (ev) => {
        try {
          const msg: ServerMessage = JSON.parse(ev.data)
          handle(msg)
        } catch (e) {
          console.error('[WS] 消息解析失败:', ev.data, e)
        }
      }
      ws.onerror = () => {
        connecting.value = false
        if (!isReconnect) showError('网络连接异常')
        // 立即 reject，避免 connectPromise 挂起直到超时
        if (!settled) {
          settled = true
          reject(new Error('WebSocket 连接错误'))
        }
      }
      ws.onclose = () => {
        connected.value = false
        connecting.value = false
        socket = null
        connectPromise = null
        stopHeartbeat()
        // 主动离开不重连
        if (intentionalClose) return
        // 若仍在房间内则尝试自动重连
        if (joinedCode && reconnectAttempts < MAX_RECONNECT) {
          scheduleReconnect()
        } else if (reconnectAttempts >= MAX_RECONNECT) {
          failed.value = true
        }
      }
      // 超时保护：超时则关闭 ws 并清理，避免 promise 泄漏与状态错乱
      setTimeout(() => {
        if (!settled) {
          ws.close()
          socket = null
          connectPromise = null
          connecting.value = false
          reject(new Error('连接超时'))
        }
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

  function joinRoom(code: string) {
    // 去重：已加入同一房间且连接正常时不重复发送 joinRoom
    if (joinedCode === code && socket && socket.readyState === WebSocket.OPEN) return
    // 记录已加入的房间号，断线重连时自动重新加入
    joinedCode = code
    // 进入新房间：清除主动关闭标记，确保断线后可正常重连
    intentionalClose = false
    failed.value = false
    reconnectAttempts = 0
    send('joinRoom', { code })
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
        joinedCode = d.code
        intentionalClose = false
        failed.value = false
        reconnectAttempts = 0
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

  // 清理房间相关本地状态（停心跳/重连、清 joinedCode 防止重连拉回、清空状态）
  // 不发送 leave，不跳转路由，供路由守卫与 leaveRoom 复用
  function cleanupRoom() {
    // 标记主动关闭，阻止 onclose 触发自动重连（用独立标志，不污染 reconnectAttempts）
    intentionalClose = true
    joinedCode = null
    stopHeartbeat()
    if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null }
    reconnecting.value = false
    failed.value = false
    connecting.value = false
    reconnectAttempts = 0
    if (errorTimer) { clearTimeout(errorTimer); errorTimer = null }
    errorToast.value = ''
    room.value = null
    myHand.value = []
    chat.value = []
    turn.value = null
    reveal.value = null
    settle.value = null
    phaseMsg.value = ''
    log.value = []
  }

  function leaveRoom() {
    // 仅发 leave 通知服务端，由路由守卫统一 cleanupRoom（避免重复清理）
    send('leave', {})
    router.push('/')
  }

  function kickSeat(seat: number) {
    send('kick', { seat })
  }

  return {
    playerId,
    name,
    connected,
    connecting,
    reconnecting,
    failed,
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
    joinRoom,
    clearReveal,
    clearSettle,
    cleanupRoom,
    leaveRoom,
    seatName,
    kickSeat,
  }
})
