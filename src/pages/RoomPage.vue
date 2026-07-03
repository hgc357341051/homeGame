<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useGameStore } from '@/stores/game'
import { GAME_META, type Card, type SeatView } from '@/types'
import Seat from '@/components/Seat.vue'
import MyHand from '@/components/MyHand.vue'
import ActionBar from '@/components/ActionBar.vue'
import ChatPanel from '@/components/ChatPanel.vue'
import SettleModal from '@/components/SettleModal.vue'
import PlayingCard from '@/components/PlayingCard.vue'

const props = defineProps<{ code: string }>()
const store = useGameStore()

const selectedCards = ref<Card[]>([])
const handRef = ref<{ clear: () => void } | null>(null)
const chatOpen = ref(false)
const copied = ref(false)
const revealVisible = ref(false)
const isMobile = ref(false)
let revealTimer: any = null

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

const isMyTurn = computed(() => store.isMyTurn && phase.value === 'playing')
const canSelectCards = computed(() => {
  if (!isMyTurn.value && !(game.value === 'nn' && phase.value === 'playing' && mySeatView.value && !mySeatView.value.hasNiu)) return false
  if (game.value === 'ddz' && store.turn?.phase === 'playing') return true
  if (game.value === 'nn' && phase.value === 'playing' && mySeatView.value && !mySeatView.value.hasNiu) return true
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
    store.send('joinRoom', { code: props.code.toUpperCase() })
  } catch {
    /* 错误已由 store 提示 */
  }
}

function copyCode() {
  const text = props.code.toUpperCase()
  if (navigator.clipboard) {
    navigator.clipboard.writeText(text).then(() => {
      copied.value = true
      setTimeout(() => (copied.value = false), 1500)
    })
  }
}

function leave() {
  store.leaveRoom()
}

function clickSeat(seat: SeatView) {
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
  joinRoom()
})

onUnmounted(() => {
  if (revealTimer) clearTimeout(revealTimer)
  window.removeEventListener('resize', checkMobile)
})
</script>

<template>
  <div class="room-page">
    <!-- 头部 -->
    <header class="room-header glass">
      <div class="room-info">
        <span class="game-icon">{{ room ? GAME_META[room.game as keyof typeof GAME_META].icon : '🎴' }}</span>
        <div class="info-text">
          <div class="game-name">{{ room?.gameLabel ?? '加载中…' }}</div>
          <div class="code-row">
            <span class="code-label">房间号</span>
            <span class="code-value">{{ code.toUpperCase() }}</span>
            <button class="copy-btn" @click="copyCode">{{ copied ? '✓ 已复制' : '复制' }}</button>
          </div>
        </div>
      </div>
      <div class="header-actions">
        <div class="conn-status">
          <span class="dot" :class="{ on: store.connected }" />
          <span class="conn-text">{{ store.connected ? '在线' : '连接中…' }}</span>
        </div>
        <button class="btn btn-ghost leave-btn" @click="leave">离开房间</button>
        <button class="btn btn-ghost chat-fab" @click="chatOpen = !chatOpen" aria-label="消息">💬</button>
      </div>
    </header>

    <!-- 加载态 -->
    <div v-if="!room" class="loading">
      <div class="deal-spinner">🎴</div>
      <div class="loading-text">正在进入房间 {{ code.toUpperCase() }}…</div>
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
                :class="{ clickable: phase === 'waiting' && (!s.playerId || s.seat === mySeat) }"
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
            <span class="spec-sub">等待空位后点击入座</span>
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
          </div>
          <div class="reveal-cards">
            <PlayingCard
              v-for="(c, i) in (store.reveal.cards || [])"
              :key="i"
              :card="c"
              size="md"
              class="anim-deal"
            />
          </div>
        </div>
      </div>
    </transition>

    <!-- 结算 -->
    <SettleModal />

    <!-- 全局错误提示 -->
    <transition name="fade">
      <div v-if="store.errorToast" class="error-toast">{{ store.errorToast }}</div>
    </transition>
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

/* ===== 错误提示 ===== */
.error-toast {
  position: fixed;
  top: 70px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 300;
  background: linear-gradient(135deg, var(--wine-2), var(--wine));
  color: var(--ivory);
  padding: 0.6rem 1.2rem;
  border-radius: 10px;
  font-size: 0.88rem;
  box-shadow: 0 10px 30px rgba(139, 38, 53, 0.4);
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
}
</style>
