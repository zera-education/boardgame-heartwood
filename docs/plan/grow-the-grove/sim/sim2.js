const G = require('./engine.js');
const N = 800, rows = [];
const cfgs = [['5 pouches', { seeds: 5, campfire: 4, mushroom: 4, path: 3, firefly: 2 }], ['8 pouches', { seeds: 8, campfire: 4, mushroom: 2, path: 2, firefly: 2 }]];
for (const [label, mix] of cfgs) { Object.keys(G.MIX).forEach(k => delete G.MIX[k]); Object.assign(G.MIX, mix);
 for (const [players, rps] of [[4, 2], [5, 2], [6, 1], [6, 2], [7, 1], [8, 1], [10, 1]]) for (const team of [false, true]) {
  let wake = 0, s3 = 0, spread = 0, met = 0, stuck = 0, turns = 0, shade = 0, told = 0, gard = 0;
  for (let k = 0; k < N; k++) {
    const g = G.newGame(Array.from({ length: players }, (_, i) => 'P' + i), { rand: G.rng(k * 977 + players), roundsPerSeason: rps, team });
    while (g.phase === 'plant') G.botPlant(g);
    while (g.phase !== 'end') { if (g.phase === 'dusk') G.nextSeason(g); else G.botTurn(g); }
    if (g.awake) { wake++; if (g.awakeAt === 3) s3++; }
    const t = g.players.map(p => G.score(g, p).total); spread += Math.max(...t) - Math.min(...t);
    met += g.metBySeason.reduce((a, s) => a + s.length, 0) / 3 / (players * (players - 1) / 2);
    stuck += g.stats.stuck; turns += g.stats.turns; shade += g.stats.shadeTold; told += g.stats.shadeTold + g.stats.sunTold;
    gard += g.players.reduce((a, p) => a + p.gardener, 0) / players;
  }
  rows.push({ mix: label, players, turnsEach: 3 * rps, stories: players * 3 * rps, bots: team ? 'team' : 'points', wake: Math.round(100 * wake / N) + '%', inAutumn: Math.round(100 * s3 / N) + '%',
    shade: Math.round(100 * shade / told) + '%', metPerSeason: Math.round(100 * met / N) + '%', stuck: (100 * stuck / turns).toFixed(1) + '%', gardenerPts: (gard / N).toFixed(1) });
 }}
console.log(JSON.stringify(rows));
console.table(rows);
