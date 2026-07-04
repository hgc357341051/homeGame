// 反馈工具：音效（WebAudio 合成，无需音频文件）+ 振动 + 设置持久化
// 音效用 OscillatorNode 合成简单提示音，避免引入音频资源

import { ref } from 'vue'

const LS_SOUND = 'fc_sound'
const LS_VIBRATE = 'fc_vibrate'

// 用户偏好（默认开启音效与振动，移动端体验更佳）
export const soundEnabled = ref<boolean>(localStorage.getItem(LS_SOUND) !== 'off')
export const vibrateEnabled = ref<boolean>(localStorage.getItem(LS_VIBRATE) !== 'off')

export function setSound(on: boolean) {
  soundEnabled.value = on
  localStorage.setItem(LS_SOUND, on ? 'on' : 'off')
}
export function setVibrate(on: boolean) {
  vibrateEnabled.value = on
  localStorage.setItem(LS_VIBRATE, on ? 'on' : 'off')
}

let audioCtx: AudioContext | null = null

// 懒初始化 AudioContext（需用户交互后才能创建，否则浏览器会阻止）
function getCtx(): AudioContext | null {
  if (typeof window === 'undefined') return null
  if (!audioCtx) {
    const AC = (window.AudioContext || (window as any).webkitAudioContext)
    if (!AC) return null
    try {
      audioCtx = new AC()
    } catch {
      return null
    }
  }
  // 浏览器策略：suspend 状态需 resume
  if (audioCtx.state === 'suspended') {
    audioCtx.resume().catch(() => {})
  }
  return audioCtx
}

// 合成一个简单音效：freq 主频率，dur 时长，type 波形，gain 音量
function tone(freq: number, dur: number, type: OscillatorType = 'sine', gain = 0.15, delay = 0) {
  if (!soundEnabled.value) return
  const ctx = getCtx()
  if (!ctx) return
  const t0 = ctx.currentTime + delay
  const osc = ctx.createOscillator()
  const g = ctx.createGain()
  osc.type = type
  osc.frequency.setValueAtTime(freq, t0)
  // 包络：快速起音，指数衰减，避免爆音
  g.gain.setValueAtTime(0, t0)
  g.gain.linearRampToValueAtTime(gain, t0 + 0.01)
  g.gain.exponentialRampToValueAtTime(0.001, t0 + dur)
  osc.connect(g).connect(ctx.destination)
  osc.start(t0)
  osc.stop(t0 + dur + 0.02)
}

// 各场景音效
export function sfxTurn() {
  // 轮到你：两声升调提示，醒目
  tone(660, 0.12, 'sine', 0.18, 0)
  tone(880, 0.16, 'sine', 0.18, 0.1)
}
export function sfxPlay() {
  // 出牌：清脆短音
  tone(520, 0.08, 'triangle', 0.14)
}
export function sfxWin() {
  // 获胜：上行琶音 C-E-G-C
  tone(523, 0.12, 'sine', 0.18, 0)
  tone(659, 0.12, 'sine', 0.18, 0.1)
  tone(784, 0.12, 'sine', 0.18, 0.2)
  tone(1047, 0.25, 'sine', 0.2, 0.3)
}
export function sfxLose() {
  // 失败：下行
  tone(440, 0.15, 'sine', 0.15, 0)
  tone(330, 0.25, 'sine', 0.15, 0.12)
}
export function sfxError() {
  // 错误：低沉短促
  tone(200, 0.15, 'sawtooth', 0.12)
}
export function sfxCoin() {
  // 筹码/跟注：金属感
  tone(880, 0.06, 'square', 0.1, 0)
  tone(1100, 0.08, 'square', 0.1, 0.04)
}

// 振动反馈（移动端）
export function vibrate(pattern: number | number[]) {
  if (!vibrateEnabled.value) return
  if (typeof navigator === 'undefined' || !navigator.vibrate) return
  try {
    navigator.vibrate(pattern)
  } catch {
    /* 忽略 */
  }
}

export function vibrateTurn() {
  // 轮到你：短-长振动，醒目
  vibrate([30, 40, 80])
}
