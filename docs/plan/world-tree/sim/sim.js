const F = require('./engine.js');
const N = +process.argv[2] || 500, CAP = 40;
const variants = [
  ['Defaults: rain +1 water, 4 springs, Breath 2 (+1 every 2 tides)', {}],
  ['No rain refill (springs only)', { rainRefill: false }],
  ['No refill at all', { rainRefill: false, springs: 0 }],
  ['Harsher Breath: 3, +1 every tide', { breathStart: 3, breathEvery: 1 }],
  ['Gentler Breath: 1, +1 every 3 tides', { breathStart: 1, breathEvery: 3 }],
  ['Treasures only in Rings 1-2 (players know)', { treasureRings: [1, 2] }],
  ['Explore also flips the hexes next to you', { exploreWide: true }],
  ['3 actions a turn', { actionsPerTurn: 3 }],
  ['3 actions + treasures in Rings 1-2', { actionsPerTurn: 3, treasureRings: [1, 2] }],
  ['3 actions, springs only', { actionsPerTurn: 3, rainRefill: false }],
  ['3 actions, springs only, treasures in Rings 1-2', { actionsPerTurn: 3, rainRefill: false, treasureRings: [1, 2] }],
  ['3 actions, springs only, Breath 3 +1 every tide', { actionsPerTurn: 3, rainRefill: false, breathStart: 3, breathEvery: 1 }],
];
const rows = [];
for (const [label, o] of variants) {
  let done = { trees: 0, fruit: 0, treasure: 0 }, won = 0, lost = 0, stalled = 0, rounds = 0, shares = { why: 0, ring: 0, harvest: 0, heartwood: 0 }, actions = 0, minWater = 0, sealedEnd = 0;
  for (let k = 0; k < N; k++) {
    const g = F.newGame(Array.from({ length: 9 }, (_, i) => 'P' + i), { rand: F.rng(k * 131 + 7), ...o });
    while (g.phase === 'enter') F.botEnter(g);
    let lowest = 99;
    const at = {};
    while (g.phase === 'turn' && g.round <= CAP) { F.botTurn(g); lowest = Math.min(lowest, ...g.players.map(p => p.water));
      const o = F.goals(g); if (o.allTrees) at.trees ??= g.round; if (o.allFruit) at.fruit ??= g.round; if (o.allTreasure) at.treasure ??= g.round; }
    if (g.result === 'won') { done.trees += at.trees || g.round; done.fruit += at.fruit || g.round; done.treasure += at.treasure || g.round; }
    if (g.result === 'won') { won++; rounds += g.round; for (const s in shares) shares[s] += g.stats.shares[s]; actions += g.stats.actions; }
    else if (g.result === 'lost') lost++; else stalled++;
    minWater += lowest;
    sealedEnd += g.tiles.filter(t => t && t.leaves >= 2).length;
  }
  const w = Math.max(won, 1), sh = Object.fromEntries(Object.entries(shares).map(([k, v]) => [k, +(v / w).toFixed(1)]));
  const totalShares = Object.values(sh).reduce((a, b) => a + b, 0);
  rows.push({ variant: label, won: Math.round(100 * won / N) + '%', lostWater: Math.round(100 * lost / N) + '%', stalled: Math.round(100 * stalled / N) + '%',
    rounds: +(rounds / w).toFixed(1), doneBy: [done.trees, done.fruit, done.treasure].map(x => (x / w).toFixed(0)).join('/'), shares: sh, totalShares: +totalShares.toFixed(1), actions: Math.round(actions / w),
    minutes: Math.round(totalShares * 1.5 + (actions / w) * 0.25 + (rounds / w) * 1.5), sealedAtEnd: +(sealedEnd / N).toFixed(1) });
}
console.log(JSON.stringify(rows));
console.table(rows.map(r => ({ ...r, shares: Object.values(r.shares).join('/') })));
