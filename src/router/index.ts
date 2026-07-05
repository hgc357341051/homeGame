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

// 离开房间路由时清理 store 状态（浏览器后退、地址栏跳转、房间间直跳等场景）
// 防止 joinedCode 残留导致重连后被强制拉回旧房间，以及后台状态累积
router.beforeEach((to, from) => {
  // 从房间页离开，或从一个房间直接跳到另一个房间，都需要清理旧房间
  const leavingRoom = from.name === 'room' && to.name !== 'room'
  const switchingRoom = from.name === 'room' && to.name === 'room' && from.params.code !== to.params.code
  if (leavingRoom || switchingRoom) {
    const store = useGameStore()
    // 仅发 leave 通知服务端释放座位，不跳转（导航已在进行中）
    store.send('leave', {})
    store.cleanupRoom()
  }
})

export default router
