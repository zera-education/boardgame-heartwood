// Grow the Grove: rules engine for the proposal prototype and the balance sim.
const Grove = (() => {
  const VALUES = [
    { l: 'Z', name: 'Zealous', icon: '☀️', color: '#e0703a' },
    { l: 'E', name: 'Excellence', icon: '⭐', color: '#e0a32e' },
    { l: 'R', name: 'Resilience', icon: '🎋', color: '#c8463f' },
    { l: 'A', name: 'Authenticity', icon: '🪞', color: '#cf6f97' },
    { l: 'O', name: 'Open-mindedness', icon: '💡', color: '#4f74b0' },
    { l: 'S', name: 'Sustainability', icon: '🌍', color: '#3f9e9a' },
  ];
  // 18 face-down discoveries on rings 2-4 (54 spaces). The Squirrel is gone: the lantern does its job.
  const MIX = { seeds: 5, campfire: 4, mushroom: 4, path: 3, firefly: 2 };
  const TOKENS = {
    seeds: ['🌱', 'Seed pouch', 'Plant a sapling on an empty open space next to you. You are its gardener.'],
    campfire: ['🔥', 'Campfire', 'Everyone answers in one sentence.'],
    mushroom: ['🍄', 'Mushrooms', '+2 bonus points.'],
    path: ['🐾', 'Hidden path', 'Move to any empty space in this ring, then tell your story there.'],
    firefly: ['✨', 'Firefly', 'Keep it: once, it is your own lantern, so you can go into the Shade with no Neighbour.'],
  };
  const SEASONS = ['Planting the Seed', 'The Sprout', 'Growing into a Tree'];

  function buildHexes(R) {
    const hs = [{ q: 0, r: 0, ring: 0, region: -1 }];
    const dirs = [[1, 0], [1, -1], [0, -1], [-1, 0], [-1, 1], [0, 1]];
    for (let k = 1; k <= R; k++) {
      let q = dirs[4][0] * k, r = dirs[4][1] * k;
      for (let side = 0; side < 6; side++)
        for (let s = 0; s < k; s++) {
          hs.push({ q, r, ring: k, region: (3 - side + 6) % 6 });
          q += dirs[side][0]; r += dirs[side][1];
        }
    }
    return hs;
  }

  function rng(seed) { // small seeded PRNG so a sim run can be repeated
    let s = seed >>> 0 || 1;
    return () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296);
  }

  function newGame(names, o = {}) {
    const rand = o.rand || Math.random;
    const hexes = buildHexes(4);
    const key = h => h.q + ',' + h.r, at = {};
    hexes.forEach((h, i) => (at[key(h)] = i));
    const adj = hexes.map(h => [[1, 0], [1, -1], [0, -1], [-1, 0], [-1, 1], [0, 1]]
      .map(([dq, dr]) => at[(h.q + dq) + ',' + (h.r + dr)]).filter(i => i !== undefined));
    const spots = hexes.map((h, i) => i).filter(i => hexes[i].ring >= 2);
    for (let i = spots.length - 1; i > 0; i--) { const j = Math.floor(rand() * (i + 1)); [spots[i], spots[j]] = [spots[j], spots[i]]; }
    const tokens = {};
    let n = 0;
    for (const [k, c] of Object.entries(MIX)) for (let x = 0; x < c; x++) tokens[spots[n++]] = { kind: k, up: false };
    const g = {
      hexes, adj, tokens, trees: {}, rand, log: [],
      team: !!o.team, rootX2: !!o.rootX2, seedStart: o.seedStart !== false, oakSeeds: o.oakSeeds !== false, roundsPerSeason: o.roundsPerSeason || (names.length <= 6 ? 2 : 1),
      players: names.map((nm, i) => ({ id: i, name: nm, color: o.colors?.[i] || '#888', value: -1, pos: -1,
        sun: 0, shade: 0, pts: 0, lantern: 0, bonus: 0, fireflies: 0, regions: new Set(), gardener: 0 })),
      phase: 'plant', season: 0, turnIdx: 0, turnsTaken: 0, turn: null,
      met: new Set(), metBySeason: [], awake: false, awakeAt: 0,
      stats: { stuck: 0, shadeAlone: 0, turns: 0, shadeTold: 0, sunTold: 0 },
    };
    return g;
  }

  const roots = g => g.hexes.map((h, i) => i).filter(i => g.hexes[i].ring === 1);
  // The Roots that must become oaks: one for each value someone at the table stands for.
  const neededRoots = g => roots(g).filter(i => g.players.some(p => p.value === g.hexes[i].region));
  // The grove grows inward: a sapling takes root at the forest edge or next to a tree.
  const plantable = (g, i) => g.hexes[i].ring === 4 || g.adj[i].some(j => g.trees[j]);
  const occupant = (g, i) => g.players.find(p => p.pos === i);
  const neighbours = (g, i, except) => g.adj[i].map(j => occupant(g, j)).filter(p => p && p !== except);
  const cur = g => g.players[g.turnIdx];

  // Edge spaces a player may plant on: the empty ones in the wedge of their value
  // (or any empty edge space once that wedge is full).
  function plantSpots(g, value) {
    const edge = g.hexes.map((h, i) => i).filter(i => g.hexes[i].ring === 4 && !occupant(g, i));
    const mine = edge.filter(i => g.hexes[i].region === value);
    return mine.length ? mine : edge;
  }

  function plant(g, hex, value) {
    const p = cur(g);
    p.pos = hex; p.value = value;
    if (g.seedStart) g.trees[hex] = { stage: 1, gardener: p.id };
    g.log.push(`${p.name} plants a seed for ${VALUES[value].name}.`);
    g.turnIdx++;
    if (g.turnIdx >= g.players.length) { recordMeetings(g); startSeason(g, 1); }
  }

  function startSeason(g, s) {
    g.season = s; g.phase = 'turn'; g.turnIdx = 0; g.turnsTaken = 0; g.met = new Set();
    recordMeetings(g);
    g.log.push(`Season ${s}: ${SEASONS[s - 1]}.`);
    newTurn(g);
  }

  function newTurn(g) {
    g.turn = { steps: 3, stopped: false, teleport: false, event: null, options: null, sowing: false };
    g.stats.turns++;
    if (!reachable(g, cur(g)).size) g.stats.stuck++;
  }

  // Spaces the player can end on this turn: up to `steps` steps through empty spaces.
  // People block; trees don't. The Heartwood opens only once it has woken.
  function reachable(g, p, steps = g.turn ? g.turn.steps : 3) {
    const seen = new Map([[p.pos, 0]]), out = new Set(), q = [p.pos];
    while (q.length) {
      const i = q.shift(), d = seen.get(i);
      if (d >= steps) continue;
      for (const j of g.adj[i]) {
        if (seen.has(j) || occupant(g, j) || (g.hexes[j].ring === 0 && !g.awake)) continue;
        seen.set(j, d + 1); out.add(j); q.push(j);
      }
    }
    return out;
  }

  function step(g, j) {
    const p = cur(g), t = g.turn;
    if (t.teleport) {
      if (g.hexes[j].ring !== g.hexes[p.pos].ring || occupant(g, j)) throw new Error('the hidden path leads to an empty space in this ring');
      p.pos = j; t.teleport = false; return afterStop(g);
    }
    if (t.stopped || t.steps < 1 || !g.adj[p.pos].includes(j) || occupant(g, j)) throw new Error('step onto an empty space next to you');
    if (g.hexes[j].ring === 0 && !g.awake) throw new Error('the Heartwood is still asleep');
    p.pos = j; t.steps--;
  }

  function stop(g) {
    const p = cur(g), t = g.turn, tok = g.tokens[p.pos];
    t.stopped = true;
    if (tok && !tok.up) {
      tok.up = true; t.event = tok.kind;
      g.log.push(`${p.name} finds ${TOKENS[tok.kind][1]}.`);
      if (tok.kind === 'mushroom') p.bonus += 2;
      if (tok.kind === 'firefly') p.fireflies++;
      if (tok.kind === 'path') { t.teleport = true; return; }
      if (tok.kind === 'seeds') t.sowing = sowSpots(g, p).length > 0;
    }
    afterStop(g);
  }

  const sowSpots = (g, p) => g.adj[p.pos].filter(j => g.hexes[j].ring > 0 && !g.trees[j] && !occupant(g, j));
  function sow(g, j) {
    const p = cur(g);
    if (!sowSpots(g, p).includes(j)) throw new Error('sow on an empty open space next to you');
    g.trees[j] = { stage: 1, gardener: p.id }; g.turn.sowing = false;
    afterStop(g);
  }

  function afterStop(g) {
    const p = cur(g), t = g.turn, h = g.hexes[p.pos], tree = g.trees[p.pos];
    const nb = neighbours(g, p.pos, p);
    t.options = {
      heartwood: h.ring === 0,
      sun: h.ring > 0,
      shade: h.ring > 0 && !!tree && (nb.length > 0 || p.fireflies > 0),
      shadeNeedsFirefly: !!tree && nb.length === 0 && p.fireflies > 0,
      tree: tree ? tree.stage : 0, neighbours: nb.map(x => x.id), plants: !tree && h.ring > 0 && plantable(g, p.pos),
    };
    if (tree && !t.options.shade && h.ring > 0) g.stats.shadeAlone++;
  }

  // side: 'sun' | 'shade' | 'heartwood' | 'pass'
  function tell(g, side, lanternId) {
    const p = cur(g), t = g.turn, i = p.pos, h = g.hexes[i], o = t.options;
    if (!t.stopped || t.teleport || t.sowing) throw new Error('stop first');
    if (side === 'shade' && !o.shade) throw new Error('no one goes into the Shade alone');
    if (side === 'heartwood' && !o.heartwood) throw new Error('only in the Heartwood');
    if (side === 'sun') {
      p.sun++; p.pts += h.ring === 1 && g.rootX2 ? 2 : 1; p.regions.add(h.region); g.stats.sunTold++;
      if (!g.trees[i] && plantable(g, i)) { g.trees[i] = { stage: 1, gardener: p.id }; g.log.push(`${p.name} tells a Sun story and plants a sapling.`); }
      else g.log.push(`${p.name} tells a Sun story.`);
    } else if (side === 'shade') {
      p.shade++; p.pts += h.ring === 1 && g.rootX2 ? 4 : 2; p.regions.add(h.region); g.stats.shadeTold++;
      if (o.shadeNeedsFirefly) p.fireflies--;
      const tr = g.trees[i];
      if (tr.stage === 1) {
        tr.stage = 2;
        if (tr.gardener !== p.id) g.players[tr.gardener].gardener++;
        if (g.oakSeeds) { // an oak drops a seed: a new sapling inward, on an empty open space next to it
          const c = g.adj[i].filter(j => g.hexes[j].ring > 0 && !g.trees[j]).sort((a, b) => g.hexes[a].ring - g.hexes[b].ring);
          if (c.length) g.trees[c[0]] = { stage: 1, gardener: p.id };
        }
        g.log.push(`${p.name} tells a Shade story; the sapling grows into an oak.`);
      } else g.log.push(`${p.name} tells a Shade story under an oak.`);
      const l = g.players[lanternId];
      if (l && o.neighbours.includes(l.id)) { l.lantern++; p.lantern++; g.log.push(`${l.name} holds the lantern and asks a follow-up.`); }
      if (!g.awake && neededRoots(g).every(r => g.trees[r]?.stage === 2)) {
        g.awake = true; g.awakeAt = g.season;
        g.log.push('Every Root the table stands for is an oak. The Heartwood wakes!');
      }
    } else if (side === 'heartwood') {
      p.heart = (p.heart || 0) + 1;
      g.log.push(`${p.name} tells a Heartwood story.`);
      const out = g.adj[i].find(j => !occupant(g, j));
      if (out !== undefined) p.pos = out; // step back out to a Root so the Heartwood stays free
    } else g.log.push(`${p.name} passes.`);
    endTurn(g);
  }

  function recordMeetings(g) {
    for (const p of g.players) if (p.pos >= 0)
      for (const q of neighbours(g, p.pos, p)) g.met.add([p.id, q.id].sort((a, b) => a - b).join('-'));
  }

  function endTurn(g) {
    recordMeetings(g);
    g.turnsTaken++;
    if (g.turnsTaken >= g.players.length * g.roundsPerSeason) {
      g.metBySeason.push([...g.met]);
      if (g.season >= 3) { g.phase = 'end'; g.turn = null; return; }
      g.phase = 'dusk'; g.turn = null; return;
    }
    g.turnIdx = (g.turnIdx + 1) % g.players.length;
    newTurn(g);
  }

  function nextSeason(g) { startSeason(g, g.season + 1); }

  function score(g, p) {
    const rootOak = r => g.trees[roots(g).find(i => g.hexes[i].region === r)]?.stage === 2;
    const s = {
      stories: p.pts + (p.heart || 0) * 4, // Sun 1, Shade 2, double at a Root; Heartwood 4
      lantern: p.lantern, values: p.regions.size, gardener: p.gardener, bonus: p.bonus,
      advocacy: p.value >= 0 && rootOak(p.value) ? 3 : 0, // stand-in: the real game counts every value in your alliance
      awake: g.awake ? 3 : 0,
    };
    s.total = Object.values(s).reduce((a, b) => a + b, 0);
    return s;
  }

  // ---- a simple bot, for the sim and the prototype's "let bots play" ----
  function botTurn(g) {
    const p = cur(g), r = g.rand;
    let best = p.pos, bv = -1;
    for (const d of [p.pos, ...reachable(g, p)]) {
      const v = valueOf(g, p, d) + r() * 0.6;
      if (v > bv) { bv = v; best = d; }
    }
    walkTo(g, p, best);
    stop(g);
    if (g.turn.teleport) {
      const ring = g.hexes[p.pos].ring;
      const c = g.hexes.map((h, i) => i).filter(i => g.hexes[i].ring === ring && !occupant(g, i));
      let b = p.pos, v = -1;
      for (const d of c) { const x = valueOf(g, p, d); if (x > v) { v = x; b = d; } }
      step(g, b);
    }
    if (g.turn.sowing) {
      const sp = sowSpots(g, p);
      sow(g, sp.sort((a, b) => g.hexes[a].ring - g.hexes[b].ring)[0]);
    }
    const o = g.turn.options;
    const side = o.heartwood ? 'heartwood' : o.shade ? 'shade' : 'sun';
    const lantern = o.neighbours.length ? o.neighbours[Math.floor(r() * o.neighbours.length)] : null;
    tell(g, side, lantern);
  }

  function valueOf(g, p, d) {
    const h = g.hexes[d], tree = g.trees[d];
    if (h.ring === 0) return 6;
    const nb = g.adj[d].map(j => occupant(g, j)).filter(x => x && x !== p).length;
    const newValue = p.regions.has(h.region) ? 0 : 1;
    let v;
    if (tree && (nb > 0 || p.fireflies > 0)) {
      v = (h.ring === 1 && g.rootX2 ? 4 : 2) + (nb > 0 ? 1 : 0) + newValue;
      if (h.ring === 1 && tree.stage === 1 && neededRoots(g).includes(d)) v += (p.value === h.region ? 1.5 : 0) + (g.team ? 1.5 : 0);
    } else {
      v = (h.ring === 1 && g.rootX2 ? 2 : 1) + newValue + (tree ? -0.5 : 0);
      if (!tree && plantable(g, d) && g.team) v += (4 - h.ring) * 0.4;
      if (h.ring === 1 && !tree && neededRoots(g).includes(d)) v += (p.value === h.region ? 1 : 0) + (g.team ? 1 : 0);
    }
    if (g.tokens[d] && !g.tokens[d].up) v += 0.7;
    return v;
  }

  function walkTo(g, p, target) {
    if (target === p.pos) return;
    const prev = new Map([[p.pos, -1]]), q = [p.pos];
    while (q.length) {
      const i = q.shift();
      if (i === target) break;
      for (const j of g.adj[i]) if (!prev.has(j) && !occupant(g, j) && (g.hexes[j].ring > 0 || g.awake)) { prev.set(j, i); q.push(j); }
    }
    const path = [];
    for (let i = target; i !== p.pos; i = prev.get(i)) path.unshift(i);
    for (const j of path) step(g, j);
  }

  function botPlant(g) {
    const v = Math.floor(g.rand() * 6), s = plantSpots(g, v);
    plant(g, s[Math.floor(g.rand() * s.length)], s.some(i => g.hexes[i].region === v) ? v : g.hexes[s[0]].region);
  }

  return { buildHexes, plantable, neededRoots, VALUES, TOKENS, SEASONS, MIX, newGame, rng, roots, occupant, neighbours, cur, plantSpots, plant, reachable,
    step, stop, sow, sowSpots, tell, nextSeason, score, botTurn, botPlant };
})();
if (typeof module !== 'undefined') module.exports = Grove;
