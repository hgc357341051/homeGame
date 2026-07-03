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
  if (from.name === 'room' && to.name !== 'room') {
    const store = useGameStore()
    // 仅发 leave 通知服务端释放座位，不跳转（导航已在进行中）
    store.send('leave', {})
    store.cleanupRoom()
  }
})

export default router
