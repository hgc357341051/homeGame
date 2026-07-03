<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useGameStore } from '@/stores/game'
import { GAME_META, type GameCode } from '@/types'

const store = useGameStore()
const name = ref(store.name)
const joinCode = ref('')
const rulesOpen = ref<GameCode | null>('ddz')

onMounted(() => {
  if (!name.value) name.value = '玩家' + Math.random().toString(36).slice(2, 8).toUpperCase()
})

function commitName() {
  store.setName(name.value.trim() || '匿名玩家')
}

function createRoom(game: GameCode) {
  commitName()
  store.connect().then(() => store.send('createRoom', { game }))
}
function joinRoom() {
  const code = joinCode.value.trim().toUpperCase()
  if (code.length !== 6) {
    return
  }
  commitName()
  store.connect().then(() => store.send('joinRoom', { code }))
}

const games = Object.keys(GAME_META) as GameCode[]

const rules: Record<GameCode, string[]> = {
  ddz: [
    '三人游戏，一副 54 张牌（含大小王）。',
    '随机叫地主，地主获得 3 张底牌，单挑两名农民。',
    '地主先出牌，按牌型大小轮流出牌或不要，谁先出完谁所在阵营胜利。',
    '出炸弹或王炸可使本局积分翻倍。',
  ],
  zjh: [
    '每人发 3 张暗牌，可看牌或闷牌下注。',
    '牌型从大到小：豹子 > 顺金 > 金花 > 顺子 > 对子 > 单张。',
    '可跟注、加注、弃牌或比牌，最后未弃牌者获胜。',
    '看牌玩家下注为闷牌玩家的两倍。',
  ],
  nn: [
    '每人发 5 张牌，庄家轮流坐庄。',
    '从 5 张中选 3 张点数和为 10 的倍数即「有牛」，剩余 2 张之和的个位为牛点。',
    '牛 7-8 倍 2，牛 9 倍 3，牛牛倍 4，五花牛/炸弹/五小牛倍数更高。',
    '闲家依次与庄家比牌，按倍数结算筹码。',
  ],
}
</script>

<template>
  <div class="page">
    <!-- 英雄区 -->
    <header class="hero">
      <div class="orbit">
        <span class="suit s1">♠</span>
        <span class="suit s2">♥</span>
        <span class="suit s3">♦</span>
        <span class="suit s4">♣</span>
      </div>
      <h1 class="title title-display gold-text">FAMILY CARDS</h1>
      <h2 class="subtitle title-zh">家庭棋牌室</h2>
      <p class="tagline">
        斗地主 · 炸金花 · 牛牛 —— 亲友聚会的私密牌桌，一码开局，全平台畅玩
      </p>
      <div class="conn">
        <span class="dot" :class="{ on: store.connected }" />
        {{ store.connected ? '已连接' : store.connecting ? '连接中…' : '未连接' }}
      </div>
    </header>

    <!-- 昵称 -->
    <section class="name-row glass">
      <label>你的昵称</label>
      <input v-model="name" @blur="commitName" @keydown.enter="commitName" maxlength="12" placeholder="输入昵称" />
    </section>

    <!-- 游戏卡片 -->
    <section class="games">
      <article
        v-for="g in games"
        :key="g"
        class="gcard glass"
        :style="{ '--accent': GAME_META[g].accent }"
        @click="createRoom(g)"
      >
        <div class="gicon">{{ GAME_META[g].icon }}</div>
        <h3 class="gname title-zh">{{ GAME_META[g].label }}</h3>
        <div class="gplayers">{{ GAME_META[g].players }}</div>
        <p class="gdesc">{{ GAME_META[g].desc }}</p>
        <button class="btn btn-gold enter">创建房间</button>
      </article>
    </section>

    <!-- 加入房间 -->
    <section class="join glass">
      <div class="join-left">
        <h3 class="title-zh">加入房间</h3>
        <p>输入房主分享的 6 位配对码，即刻入桌</p>
      </div>
      <div class="join-right">
        <input
          v-model="joinCode"
          class="code-input"
          maxlength="6"
          placeholder="配对码"
          @keydown.enter="joinRoom"
        />
        <button class="btn btn-gold" :disabled="joinCode.length !== 6" @click="joinRoom">进入</button>
      </div>
    </section>

    <!-- 规则 -->
    <section class="rules">
      <h3 class="title-zh rules-title">游戏规则</h3>
      <div class="rules-list">
        <div v-for="g in games" :key="g" class="rule-item glass">
          <button class="rule-head" @click="rulesOpen = rulesOpen === g ? null : g">
            <span>{{ GAME_META[g].icon }} {{ GAME_META[g].label }}</span>
            <span class="arrow" :class="{ open: rulesOpen === g }">▾</span>
          </button>
          <transition name="expand">
            <ul v-if="rulesOpen === g" class="rule-body">
              <li v-for="(line, i) in rules[g]" :key="i">{{ line }}</li>
            </ul>
          </transition>
        </div>
      </div>
    </section>

    <footer class="foot">
      <p>服务端权威架构 · 他人手牌绝不下发 · 从根源杜绝偷牌</p>
    </footer>
  </div>
</template>

<style scoped>
.page {
  max-width: 1100px;
  margin: 0 auto;
  padding: 2.5rem 1.2rem 4rem;
}
.hero {
  text-align: center;
  position: relative;
  padding: 1.5rem 0 2.5rem;
}
.orbit {
  position: absolute;
  inset: 0;
  pointer-events: none;
}
.suit {
  position: absolute;
  font-size: 2rem;
  color: var(--gold);
  opacity: 0.25;
  animation: floatSuit 6s ease-in-out infinite;
}
.s1 {
  top: 10%;
  left: 18%;
}
.s2 {
  top: 20%;
  right: 16%;
  color: var(--wine-2);
  animation-delay: 1s;
}
.s3 {
  bottom: 25%;
  left: 12%;
  color: var(--wine-2);
  animation-delay: 2s;
}
.s4 {
  bottom: 15%;
  right: 14%;
  animation-delay: 3s;
}
@keyframes floatSuit {
  0%,
  100% {
    transform: translateY(0) rotate(0);
  }
  50% {
    transform: translateY(-14px) rotate(8deg);
  }
}
.title {
  font-size: clamp(2.4rem, 6vw, 4.2rem);
  margin: 0;
  font-weight: 700;
}
.subtitle {
  font-size: clamp(1.6rem, 4vw, 2.6rem);
  margin: 0.3rem 0;
  color: var(--ivory);
  letter-spacing: 0.15em;
}
.tagline {
  color: var(--ivory-dim);
  max-width: 560px;
  margin: 0.6rem auto 0;
  font-size: 0.95rem;
}
.conn {
  margin-top: 1rem;
  font-size: 0.8rem;
  color: var(--ivory-dim);
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
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

.name-row {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.8rem 1.2rem;
  border-radius: 14px;
  margin-bottom: 1.5rem;
}
.name-row label {
  color: var(--gold-soft);
  font-weight: 600;
  white-space: nowrap;
}
.name-row input {
  flex: 1;
  background: rgba(7, 32, 24, 0.6);
  border: 1px solid rgba(212, 175, 55, 0.25);
  border-radius: 9px;
  padding: 0.55rem 0.9rem;
  color: var(--ivory);
  outline: none;
}
.name-row input:focus {
  border-color: var(--gold);
}

.games {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1.2rem;
  margin-bottom: 2rem;
}
.gcard {
  border-radius: 18px;
  padding: 1.6rem 1.2rem;
  text-align: center;
  cursor: pointer;
  transition: transform 0.25s ease, box-shadow 0.25s ease, border-color 0.25s ease;
  position: relative;
  overflow: hidden;
}
.gcard::before {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at 50% 0%, var(--accent), transparent 60%);
  opacity: 0.15;
  transition: opacity 0.3s ease;
}
.gcard:hover {
  transform: translateY(-6px);
  border-color: var(--accent);
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.5), 0 0 30px color-mix(in srgb, var(--accent) 40%, transparent);
}
.gcard:hover::before {
  opacity: 0.3;
}
.gicon {
  font-size: 3rem;
  filter: drop-shadow(0 4px 10px rgba(0, 0, 0, 0.5));
}
.gname {
  font-size: 1.5rem;
  margin: 0.4rem 0 0.2rem;
  color: var(--ivory);
}
.gplayers {
  font-size: 0.78rem;
  color: var(--gold-soft);
  letter-spacing: 0.1em;
  margin-bottom: 0.6rem;
}
.gdesc {
  color: var(--ivory-dim);
  font-size: 0.82rem;
  min-height: 2.6em;
  margin: 0 0 1rem;
}
.enter {
  width: 100%;
}

.join {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.3rem 1.6rem;
  border-radius: 16px;
  margin-bottom: 2rem;
  flex-wrap: wrap;
}
.join-left h3 {
  font-size: 1.3rem;
  color: var(--ivory);
  margin: 0;
}
.join-left p {
  color: var(--ivory-dim);
  font-size: 0.85rem;
  margin: 0.2rem 0 0;
}
.join-right {
  display: flex;
  gap: 0.6rem;
}
.code-input {
  width: 140px;
  text-align: center;
  letter-spacing: 0.4em;
  font-size: 1.2rem;
  font-weight: 700;
  text-transform: uppercase;
  background: rgba(7, 32, 24, 0.7);
  border: 1px solid rgba(212, 175, 55, 0.35);
  border-radius: 10px;
  padding: 0.6rem 0.8rem;
  color: var(--gold-soft);
  outline: none;
}
.code-input:focus {
  border-color: var(--gold);
  box-shadow: 0 0 14px var(--gold-glow);
}

.rules-title {
  font-size: 1.4rem;
  color: var(--ivory);
  margin: 0 0 1rem;
  text-align: center;
}
.rules-list {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  max-width: 720px;
  margin: 0 auto 2rem;
}
.rule-item {
  border-radius: 12px;
  overflow: hidden;
}
.rule-head {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.9rem 1.2rem;
  background: none;
  border: none;
  color: var(--ivory);
  font-family: var(--font-zh);
  font-weight: 700;
  font-size: 1rem;
  cursor: pointer;
}
.arrow {
  transition: transform 0.25s ease;
  color: var(--gold);
}
.arrow.open {
  transform: rotate(180deg);
}
.rule-body {
  margin: 0;
  padding: 0 1.4rem 1rem 2.2rem;
  color: var(--ivory-dim);
  font-size: 0.85rem;
  line-height: 1.7;
}
.rule-body li {
  margin: 0.2rem 0;
}
.expand-enter-active,
.expand-leave-active {
  transition: all 0.25s ease;
  overflow: hidden;
}
.expand-enter-from,
.expand-leave-to {
  opacity: 0;
  max-height: 0;
}
.expand-enter-to,
.expand-leave-from {
  opacity: 1;
  max-height: 300px;
}

.foot {
  text-align: center;
  color: var(--muted);
  font-size: 0.78rem;
  padding-top: 1rem;
  border-top: 1px solid rgba(212, 175, 55, 0.1);
}

@media (max-width: 768px) {
  .games {
    grid-template-columns: 1fr;
  }
  .page {
    padding: 1.5rem 0.9rem 3rem;
  }
  .join {
    flex-direction: column;
    align-items: stretch;
    text-align: center;
  }
  .join-right {
    justify-content: center;
  }
}
</style>
