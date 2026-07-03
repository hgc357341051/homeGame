<script setup lang="ts">
import { computed } from 'vue'
import type { Card } from '@/types'
import { isRedSuit } from '@/types'

const props = withDefaults(
  defineProps<{
    card?: Card | null
    faceDown?: boolean
    selected?: boolean
    size?: 'xs' | 'sm' | 'md' | 'lg'
    dim?: boolean
    highlight?: boolean
  }>(),
  { size: 'md' },
)

const isJoker = computed(() => props.card && (props.card.rank === '小王' || props.card.rank === '大王'))
const isBigJoker = computed(() => props.card?.rank === '大王')
const red = computed(() => (props.card ? isRedSuit(props.card.suit) || props.card.rank === '大王' : false))

const sizeMap = {
  xs: { w: 34, h: 48, fs: 13, big: 18 },
  sm: { w: 44, h: 62, fs: 15, big: 22 },
  md: { w: 58, h: 82, fs: 18, big: 30 },
  lg: { w: 70, h: 100, fs: 22, big: 38 },
}
const s = computed(() => sizeMap[props.size])

const rankDisplay = computed(() => {
  if (!props.card) return ''
  if (props.card.rank === '小王') return '小王'
  if (props.card.rank === '大王') return '大王'
  return props.card.rank === '10' ? '10' : props.card.rank
})
</script>

<template>
  <div
    class="pc"
    :class="{ selected, dim, highlight, 'face-down': faceDown }"
    :style="{ width: s.w + 'px', height: s.h + 'px' }"
  >
    <!-- 卡背 -->
    <div v-if="faceDown" class="back">
      <div class="back-pattern" />
      <div class="back-emblem">♠</div>
    </div>

    <!-- 卡面 -->
    <div v-else-if="card" class="face" :class="{ red }">
      <template v-if="isJoker">
        <div class="corner tl">
          <span class="rk">{{ isBigJoker ? '大' : '小' }}</span>
          <span class="st">JOKER</span>
        </div>
        <div class="center joker" :class="{ big: isBigJoker }">
          {{ isBigJoker ? '★' : '☆' }}
        </div>
        <div class="corner br">
          <span class="rk">{{ isBigJoker ? '大' : '小' }}</span>
          <span class="st">JOKER</span>
        </div>
      </template>
      <template v-else>
        <div class="corner tl">
          <span class="rk">{{ rankDisplay }}</span>
          <span class="st">{{ card.suit }}</span>
        </div>
        <div class="center">{{ card.suit }}</div>
        <div class="corner br">
          <span class="rk">{{ rankDisplay }}</span>
          <span class="st">{{ card.suit }}</span>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.pc {
  position: relative;
  border-radius: 7px;
  flex-shrink: 0;
  cursor: default;
  transition: transform 0.18s cubic-bezier(0.2, 0.9, 0.3, 1.2), box-shadow 0.2s ease;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.45), 0 1px 0 rgba(255, 255, 255, 0.05) inset;
  user-select: none;
}
.face,
.back {
  position: absolute;
  inset: 0;
  border-radius: 7px;
  overflow: hidden;
}
.face {
  background: linear-gradient(160deg, #fbf9f0, #f1ecdd 60%, #e6dfca);
  border: 1px solid rgba(0, 0, 0, 0.12);
  color: #1a1a1a;
  display: flex;
  align-items: center;
  justify-content: center;
}
.face.red {
  color: #c0392b;
}

.corner {
  position: absolute;
  display: flex;
  flex-direction: column;
  align-items: center;
  line-height: 1;
  font-family: var(--font-card);
}
.corner.tl {
  top: 4px;
  left: 4px;
}
.corner.br {
  bottom: 4px;
  right: 4px;
  transform: rotate(180deg);
}
.rk {
  font-size: v-bind('s.fs + "px"');
  font-weight: 700;
}
.st {
  font-size: v-bind('Math.max(9, s.fs - 5) + "px"');
  opacity: 0.85;
}
.center {
  font-size: v-bind('s.big + "px"');
  font-family: var(--font-card);
  opacity: 0.92;
}
.center.joker {
  font-size: v-bind('(s.big - 4) + "px"');
  color: #1a1a1a;
}
.center.joker.big {
  color: #c0392b;
}
.corner .st:empty {
  display: none;
}

/* 卡背 */
.back {
  background:
    repeating-linear-gradient(45deg, rgba(212, 175, 55, 0.18) 0 4px, transparent 4px 8px),
    linear-gradient(135deg, #14503c, #0b3d2e 60%, #06241a);
  border: 1px solid rgba(212, 175, 55, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
}
.back-pattern {
  position: absolute;
  inset: 4px;
  border: 1px solid rgba(212, 175, 55, 0.35);
  border-radius: 5px;
}
.back-emblem {
  font-size: v-bind('(s.big - 6) + "px"');
  color: var(--gold);
  text-shadow: 0 0 8px rgba(212, 175, 55, 0.6);
}

/* 状态 */
.selected {
  transform: translateY(-18px);
  box-shadow: 0 10px 24px rgba(212, 175, 55, 0.4), 0 0 0 2px var(--gold);
}
.dim {
  opacity: 0.45;
  filter: grayscale(0.5);
}
.highlight {
  box-shadow: 0 0 0 2px var(--gold), 0 8px 20px rgba(212, 175, 55, 0.5);
}
</style>
