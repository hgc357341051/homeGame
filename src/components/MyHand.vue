<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Card } from '@/types'
import { cardKey } from '@/types'
import PlayingCard from './PlayingCard.vue'

const props = withDefaults(
  defineProps<{
    cards: Card[]
    selectable?: boolean
    maxSelect?: number // 0 = 无限制
    size?: 'sm' | 'md' | 'lg'
  }>(),
  { selectable: false, maxSelect: 0, size: 'md' },
)

const emit = defineEmits<{ (e: 'change', selected: Card[]): void }>()
const selectedKeys = ref<Set<string>>(new Set())

watch(
  () => props.cards,
  () => selectedKeys.value.clear(),
)

// 根据牌数与尺寸动态计算重叠间距，避免大量牌时溢出
const overlapPx = computed(() => {
  const n = props.cards.length
  const sm = props.size === 'sm'
  if (n <= 5) return sm ? -18 : -26
  if (n <= 10) return sm ? -26 : -34
  return sm ? -30 : -42
})

function toggle(c: Card) {
  if (!props.selectable) return
  const k = cardKey(c)
  if (selectedKeys.value.has(k)) {
    selectedKeys.value.delete(k)
  } else {
    if (props.maxSelect > 0 && selectedKeys.value.size >= props.maxSelect) {
      // 达到上限：替换最早选择的（牛牛选 3 张场景）
      const first = selectedKeys.value.values().next().value
      if (first) selectedKeys.value.delete(first)
    }
    selectedKeys.value.add(k)
  }
  selectedKeys.value = new Set(selectedKeys.value)
  emit(
    'change',
    props.cards.filter((c) => selectedKeys.value.has(cardKey(c))),
  )
}

function isSelected(c: Card) {
  return selectedKeys.value.has(cardKey(c))
}

const selectedCount = computed(() => selectedKeys.value.size)
defineExpose({
  clear: () => {
    selectedKeys.value.clear()
    selectedKeys.value = new Set(selectedKeys.value)
    emit('change', [])
  },
  selectedCount,
})
</script>

<template>
  <div class="hand" :class="{ selectable }">
    <div class="cards">
      <div
        v-for="(c, i) in cards"
        :key="cardKey(c) + i"
        class="slot"
        :class="{ sel: isSelected(c) }"
        :style="{ zIndex: i, marginLeft: i === 0 ? '0' : overlapPx + 'px' }"
        @click="toggle(c)"
      >
        <PlayingCard :card="c" :selected="isSelected(c)" :size="size" />
      </div>
      <div v-if="cards.length === 0" class="empty-hand">等待发牌…</div>
    </div>
  </div>
</template>

<style scoped>
.hand {
  width: 100%;
  display: flex;
  justify-content: center;
  align-items: flex-end;
  padding: 0 8px;
}
.cards {
  display: flex;
  align-items: flex-end;
  position: relative;
  min-height: 96px;
}
.slot {
  transition: transform 0.18s ease;
  cursor: default;
}
.selectable .slot {
  cursor: pointer;
}
.selectable .slot:hover {
  transform: translateY(-8px);
}
.slot.sel {
  /* 选中上浮由 PlayingCard 内部处理 */
}
.empty-hand {
  color: var(--muted);
  font-size: 0.9rem;
  padding: 2rem;
}

@media (max-width: 768px) {
  .cards {
    min-height: 70px;
  }
}
</style>
