<script setup lang="ts">
import { computed, ref, watch, onUnmounted } from 'vue'
import type { SeatView } from '@/types'
import { useGameStore } from '@/stores/game'

const props = defineProps<{
  seat: SeatView
  isCurrent: boolean
  isMe: boolean
  position: 'top' | 'left' | 'right' | 'bottom'
  compact?: boolean
}>()

const store = useGameStore()

const statusBadge = computed(() => {
  if (props.seat.isFolded) return { text: '弃牌', cls: 'folded' }
  if (props.seat.isLandlord) return { text: '地主', cls: 'landlord' }
  if (props.seat.isDealer) return { text: '庄家', cls: 'dealer' }
  return null
})

const empty = computed(() => !props.seat.playerId)

// 本地掉线倒计时：服务端只在 broadcastState 时下发 offlineLeft，
// 客户端每秒自减以保证 UI 连续刷新（避免在无广播时卡住）
const localOfflineLeft = ref(0)
let offlineTimer: ReturnType<typeof setInterval> | null = null
watch(
  () => props.seat.offlineLeft,
  (v) => {
    localOfflineLeft.value = v || 0
  },
  { immediate: true },
)
watch(
  localOfflineLeft,
  (v) => {
    if (v > 0 && !offlineTimer) {
      offlineTimer = setInterval(() => {
        if (localOfflineLeft.value > 0) {
          localOfflineLeft.value--
        } else if (offlineTimer) {
          clearInterval(offlineTimer)
          offlineTimer = null
        }
      }, 1000)
    } else if (v <= 0 && offlineTimer) {
      clearInterval(offlineTimer)
      offlineTimer = null
    }
  },
  { immediate: true },
)
onUnmounted(() => {
  if (offlineTimer) clearInterval(offlineTimer)
})
const offlineLeftText = computed(() => {
  if (!props.seat.offline || localOfflineLeft.value <= 0) return ''
  const m = Math.floor(localOfflineLeft.value / 60)
  const s = localOfflineLeft.value % 60
  return `${m}:${s.toString().padStart(2, '0')}`
})

function renameSeat() {
  if (empty.value) return
  const newName = window.prompt('修改昵称（1-16字）', props.seat.name)
  if (newName) {
    const trimmed = newName.trim().slice(0, 16)
    if (trimmed) {
      store.send('rename', { seat: props.seat.seat, name: trimmed })
    }
  }
}
</script>

<template>
  <div class="seat" :class="[position, { current: isCurrent, me: isMe, empty }]">
    <div class="avatar-wrap">
      <div class="avatar" v-if="!empty">{{ seat.avatar }}</div>
      <div class="avatar placeholder" v-else>+</div>
      <div class="online-dot" :class="{ on: seat.online }" v-if="!empty" />
    </div>

    <div class="info" v-if="!empty">
      <div class="name-line">
        <span class="name clickable" @click="renameSeat" title="点击修改昵称">{{ seat.name }}</span>
        <span class="owner-tag" v-if="seat.isOwner">房主</span>
        <span class="badge" :class="statusBadge.cls" v-if="statusBadge">{{ statusBadge.text }}</span>
        <span class="badge offline-badge" v-if="seat.offline" :title="store.isOwner ? '点击踢人释放座位' : '掉线保留中'">
          掉线 {{ offlineLeftText }}
        </span>
        <span class="badge looked" v-if="seat.isLooked && !seat.isFolded && !seat.offline">看牌</span>
        <span class="badge revealed" v-if="seat.isRevealed">已开牌</span>
        <span class="badge looking" v-if="seat.lookedIndices && !seat.isRevealed && !seat.isFolded">
          看{{ seat.lookedIndices.filter(Boolean).length }}/{{ seat.lookedIndices.length }}
        </span>
      </div>
      <div class="meta">
        <span class="chip">🪙 {{ seat.chips }}</span>
        <span class="card-cnt" v-if="seat.cardCount > 0">🂠 {{ seat.cardCount }}</span>
        <span class="bet" v-if="seat.currentBet">注 {{ seat.currentBet }}</span>
        <span class="ready" v-if="seat.ready">✓ 已准备</span>
      </div>
      <div class="niu" v-if="seat.hasNiu">牛 {{ seat.niuValue === 0 ? '没' : (seat.niuValue === 10 ? '牛' : seat.niuValue) }}</div>
    </div>
    <div class="info empty-info" v-else>
      <span class="empty-text">空位</span>
    </div>

    <div class="turn-ring" v-if="isCurrent" />
  </div>
</template>

<style scoped>
.seat {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.55rem 0.7rem;
  border-radius: 14px;
  min-width: 150px;
  background: linear-gradient(160deg, rgba(20, 80, 60, 0.32), rgba(7, 32, 24, 0.5));
  border: 1px solid rgba(212, 175, 55, 0.22);
  transition: all 0.25s ease;
}
.seat.me {
  border-color: rgba(212, 175, 55, 0.6);
}
.seat.current {
  border-color: var(--gold);
  box-shadow: 0 0 0 1px var(--gold), 0 0 24px var(--gold-glow);
}
.seat.empty {
  opacity: 0.55;
  border-style: dashed;
}
.avatar-wrap {
  position: relative;
  flex-shrink: 0;
}
.avatar {
  width: 42px;
  height: 42px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  background: radial-gradient(circle at 30% 30%, var(--felt-2), var(--felt));
  border: 2px solid var(--gold-deep);
  box-shadow: inset 0 0 8px rgba(0, 0, 0, 0.4);
}
.avatar.placeholder {
  color: var(--gold-soft);
  font-size: 22px;
  border-style: dashed;
}
.online-dot {
  position: absolute;
  right: -1px;
  bottom: -1px;
  width: 11px;
  height: 11px;
  border-radius: 50%;
  background: #555;
  border: 2px solid var(--ink);
}
.online-dot.on {
  background: #4ade80;
  box-shadow: 0 0 6px #4ade80;
}
.info {
  display: flex;
  flex-direction: column;
  gap: 0.18rem;
  min-width: 0;
}
.name-line {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  flex-wrap: wrap;
}
.name {
  font-weight: 600;
  font-size: 0.92rem;
  color: var(--ivory);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 90px;
}
.name.clickable {
  cursor: pointer;
  text-decoration: underline dotted rgba(212, 175, 55, 0.4);
}
.name.clickable:hover {
  color: var(--gold-soft);
}
.owner-tag {
  font-size: 0.62rem;
  padding: 0.05rem 0.35rem;
  border-radius: 5px;
  background: rgba(212, 175, 55, 0.2);
  color: var(--gold-soft);
  border: 1px solid rgba(212, 175, 55, 0.4);
}
.badge {
  font-size: 0.62rem;
  padding: 0.05rem 0.4rem;
  border-radius: 5px;
  font-weight: 600;
}
.badge.landlord {
  background: var(--wine);
  color: var(--ivory);
}
.badge.dealer {
  background: var(--gold);
  color: var(--ink);
}
.badge.folded {
  background: #444;
  color: var(--ivory-dim);
}
.badge.looked {
  background: rgba(46, 125, 91, 0.4);
  color: #9fe3c4;
  border: 1px solid rgba(46, 125, 91, 0.6);
}
.badge.revealed {
  background: rgba(212, 175, 55, 0.3);
  color: var(--gold-soft);
  border: 1px solid var(--gold);
}
.badge.looking {
  background: rgba(139, 38, 53, 0.3);
  color: #ffb38a;
  border: 1px solid rgba(139, 38, 53, 0.5);
}
.badge.offline-badge {
  background: rgba(251, 191, 36, 0.25);
  color: #fbbf24;
  border: 1px solid rgba(251, 191, 36, 0.6);
  animation: pulseGold 1.6s ease-in-out infinite;
}
.meta {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.72rem;
  color: var(--ivory-dim);
  flex-wrap: wrap;
}
.card-cnt {
  color: var(--gold-soft);
}
.bet {
  color: #ffb38a;
}
.ready {
  color: #4ade80;
}
.niu {
  font-size: 0.7rem;
  color: var(--gold-soft);
  font-weight: 600;
}
.empty-info {
  justify-content: center;
}
.empty-text {
  color: var(--muted);
  font-size: 0.8rem;
}
.turn-ring {
  position: absolute;
  inset: -3px;
  border-radius: 16px;
  border: 2px solid var(--gold);
  pointer-events: none;
  animation: pulseGold 1.6s ease-in-out infinite;
}

@media (max-width: 768px) {
  .seat {
    min-width: auto;
    padding: 0.4rem 0.45rem;
    gap: 0.4rem;
  }
  .avatar {
    width: 34px;
    height: 34px;
    font-size: 19px;
  }
  .name {
    max-width: 60px;
    font-size: 0.8rem;
  }
  .meta {
    font-size: 0.64rem;
    gap: 0.3rem;
  }
}
</style>
