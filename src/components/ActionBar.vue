<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { Card, SeatView } from '@/types'
import { useGameStore } from '@/stores/game'
import { identifyDDZPlay } from '@/utils/ddzTypes'

const props = defineProps<{ selectedCards: Card[] }>()
const store = useGameStore()

const room = computed(() => store.room)
const phase = computed(() => room.value?.phase)
const turn = computed(() => store.turn)
const isMyTurn = computed(() => turn.value?.seat === room.value?.mySeat && phase.value === 'playing')
const mySeat = computed(() => room.value?.seats[room.value?.mySeat ?? -1])
const isOwner = computed(() => store.isOwner)

// DDZ 选中牌的牌型识别提示（仅显示，校验仍由服务端负责）
const ddzPlayType = computed(() => {
  if (room.value?.game !== 'ddz' || !isMyTurn.value || turn.value?.phase !== 'playing') return null
  if (props.selectedCards.length === 0) return null
  return identifyDDZPlay(props.selectedCards)
})

const readyCount = computed(() => {
  if (!room.value) return 0
  return room.value.seats.filter((s) => s.playerId && s.ready).length
})
const canStart = computed(() => readyCount.value >= (room.value?.minPlayers ?? 99))

// 蒙牌模式直接绑定服务端权威值，避免本地 ref 与远端不一致（服务端拒绝时 checkbox 自动回滚）
function toggleBlindMode(e: Event) {
  const checked = (e.target as HTMLInputElement).checked
  store.send('setBlindMode', { blindMode: checked })
}

// 炸金花比牌目标选择
const pickingCompare = ref(false)
const compareTargets = computed<SeatView[]>(() => {
  if (!room.value || room.value.game !== 'zjh') return []
  return room.value.seats.filter((s) => s.playerId && !s.isFolded && s.seat !== room.value!.mySeat)
})

// 回合变化时复位比牌面板，避免显示过期目标（已弃牌/离场玩家）
watch(() => store.turn, () => { pickingCompare.value = false })

// 蒙牌模式下未查看的牌索引；已开牌后不再显示看牌/开牌按钮
const unlookedIndices = computed(() => {
  if (!mySeat.value?.lookedIndices || mySeat.value.isRevealed) return []
  return mySeat.value.lookedIndices.map((looked, i) => ({ i, looked })).filter(x => !x.looked).map(x => x.i)
})

// 防抖：动作发出后短暂禁用按钮，避免重复点击发多次动作
const acting = ref(false)
let actingTimer: any = null
function guardAct() {
  if (acting.value) return false
  acting.value = true
  if (actingTimer) clearTimeout(actingTimer)
  actingTimer = setTimeout(() => { acting.value = false }, 400)
  return true
}
function act(type: string, data: any = {}) {
  if (!guardAct()) return
  store.send(type, data)
}
function ready() {
  act('ready')
}
function start() {
  act('start')
}
function sit(seat: number) {
  act('sit', { seat })
}
function playCards() {
  if (props.selectedCards.length === 0) return
  act('play', { cards: props.selectedCards })
}
function callLandlord(call: boolean) {
  act('callLandlord', { call })
}
function pass() {
  act('pass')
}
function look() {
  act('look')
}
function lookCard(index: number) {
  act('lookCard', { index })
}
function reveal() {
  act('reveal')
}
function callBet() {
  act('call')
}
function raise() {
  act('raise')
}
function fold() {
  act('fold')
}
function doCompare(target?: number) {
  if (!guardAct()) return
  pickingCompare.value = false
  store.send('compare', target !== undefined ? { target } : {})
}
function niuniuConfirm() {
  if (props.selectedCards.length === 3) {
    act('niuniuSet', { cards: props.selectedCards })
  } else {
    act('niuniuSet', {})
  }
}
</script>

<template>
  <div class="action-bar glass">
    <!-- 等待大厅阶段 -->
    <template v-if="phase === 'waiting'">
      <button v-if="mySeat && !mySeat.ready" class="btn btn-gold" :disabled="acting" @click="ready">准备</button>
      <button v-if="mySeat && mySeat.ready" class="btn btn-ghost" :disabled="acting" @click="ready">取消准备</button>
      <!-- 炸金花蒙牌模式开关（仅房主），直接绑定服务端权威值 -->
      <label v-if="isOwner && room?.game === 'zjh'" class="blind-toggle">
        <input type="checkbox" :checked="!!room?.blindMode" @change="toggleBlindMode" />
        <span>蒙牌模式</span>
      </label>
      <button v-if="isOwner" class="btn btn-gold" :disabled="!canStart || acting" @click="start">
        开局 {{ readyCount }}/{{ room?.minPlayers }}+
      </button>
      <span v-if="!isOwner" class="hint">等待房主开局（已准备 {{ readyCount }} 人）</span>
      <button v-if="mySeat" class="btn btn-ghost" :disabled="acting" @click="act('stand')">离座旁观</button>
    </template>

    <!-- 斗地主叫地主 -->
    <template v-else-if="isMyTurn && turn?.phase === 'callLandlord'">
      <span class="prompt">是否当地主？</span>
      <button class="btn btn-wine" :disabled="acting" @click="callLandlord(true)">叫地主</button>
      <button class="btn btn-ghost" :disabled="acting" @click="callLandlord(false)">不叫</button>
    </template>

    <!-- 斗地主出牌 -->
    <template v-else-if="isMyTurn && room?.game === 'ddz' && turn?.phase === 'playing'">
      <span v-if="ddzPlayType" class="play-type" :class="{ invalid: !ddzPlayType.valid }">{{ ddzPlayType.name }}</span>
      <button class="btn btn-gold" :disabled="selectedCards.length === 0 || acting" @click="playCards">
        出牌 ({{ selectedCards.length }})
      </button>
      <button v-if="room.publicArea.lastPlay" class="btn btn-ghost" :disabled="acting" @click="pass">不要</button>
      <span v-else class="prompt">自由出牌</span>
    </template>

    <!-- 炸金花下注 -->
    <template v-else-if="isMyTurn && room?.game === 'zjh' && turn?.phase === 'betting'">
      <template v-if="pickingCompare">
        <span class="prompt">选择比牌对象</span>
        <button
          v-for="t in compareTargets"
          :key="t.seat"
          class="btn btn-ghost"
          :disabled="acting"
          @click="doCompare(t.seat)"
        >
          {{ t.name }}
        </button>
        <button class="btn btn-ghost" @click="pickingCompare = false">取消</button>
      </template>
      <template v-else>
        <!-- 蒙牌模式：逐张看牌 + 开牌（已开牌后不再显示） -->
        <template v-if="turn?.blindMode && !mySeat?.isRevealed">
          <button
            v-for="idx in unlookedIndices"
            :key="idx"
            class="btn btn-ghost"
            :disabled="acting"
            @click="lookCard(idx)"
          >
            看第{{ idx + 1 }}张
          </button>
          <button class="btn btn-ghost" :disabled="acting" @click="reveal">开牌</button>
        </template>
        <!-- 非蒙牌模式：看牌 -->
        <button v-if="turn?.actions?.includes('look')" class="btn btn-ghost" :disabled="acting" @click="look">看牌</button>
        <button class="btn btn-gold" :disabled="acting" @click="callBet">跟注 {{ turn?.callCost }}</button>
        <button class="btn btn-ghost" :disabled="acting" @click="raise">加注</button>
        <button v-if="turn?.actions?.includes('compare')" class="btn btn-ghost" :disabled="acting" @click="pickingCompare = true">
          比牌
        </button>
        <button class="btn btn-wine" :disabled="acting" @click="fold">弃牌</button>
      </template>
    </template>

    <!-- 牛牛押注阶段 -->
    <template v-else-if="isMyTurn && room?.game === 'nn' && turn?.phase === 'betting'">
      <span class="prompt">底池 {{ turn?.pot }} · 当前注 {{ turn?.currentBet }}</span>
      <button class="btn btn-gold" :disabled="acting" @click="callBet">跟注 {{ turn?.currentBet }}</button>
      <button class="btn btn-ghost" :disabled="acting" @click="raise">加注</button>
      <button class="btn btn-wine" :disabled="acting" @click="fold">弃牌</button>
    </template>

    <!-- 牛牛凑牛 -->
    <template v-else-if="room?.game === 'nn' && phase === 'playing' && turn?.phase === 'setNiu' && mySeat && !mySeat.hasNiu && !mySeat.isFolded">
      <span class="prompt">选 3 张凑牛（或直接确认自动）</span>
      <button class="btn btn-gold" :disabled="acting" @click="niuniuConfirm">
        确认 {{ selectedCards.length === 3 ? '(已选3张)' : '(自动)' }}
      </button>
    </template>

    <!-- 结算阶段 -->
    <template v-else-if="phase === 'settled'">
      <span class="prompt">本局结束</span>
      <button v-if="isOwner" class="btn btn-gold" :disabled="acting" @click="start">再来一局</button>
      <span v-else class="hint">等待房主开始下一局</span>
    </template>

    <!-- 非自己回合 -->
    <template v-else-if="phase === 'playing'">
      <span class="hint">{{ turn ? `等待 ${room?.seats[turn.seat]?.name ?? ''} 出手…` : '对局中…' }}</span>
    </template>
  </div>
</template>

<style scoped>
.action-bar {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.7rem;
  padding: 0.6rem 1rem;
  border-radius: 16px;
  min-height: 58px;
  flex-wrap: wrap;
}
.prompt {
  color: var(--gold-soft);
  font-size: 0.9rem;
  font-weight: 500;
}
.play-type {
  padding: 0.25rem 0.7rem;
  border-radius: 8px;
  background: rgba(212, 175, 55, 0.15);
  border: 1px solid rgba(212, 175, 55, 0.4);
  color: var(--gold);
  font-size: 0.82rem;
  font-weight: 600;
  white-space: nowrap;
}
.play-type.invalid {
  background: rgba(239, 68, 68, 0.12);
  border-color: rgba(239, 68, 68, 0.4);
  color: #ff9b9b;
}
.hint {
  color: var(--ivory-dim);
  font-size: 0.85rem;
}
.blind-toggle {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.85rem;
  color: var(--ivory-dim);
  cursor: pointer;
  user-select: none;
}
.blind-toggle input {
  accent-color: var(--gold);
}
@media (max-width: 768px) {
  .action-bar {
    gap: 0.4rem;
    padding: 0.45rem 0.5rem;
  }
  .btn {
    padding: 0.5rem 0.8rem;
    font-size: 0.82rem;
  }
}
</style>
