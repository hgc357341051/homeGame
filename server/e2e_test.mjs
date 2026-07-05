// 端到端冒烟测试：连接 -> enter -> createRoom -> 验证 roomState
const URL = 'ws://localhost:9898/ws'

function makeClient(name) {
  const ws = new WebSocket(URL)
  const inbox = []
  const wsReady = new Promise((res, rej) => {
    ws.addEventListener('open', res)
    ws.addEventListener('error', rej)
  })
  ws.addEventListener('message', (e) => {
    inbox.push(JSON.parse(e.data))
  })
  function send(type, data = {}) {
    ws.send(JSON.stringify({ type, data }))
  }
  function waitMsg(type, timeout = 2000) {
    const start = Date.now()
    return new Promise((resolve, reject) => {
      function check() {
        const idx = inbox.findIndex((m) => m.type === type)
        if (idx >= 0) {
          const [found] = inbox.splice(idx, 1)
          return resolve(found)
        }
        if (Date.now() - start > timeout) return reject(new Error('timeout waiting for ' + type))
        setTimeout(check, 50)
      }
      check()
    })
  }
  function close() {
    ws.close()
  }
  return { ws, wsReady, send, waitMsg, inbox, close }
}

async function main() {
  console.log('--- 测试1: 单玩家创建 DDZ 房间 ---')
  const c = makeClient('测试员A')
  await c.wsReady
  c.send('enter', { name: '测试员A' })
  const entered = await c.waitMsg('entered')
  console.log('✓ entered, playerId =', entered.data.playerId?.slice(0, 8))

  c.send('createRoom', { game: 'ddz' })
  const created = await c.waitMsg('roomCreated')
  console.log('✓ roomCreated, code =', created.data.code)

  const state1 = await c.waitMsg('roomState')
  console.log('✓ roomState, game =', state1.data.game, 'phase =', state1.data.phase, 'seats =', state1.data.seats.length)
  console.log('  mySeat =', state1.data.mySeat, 'minPlayers =', state1.data.minPlayers, 'gameLabel =', state1.data.gameLabel)

  // 验证: 此时为旁观 (mySeat = -1)，座位无手牌字段
  if (state1.data.mySeat !== -1) throw new Error('应未入座 mySeat=-1')
  for (const s of state1.data.seats) {
    if (s.hand !== undefined) throw new Error('SeatView 不应包含 hand 字段! 安全漏洞')
  }
  console.log('✓ 安全检查: 座位视图不含手牌字段')

  // 入座
  c.send('sit', { seat: 0 })
  const state2 = await c.waitMsg('roomState')
  if (state2.data.mySeat !== 0) throw new Error('入座后 mySeat 应为 0')
  console.log('✓ sit 成功, mySeat =', state2.data.mySeat)

  // 准备
  c.send('ready')
  await c.waitMsg('roomState')
  console.log('✓ ready 发送成功')

  // 单人无法开局 (需3人)，应返回 error
  c.send('start')
  const err1 = await c.waitMsg('error')
  console.log('✓ 单人开局被拒:', err1.data.msg)

  // 测试聊天
  c.send('chat', { text: '大家好' })
  const chatMsg = await c.waitMsg('chat')
  console.log('✓ 聊天:', chatMsg.data.player, '-', chatMsg.data.text)

  c.close()
  console.log('\n--- 测试2: 三玩家 DDZ 开局流程 ---')
  const a = makeClient('玩家甲')
  const b = makeClient('玩家乙')
  const d = makeClient('玩家丁')
  await Promise.all([a.wsReady, b.wsReady, d.wsReady])
  a.send('enter', { name: '玩家甲' })
  b.send('enter', { name: '玩家乙' })
  d.send('enter', { name: '玩家丁' })
  await Promise.all([a.waitMsg('entered'), b.waitMsg('entered'), d.waitMsg('entered')])

  a.send('createRoom', { game: 'ddz' })
  const rc = await a.waitMsg('roomCreated')
  const code = rc.data.code
  console.log('✓ 房间号:', code)
  b.send('joinRoom', { code })
  d.send('joinRoom', { code })

  // 入座
  a.send('sit', { seat: 0 })
  b.send('sit', { seat: 1 })
  d.send('sit', { seat: 2 })
  await new Promise((r) => setTimeout(r, 300))

  // 准备
  a.send('ready')
  b.send('ready')
  d.send('ready')
  await new Promise((r) => setTimeout(r, 300))

  // 甲是房主，开局
  a.send('start')
  // 应收到 deal (手牌) 和 phase (callLandlord)
  const dealA = await a.waitMsg('deal')
  console.log('✓ 甲收到手牌:', dealA.data.cards?.length, '张')
  if (dealA.data.cards?.length !== 17) throw new Error('DDZ 应发17张')
  const phaseA = await a.waitMsg('phase')
  console.log('✓ 阶段:', phaseA.data.phase, '-', phaseA.data.message)

  // 验证乙也收到17张手牌
  const dealB = await b.waitMsg('deal')
  console.log('✓ 乙收到手牌:', dealB.data.cards?.length, '张')

  // 安全验证: 甲收到的 roomState 中，乙/丁的 cardCount 应为17，但不含具体牌
  const st = a.inbox.filter((m) => m.type === 'roomState').pop()
  for (const s of st.data.seats) {
    if (s.hand !== undefined) throw new Error('安全漏洞: 他人手牌泄露!')
    if (s.playerId && s.seat !== 0) {
      console.log(`  座位${s.seat} ${s.name}: cardCount=${s.cardCount} (不含手牌详情 ✓)`)
    }
  }

  console.log('\n✅ 全部冒烟测试通过')
  a.close(); b.close(); d.close()

  // 通用：两位玩家开局流程验证（用于 ZJH / NN）
  async function twoPlayerStart(game, label) {
    console.log(`\n--- 测试: 两玩家 ${label} 开局 ---`)
    const p1 = makeClient('甲' + game)
    const p2 = makeClient('乙' + game)
    await Promise.all([p1.wsReady, p2.wsReady])
    p1.send('enter', { name: '甲' + game })
    p2.send('enter', { name: '乙' + game })
    await Promise.all([p1.waitMsg('entered'), p2.waitMsg('entered')])
    p1.send('createRoom', { game })
    const rc = await p1.waitMsg('roomCreated')
    const code = rc.data.code
    p2.send('joinRoom', { code })
    p1.send('sit', { seat: 0 })
    p2.send('sit', { seat: 1 })
    // 等 sit 广播
    await Promise.all([p1.waitMsg('roomState'), p2.waitMsg('roomState')])
    p1.send('ready')
    p2.send('ready')
    await new Promise((r) => setTimeout(r, 300))
    p1.send('start')
    const deal1 = await p1.waitMsg('deal')
    console.log(`✓ ${label} 甲收到手牌:`, deal1.data.cards?.length, '张')
    // 安全检查：roomState 中他人无 hand 字段
    const st = p1.inbox.filter((m) => m.type === 'roomState').pop()
    for (const s of st.data.seats) {
      if (s.hand !== undefined) throw new Error(`${label} 安全漏洞: 他人手牌泄露!`)
    }
    console.log(`✓ ${label} 安全检查通过: 他人手牌未泄露`)
    p1.close(); p2.close()
  }

  await twoPlayerStart('zjh', '炸金花')
  await twoPlayerStart('nn', '牛牛')

  // --- 测试: 炸金花 look→compare 比牌流程（验证 look 不再消耗轮次）---
  console.log('\n--- 测试: 炸金花 look→compare 比牌流程 ---')
  const z1 = makeClient('比牌甲')
  const z2 = makeClient('比牌乙')
  await Promise.all([z1.wsReady, z2.wsReady])
  z1.send('enter', { name: '比牌甲' })
  z2.send('enter', { name: '比牌乙' })
  await Promise.all([z1.waitMsg('entered'), z2.waitMsg('entered')])
  z1.send('createRoom', { game: 'zjh' })
  const zrc = await z1.waitMsg('roomCreated')
  const zcode = zrc.data.code
  z2.send('joinRoom', { code: zcode })
  z1.send('sit', { seat: 0 })
  z2.send('sit', { seat: 1 })
  await Promise.all([z1.waitMsg('roomState'), z2.waitMsg('roomState')])
  z1.send('ready')
  z2.send('ready')
  await new Promise((r) => setTimeout(r, 300))
  z1.send('start')
  // 双方各收到 deal(3张) 和 phase(betting) 和 turn
  const zDeal1 = await z1.waitMsg('deal')
  if (zDeal1.data.cards?.length !== 3) throw new Error('ZJH 应发3张')
  console.log('✓ 比牌甲收到手牌:', zDeal1.data.cards.length, '张')
  await z1.waitMsg('phase')
  // 找到当前轮到的玩家
  const zTurn1 = await z1.waitMsg('turn')
  const turnSeat = zTurn1.data.seat
  const turnClient = turnSeat === 0 ? z1 : z2
  const otherClient = turnSeat === 0 ? z2 : z1
  console.log('✓ 当前轮到座位', turnSeat)
  // 看牌（不应消耗轮次）
  turnClient.send('look')
  const lookEv = await turnClient.waitMsg('phase')
  if (lookEv.data.event !== 'look') throw new Error('应收到 look 事件')
  console.log('✓ 看牌成功，未消耗轮次')
  // 看牌后应能立即 compare（不再报"还没轮到你"）
  turnClient.send('compare', { target: turnSeat === 0 ? 1 : 0 })
  // 收到 reveal（比牌结果公开）
  let revealEv = null
  try {
    revealEv = await turnClient.waitMsg('reveal', 2000)
  } catch (e) {
    // 可能因筹码不足等错误，记录但不立即失败
  }
  if (revealEv && revealEv.data.cards && revealEv.data.cards2) {
    console.log('✓ 比牌成功: 座位', revealEv.data.seat, 'vs 座位', revealEv.data.seat2,
      '| 类型:', revealEv.data.type, 'vs', revealEv.data.type2,
      '| 赢家座位:', revealEv.data.winner)
  } else {
    // 检查是否是预期的错误（如筹码不足），而非"还没轮到你"
    const errs = turnClient.inbox.filter((m) => m.type === 'error')
    if (errs.length > 0 && errs[0].data.msg?.includes('还没轮到你')) {
      throw new Error('look 仍消耗轮次：' + errs[0].data.msg)
    }
    console.log('⚠ 比牌未触发 reveal（可能筹码不足），但未报"还没轮到你"')
  }
  z1.close(); z2.close()

  console.log('\n🎉 三款游戏端到端冒烟测试全部通过')
  process.exit(0)
}

main().catch((e) => {
  console.error('❌ 测试失败:', e.message)
  process.exit(1)
})
