<script setup lang="ts">
import { onMounted } from 'vue'
import { useGameStore } from '@/stores/game'

const store = useGameStore()
onMounted(() => {
  store.connect().catch(() => {})
})
</script>

<template>
  <router-view v-slot="{ Component }">
    <transition name="fade" mode="out-in">
      <component :is="Component" />
    </transition>
  </router-view>

  <!-- 全局错误提示 -->
  <transition name="toast">
    <div v-if="store.errorToast" class="error-toast">
      <span>⚠</span> {{ store.errorToast }}
    </div>
  </transition>
</template>

<style scoped>
.error-toast {
  position: fixed;
  top: 24px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 9999;
  background: linear-gradient(135deg, #b8334a, #8b2635);
  color: #f4f1e8;
  padding: 0.7rem 1.4rem;
  border-radius: 12px;
  border: 1px solid rgba(244, 241, 232, 0.3);
  box-shadow: 0 12px 36px rgba(0, 0, 0, 0.5);
  font-size: 0.92rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.25s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s cubic-bezier(0.2, 0.9, 0.3, 1.2);
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(-16px);
}
</style>
