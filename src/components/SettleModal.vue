<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '@/stores/game'

const store = useGameStore()
const show = computed(() => !!store.settle)

const data = computed(() => store.settle)
const winnerName = computed(() => {
  const r = data.value?.results?.find((x: any) => x.win)
  return r?.name ?? ''
})
const isWin = computed(() => {
  const me = data.value?.results?.find((x: any) => x.seat === store.room?.mySeat)
  return me?.delta > 0
})

function close() {
  store.clearSettle()
}
function again() {
  store.send('start')
  store.clearSettle()
}

// 金币雨
const coins = Array.from({ length: 18 }, (_, i) => ({
  left: Math.random() * 100,
  delay: Math.random() * 0.8,
  dur: 1.6 + Math.random() * 1.2,
  size: 14 + Math.random() * 14,
}))
</script>

<template>
  <transition name="modal">
    <div v-if="show" class="mask" @click.self="close">
      <div class="card gold-border">
        <div class="coin-rain" v-if="isWin">
          <span
            v-for="(c, i) in coins"
            :key="i"
            class="coin"
            :style="{
              left: c.left + '%',
              animationDelay: c.delay + 's',
              animationDuration: c.dur + 's',
              fontSize: c.size + 'px',
            }"
            >🪙</span
          >
        </div>

        <div class="head">
          <div class="title" :class="{ win: isWin, lose: !isWin }">
            <template v-if="data?.game === 'ddz'">
              {{ data?.landlordWin ? '地主胜利' : '农民胜利' }}
            </template>
            <template v-else>
              {{ winnerName }} 获胜
            </template>
          </div>
          <div class="sub">{{ isWin ? '恭喜你赢得本局' : '再接再厉' }}</div>
        </div>

        <div class="results">
          <div
            v-for="r in data?.results"
            :key="r.seat"
            class="row"
            :class="{ win: r.win, me: r.seat === store.room?.mySeat }"
          >
            <span class="nm">
              {{ r.name }}
              <span class="tag" v-if="r.isLandlord">地主</span>
              <span class="tag dealer" v-if="r.isDealer">庄</span>
            </span>
            <span class="extra" v-if="r.niuName">{{ r.niuName }}</span>
            <span class="delta" :class="{ pos: r.delta > 0, neg: r.delta < 0 }">
              {{ r.delta > 0 ? '+' : '' }}{{ r.delta }}
            </span>
            <span class="chips">🪙 {{ r.chips }}</span>
          </div>
        </div>

        <div class="footer">
          <button v-if="store.isOwner" class="btn btn-gold" @click="again">再来一局</button>
          <button class="btn btn-ghost" @click="close">关闭</button>
        </div>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.mask {
  position: fixed;
  inset: 0;
  z-index: 200;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(3, 12, 8, 0.72);
  backdrop-filter: blur(6px);
  padding: 1rem;
}
.card {
  position: relative;
  width: min(440px, 92vw);
  border-radius: 20px;
  padding: 1.8rem 1.6rem 1.4rem;
  text-align: center;
  overflow: hidden;
  box-shadow: 0 30px 80px rgba(0, 0, 0, 0.6);
}
.coin-rain {
  position: absolute;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
}
.coin {
  position: absolute;
  top: -30px;
  animation-name: coinFall;
  animation-timing-function: linear;
  animation-iteration-count: infinite;
}
.head {
  margin-bottom: 1rem;
}
.title {
  font-family: var(--font-zh);
  font-weight: 900;
  font-size: 2rem;
  letter-spacing: 0.1em;
}
.title.win {
  color: var(--gold);
  text-shadow: 0 0 24px var(--gold-glow);
}
.title.lose {
  color: var(--wine-2);
}
.sub {
  color: var(--ivory-dim);
  font-size: 0.88rem;
  margin-top: 0.3rem;
}
.results {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  margin: 1rem 0;
}
.row {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.55rem 0.8rem;
  border-radius: 10px;
  background: rgba(7, 32, 24, 0.5);
  border: 1px solid rgba(212, 175, 55, 0.15);
}
.row.win {
  border-color: rgba(212, 175, 55, 0.5);
  background: rgba(212, 175, 55, 0.08);
}
.row.me {
  box-shadow: inset 0 0 0 1px var(--gold);
}
.nm {
  flex: 1;
  text-align: left;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 0.3rem;
}
.tag {
  font-size: 0.62rem;
  padding: 0 0.3rem;
  border-radius: 4px;
  background: var(--wine);
  color: var(--ivory);
}
.tag.dealer {
  background: var(--gold);
  color: var(--ink);
}
.extra {
  color: var(--gold-soft);
  font-size: 0.82rem;
}
.delta {
  font-weight: 700;
  font-size: 1.05rem;
  min-width: 50px;
}
.delta.pos {
  color: #6ee7a8;
}
.delta.neg {
  color: #ff9b9b;
}
.chips {
  color: var(--ivory-dim);
  font-size: 0.8rem;
  min-width: 60px;
  text-align: right;
}
.footer {
  display: flex;
  gap: 0.6rem;
  justify-content: center;
  margin-top: 0.6rem;
}
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.25s ease;
}
.modal-enter-active .card,
.modal-leave-active .card {
  transition: transform 0.3s cubic-bezier(0.2, 0.9, 0.3, 1.2);
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
.modal-enter-from .card,
.modal-leave-to .card {
  transform: scale(0.85) translateY(20px);
}
</style>
