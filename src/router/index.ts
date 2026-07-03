import { createRouter, createWebHistory } from 'vue-router'
import HomePage from '@/pages/HomePage.vue'
import RoomPage from '@/pages/RoomPage.vue'
import { useGameStore } from '@/stores/game'

const routes = [
  {
    path: '/',
    name: 'home',
    component: HomePage,
  },
  {
    path: '/room/:code',
    name: 'room',
    component: RoomPage,
    props: true,
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// 离开房间路由时清理 store 状态（浏览器后退、地址栏跳转等场景）
// 防止 joinedCode 残留导致重连后被强制拉回旧房间，以及后台状态累积
router.beforeEach((to, from) => {
  // room → 非房间：发 leave 并清理
  if (from.name === 'room' && to.name !== 'room') {
    const store = useGameStore()
    store.send('leave', {})
    store.cleanupRoom()
  }
  // room → room 不同 code：清理旧房间状态（组件实例复用，onMounted 不再触发）
  // 由 RoomPage watch props.code 负责重新 joinRoom
  if (from.name === 'room' && to.name === 'room' && from.params.code !== to.params.code) {
    const store = useGameStore()
    store.send('leave', {})
    store.cleanupRoom()
  }
})

export default router
