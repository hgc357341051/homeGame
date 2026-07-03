<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Card, SeatView } from '@/types'
import { useGameStore } from '@/stores/game'

const props = defineProps<{ selectedCards: Card[] }>()
const store = useGameStore()

const room = computed(() => store.room)
const phase = computed(() => room.value?.phase)
const turn = computed(() => store.turn)
const isMyTurn = computed(() => turn.value?.seat === room.value?.mySeat && phase.value === 'playing')
const mySeat = computed(() => room.value?.seats[room.value?.mySeat ?? -1])
const isOwner = computed(() => store.isOwner)

const readyCount = computed(() => {
  if (!room.value) return 0
  return room.value.seats.filter((s) => s.playerId && s.ready).length
})
const canStart = computed(() => readyCount.value >= (room.value?.minPlayers ?? 99))

// 炸金花比牌目标选择
const pickingCompare = ref(false)
const compareTargets = computed<SeatView[]>(() => {
  if (!room.value || room.value.game !== 'zjh') return []
  return room.value.seats.filter((s) => s.playerId && !s.isFolded && s.seat !== room.value!.mySeat)
})

function act(type: string, data: any = {}) {
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
  pickingCompare.value = false
  act('compare', target !== undefined ? { target } : {})
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
      <button v-if="mySeat && !mySeat.ready" class="btn btn-gold" @click="ready">准备</button>
      <button v-if="mySeat && mySeat.ready" class="btn btn-ghost" @click="ready">取消准备</button>
      <button v-if="isOwner" class="btn btn-gold" :disabled="!canStart" @click="start">
        开局 {{ readyCount }}/{{ room?.minPlayers }}+
      </button>
      <span v-if="!isOwner" class="hint">等待房主开局（已准备 {{ readyCount }} 人）</span>
      <button v-if="mySeat" class="btn btn-ghost" @click="sit(-1)">离座旁观</button>
    </template>

    <!-- 斗地主叫地主 -->
    <template v-else-if="isMyTurn && turn?.phase === 'callLandlord'">
      <span class="prompt">是否当地主？</span>
      <button class="btn btn-wine" @click="callLandlord(true)">叫地主</button>
      <button class="btn btn-ghost" @click="callLandlord(false)">不叫</button>
    </template>

    <!-- 斗地主出牌 -->
    <template v-else-if="isMyTurn && room?.game === 'ddz' && turn?.phase === 'playing'">
      <button class="btn btn-gold" :disabled="selectedCards.length === 0" @click="playCards">
        出牌 ({{ selectedCards.length }})
      </button>
      <button v-if="room.publicArea.lastPlay" class="btn btn-ghost" @click="pass">不要</button>
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
          @click="doCompare(t.seat)"
        >
          {{ t.name }}
        </button>
        <button class="btn btn-ghost" @click="pickingCompare = false">取消</button>
      </template>
      <template v-else>
        <button v-if="turn?.actions?.includes('look')" class="btn btn-ghost" @click="look">看牌</button>
        <button class="btn btn-gold" @click="callBet">跟注 {{ turn?.callCost }}</button>
        <button class="btn btn-ghost" @click="raise">加注</button>
        <button v-if="turn?.actions?.includes('compare')" class="btn btn-ghost" @click="pickingCompare = true">
          比牌
        </button>
        <button class="btn btn-wine" @click="fold">弃牌</button>
      </template>
    </template>

    <!-- 牛牛凑牛 -->
    <template v-else-if="room?.game === 'nn' && phase === 'playing' && mySeat && !mySeat.hasNiu">
      <span class="prompt">选 3 张凑牛（或直接确认自动）</span>
      <button class="btn btn-gold" @click="niuniuConfirm">
        确认 {{ selectedCards.length === 3 ? '(已选3张)' : '(自动)' }}
      </button>
    </template>

    <!-- 结算阶段 -->
    <template v-else-if="phase === 'settled'">
      <span class="prompt">本局结束</span>
      <button v-if="isOwner" class="btn btn-gold" @click="start">再来一局</button>
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
.hint {
  color: var(--ivory-dim);
  font-size: 0.85rem;
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
