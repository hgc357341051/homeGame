<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
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

// 牌变化时（发牌/换局）清空选择并通知父组件，避免残留上一轮选择
watch(
  () => props.cards,
  () => {
    if (selectedKeys.value.size > 0) {
      selectedKeys.value = new Set()
      emit('change', [])
    }
  },
)

// 根据牌数与尺寸动态计算重叠间距，避免大量牌时溢出
const overlapPx = computed(() => {
  const n = props.cards.length
  const sm = props.size === 'sm'
  if (n <= 5) return sm ? -12 : -16
  if (n <= 10) return sm ? -18 : -24
  return sm ? -24 : -32
})

// ===== 拖拽滑动选中 =====
const dragging = ref(false)
const dragMode = ref<'select' | 'deselect'>('select')

function emitChange() {
  selectedKeys.value = new Set(selectedKeys.value)
  emit(
    'change',
    props.cards.filter((c) => selectedKeys.value.has(cardKey(c))),
  )
}

function selectCard(c: Card) {
  const k = cardKey(c)
  if (selectedKeys.value.has(k)) return
  if (props.maxSelect > 0 && selectedKeys.value.size >= props.maxSelect) {
    const first = selectedKeys.value.values().next().value
    if (first) selectedKeys.value.delete(first)
  }
  selectedKeys.value.add(k)
}

function deselectCard(c: Card) {
  selectedKeys.value.delete(cardKey(c))
}

function toggle(c: Card) {
  if (!props.selectable) return
  const k = cardKey(c)
  if (selectedKeys.value.has(k)) {
    deselectCard(c)
  } else {
    selectCard(c)
  }
  emitChange()
}

function onDragStart(c: Card, e: MouseEvent) {
  if (!props.selectable) return
  // 仅左键触发
  if (e.button !== 0) return
  e.preventDefault()
  dragging.value = true
  // 根据起始牌状态决定本次拖拽模式：选中→取消模式，未选中→选中模式
  dragMode.value = selectedKeys.value.has(cardKey(c)) ? 'deselect' : 'select'
  applyDrag(c)
}

function onDragEnter(c: Card) {
  if (!dragging.value) return
  applyDrag(c)
}

function applyDrag(c: Card) {
  if (dragMode.value === 'select') {
    selectCard(c)
  } else {
    deselectCard(c)
  }
  emitChange()
}

function stopDrag() {
  dragging.value = false
}

// 触摸滑动多选（移动端）：通过 elementFromPoint 命中牌
function onTouchStart(c: Card, e: TouchEvent) {
  if (!props.selectable) return
  dragging.value = true
  dragMode.value = selectedKeys.value.has(cardKey(c)) ? 'deselect' : 'select'
  applyDrag(c)
}

function onTouchMove(e: TouchEvent) {
  if (!dragging.value) return
  e.preventDefault()
  const t = e.touches[0]
  const el = document.elementFromPoint(t.clientX, t.clientY) as HTMLElement | null
  // 命中的元素可能是 PlayingCard 内部，向上找最近的 .slot
  const slot = el?.closest('.slot') as HTMLElement | null
  if (slot) {
    const idx = Number(slot.dataset.idx)
    if (!Number.isNaN(idx) && idx >= 0 && idx < props.cards.length) {
      applyDrag(props.cards[idx])
    }
  }
}

function onTouchEnd() {
  dragging.value = false
}

onMounted(() => {
  window.addEventListener('mouseup', stopDrag)
  window.addEventListener('touchend', onTouchEnd)
})
onUnmounted(() => {
  window.removeEventListener('mouseup', stopDrag)
  window.removeEventListener('touchend', onTouchEnd)
})

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
  <div class="hand" :class="{ selectable, dragging }">
    <div class="cards">
      <div
        v-for="(c, i) in cards"
        :key="cardKey(c) + i"
        class="slot"
        :data-idx="i"
        :class="{ sel: isSelected(c) }"
        :style="{ zIndex: i, marginLeft: i === 0 ? '0' : overlapPx + 'px' }"
        @click="toggle(c)"
        @mousedown="onDragStart(c, $event)"
        @mouseenter="onDragEnter(c)"
        @touchstart="onTouchStart(c, $event)"
        @touchmove="onTouchMove"
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
  min-height: 120px;
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
/* 拖拽时禁用 hover 上浮，避免与选中态叠加跳动 */
.selectable.dragging .slot:hover {
  transform: none;
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
    min-height: 90px;
  }
}
</style>
