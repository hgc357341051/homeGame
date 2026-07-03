<script setup lang="ts">
import { ref, nextTick, watch, computed, onMounted } from 'vue'
import { useGameStore } from '@/stores/game'

const store = useGameStore()
const text = ref('')
const listRef = ref<HTMLElement>()

const quick = ['打到我了！', '稳住能赢', '再来一局', '好牌！', '我要炸了', '让我想想']

// 发送防抖：避免按住回车刷屏
let lastSent = 0
function canSend() {
  const now = Date.now()
  if (now - lastSent < 300) return false
  lastSent = now
  return true
}
function send() {
  const t = text.value.trim()
  if (!t || !canSend()) return
  store.send('chat', { text: t })
  text.value = ''
}
function sendQuick(q: string) {
  if (!canSend()) return
  store.send('chat', { text: q })
}

const items = computed(() => {
  // 合并聊天与事件日志，按统一时间戳排序展示
  const chatItems = store.chat.map((c) => ({
    kind: c.system ? ('system' as const) : ('chat' as const),
    ts: c.ts, player: c.player, text: c.text
  }))
  // log 项使用存储的时间戳（与 chat 同为 Date.now() 量级），保证正确交错
  const logItems = store.log.map((l) => ({ kind: 'log' as const, ts: l.ts, player: '', text: l.text, id: l.id }))
  return [...chatItems, ...logItems].sort((a, b) => a.ts - b.ts)
})

// 挂载时若有历史消息也滚到底
onMounted(async () => {
  await nextTick()
  if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight
})

watch(
  () => items.value.length,
  async () => {
    await nextTick()
    if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight
  },
)
</script>

<template>
  <div class="chat glass">
    <div class="head">
      <span>消息 / 牌局动态</span>
    </div>
    <div class="list" ref="listRef">
      <div v-for="it in items" :key="it.kind + '-' + it.ts + '-' + (it as any).id" class="item" :class="it.kind">
        <template v-if="it.kind === 'chat'">
          <span class="who">{{ it.player }}</span>
          <span class="msg">{{ it.text }}</span>
        </template>
        <template v-else-if="it.kind === 'system'">
          <span class="sys">{{ it.text }}</span>
        </template>
        <template v-else>
          <span class="log">· {{ it.text }}</span>
        </template>
      </div>
      <div v-if="items.length === 0" class="empty">暂无消息</div>
    </div>
    <div class="quick">
      <button v-for="q in quick" :key="q" class="qbtn" @click="sendQuick(q)">{{ q }}</button>
    </div>
    <div class="input-row">
      <input
        v-model="text"
        @keydown.enter="send"
        placeholder="发个消息…"
        maxlength="200"
      />
      <button class="btn btn-gold" @click="send">发送</button>
    </div>
  </div>
</template>

<style scoped>
.chat {
  display: flex;
  flex-direction: column;
  height: 100%;
  border-radius: 16px;
  overflow: hidden;
}
.head {
  padding: 0.7rem 1rem;
  font-family: var(--font-zh);
  font-weight: 700;
  color: var(--gold-soft);
  border-bottom: 1px solid rgba(212, 175, 55, 0.2);
  font-size: 0.9rem;
}
.list {
  flex: 1;
  overflow-y: auto;
  padding: 0.6rem 0.8rem;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  min-height: 120px;
}
.item {
  font-size: 0.82rem;
  line-height: 1.4;
  word-break: break-all;
}
.item.chat .who {
  color: var(--gold-soft);
  font-weight: 600;
  margin-right: 0.35rem;
}
.item.chat .msg {
  color: var(--ivory);
}
.item.log .log {
  color: var(--muted);
  font-size: 0.76rem;
}
.item.system {
  text-align: center;
}
.item.system .sys {
  color: var(--gold-soft);
  font-size: 0.74rem;
  opacity: 0.7;
  background: rgba(212, 175, 55, 0.08);
  padding: 0.15rem 0.6rem;
  border-radius: 8px;
}
.empty {
  color: var(--muted);
  text-align: center;
  margin-top: 2rem;
  font-size: 0.8rem;
}
.quick {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
  padding: 0.4rem 0.7rem;
  border-top: 1px solid rgba(212, 175, 55, 0.12);
}
.qbtn {
  font-size: 0.7rem;
  padding: 0.2rem 0.5rem;
  border-radius: 8px;
  background: rgba(20, 80, 60, 0.4);
  border: 1px solid rgba(212, 175, 55, 0.25);
  color: var(--ivory-dim);
  cursor: pointer;
  transition: all 0.15s ease;
}
.qbtn:hover {
  border-color: var(--gold);
  color: var(--gold-soft);
}
.input-row {
  display: flex;
  gap: 0.4rem;
  padding: 0.55rem 0.7rem;
  border-top: 1px solid rgba(212, 175, 55, 0.12);
}
.input-row input {
  flex: 1;
  background: rgba(7, 32, 24, 0.6);
  border: 1px solid rgba(212, 175, 55, 0.25);
  border-radius: 9px;
  padding: 0.45rem 0.7rem;
  color: var(--ivory);
  outline: none;
  font-size: 0.85rem;
}
.input-row input:focus {
  border-color: var(--gold);
}
.input-row .btn {
  padding: 0.45rem 0.9rem;
}
</style>
