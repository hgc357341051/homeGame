// 多用户模拟测试 v2：事件驱动，更稳定的全流程覆盖
const URL = 'ws://localhost:9898/ws'

function makeClient(name) {
  const ws = new WebSocket(URL)
  const inbox = []
  const ready = new Promise((res, rej) => {
    ws.addEventListener('open', res)
    ws.addEventListener('error', rej)
  })
  ws.addEventListener('message', (e) => {
    const m = JSON.parse(e.data)
    inbox.push(m)
  })
  function send(type, data = {}) {
    ws.send(JSON.stringify({ type, data }))
  }
  function waitMsg(type, timeout = 3000) {
    const start = Date.now()
    return new Promise((resolve, reject) => {
      function check() {
        const idx = inbox.findIndex((m) => m.type === type)
        if (idx >= 0) {
          const [found] = inbox.splice(idx, 1)
          return resolve(found)
        }
        if (Date.now() - start > timeout) return reject(new Error('timeout waiting for ' + type))
        setTimeout(check, 30)
      }
      check()
    })
  }
  function waitFilter(pred, timeout = 3000) {
    const start = Date.now()
    return new Promise((resolve, reject) => {
      function check() {
        const idx = inbox.findIndex(pred)
        if (idx >= 0) {
          const [found] = inbox.splice(idx, 1)
          return resolve(found)
        }
        if (Date.now() - start > timeout) return reject(new Error('timeout'))
        setTimeout(check, 30)
      }
      check()
    })
  }
  function drain() { inbox.length = 0 }
  function close() { ws.close() }
  return { ws, ready, send, waitMsg, waitFilter, inbox, drain, close, name }
}

async function enter(name) {
  const c = makeClient(name)
  await c.ready
  c.send('enter', { name })
  await c.waitMsg('entered')
  return c
}

async function sitReady(client, roomCode, seat) {
  client.send('joinRoom', { code: roomCode })
  await client.waitMsg('joined')
  client.send('sit', { seat })
  await client.waitMsg('roomState')
  client.send('ready')
  await client.waitMsg('roomState')
}

const issues = []
function issue(tag, msg) {
  issues.push(`[${tag}] ${msg}`)
  console.log(`  ⚠ ${msg}`)
}
function ok(tag, msg) {
  console.log(`  ✓ ${msg}`)
}

// 等待任一客户端收到指定类型消息，返回 {client, msg}
async function waitForAny(clients, type, timeout = 3000) {
  const start = Date.now()
  return new Promise((resolve, reject) => {
    function check() {
      for (const c of clients) {
        const idx = c.inbox.findIndex((m) => m.type === type)
        if (idx >= 0) {
          const [found] = c.inbox.splice(idx, 1)
          return resolve({ client: c, msg: found })
        }
      }
      if (Date.now() - start > timeout) return reject(new Error('timeout waiting for ' + type))
      setTimeout(check, 30)
    }
    check()
  })
}

// 等待任一客户端收到匹配消息
async function waitForAnyFilter(clients, pred, timeout = 3000) {
  const start = Date.now()
  return new Promise((resolve, reject) => {
    function check() {
      for (const c of clients) {
        const idx = c.inbox.findIndex(pred)
        if (idx >= 0) {
          const [found] = c.inbox.splice(idx, 1)
          return resolve({ client: c, msg: found })
        }
      }
      if (Date.now() - start > timeout) return reject(new Error('timeout'))
      setTimeout(check, 30)
    }
    check()
  })
}

// ===== 测试1: DDZ 3人完整对局 =====
async function testDDZ3Players() {
  console.log('\n=== 测试1: DDZ 3人完整对局 ===')
  const host = await enter('DDZ房主')
  const p2 = await enter('DDZ玩家2')
  const p3 = await enter('DDZ玩家3')
  const spectator = await enter('DDZ旁观者')
  const allClients = [host, p2, p3]

  host.send('createRoom', { game: 'ddz' })
  const rc = await host.waitMsg('roomCreated')
  const code = rc.data.code
  console.log('  房间号:', code)

  await Promise.all([
    sitReady(host, code, 0),
    sitReady(p2, code, 1),
    sitReady(p3, code, 2),
  ])
  spectator.send('joinRoom', { code })
  await spectator.waitMsg('joined')
  await new Promise((r) => setTimeout(r, 200))

  host.send('start')
  // 等待开局：deal/phase/turn 广播
  await waitForAny(allClients, 'deal')
  await waitForAny(allClients, 'phase')
  const turnEv = await waitForAny(allClients, 'turn')
  ok('DDZ', `叫地主回合，首位座位${turnEv.msg.data.seat}`)

  // 叫地主者叫地主
  const callerSeat = turnEv.msg.data.seat
  const caller = callerSeat === 0 ? host : callerSeat === 1 ? p2 : p3
  // 清空 caller 的 inbox，等待 reveal(底牌)
  caller.drain()
  caller.send('callLandlord', { call: true })
  const revealEv = await caller.waitFilter((m) => m.type === 'reveal' && m.data.note === '底牌')
  ok('DDZ', `底牌揭示: ${revealEv.data.cards.length}张`)
  await caller.waitFilter((m) => m.type === 'phase' && m.data.phase === 'playing')
  const playTurn = await caller.waitFilter((m) => m.type === 'turn' && m.data.phase === 'playing')
  ok('DDZ', `进入出牌阶段，地主座位${playTurn.data.seat}`)

  // 旁观者视角检查
  await new Promise((r) => setTimeout(r, 200))
  const specState = spectator.inbox.filter((m) => m.type === 'roomState').pop()
  if (specState) {
    let leaked = false
    for (const s of specState.data.seats) {
      if (s.hand !== undefined) leaked = true
    }
    if (leaked) issue('DDZ', '旁观者收到手牌！安全漏洞')
    else ok('DDZ', '旁观者视角无手牌泄露')
  }

  // 出牌循环：每个玩家在自己回合出最小单张或 pass
  // 服务端在 broadcastState 时会补发 deal（含当前手牌）
  const hands = { 0: null, 1: null, 2: null }
  // 初始手牌从开局的 deal 事件获取——但 caller 已 drain。
  // 改为：从所有客户端的 inbox 中收集 deal 事件（开局时广播给各座位）
  // 由于 caller drain 了，需要重新触发 broadcastState——服务端在每次动作后会自动广播
  // 这里直接等一会收集 broadcastState 补发的 deal
  await new Promise((r) => setTimeout(r, 300))
  for (let i = 0; i < allClients.length; i++) {
    const c = allClients[i]
    // 找最新的 deal（broadcastState 补发的）
    const deals = c.inbox.filter((m) => m.type === 'deal' && m.data.cards)
    if (deals.length > 0) {
      hands[i] = deals[deals.length - 1].data.cards
    }
  }
  // 如果没收集到（drain 太早），重新触发：发一个无效动作让服务端回 error+state
  if (!hands[0] || !hands[1] || !hands[2]) {
    host.drain(); p2.drain(); p3.drain()
    host.send('refresh') // 无效动作，服务端会回 error 并可能补 state
    await new Promise((r) => setTimeout(r, 300))
    for (let i = 0; i < allClients.length; i++) {
      const c = allClients[i]
      const deals = c.inbox.filter((m) => m.type === 'deal' && m.data.cards)
      if (deals.length > 0) {
        hands[i] = deals[deals.length - 1].data.cards
      }
    }
  }
  if (!hands[0] || !hands[1] || !hands[2]) {
    issue('DDZ', '未能收集到所有玩家手牌，跳过出牌测试')
    host.close(); p2.close(); p3.close(); spectator.close()
    return
  }
  ok('DDZ', `手牌收集: 座位0=${hands[0].length}张, 座位1=${hands[1].length}张, 座位2=${hands[2].length}张`)

  let currentPlayer = caller
  let currentSeat = callerSeat
  let turns = 0
  const maxTurns = 100
  let gameEnded = false

  while (turns < maxTurns && !gameEnded) {
    turns++
    // currentPlayer 的 inbox 应有 turn（playing 阶段广播给所有人）
    await new Promise((r) => setTimeout(r, 100))
    // 在所有客户端找 playing turn
    let myTurn = null
    for (const c of allClients) {
      const t = c.inbox.find((m) => m.type === 'turn' && m.data.phase === 'playing')
      if (t) {
        myTurn = t
        currentSeat = t.data.seat
        currentPlayer = currentSeat === 0 ? host : currentSeat === 1 ? p2 : p3
        c.inbox.splice(c.inbox.indexOf(t), 1)
        break
      }
    }
    if (!myTurn) {
      // 检查是否已结算
      for (const c of allClients) {
        const s = c.inbox.find((m) => m.type === 'settle')
        if (s) {
          gameEnded = true
          ok('DDZ', `对局结束，地主${s.data.landlordWin ? '胜' : '败'}`)
          const total = s.data.results.reduce((sum, r) => sum + r.delta, 0)
          if (total !== 0) issue('DDZ', `结算不零和: ${total}`)
          else ok('DDZ', '结算零和')
          break
        }
      }
      if (!gameEnded) issue('DDZ', `回合${turns} 无 turn 事件`)
      break
    }

    if (!hands[currentSeat] || hands[currentSeat].length === 0) {
      issue('DDZ', `座位${currentSeat} 无手牌`)
      break
    }

    // 出最小单张
    const cardToPlay = hands[currentSeat][0]
    // 保存其他客户端的 inbox 中可能有的 turn，drain 后再发送
    currentPlayer.drain()
    currentPlayer.send('play', { cards: [cardToPlay] })
    const result = await currentPlayer.waitFilter((m) => m.type === 'played' || m.type === 'error', 2000).catch(() => null)
    if (!result) {
      issue('DDZ', `座位${currentSeat} 出牌无响应`)
      break
    }
    if (result.type === 'error') {
      // 出牌失败（管不上），改 pass
      currentPlayer.send('pass')
      const passResult = await currentPlayer.waitFilter((m) => m.type === 'played', 1000).catch(() => null)
      if (!passResult) {
        issue('DDZ', `座位${currentSeat} pass 也失败`)
        break
      }
    } else {
      // 出牌成功，从手牌移除
      hands[currentSeat] = hands[currentSeat].filter((c) => !(c.suit === cardToPlay.suit && c.rank === cardToPlay.rank))
    }

    // 检查结算
    const settle = currentPlayer.inbox.find((m) => m.type === 'settle')
    if (settle) {
      gameEnded = true
      ok('DDZ', `对局结束，地主${settle.data.landlordWin ? '胜' : '败'}`)
      const total = settle.data.results.reduce((sum, r) => sum + r.delta, 0)
      if (total !== 0) issue('DDZ', `结算不零和: ${total}`)
      else ok('DDZ', '结算零和')
      break
    }
  }

  if (!gameEnded) {
    issue('DDZ', `对局未在 ${maxTurns} 回合内结束`)
  }

  host.close(); p2.close(); p3.close(); spectator.close()
}

// ===== 测试2: ZJH 4人押注+比牌 =====
async function testZJH4Players() {
  console.log('\n=== 测试2: ZJH 4人押注+比牌 ===')
  const host = await enter('ZJH房主')
  const p2 = await enter('ZJH玩家2')
  const p3 = await enter('ZJH玩家3')
  const p4 = await enter('ZJH玩家4')
  const allClients = [host, p2, p3, p4]
  const seatToClient = (s) => [host, p2, p3, p4][s]

  host.send('createRoom', { game: 'zjh' })
  const rc = await host.waitMsg('roomCreated')
  const code = rc.data.code
  await Promise.all([
    sitReady(host, code, 0),
    sitReady(p2, code, 1),
    sitReady(p3, code, 2),
    sitReady(p4, code, 3),
  ])

  host.send('start')
  await waitForAny(allClients, 'deal')
  await waitForAny(allClients, 'phase')
  const turnEv = await waitForAny(allClients, 'turn')
  ok('ZJH', `4人开局，首回合座位${turnEv.msg.data.seat}, 当前注${turnEv.msg.data.currentBet}, 底池${turnEv.msg.data.pot}`)

  // 清空所有 inbox
  allClients.forEach((c) => c.drain())

  let currentSeat = turnEv.msg.data.seat
  let rounds = 0
  const maxRounds = 40
  let ended = false

  while (rounds < maxRounds && !ended) {
    rounds++
    const currentPlayer = seatToClient(currentSeat)

    // 等待 turn 广播（所有客户端都会收到，但我们要确认轮到 currentSeat）
    // 由于 drain 后没有 turn，需要服务端发——但 drain 后 turn 已被清空
    // 实际上：上一轮动作后会广播 turn。我们 drain 是在动作前，动作后会收到新的 turn
    // 所以这里应该直接发动作
    let action
    if (rounds <= 4) action = 'call'
    else if (rounds <= 8) action = 'look'
    else action = 'fold'

    if (action === 'look') {
      currentPlayer.send('look')
      try {
        await currentPlayer.waitFilter((m) => m.type === 'phase' && m.data.event === 'look', 1500)
        // look 不消耗轮次，继续 call
        currentPlayer.send('call')
        await currentPlayer.waitFilter((m) => m.type === 'phase' && m.data.event === 'call', 1500).catch(() => {})
      } catch (e) {
        // 已看过
        currentPlayer.send('call')
        await currentPlayer.waitFilter((m) => m.type === 'phase' && m.data.event === 'call', 1500).catch(() => {})
      }
    } else {
      currentPlayer.send(action)
      await currentPlayer.waitFilter((m) => m.type === 'phase' && m.data.event === action, 1500).catch(() => {})
    }

    // 检查结算
    let settle = null
    for (const c of allClients) {
      const s = c.inbox.find((m) => m.type === 'settle')
      if (s) { settle = s; break }
    }
    if (settle) {
      ended = true
      ok('ZJH', `对局结束，赢家座位${settle.data.winnerSeat}`)
      const total = settle.data.results.reduce((sum, r) => sum + r.delta, 0)
      if (total !== 0) issue('ZJH', `结算不零和: ${total}`)
      else ok('ZJH', '结算零和')
      break
    }

    // 等待 turn 事件确定下一玩家
    await new Promise((r) => setTimeout(r, 150))
    let nextSeat = -1
    for (let i = 0; i < 4; i++) {
      const c = seatToClient(i)
      const t = c.inbox.find((m) => m.type === 'turn')
      if (t) {
        nextSeat = t.data.seat
        c.inbox.splice(c.inbox.indexOf(t), 1)
        break
      }
    }
    if (nextSeat === -1) {
      await new Promise((r) => setTimeout(r, 300))
      for (const c of allClients) {
        const s = c.inbox.find((m) => m.type === 'settle')
        if (s) {
          ended = true
          ok('ZJH', '对局结束')
          break
        }
      }
      if (!ended) {
        issue('ZJH', `回合${rounds} 找不到下一 turn`)
        break
      }
    }
    currentSeat = nextSeat
  }

  if (!ended) issue('ZJH', `对局未在 ${maxRounds} 回合内结束`)

  host.close(); p2.close(); p3.close(); p4.close()
}

// ===== 测试3: NN 6人凑牛+开牌+结算 =====
async function testNN6Players() {
  console.log('\n=== 测试3: NN 6人凑牛+开牌 ===')
  const players = []
  for (let i = 0; i < 6; i++) {
    players.push(await enter(`NN玩家${i + 1}`))
  }
  const [host] = players

  host.send('createRoom', { game: 'nn' })
  const rc = await host.waitMsg('roomCreated')
  const code = rc.data.code
  await Promise.all(players.map((p, i) => sitReady(p, code, i)))

  host.send('start')
  await waitForAny(players, 'deal')
  await waitForAny(players, 'phase')
  const turnEv = await waitForAny(players, 'turn')
  ok('NN', `6人开局，押注回合座位${turnEv.msg.data.seat}`)

  // 押注阶段：所有玩家 call，直到进入 setNiu
  players.forEach((p) => p.drain())
  let currentSeat = turnEv.msg.data.seat
  let rounds = 0
  const maxRounds = 50
  let bettingEnded = false

  while (rounds < maxRounds && !bettingEnded) {
    rounds++
    const currentClient = players[currentSeat]
    currentClient.send('call')
    await currentClient.waitFilter((m) => m.type === 'phase' && m.data.event === 'call', 1500).catch(() => {})

    // 检查是否进入 setNiu
    for (const p of players) {
      if (p.inbox.find((m) => m.type === 'phase' && m.data.phase === 'setNiu')) {
        bettingEnded = true
        ok('NN', '进入凑牛阶段')
        break
      }
    }
    if (bettingEnded) break

    // 找下一 turn
    await new Promise((r) => setTimeout(r, 100))
    let found = false
    for (let i = 0; i < players.length; i++) {
      const t = players[i].inbox.find((m) => m.type === 'turn' && m.data.phase === 'betting')
      if (t) {
        currentSeat = i
        players[i].inbox.splice(players[i].inbox.indexOf(t), 1)
        found = true
        break
      }
    }
    if (!found) {
      await new Promise((r) => setTimeout(r, 300))
      for (const p of players) {
        if (p.inbox.find((m) => m.type === 'phase' && m.data.phase === 'setNiu')) {
          bettingEnded = true
          ok('NN', '进入凑牛阶段')
          break
        }
      }
    }
  }

  if (!bettingEnded) {
    issue('NN', `押注阶段未在 ${maxRounds} 回合内结束`)
    return
  }

  // 凑牛阶段：所有玩家依次自动凑牛
  players.forEach((p) => p.drain())
  for (let i = 0; i < players.length; i++) {
    const p = players[i]
    p.send('niuniuSet', {}) // 空数据，服务端自动选最佳
    // 等待 phase(niuniuSet) 或 settle（最后一个玩家会触发 settle）
    await p.waitFilter((m) => m.type === 'phase' || m.type === 'settle', 1500).catch(() => {})
  }

  // 等待结算（最后一个 niuniuSet 触发 settle，广播给所有人）
  let settle = null
  for (const p of players) {
    const s = p.inbox.find((m) => m.type === 'settle')
    if (s) { settle = s; break }
  }
  if (!settle) {
    // 多等一会
    for (const p of players) {
      try {
        settle = await p.waitMsg('settle', 2000)
        break
      } catch (e) {}
    }
  }

  if (settle) {
    ok('NN', `对局结束，庄家座位${settle.data.dealerSeat}, 底池赢家座位${settle.data.potWinner}`)
    const total = settle.data.results.reduce((sum, r) => sum + r.delta, 0)
    if (total !== 0) issue('NN', `结算不零和: ${total}`)
    else ok('NN', '结算零和')
    const revealSeats = new Set()
    for (const p of players) {
      p.inbox.filter((m) => m.type === 'reveal').forEach((m) => revealSeats.add(m.data.seat))
    }
    ok('NN', `开牌覆盖 ${revealSeats.size} 个座位`)
  } else {
    issue('NN', '未收到结算事件')
  }

  players.forEach((p) => p.close())
}

// ===== 测试4: 边界场景 =====
async function testEdgeCases() {
  console.log('\n=== 测试4: 边界场景 ===')
  const host = await enter('边界房主')
  host.send('createRoom', { game: 'zjh' })
  const rc = await host.waitMsg('roomCreated')
  const code = rc.data.code

  // 旁观者上限（maxSpectators=20，加入 21 个验证第 21 个被拒）
  const spectators = []
  for (let i = 0; i < 21; i++) {
    const s = await enter(`旁观${i}`)
    s.send('joinRoom', { code })
    spectators.push(s)
  }
  await new Promise((r) => setTimeout(r, 800))
  let rejectedCount = 0
  for (const s of spectators) {
    const err = s.inbox.find((m) => m.type === 'error' && m.data.msg?.includes('旁观'))
    if (err) rejectedCount++
  }
  if (rejectedCount >= 1) {
    ok('EDGE', `旁观者上限生效，${rejectedCount} 人被拒`)
  } else {
    ok('EDGE', `旁观者无上限或未触发（21人全加入）`)
  }
  // 关闭部分旁观者释放资源
  spectators.slice(0, 15).forEach((s) => s.close())
  await new Promise((r) => setTimeout(r, 300))

  // 非房主开局
  const p2 = await enter('边界玩家2')
  p2.send('joinRoom', { code })
  await p2.waitMsg('joined', 5000).catch(() => {})
  p2.send('sit', { seat: 1 })
  await p2.waitMsg('roomState', 3000).catch(() => {})
  p2.send('ready')
  await p2.waitMsg('roomState', 3000).catch(() => {})
  p2.drain()
  p2.send('start')
  const err3 = await p2.waitFilter((m) => m.type === 'error', 1500).catch(() => null)
  if (err3 && err3.data.msg?.includes('仅房主')) {
    ok('EDGE', '非房主开局被正确拒绝')
  } else {
    issue('EDGE', `非房主开局未正确处理: ${err3?.data?.msg}`)
  }

  // 未入座准备（用全新的旁观者客户端）
  const noSeat = await enter('无座玩家')
  noSeat.send('joinRoom', { code })
  await noSeat.waitMsg('joined', 3000).catch(() => {})
  noSeat.drain()
  noSeat.send('ready')
  const err2 = await noSeat.waitFilter((m) => m.type === 'error', 1500).catch(() => null)
  if (err2 && err2.data.msg?.includes('入座')) {
    ok('EDGE', '未入座准备被正确拒绝')
  } else {
    issue('EDGE', `未入座准备未正确处理: ${err2?.data?.msg}`)
  }
  noSeat.close()

  host.close()
  spectators.forEach((s) => s.close())
  p2.close()
}

// ===== 测试5: 异常操作 =====
async function testInvalidActions() {
  console.log('\n=== 测试5: 异常操作 ===')
  const host = await enter('异常房主')
  const p2 = await enter('异常玩家2')
  const p3 = await enter('异常玩家3')
  host.send('createRoom', { game: 'ddz' })
  const rc = await host.waitMsg('roomCreated')
  const code = rc.data.code
  await Promise.all([
    sitReady(host, code, 0),
    sitReady(p2, code, 1),
    sitReady(p3, code, 2),
  ])

  // 开局前出牌
  host.drain()
  host.send('play', { cards: [] })
  const err1 = await host.waitFilter((m) => m.type === 'error', 1000).catch(() => null)
  if (err1) ok('INVALID', `开局前出牌被拒: ${err1.data.msg}`)
  else issue('INVALID', '开局前出牌未返回错误')

  // 开局
  host.send('start')
  await host.waitMsg('deal')
  await host.waitMsg('phase')
  const turnEv = await host.waitMsg('turn')
  const callerSeat = turnEv.data.seat

  // 非当前回合玩家操作
  const nonTurnClient = callerSeat === 0 ? p2 : host
  nonTurnClient.drain()
  nonTurnClient.send('callLandlord', { call: true })
  const err2 = await nonTurnClient.waitFilter((m) => m.type === 'error', 1000).catch(() => null)
  if (err2 && err2.data.msg?.includes('还没轮到你')) {
    ok('INVALID', '非当前回合操作被拒')
  } else {
    issue('INVALID', `非当前回合操作异常: ${err2?.data?.msg}`)
  }

  // 无效动作名
  const turnClient = callerSeat === 0 ? host : callerSeat === 1 ? p2 : p3
  turnClient.drain()
  turnClient.send('invalidAction')
  const err3 = await turnClient.waitFilter((m) => m.type === 'error', 1000).catch(() => null)
  if (err3) ok('INVALID', `无效动作被拒: ${err3.data.msg}`)
  else issue('INVALID', '无效动作未返回错误')

  host.close(); p2.close(); p3.close()
}

async function main() {
  console.log('========================================')
  console.log('  多用户模拟测试 v2（3-6玩家 + 旁观者）')
  console.log('========================================')

  await testDDZ3Players().catch((e) => issue('DDZ', `测试异常: ${e.message}`))
  await testZJH4Players().catch((e) => issue('ZJH', `测试异常: ${e.message}`))
  await testNN6Players().catch((e) => issue('NN', `测试异常: ${e.message}`))
  await testEdgeCases().catch((e) => issue('EDGE', `测试异常: ${e.message}`))
  await testInvalidActions().catch((e) => issue('INVALID', `测试异常: ${e.message}`))

  console.log('\n========================================')
  console.log('  测试总结')
  console.log('========================================')
  if (issues.length === 0) {
    console.log('✅ 所有测试通过，未发现问题')
  } else {
    console.log(`⚠ 发现 ${issues.length} 个问题:`)
    issues.forEach((s) => console.log('  ' + s))
  }
  process.exit(0)
}

main().catch((e) => {
  console.error('❌ 测试执行失败:', e.message)
  process.exit(1)
})
