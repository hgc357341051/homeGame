<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useGameStore } from '@/stores/game'
import { GAME_META, type Card, type SeatView } from '@/types'
import { soundEnabled, vibrateEnabled, setSound, setVibrate, sfxTurn } from '@/utils/feedback'
import Seat from '@/components/Seat.vue'
import MyHand from '@/components/MyHand.vue'
import ActionBar from '@/components/ActionBar.vue'
import ChatPanel from '@/components/ChatPanel.vue'
import SettleModal from '@/components/SettleModal.vue'
import PlayingCard from '@/components/PlayingCard.vue'

const props = defineProps<{ code: string }>()
const store = useGameStore()
const router = useRouter()

const selectedCards = ref<Card[]>([])
const handRef = ref<{ clear: () => void } | null>(null)
const chatOpen = ref(false)
const copied = ref(false)
const revealVisible = ref(false)
const isMobile = ref(false)
// 横屏翻转：用户在竖屏锁定时点击按钮，CSS 旋转整个房间视图为横屏布局
const landscapeMode = ref(false)
// 房间号大字分享弹窗：点击房间号弹出，便于远距离查看/截图分享
const shareOpen = ref(false)
// 设置面板：音效/振动开关
const settingsOpen = ref(false)
// 快捷表情/短语浮窗：游戏内一键互动，无需打开完整聊天面板
const quickOpen = ref(false)
let revealTimer: any = null

// 快捷互动内容：表情用于即时反应，短语用于常见沟通
const quickEmojis = ['😀', '😎', '🤔', '😏', '😂', '😢', '😡', '😱', '👍', '👏', '🙏', '💪', '🔥', '💥', '🎉', '🍀']
const quickPhrases = ['快出牌呀', '稳住能赢', '好牌！', '我要炸了', '让我想想', '别走啊', '再来一局', '打得好']

// 快捷互动发送：复用 chat 通道，自带 300ms 防抖避免刷屏
let lastQuickSent = 0
function sendQuick(text: string) {
  const now = Date.now()
  if (now - lastQuickSent < 300) return
  lastQuickSent = now
  store.send('chat', { text })
  quickOpen.value = false
}

const room = computed(() => store.room)
const phase = computed(() => room.value?.phase)
const game = computed(() => room.value?.game)
const seats = computed(() => room.value?.seats ?? [])
const mySeat = computed(() => room.value?.mySeat ?? -1)
const maxPlayers = computed(() => room.value?.maxPlayers ?? 0)
const publicArea = computed(() => room.value?.publicArea ?? ({} as any))

const mySeatView = computed(() => {
  if (mySeat.value < 0) return null
  return seats.value[mySeat.value] ?? null
})

// 空位数量：用于旁观者入座引导
const emptySeatCount = computed(() => {
  if (!room.value) return 0
  return seats.value.filter((s) => !s.playerId).length
})

const isMyTurn = computed(() => store.isMyTurn && phase.value === 'playing')
const canSelectCards = computed(() => {
  if (!isMyTurn.value && !(game.value === 'nn' && store.turn?.phase === 'setNiu' && mySeatView.value && !mySeatView.value.hasNiu)) return false
  if (game.value === 'ddz' && store.turn?.phase === 'playing') return true
  if (game.value === 'nn' && store.turn?.phase === 'setNiu' && mySeatView.value && !mySeatView.value.hasNiu) return true
  return false
})
const maxSelect = computed(() => (game.value === 'nn' ? 3 : 0))

const handCardSize = computed<'sm' | 'md'>(() => {
  if (isMobile.value && store.myHand.length > 10) return 'sm'
  if (store.myHand.length > 15) return 'sm'
  return 'md'
})

const lastPlay = computed(() => publicArea.value?.lastPlay)
const bottomCards = computed(() => publicArea.value?.bottomCards ?? [])
const lastPlays = computed(() => publicArea.value?.lastPlays ?? [])
const phaseMessage = computed(() => publicArea.value?.message || store.phaseMsg)

function checkMobile() {
  isMobile.value = window.innerWidth < 900
}

function toggleLandscape() {
  landscapeMode.value = !landscapeMode.value
}

// 退出横屏翻转：系统已横屏时无需翻转，自动关闭
watch(isMobile, () => {
  if (!isMobile.value) landscapeMode.value = false
})

function seatAngle(seatIndex: number): number {
  const N = maxPlayers.value || 1
  if (mySeat.value >= 0) {
    const offset = 180 - (mySeat.value * 360) / N
    return (((seatIndex * 360) / N + offset) % 360 + 360) % 360
  }
  return (((seatIndex * 360) / N + 180) % 360 + 360) % 360
}

function seatStyle(seatIndex: number): Record<string, string> {
  const angle = seatAngle(seatIndex)
  const rad = (angle * Math.PI) / 180
  const Rx = 43
  const Ry = 37
  const x = 50 + Rx * Math.sin(rad)
  const y = 50 - Ry * Math.cos(rad)
  return {
    left: `${x}%`,
    top: `${y}%`,
    transform: 'translate(-50%, -50%)',
  }
}

function seatPosition(seatIndex: number): 'top' | 'left' | 'right' | 'bottom' {
  const a = seatAngle(seatIndex)
  if (a < 45 || a >= 315) return 'top'
  if (a < 135) return 'right'
  if (a < 225) return 'bottom'
  return 'left'
}

function onCardsChange(cards: Card[]) {
  selectedCards.value = cards
}

function clearSelection() {
  handRef.value?.clear()
  selectedCards.value = []
}

// 回合或阶段变化时清空选择
watch(
  () => store.turn,
  () => clearSelection(),
)
watch(
  () => phase.value,
  () => clearSelection(),
)

// 路由 room→room 切换时组件实例复用，onMounted 不再触发，需监听 code 重新进入新房间
watch(
  () => props.code,
  (code) => {
    if (!code) return
    store.cleanupRoom()
    joinRoom()
  },
)

// 亮牌浮层自动消失
watch(
  () => store.reveal,
  (rv) => {
    if (rv) {
      revealVisible.value = true
      if (revealTimer) clearTimeout(revealTimer)
      revealTimer = setTimeout(() => {
        revealVisible.value = false
      }, 3200)
    }
  },
)

async function joinRoom() {
  try {
    await store.connect()
    store.joinRoom(props.code.toUpperCase())
  } catch {
    /* 错误已由 store 提示 */
  }
}

// 房间不可达时重试：重新连接并 joinRoom
async function retryJoin() {
  store.roomError = ''
  await joinRoom()
}

// 返回首页：由路由守卫统一 cleanupRoom
function backHome() {
  router.push('/')
}

function copyCode() {
  const text = props.code.toUpperCase()
  // 优先用 Clipboard API，失败则降级 execCommand，兼容非 HTTPS 与旧浏览器
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(
      () => {
        copied.value = true
        setTimeout(() => (copied.value = false), 1500)
      },
      () => fallbackCopy(text),
    )
  } else {
    fallbackCopy(text)
  }
}

function fallbackCopy(text: string) {
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  } catch {
    /* 复制失败静默 */
  }
}

// 分享弹窗内的复制按钮：复制后关闭弹窗
function copyAndCloseShare() {
  copyCode()
  shareOpen.value = false
}

// 设置面板：音效/振动开关
function toggleSound() {
  setSound(!soundEnabled.value)
  // 开启时播放试听音
  if (soundEnabled.value) sfxTurn()
}
function toggleVibrate() {
  setVibrate(!vibrateEnabled.value)
}

function leave() {
  // 对局中误点离开会触发 3 分钟座位保留，需二次确认
  if (phase.value === 'playing' && mySeatView.value) {
    if (!window.confirm('对局进行中，确定离开吗？座位将保留 3 分钟供你重连。')) {
      return
    }
  }
  store.leaveRoom()
}

function clickSeat(seat: SeatView) {
  // 房主点击掉线座位：踢人释放
  if (seat.offline && store.isOwner) {
    if (window.confirm(`踢出掉线的 ${seat.name}？`)) {
      store.kickSeat(seat.seat)
    }
    return
  }
  if (phase.value !== 'waiting') return
  if (!seat.playerId) {
    store.send('sit', { seat: seat.seat })
  } else if (seat.seat === mySeat.value) {
    store.send('stand')
  }
}

onMounted(() => {
  if (!store.name) {
    store.setName('玩家' + Math.random().toString(36).slice(2, 8).toUpperCase())
  }
  checkMobile()
  window.addEventListener('resize', checkMobile)
  window.addEventListener('keydown', onKeydown)
  joinRoom()
})

onUnmounted(() => {
  if (revealTimer) clearTimeout(revealTimer)
  window.removeEventListener('resize', checkMobile)
  window.removeEventListener('keydown', onKeydown)
})

// 键盘快捷键：提升桌面端出牌效率
// 回车=出牌/确认/再来一局，空格=不要/关闭弹窗，R=准备，Esc=关闭弹窗
function onKeydown(e: KeyboardEvent) {
  // 输入框聚焦时不拦截
  const tag = (e.target as HTMLElement)?.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA') return
  // 弹窗优先级最高
  if (store.settle) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      if (store.isOwner) store.send('start')
      store.clearSettle()
    } else if (e.key === 'Escape') {
      store.clearSettle()
    }
    return
  }
  if (revealVisible.value) {
    if (e.key === 'Escape' || e.key === ' ' || e.key === 'Enter') {
      e.preventDefault()
      revealVisible.value = false
    }
    return
  }
  if (shareOpen.value || settingsOpen.value || chatOpen.value || quickOpen.value) {
    if (e.key === 'Escape') {
      shareOpen.value = false
      settingsOpen.value = false
      chatOpen.value = false
      quickOpen.value = false
    }
    return
  }
  // 等待阶段：R 切换准备
  if (phase.value === 'waiting' && mySeatView.value && (e.key === 'r' || e.key === 'R')) {
    e.preventDefault()
    store.send('ready')
    return
  }
  // 自己回合：回车=主操作（出牌/跟注/确认），空格=不要/弃牌
  if (!store.isMyTurn && !(game.value === 'nn' && store.turn?.phase === 'setNiu' && mySeatView.value && !mySeatView.value.hasNiu)) return
  const t = store.turn
  if (!t) return
  if (e.key === 'Enter') {
    e.preventDefault()
    if (game.value === 'ddz' && t.phase === 'playing') {
      if (selectedCards.value.length > 0) store.send('play', { cards: selectedCards.value })
    } else if (game.value === 'ddz' && t.phase === 'callLandlord') {
      store.send('callLandlord', { call: true })
    } else if (game.value === 'zjh' && t.phase === 'betting') {
      store.send('call', {})
    } else if (game.value === 'nn' && t.phase === 'betting') {
      store.send('call', {})
    } else if (game.value === 'nn' && t.phase === 'setNiu' && mySeatView.value && !mySeatView.value.hasNiu) {
      store.send('niuniuSet', selectedCards.value.length === 3 ? { cards: selectedCards.value } : {})
    }
  } else if (e.key === ' ') {
    e.preventDefault()
    if (game.value === 'ddz' && t.phase === 'playing') {
      if (room.value?.publicArea.lastPlay) store.send('pass', {})
    } else if (game.value === 'ddz' && t.phase === 'callLandlord') {
      store.send('callLandlord', { call: false })
    } else if ((game.value === 'zjh' || game.value === 'nn') && t.phase === 'betting') {
      store.send('fold', {})
    }
  }
}
</script>

<template>
  <div class="room-page" :class="{ 'landscape-rotate': landscapeMode }">
    <!-- 头部 -->
    <header class="room-header glass">
      <div class="room-info">
        <span class="game-icon">{{ room ? GAME_META[room.game as keyof typeof GAME_META].icon : '🎴' }}</span>
        <div class="info-text">
          <div class="game-name">{{ room?.gameLabel ?? '加载中…' }}</div>
          <div class="code-row">
            <span class="code-label">房间号</span>
            <span class="code-value clickable-code" @click="shareOpen = true" title="点击大字分享">{{ code.toUpperCase() }}</span>
            <button class="copy-btn" @click="copyCode">{{ copied ? '✓ 已复制' : '复制' }}</button>
          </div>
        </div>
      </div>
      <div class="header-actions">
        <div class="conn-status">
          <span class="dot" :class="{ on: store.connected, reconnect: store.reconnecting, fail: store.failed || !store.isOnline }" />
          <span class="conn-text">
            {{ !store.isOnline ? '网络已断开' : (store.failed ? '已断开' : (store.reconnecting ? `重连中 ${store.reconnectAttemptCount}/${8}…` : (store.connected ? '在线' : '连接中…'))) }}
          </span>
        </div>
        <button
          v-if="isMobile"
          class="btn btn-ghost icon-btn"
          :class="{ active: landscapeMode }"
          @click="toggleLandscape"
          :aria-label="landscapeMode ? '退出横屏' : '横屏显示'"
          :title="landscapeMode ? '退出横屏' : '横屏显示'"
        >↻</button>
        <button
          class="btn btn-ghost icon-btn"
          @click="settingsOpen = true"
          aria-label="设置"
          title="设置"
        >⚙</button>
        <button class="btn btn-ghost leave-btn" @click="leave">离开房间</button>
        <button
          class="btn btn-ghost icon-btn quick-fab"
          :class="{ active: quickOpen }"
          @click="quickOpen = !quickOpen"
          aria-label="快捷互动"
          title="快捷表情/短语"
        >😀</button>
        <button class="btn btn-ghost chat-fab" @click="chatOpen = !chatOpen" aria-label="消息">💬</button>
      </div>
    </header>

    <!-- 加载态 / 房间不可达 -->
    <div v-if="!room" class="loading">
      <template v-if="store.roomError">
        <div class="err-icon">🚪</div>
        <div class="err-title">{{ store.roomError }}</div>
        <div class="err-sub">房间可能已解散或配对码有误</div>
        <div class="err-actions">
          <button class="btn btn-gold" @click="retryJoin">重试</button>
          <button class="btn btn-ghost" @click="backHome">返回首页</button>
        </div>
      </template>
      <template v-else>
        <div class="deal-spinner">🎴</div>
        <div class="loading-text">正在进入房间 {{ code.toUpperCase() }}…</div>
      </template>
    </div>

    <!-- 主体 -->
    <div v-else class="room-body">
      <main class="main-area">
        <!-- 牌桌 -->
        <div class="table-wrap">
          <div class="felt-table">
            <div class="felt-inner">
              <!-- 桌面中心徽标 -->
              <div class="table-emblem">
                <span class="emblem-suit">♠</span>
                <span class="emblem-text">{{ room.gameLabel }}</span>
                <span class="emblem-suit">♣</span>
              </div>

              <!-- 中央公共区 -->
              <div class="public-area">
                <template v-if="phase === 'waiting'">
                  <div class="wait-hint">
                    <div class="hint-icon">🪑</div>
                    <div class="hint-main">点击空位入座</div>
                    <div class="hint-sub">
                      需 {{ room.minPlayers }} 人准备，房主即可开局
                    </div>
                  </div>
                </template>
                <template v-else>
                  <!-- DDZ 底牌 -->
                  <div v-if="bottomCards.length" class="bottom-cards">
                    <div class="area-label">底牌</div>
                    <div class="mini-cards">
                      <PlayingCard
                        v-for="(c, i) in bottomCards"
                        :key="'b' + i"
                        :card="c"
                        size="sm"
                        class="anim-deal"
                      />
                    </div>
                  </div>

                  <!-- 上次出牌 -->
                  <div v-if="lastPlay" class="last-play">
                    <div class="area-label">
                      <template v-if="lastPlay.pass">{{ seats[lastPlay.seat]?.name }} 不要</template>
                      <template v-else>{{ seats[lastPlay.seat]?.name }} 出牌</template>
                    </div>
                    <div v-if="!lastPlay.pass" class="mini-cards">
                      <PlayingCard
                        v-for="(c, i) in lastPlay.cards"
                        :key="'p' + i"
                        :card="c"
                        size="sm"
                      />
                    </div>
                  </div>

                  <!-- 牛牛各玩家牌 -->
                  <div v-if="lastPlays.length" class="last-plays">
                    <div v-for="lp in lastPlays" :key="lp.seat" class="lp-row">
                      <span class="lp-name">{{ seats[lp.seat]?.name }}</span>
                      <div class="mini-cards">
                        <PlayingCard v-for="(c, i) in lp.cards" :key="i" :card="c" size="xs" />
                      </div>
                    </div>
                  </div>

                  <!-- 炸金花底池 -->
                  <div v-if="publicArea.pot" class="pot-display">
                    <div class="pot-coin">🪙</div>
                    <div class="pot-amount">{{ publicArea.pot }}</div>
                    <div class="pot-label">底池</div>
                    <div class="pot-sub">当前注 {{ publicArea.currentBet }}</div>
                  </div>

                  <!-- 牛牛庄家 -->
                  <div v-if="game === 'nn' && publicArea.dealerSeat !== undefined && publicArea.dealerSeat >= 0" class="nn-dealer">
                    <span class="chip">👨‍🌾 庄家 {{ seats[publicArea.dealerSeat]?.name }}</span>
                  </div>

                  <!-- 阶段提示 -->
                  <div v-if="phaseMessage && !lastPlay && !publicArea.pot" class="phase-msg">
                    {{ phaseMessage }}
                  </div>
                </template>
              </div>

              <!-- 座位 -->
              <div
                v-for="s in seats"
                :key="s.seat"
                class="seat-slot"
                :class="{ clickable: (phase === 'waiting' && (!s.playerId || s.seat === mySeat)) || (s.offline && store.isOwner) }"
                :style="seatStyle(s.seat)"
                @click="clickSeat(s)"
              >
                <Seat
                  :seat="s"
                  :is-current="store.turn?.seat === s.seat && phase === 'playing'"
                  :is-me="s.seat === mySeat"
                  :position="seatPosition(s.seat)"
                  :compact="isMobile"
                />
              </div>
            </div>
          </div>
        </div>

        <!-- 我的手牌 + 操作栏 -->
        <div class="my-area" v-if="mySeatView">
          <div class="my-hand-wrap">
            <MyHand
              ref="handRef"
              :cards="store.myHand"
              :selectable="canSelectCards"
              :max-select="maxSelect"
              :size="handCardSize"
              @change="onCardsChange"
            />
          </div>
          <ActionBar :selected-cards="selectedCards" />
        </div>
        <div class="my-area spectator-area" v-else>
          <div class="spec-hint">
            <span>👀 旁观中</span>
            <span class="spec-sub">
              <template v-if="emptySeatCount > 0">点击桌上空位入座（{{ emptySeatCount }} 个空位）</template>
              <template v-else>座位已满，等待玩家离开</template>
            </span>
          </div>
        </div>
      </main>

      <!-- 聊天侧栏（桌面） -->
      <aside class="chat-sidebar">
        <ChatPanel />
      </aside>
    </div>

    <!-- 聊天抽屉（移动） -->
    <transition name="slide-up">
      <div v-if="chatOpen" class="chat-drawer">
        <div class="drawer-head">
          <span>消息动态</span>
          <button class="drawer-close" @click="chatOpen = false">✕</button>
        </div>
        <ChatPanel />
      </div>
    </transition>

    <!-- 亮牌浮层 -->
    <transition name="fade">
      <div v-if="revealVisible && store.reveal" class="reveal-overlay" @click="revealVisible = false">
        <div class="reveal-card gold-border">
          <div class="reveal-head">
            <span class="reveal-name">{{ seats[store.reveal.seat]?.name ?? '玩家' }}</span>
            <span class="reveal-note" v-if="store.reveal.note">{{ store.reveal.note }}</span>
            <span class="reveal-note" v-if="store.reveal.niuName">{{ store.reveal.niuName }}</span>
            <span class="reveal-note" v-if="store.reveal.type">· {{ store.reveal.type }}</span>
            <span class="reveal-winner" v-if="store.reveal.winner !== undefined && store.reveal.loser !== undefined">
              {{ store.reveal.winner === store.reveal.seat ? '胜' : '负' }}
            </span>
          </div>
          <div class="reveal-cards">
            <PlayingCard
              v-for="(c, i) in (store.reveal.cards || [])"
              :key="'a' + i"
              :card="c"
              size="md"
              class="anim-deal"
            />
          </div>
          <!-- 比牌：展示对手手牌与胜负 -->
          <template v-if="store.reveal.cards2">
            <div class="reveal-vs">VS</div>
            <div class="reveal-head">
              <span class="reveal-name">{{ seats[store.reveal.seat2]?.name ?? '玩家' }}</span>
              <span class="reveal-note" v-if="store.reveal.type2">· {{ store.reveal.type2 }}</span>
              <span class="reveal-winner" v-if="store.reveal.winner !== undefined && store.reveal.loser !== undefined">
                {{ store.reveal.winner === store.reveal.seat2 ? '胜' : '负' }}
              </span>
            </div>
            <div class="reveal-cards">
              <PlayingCard
                v-for="(c, i) in store.reveal.cards2"
                :key="'b' + i"
                :card="c"
                size="md"
                class="anim-deal"
              />
            </div>
          </template>
        </div>
      </div>
    </transition>

    <!-- 结算 -->
    <SettleModal />

    <!-- 房间号大字分享弹窗 -->
    <transition name="fade">
      <div v-if="shareOpen" class="share-overlay" @click="shareOpen = false">
        <div class="share-card gold-border" @click.stop>
          <div class="share-title">房间配对码</div>
          <div class="share-code">{{ code.toUpperCase() }}</div>
          <div class="share-game">{{ room?.gameLabel ?? '' }}</div>
          <div class="share-hint">把配对码告诉朋友，对方在首页输入即可加入</div>
          <div class="share-actions">
            <button class="btn btn-ghost" @click="shareOpen = false">关闭</button>
            <button class="btn btn-gold" @click="copyAndCloseShare">{{ copied ? '✓ 已复制' : '复制配对码' }}</button>
          </div>
        </div>
      </div>
    </transition>

    <!-- 设置面板 -->
    <transition name="fade">
      <div v-if="settingsOpen" class="share-overlay" @click="settingsOpen = false">
        <div class="settings-card gold-border" @click.stop>
          <div class="share-title">设置</div>
          <label class="setting-row">
            <span class="setting-label">🔊 音效</span>
            <input type="checkbox" :checked="soundEnabled" @change="toggleSound" />
            <span class="setting-switch" :class="{ on: soundEnabled }"><i></i></span>
          </label>
          <label class="setting-row">
            <span class="setting-label">📳 振动</span>
            <input type="checkbox" :checked="vibrateEnabled" @change="toggleVibrate" />
            <span class="setting-switch" :class="{ on: vibrateEnabled }"><i></i></span>
          </label>
          <div class="setting-note">设置自动保存，下次访问仍生效</div>
          <div class="share-actions">
            <button class="btn btn-gold" @click="settingsOpen = false">完成</button>
          </div>
        </div>
      </div>
    </transition>

    <!-- 快捷表情/短语浮窗：游戏内即时互动，点击遮罩或 Esc 关闭 -->
    <transition name="fade">
      <div v-if="quickOpen" class="quick-overlay" @click="quickOpen = false">
        <div class="quick-pop gold-border" @click.stop>
          <div class="quick-section">
            <div class="quick-label">表情</div>
            <div class="emoji-grid">
              <button
                v-for="e in quickEmojis"
                :key="e"
                class="emoji-btn"
                @click="sendQuick(e)"
              >{{ e }}</button>
            </div>
          </div>
          <div class="quick-section">
            <div class="quick-label">短语</div>
            <div class="phrase-grid">
              <button
                v-for="p in quickPhrases"
                :key="p"
                class="phrase-btn"
                @click="sendQuick(p)"
              >{{ p }}</button>
            </div>
          </div>
        </div>
      </div>
    </transition>
    <!-- 错误提示统一由 App.vue 全局渲染，避免重复 -->
  </div>
</template>

<style scoped>
.room-page {
  height: 100vh;
  height: 100dvh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* ===== 头部 ===== */
.room-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.7rem 1.1rem;
  gap: 1rem;
  border-radius: 0;
  border-left: none;
  border-right: none;
  border-top: none;
  flex-shrink: 0;
}
.room-info {
  display: flex;
  align-items: center;
  gap: 0.8rem;
}
.game-icon {
  font-size: 2rem;
  filter: drop-shadow(0 2px 6px rgba(0, 0, 0, 0.5));
}
.info-text {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}
.game-name {
  font-family: var(--font-zh);
  font-weight: 700;
  font-size: 1.1rem;
  color: var(--ivory);
  letter-spacing: 0.05em;
}
.code-row {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}
.code-label {
  font-size: 0.7rem;
  color: var(--ivory-dim);
}
.code-value {
  font-family: var(--font-display);
  font-weight: 700;
  letter-spacing: 0.25em;
  color: var(--gold);
  font-size: 1rem;
}
.copy-btn {
  font-size: 0.68rem;
  padding: 0.15rem 0.5rem;
  border-radius: 6px;
  background: rgba(212, 175, 55, 0.15);
  border: 1px solid rgba(212, 175, 55, 0.4);
  color: var(--gold-soft);
  cursor: pointer;
  transition: all 0.15s ease;
}
.copy-btn:hover {
  background: rgba(212, 175, 55, 0.3);
}
.header-actions {
  display: flex;
  align-items: center;
  gap: 0.7rem;
}
.conn-status {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.78rem;
  color: var(--ivory-dim);
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #888;
}
.dot.on {
  background: #4ade80;
  box-shadow: 0 0 8px #4ade80;
}
.dot.reconnect {
  background: #fbbf24;
  box-shadow: 0 0 8px #fbbf24;
  animation: pulseGold 1s ease-in-out infinite;
}
.dot.fail {
  background: #ef4444;
  box-shadow: 0 0 8px #ef4444;
}
.leave-btn {
  padding: 0.45rem 0.9rem;
  font-size: 0.82rem;
}
.chat-fab {
  display: none;
  padding: 0.45rem 0.7rem;
  font-size: 1rem;
}

/* ===== 加载态 ===== */
.loading {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 1rem;
}
.deal-spinner {
  font-size: 4rem;
  animation: floatUp 1.4s ease-in-out infinite alternate;
}
.loading-text {
  color: var(--ivory-dim);
  font-size: 0.95rem;
  letter-spacing: 0.1em;
}
.err-icon {
  font-size: 3.5rem;
  opacity: 0.8;
}
.err-title {
  font-family: var(--font-zh);
  font-weight: 700;
  font-size: 1.2rem;
  color: var(--wine-2);
  text-align: center;
  max-width: 320px;
}
.err-sub {
  color: var(--ivory-dim);
  font-size: 0.85rem;
}
.err-actions {
  display: flex;
  gap: 0.8rem;
  margin-top: 0.5rem;
}

/* ===== 主体 ===== */
.room-body {
  flex: 1;
  display: flex;
  overflow: hidden;
  min-height: 0;
}
.main-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
}

/* ===== 牌桌 ===== */
.table-wrap {
  flex: 1;
  padding: 1rem;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 0;
}
.felt-table {
  position: relative;
  width: 100%;
  max-width: 1100px;
  height: 100%;
  min-height: 380px;
  border-radius: 28px;
  background: linear-gradient(135deg, #3a2418 0%, #2a1810 50%, #1f110a 100%);
  padding: 10px;
  box-shadow: 0 25px 70px rgba(0, 0, 0, 0.6), inset 0 1px 0 rgba(212, 175, 55, 0.1);
}
.felt-inner {
  position: absolute;
  inset: 10px;
  border-radius: 20px;
  background: radial-gradient(
    ellipse at center,
    var(--felt-2) 0%,
    var(--felt) 55%,
    var(--felt-edge) 100%
  );
  border: 2px solid var(--gold-deep);
  box-shadow: inset 0 0 80px rgba(0, 0, 0, 0.55), inset 0 0 0 5px rgba(212, 175, 55, 0.05);
  overflow: visible;
}
.table-emblem {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  align-items: center;
  gap: 0.8rem;
  font-family: var(--font-display);
  font-size: 1.4rem;
  color: rgba(212, 175, 55, 0.08);
  pointer-events: none;
  user-select: none;
  letter-spacing: 0.15em;
}
.emblem-suit {
  font-size: 1.8rem;
}

/* ===== 公共区 ===== */
.public-area {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  min-width: 160px;
  max-width: 80%;
  text-align: center;
  z-index: 1;
}
.wait-hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.3rem;
  color: var(--ivory-dim);
}
.hint-icon {
  font-size: 2.2rem;
  opacity: 0.7;
}
.hint-main {
  font-family: var(--font-zh);
  font-weight: 700;
  color: var(--gold-soft);
  font-size: 1rem;
}
.hint-sub {
  font-size: 0.78rem;
  color: var(--muted);
}
.area-label {
  font-size: 0.72rem;
  color: var(--gold-soft);
  letter-spacing: 0.1em;
  margin-bottom: 0.2rem;
}
.mini-cards {
  display: flex;
  gap: 4px;
  justify-content: center;
  flex-wrap: wrap;
}
.last-play {
  display: flex;
  flex-direction: column;
  align-items: center;
}
.bottom-cards {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-bottom: 0.4rem;
}
.pot-display {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.1rem;
}
.pot-coin {
  font-size: 1.8rem;
  animation: pulseGold 2s ease-in-out infinite;
}
.pot-amount {
  font-family: var(--font-display);
  font-size: 1.6rem;
  font-weight: 700;
  color: var(--gold);
  text-shadow: 0 0 12px var(--gold-glow);
}
.pot-label {
  font-size: 0.7rem;
  color: var(--gold-soft);
  letter-spacing: 0.15em;
}
.pot-sub {
  font-size: 0.68rem;
  color: var(--ivory-dim);
}
.nn-dealer {
  margin-top: 0.3rem;
}
.phase-msg {
  font-size: 0.82rem;
  color: var(--gold-soft);
  padding: 0.3rem 0.8rem;
  border-radius: 8px;
  background: rgba(7, 32, 24, 0.5);
  border: 1px solid rgba(212, 175, 55, 0.2);
  max-width: 280px;
}
.last-plays {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}
.lp-row {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}
.lp-name {
  font-size: 0.72rem;
  color: var(--gold-soft);
  min-width: 50px;
  text-align: right;
}

/* ===== 座位 ===== */
.seat-slot {
  position: absolute;
  z-index: 2;
}
.seat-slot.clickable {
  cursor: pointer;
}
.seat-slot.clickable:hover ::v-deep(.seat) {
  border-color: var(--gold);
  box-shadow: 0 0 16px var(--gold-glow);
  transform: translateY(-2px);
}

/* ===== 我的手牌区 ===== */
.my-area {
  flex-shrink: 0;
  padding: 0.6rem 1rem 0.8rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  background: linear-gradient(180deg, transparent, rgba(7, 32, 24, 0.5));
}
.my-hand-wrap {
  display: flex;
  justify-content: center;
  align-items: flex-end;
  min-height: 120px;
}
.spectator-area {
  align-items: center;
}
.spec-hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.2rem;
  color: var(--ivory-dim);
  padding: 0.8rem;
}
.spec-hint span:first-child {
  font-size: 1rem;
}
.spec-sub {
  font-size: 0.78rem;
  color: var(--muted);
}

/* ===== 聊天侧栏 ===== */
.chat-sidebar {
  width: 300px;
  flex-shrink: 0;
  padding: 0.8rem 0.8rem 0.8rem 0;
  display: flex;
}
.chat-sidebar > * {
  flex: 1;
}

/* ===== 聊天抽屉（移动） ===== */
.chat-drawer {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  height: 65vh;
  z-index: 150;
  background: var(--ink-2);
  border-top: 2px solid var(--gold);
  border-radius: 20px 20px 0 0;
  display: flex;
  flex-direction: column;
  box-shadow: 0 -10px 40px rgba(0, 0, 0, 0.5);
}
.drawer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.7rem 1rem;
  border-bottom: 1px solid rgba(212, 175, 55, 0.2);
  font-family: var(--font-zh);
  font-weight: 700;
  color: var(--gold-soft);
}
.drawer-close {
  background: none;
  border: none;
  color: var(--ivory-dim);
  font-size: 1.1rem;
  cursor: pointer;
}
.chat-drawer > :deep(.chat) {
  border: none;
  border-radius: 0;
}

/* ===== 亮牌浮层 ===== */
.reveal-overlay {
  position: fixed;
  inset: 0;
  z-index: 180;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(3, 12, 8, 0.7);
  backdrop-filter: blur(4px);
  cursor: pointer;
}
.reveal-card {
  border-radius: 18px;
  padding: 1.4rem 1.6rem;
  text-align: center;
  box-shadow: 0 30px 80px rgba(0, 0, 0, 0.7);
}
.reveal-head {
  margin-bottom: 0.9rem;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.reveal-name {
  font-family: var(--font-zh);
  font-weight: 700;
  font-size: 1.2rem;
  color: var(--gold);
}
.reveal-note {
  font-size: 0.82rem;
  color: var(--ivory-dim);
}
.reveal-cards {
  display: flex;
  gap: 6px;
  justify-content: center;
  flex-wrap: wrap;
}
.reveal-vs {
  text-align: center;
  font-weight: 700;
  color: var(--gold);
  margin: 0.4rem 0;
  font-size: 1.1rem;
  letter-spacing: 0.2em;
}
.reveal-winner {
  font-size: 0.75rem;
  padding: 0.05rem 0.45rem;
  border-radius: 6px;
  background: var(--gold);
  color: var(--ink);
  font-weight: 700;
}

/* ===== 房间号分享弹窗 ===== */
.clickable-code {
  cursor: pointer;
  transition: color 0.15s ease;
}
.clickable-code:hover {
  color: var(--gold-soft);
  text-decoration: underline;
}
.share-overlay {
  position: fixed;
  inset: 0;
  z-index: 200;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(3, 12, 8, 0.8);
  backdrop-filter: blur(6px);
  padding: 1rem;
}
.share-card {
  border-radius: 20px;
  padding: 2rem 2.5rem;
  text-align: center;
  max-width: 90vw;
}
.share-title {
  font-size: 0.85rem;
  color: var(--ivory-dim);
  letter-spacing: 0.2em;
  margin-bottom: 0.5rem;
}
.share-code {
  font-family: var(--font-display);
  font-weight: 700;
  font-size: clamp(3rem, 12vw, 5.5rem);
  letter-spacing: 0.25em;
  color: var(--gold);
  text-shadow: 0 0 24px var(--gold-glow);
  margin: 0.5rem 0;
  line-height: 1.1;
}
.share-game {
  font-family: var(--font-zh);
  font-weight: 700;
  color: var(--gold-soft);
  font-size: 1.1rem;
  margin-bottom: 1rem;
}
.share-hint {
  font-size: 0.82rem;
  color: var(--ivory-dim);
  margin-bottom: 1.5rem;
}
.share-actions {
  display: flex;
  gap: 0.8rem;
  justify-content: center;
}

/* ===== 设置面板 ===== */
.settings-card {
  border-radius: 18px;
  padding: 1.6rem 1.8rem;
  width: min(320px, 90vw);
  text-align: center;
}
.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.7rem 0.5rem;
  cursor: pointer;
  position: relative;
}
.setting-row input[type="checkbox"] {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}
.setting-label {
  font-size: 0.95rem;
  color: var(--ivory);
}
.setting-switch {
  width: 44px;
  height: 24px;
  border-radius: 999px;
  background: rgba(7, 32, 24, 0.8);
  border: 1px solid rgba(212, 175, 55, 0.3);
  position: relative;
  transition: background 0.2s ease;
}
.setting-switch i {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: var(--ivory-dim);
  transition: transform 0.2s ease, background 0.2s ease;
}
.setting-switch.on {
  background: linear-gradient(135deg, var(--gold-soft), var(--gold-deep));
}
.setting-switch.on i {
  transform: translateX(20px);
  background: var(--ink);
}
.setting-note {
  font-size: 0.75rem;
  color: var(--muted);
  margin: 0.5rem 0 1rem;
}

/* ===== 快捷表情/短语浮窗 ===== */
.quick-fab {
  font-size: 1.1rem;
}
.quick-overlay {
  position: fixed;
  inset: 0;
  z-index: 200;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(3, 12, 8, 0.55);
  backdrop-filter: blur(3px);
  padding: 1rem;
}
.quick-pop {
  border-radius: 16px;
  padding: 1.1rem 1.2rem;
  width: min(360px, 92vw);
  background: var(--ink-2);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.6);
}
.quick-section + .quick-section {
  margin-top: 0.9rem;
}
.quick-label {
  font-size: 0.72rem;
  color: var(--ivory-dim);
  letter-spacing: 0.15em;
  margin-bottom: 0.5rem;
}
.emoji-grid {
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: 0.25rem;
}
.emoji-btn {
  font-size: 1.4rem;
  padding: 0.25rem 0;
  border-radius: 8px;
  background: rgba(20, 80, 60, 0.25);
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.12s ease;
  line-height: 1.2;
}
.emoji-btn:hover {
  background: rgba(212, 175, 55, 0.15);
  border-color: rgba(212, 175, 55, 0.4);
  transform: scale(1.12);
}
.emoji-btn:active {
  transform: scale(0.95);
}
.phrase-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}
.phrase-btn {
  font-size: 0.82rem;
  padding: 0.35rem 0.7rem;
  border-radius: 999px;
  background: rgba(20, 80, 60, 0.35);
  border: 1px solid rgba(212, 175, 55, 0.3);
  color: var(--ivory);
  cursor: pointer;
  transition: all 0.15s ease;
}
.phrase-btn:hover {
  background: rgba(212, 175, 55, 0.18);
  border-color: var(--gold);
  color: var(--gold-soft);
}
.phrase-btn:active {
  transform: scale(0.96);
}

/* ===== 过渡 ===== */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.25s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
.slide-up-enter-active,
.slide-up-leave-active {
  transition: transform 0.3s cubic-bezier(0.2, 0.9, 0.3, 1.1);
}
.slide-up-enter-from,
.slide-up-leave-to {
  transform: translateY(100%);
}

/* ===== 响应式 ===== */
/* 安全区：iPhone 刘海/底部 home indicator 不遮挡内容 */
@supports (padding: max(0px)) {
  .room-header {
    padding-left: max(1.1rem, env(safe-area-inset-left));
    padding-right: max(1.1rem, env(safe-area-inset-right));
    padding-top: max(0.7rem, env(safe-area-inset-top));
  }
  .my-area {
    padding-bottom: max(0.8rem, env(safe-area-inset-bottom));
  }
}

/* 横屏翻转按钮 */
.icon-btn {
  padding: 0.45rem 0.6rem;
  font-size: 1.1rem;
  line-height: 1;
  min-width: 40px;
  min-height: 40px;
}
.icon-btn.active {
  background: linear-gradient(135deg, var(--gold-soft), var(--gold));
  color: var(--ink);
  border-color: var(--gold);
}

/* 横屏翻转：竖屏锁定时旋转整个房间视图
   原理：旋转 90° 后宽高互换，用 100vh 作宽、100vw 作高，origin 居中 */
.room-page.landscape-rotate {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vh;
  height: 100vw;
  transform: rotate(90deg) translateY(-100vw);
  transform-origin: top left;
  z-index: 1000;
}
/* 翻转模式下内部布局按横屏（宽屏）优化 */
.room-page.landscape-rotate .chat-sidebar {
  display: flex;
}
.room-page.landscape-rotate .chat-fab {
  display: none;
}

@media (max-width: 900px) {
  .chat-sidebar {
    display: none;
  }
  .chat-fab {
    display: inline-flex;
  }
  .room-header {
    padding: 0.55rem 0.7rem;
  }
  .game-icon {
    font-size: 1.6rem;
  }
  .game-name {
    font-size: 0.95rem;
  }
  .conn-text {
    display: none;
  }
  .leave-btn {
    padding: 0.4rem 0.7rem;
    font-size: 0.78rem;
    min-height: 40px;
  }
  .chat-fab {
    min-width: 40px;
    min-height: 40px;
  }
  .table-wrap {
    padding: 0.5rem;
  }
  .felt-table {
    min-height: 320px;
    border-radius: 20px;
  }
  .felt-inner {
    border-radius: 14px;
  }
  .table-emblem {
    font-size: 1rem;
  }
  .emblem-suit {
    font-size: 1.3rem;
  }
  .my-area {
    padding: 0.4rem 0.5rem 0.5rem;
    gap: 0.35rem;
  }
  .my-hand-wrap {
    min-height: 90px;
  }
  .public-area {
    min-width: 120px;
  }
  /* 触摸目标 ≥ 40px，避免误触 */
  .copy-btn {
    padding: 0.3rem 0.6rem;
    font-size: 0.72rem;
  }
}

@media (max-width: 480px) {
  .felt-table {
    min-height: 280px;
  }
  .pot-amount {
    font-size: 1.3rem;
  }
  .pot-coin {
    font-size: 1.4rem;
  }
  /* 超窄屏：缩小头部信息密度 */
  .room-header {
    padding: 0.4rem 0.5rem;
    gap: 0.5rem;
  }
  .game-icon {
    font-size: 1.3rem;
  }
  .game-name {
    font-size: 0.82rem;
  }
  .code-label {
    display: none;
  }
  .code-value {
    font-size: 0.85rem;
  }
}
</style>
