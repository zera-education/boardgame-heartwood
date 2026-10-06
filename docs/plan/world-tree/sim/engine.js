// World Tree: rules engine for the proposal's Keeper-screen prototype and the balance sim.
// Cooperative: the team wakes the forest, or everyone runs out of water.
const Forest = (() => {
  const VALUES = [
    { l: 'Z', name: 'Zealous', icon: '☀️', color: '#e0703a', tagline: 'Be zealous, not jealous.' },
    { l: 'E', name: 'Excellence', icon: '⭐', color: '#e0a32e', tagline: 'Beyond expectation.' },
    { l: 'R', name: 'Resilience', icon: '🎋', color: '#c8463f', tagline: 'Anti-fragile.' },
    { l: 'A', name: 'Authenticity', icon: '🪞', color: '#cf6f97', tagline: 'Inclusive education.' },
    { l: 'O', name: 'Open-mindedness', icon: '💡', color: '#4f74b0', tagline: 'Growth mindset.' },
    { l: 'S', name: 'Sustainability', icon: '🌍', color: '#3f9e9a', tagline: 'Start with the end in mind.' },
  ];
  const STAGES = ['', 'Seeded', 'Grass', 'Shrub', 'Big Tree'];
  const STAGE_ICON = ['', '🌰', '🌱', '🌿', '🌳'];
  const WEATHER = { sun: ['☀️', 'Sun'], rain: ['🌧️', 'Rain'], fog: ['🌫️', 'Fog'] };
  const TYPES = ['', 'Reformer', 'Helper', 'Achiever', 'Individualist', 'Investigator', 'Loyalist', 'Enthusiast', 'Challenger', 'Peacemaker'];
  const ROLE_TEXT = ['',
    'When you Clear, remove 2 layers (still 1 action in rain).',
    'Water, Tend or Clear a hex next to you without standing on it.',
    'After you Sow, the hex grows straight to Grass.',
    'When you Explore, also peek at one unexplored hex next to you.',
    'See which hexes the next Forest Breath will hit; once a round, see one sector\'s next weather.',
    'Plants on your hex and next to you are safe from dead leaves.',
    'One Move a turn may be 2 hexes (not in fog, not through sealed hexes).',
    'You may enter sealed hexes, and bring one teammate from your hex when you Move.',
    'Teammates in a chain up to 2 hexes long through you can pass things to each other.'];
  const MAX_WATER = 5, START_WATER = 4, MAX_FRUIT = 2;
  const DIRS = [[1, 0], [1, -1], [0, -1], [-1, 0], [-1, 1], [0, 1]];

  function buildHexes(R) {
    const hs = [{ q: 0, r: 0, ring: 0, sector: -1 }];
    for (let k = 1; k <= R; k++) {
      let q = DIRS[4][0] * k, r = DIRS[4][1] * k;
      for (let side = 0; side < 6; side++)
        for (let s = 0; s < k; s++) {
          hs.push({ q, r, ring: k, sector: (3 - side + 6) % 6 });
          q += DIRS[side][0]; r += DIRS[side][1];
        }
    }
    return hs;
  }
  function rng(seed) { let s = seed >>> 0 || 1; return () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296); }

  // o: {rand, springs, rainRefill, breathStart, breathEvery, types:[[...] per player]}
  function newGame(names, o = {}) {
    const rand = o.rand || Math.random;
    const hexes = buildHexes(4), at = {};
    hexes.forEach((h, i) => (at[h.q + ',' + h.r] = i));
    const adj = hexes.map(h => DIRS.map(([dq, dr]) => at[(h.q + dq) + ',' + (h.r + dr)]).filter(i => i !== undefined));
    const springs = o.springs ?? 4;
    const kinds = Array(60).fill('empty');
    for (let k = 0; k < springs; k++) kinds[k] = 'spring';
    shuffle(kinds, rand);
    // Treasures hide anywhere, or only in the rings listed in o.treasureRings.
    const spots = shuffle(hexes.map((h, i) => i).filter(i => i > 0 && (!o.treasureRings || o.treasureRings.includes(hexes[i].ring))), rand);
    for (const i of spots.slice(0, 3)) kinds[i - 1] = 'treasure';
    const tiles = hexes.map((h, i) => i === 0 ? null : { kind: kinds[i - 1], up: false, stage: 0, leaves: 0, treasure: kinds[i - 1] === 'treasure', harvested: false });
    const g = {
      hexes, adj, tiles, rand, log: [], events: [],
      actionsPerTurn: o.actionsPerTurn ?? 2, exploreWide: !!o.exploreWide, treasureRings: o.treasureRings || null,
      rainRefill: o.rainRefill ?? true, breathStart: o.breathStart ?? 2, breathEvery: o.breathEvery ?? 2, breathMax: o.breathMax ?? 6,
      players: names.map((nm, i) => ({ id: i, name: nm, color: o.colors?.[i] || '#888', value: -1, pos: -1, water: START_WATER,
        fruit: 0, treasure: 0, placed: 0, types: (o.types?.[i]) || [(i % 9) + 1], reached: {} })),
      weather: [], nextWeather: [], breath: [], tide: 0, round: 1,
      phase: 'enter', turnIdx: 0, turn: null, offered: { fruit: 0, treasure: 0 }, shares: [], result: null,
      stats: { actions: 0, shares: { why: 0, ring: 0, harvest: 0, heartwood: 0 } },
    };
    for (let s = 0; s < 6; s++) g.weather[s] = drawWeather(g);
    for (let s = 0; s < 6; s++) g.nextWeather[s] = drawWeather(g);
    g.breath = rollBreath(g);
    return g;
  }
  function shuffle(a, rand) { for (let i = a.length - 1; i > 0; i--) { const j = Math.floor(rand() * (i + 1)); [a[i], a[j]] = [a[j], a[i]]; } return a; }
  const drawWeather = g => ['sun', 'rain', 'fog'][Math.floor(g.rand() * 3)];
  const breathLevel = g => Math.min(g.breathMax, g.breathStart + Math.floor(g.tide / g.breathEvery));
  // The next Forest Breath is decided ahead, so the Investigator can see it: each pick is a hex, and its
  // dead leaves land one step toward the centre (a Ring 1 hex keeps them; the World Tree never gets any).
  function rollBreath(g) {
    const out = [];
    for (let k = 0; k < breathLevel(g); k++) {
      const from = 1 + Math.floor(g.rand() * 60), h = g.hexes[from];
      const inward = g.adj[from].filter(j => g.hexes[j].ring === h.ring - 1 && j !== 0);
      out.push({ from, to: inward.length ? inward[Math.floor(g.rand() * inward.length)] : from });
    }
    return out;
  }

  const cur = g => g.players[g.turnIdx];
  const has = (p, t) => p.types.includes(t);
  const wx = (g, i) => (i === 0 ? null : g.weather[g.hexes[i].sector]);
  const sealed = (g, i) => i !== 0 && g.tiles[i].leaves >= 2;
  const here = (g, i) => g.players.filter(p => p.pos === i);

  // ---- entering: choose a value, stand on Ring 4 of its sector, say why ----
  function enterSpots(g, value) { return g.hexes.map((h, i) => i).filter(i => g.hexes[i].ring === 4 && g.hexes[i].sector === value); }
  function enter(g, value, hex) {
    const p = cur(g);
    if (g.phase !== 'enter') throw new Error('the game has started');
    if (!enterSpots(g, value).includes(hex)) throw new Error('stand on the outer ring of your value\'s sector');
    p.value = value; p.pos = hex;
    share(g, 'why', p, `Why do you stand for ${VALUES[value].name}?`, VALUES[value].tagline);
    g.log.push(`${p.name} stands for ${VALUES[value].name}.`);
    if (++g.turnIdx >= g.players.length) { g.phase = 'turn'; g.turnIdx = 0; newTurn(g); g.log.push('Round 1.'); }
  }

  function share(g, kind, p, prompt, sub) { g.shares.push({ kind, pid: p.id, prompt, sub }); g.stats.shares[kind]++; }

  function newTurn(g) { g.turn = { pid: cur(g).id, actions: g.actionsPerTurn, enthusiast: false, investigated: false }; }

  function spend(g, n) {
    if (g.phase !== 'turn') throw new Error('not now');
    if (g.turn.actions < n) throw new Error(n > 1 ? `that costs ${n} actions here` : 'no actions left');
    g.turn.actions -= n; g.stats.actions += n;
  }

  // ---- actions ----
  function canEnterHex(g, p, j) { return !sealed(g, j) || has(p, 8); }
  function moveTargets(g, p) {
    if (p.water <= 0 || g.turn.actions < 1) return { one: [], two: [] };
    const one = g.adj[p.pos].filter(j => canEnterHex(g, p, j));
    let two = [];
    if (has(p, 7) && !g.turn.enthusiast && wx(g, p.pos) !== 'fog')
      for (const m of g.adj[p.pos].filter(j => !sealed(g, j) && wx(g, j) !== 'fog'))
        for (const j of g.adj[m]) if (j !== p.pos && !sealed(g, j) && wx(g, j) !== 'fog' && !one.includes(j) && !two.includes(j)) two.push(j);
    return { one, two };
  }
  function move(g, j, bring) {
    const p = cur(g), from = p.pos;
    if (p.water <= 0) throw new Error(`${p.name} has no water and can't move until someone gives 1`);
    const { one, two } = moveTargets(g, p);
    const double = !one.includes(j);
    if (!one.includes(j) && !two.includes(j)) throw new Error('move to a hex next to you (no sealed hexes)');
    let mate = null;
    if (bring != null) {
      mate = g.players[bring];
      if (!has(p, 8) || mate.pos !== from || mate === p) throw new Error('only a Challenger brings a teammate from their hex');
    }
    spend(g, 1);
    if (double) g.turn.enthusiast = true;
    p.pos = j;
    if (mate) mate.pos = j;
    g.log.push(`${p.name} moves${mate ? ` with ${mate.name}` : ''}.`);
    for (const x of mate ? [p, mate] : [p]) deeper(g, x, from, j);
    check(g);
  }
  // Crossing into a deeper ring draws that ring's prompt card, the first time each player reaches it.
  function deeper(g, p, from, to) {
    const r = g.hexes[to].ring;
    if (r >= 1 && r < g.hexes[from].ring && !p.reached[r]) { p.reached[r] = true; share(g, 'ring', p, null, r); }
  }
  function explore(g) {
    const p = cur(g), t = g.tiles[p.pos];
    if (!t || t.up) throw new Error('nothing to explore here');
    spend(g, wx(g, p.pos) === 'fog' ? 2 : 1);
    t.up = true;
    if (g.exploreWide) for (const j of g.adj[p.pos]) if (j && !g.tiles[j].up) { g.tiles[j].up = true; g.events.push({ type: 'flip', hex: j }); }
    const what = t.kind === 'treasure' ? 'a treasure! 💎' : t.kind === 'spring' ? 'a spring 💧' : 'an empty clearing';
    g.log.push(`${p.name} explores and finds ${what}.`);
    g.events.push({ type: 'flip', hex: p.pos });
    if (has(p, 4)) {
      const peek = g.adj[p.pos].find(j => j && !g.tiles[j].up);
      if (peek !== undefined) g.events.push({ type: 'peek', pid: p.id, hex: peek, kind: g.tiles[peek].kind });
    }
  }
  function sowable(t) { return t && t.up && t.kind !== 'spring' && !t.treasure && t.stage === 0 && t.leaves < 2; }
  function sow(g) {
    const p = cur(g), t = g.tiles[p.pos];
    if (!sowable(t)) throw new Error('sow on an explored, empty hex');
    spend(g, 1);
    t.stage = has(p, 3) ? 2 : 1;
    g.log.push(`${p.name} sows a seed${t.stage === 2 ? ' and it springs up as Grass' : ''}.`);
    g.events.push({ type: 'grow', hex: p.pos });
  }
  function reach(g, p, j) { // own hex, or next to you for a Helper
    if (j === p.pos) return true;
    if (has(p, 2) && g.adj[p.pos].includes(j)) return true;
    throw new Error('stand on it (a Helper can reach a hex next to them)');
  }
  function water(g, j = cur(g).pos) {
    const p = cur(g), t = g.tiles[j];
    reach(g, p, j);
    if (!t || (t.stage !== 1 && t.stage !== 2) || t.leaves >= 2) throw new Error('water a Seeded or Grass hex');
    if (p.water < 1) throw new Error(`${p.name} has no water`);
    spend(g, 1); p.water--; t.stage++;
    g.log.push(`${p.name} waters it: ${STAGES[t.stage]}.`);
    g.events.push({ type: 'grow', hex: j });
  }
  function tend(g, j = cur(g).pos) {
    const p = cur(g), t = g.tiles[j];
    reach(g, p, j);
    if (!t || t.stage !== 3 || t.leaves >= 2) throw new Error('tend a Shrub');
    spend(g, 1); t.stage = 4;
    g.log.push(`${p.name} tends the Shrub: a Big Tree in ${VALUES[g.hexes[j].sector].name}!`);
    g.events.push({ type: 'grow', hex: j });
    check(g);
  }
  function clear(g, j) {
    const p = cur(g), t = g.tiles[j];
    if (j !== p.pos && !g.adj[p.pos].includes(j)) throw new Error('clear your hex or one next to you');
    if (!t || !t.leaves) throw new Error('no dead leaves there');
    spend(g, wx(g, j) === 'rain' && !has(p, 1) ? 2 : 1);
    t.leaves = Math.max(0, t.leaves - (has(p, 1) ? 2 : 1));
    g.log.push(`${p.name} clears dead leaves.`);
    g.events.push({ type: 'clear', hex: j });
  }
  function harvest(g) {
    const p = cur(g), t = g.tiles[p.pos];
    if (!t || t.stage !== 4 || t.leaves >= 2) throw new Error('harvest from a Big Tree');
    if (wx(g, p.pos) === 'rain') throw new Error('no harvest in the rain');
    if (t.harvested) throw new Error('this tree was harvested since the last Forest Tide');
    if (p.fruit >= MAX_FRUIT) throw new Error('you can carry 2 fruit');
    spend(g, 1); t.harvested = true; p.fruit++;
    const v = VALUES[g.hexes[p.pos].sector];
    share(g, 'harvest', p, `Tell us a story from your own life about ${v.name}.`, v.tagline);
    g.log.push(`${p.name} harvests a fruit 🍎.`);
    g.events.push({ type: 'harvest', hex: p.pos, pid: p.id });
  }
  function take(g) {
    const p = cur(g), t = g.tiles[p.pos];
    if (!t || !t.up || !t.treasure) throw new Error('no treasure here');
    if (p.treasure) throw new Error('you can carry 1 treasure');
    spend(g, 1); t.treasure = false; p.treasure = 1;
    g.log.push(`${p.name} takes the treasure 💎.`);
  }
  function drink(g) {
    const p = cur(g), t = g.tiles[p.pos];
    if (!t || !t.up || t.kind !== 'spring') throw new Error('drink at a spring');
    if (p.water >= MAX_WATER) throw new Error('already full');
    spend(g, 1); p.water = MAX_WATER;
    g.log.push(`${p.name} fills up at the spring 💧.`);
  }
  // Offering on the World Tree is free. A fruit comes with a Heartwood question.
  function offer(g, pid, item) {
    const p = g.players[pid];
    if (p.pos !== 0) throw new Error('offer on the World Tree');
    if (item === 'fruit') {
      if (!p.fruit) throw new Error('no fruit');
      p.fruit--; p.placed++; g.offered.fruit++;
      share(g, 'heartwood', p, null, 'heartwood');
      g.log.push(`${p.name} places a fruit on the World Tree.`);
    } else {
      if (!p.treasure) throw new Error('no treasure');
      p.treasure = 0; g.offered.treasure++;
      g.log.push(`${p.name} places a treasure on the World Tree (${g.offered.treasure}/3).`);
    }
    g.events.push({ type: 'offer', pid, item });
    check(g);
  }
  // Passing is free: on the same hex, or along a chain of up to 2 steps of neighbouring
  // occupied hexes that runs through a Peacemaker.
  function canPass(g, a, b) {
    if (a === b) return false;
    if (a.pos === b.pos) return true;
    const peace = g.players.filter(p => has(p, 9)).map(p => p.pos);
    if (!peace.length) return false;
    const occ = new Set(g.players.map(p => p.pos));
    const nb = i => g.adj[i].filter(j => occ.has(j));
    if (nb(a.pos).includes(b.pos)) return peace.includes(a.pos) || peace.includes(b.pos);
    for (const m of nb(a.pos)) if (nb(m).includes(b.pos) && [a.pos, m, b.pos].some(i => peace.includes(i))) return true;
    return false;
  }
  function pass(g, from, to, item) {
    const a = g.players[from], b = g.players[to];
    if (!canPass(g, a, b)) throw new Error('pass on the same hex, or along a chain through a Peacemaker');
    if (item === 'water') { if (a.water < 1 || b.water >= MAX_WATER) throw new Error('no water to pass, or they are full'); a.water--; b.water++; }
    else if (item === 'fruit') { if (!a.fruit || b.fruit >= MAX_FRUIT) throw new Error('no fruit to pass, or they carry 2'); a.fruit--; b.fruit++; }
    else { if (!a.treasure || b.treasure) throw new Error('no treasure to pass, or they carry one'); a.treasure--; b.treasure++; }
    g.log.push(`${a.name} passes ${item} to ${b.name}.`);
    g.events.push({ type: 'pass', from, to, item });
    check(g);
  }
  function investigate(g, sector) {
    const p = cur(g);
    if (!has(p, 5) || g.turn.investigated) throw new Error('the Investigator looks once a round');
    g.turn.investigated = true;
    g.events.push({ type: 'peekWeather', pid: p.id, sector, weather: g.nextWeather[sector] });
  }

  function endTurn(g) {
    if (g.phase !== 'turn') return;
    if (++g.turnIdx >= g.players.length) { forestTide(g); if (g.phase !== 'turn') return; g.turnIdx = 0; g.round++; }
    newTurn(g);
  }

  function forestTide(g) {
    g.tide++;
    const ev = { type: 'tide', weather: [], growth: [], dry: [], wet: [], leaves: [] };
    g.weather = g.nextWeather.slice();
    for (let s = 0; s < 6; s++) g.nextWeather[s] = drawWeather(g);
    ev.weather = g.weather.slice();
    g.tiles.forEach((t, i) => {
      if (!t) return;
      t.harvested = false;
      if (g.weather[g.hexes[i].sector] === 'rain' && (t.stage === 1 || t.stage === 2) && t.leaves < 2) { t.stage++; ev.growth.push(i); }
    });
    for (const p of g.players) {
      const w = wx(g, p.pos);
      if (w === 'sun' && g.tiles[p.pos].stage !== 4 && p.water > 0) { p.water--; ev.dry.push(p.id); }
      if (w === 'rain' && g.rainRefill && p.water < MAX_WATER) { p.water++; ev.wet.push(p.id); }
    }
    const safe = new Set();
    for (const p of g.players.filter(p => has(p, 6))) { safe.add(p.pos); g.adj[p.pos].forEach(j => safe.add(j)); }
    for (const b of g.breath) {
      const t = g.tiles[b.to];
      if (t.leaves >= 2) continue;
      t.leaves++;
      if (t.stage >= 1 && t.stage <= 3 && !safe.has(b.to)) t.stage--;
      ev.leaves.push(b);
    }
    g.breath = rollBreath(g);
    g.events.push(ev);
    g.log.push(`🌬️ Forest Tide ${g.tide}: ${g.weather.map(w => WEATHER[w][0]).join(' ')} · ${ev.leaves.length} drifts of dead leaves.`);
    check(g);
  }

  function goals(g) {
    const trees = [0, 1, 2, 3, 4, 5].map(s => g.tiles.some((t, i) => t && t.stage === 4 && g.hexes[i].sector === s));
    return {
      trees, allTrees: trees.every(Boolean),
      onTree: g.players.filter(p => p.pos === 0).length, allOnTree: g.players.every(p => p.pos === 0),
      fruit: g.players.filter(p => p.placed > 0).length, allFruit: g.players.every(p => p.placed > 0),
      treasure: g.offered.treasure, allTreasure: g.offered.treasure >= 3,
    };
  }
  function check(g) {
    if (g.phase !== 'turn') return;
    const o = goals(g);
    if (o.allTrees && o.allOnTree && o.allFruit && o.allTreasure) { g.phase = 'end'; g.result = 'won'; g.log.push('🌳 The forest wakes!'); g.events.push({ type: 'wake' }); }
    else if (g.players.every(p => p.water <= 0)) { g.phase = 'end'; g.result = 'lost'; g.log.push('Everyone is out of water. The forest sleeps.'); }
  }

  // ---- a team bot, for the sim and the prototype's "bot plays" ----
  function path(g, p, from, goal) { // BFS through enterable hexes; returns the hex list after `from`
    const prev = new Map([[from, -1]]), q = [from];
    while (q.length) {
      const i = q.shift();
      if (goal(i)) { const out = []; for (let k = i; k !== from; k = prev.get(k)) out.unshift(k); return out; }
      for (const j of g.adj[i]) if (!prev.has(j) && canEnterHex(g, p, j)) { prev.set(j, i); q.push(j); }
    }
    return null;
  }
  function pathAny(g, from, goal) { // ignoring dead leaves, to find what to clear
    const prev = new Map([[from, -1]]), q = [from];
    while (q.length) {
      const i = q.shift();
      if (goal(i)) { const out = []; for (let k = i; k !== from; k = prev.get(k)) out.unshift(k); return out; }
      for (const j of g.adj[i]) if (!prev.has(j)) { prev.set(j, i); q.push(j); }
    }
    return null;
  }
  function goToward(g, p, goal) {
    const route = path(g, p, p.pos, goal);
    if (route && route.length) { move(g, route[0]); return true; }
    if (route) return false;
    const r = pathAny(g, p.pos, goal); // blocked: clear the first sealed hex on the way
    if (!r) return false;
    const block = r.find(j => sealed(g, j) && !canEnterHex(g, p, j));
    if (block === r[0]) { clear(g, block); return true; }
    if (canEnterHex(g, p, r[0])) { move(g, r[0]); return true; }
    return false;
  }
  function tryDo(fn) { try { fn(); return true; } catch (e) { return false; } }

  function botAct(g) { // one action (or a free step); false when the bot has nothing useful to do
    const p = cur(g), t = g.tiles[p.pos], o = goals(g);
    // free: share water with a dry teammate on the same hex, offer on the World Tree
    for (const q of g.players) if (q !== p && q.water === 0 && p.water >= 2 && canPass(g, p, q)) { pass(g, p.id, q.id, 'water'); return true; }
    if (p.pos === 0) {
      if (p.fruit && (!p.placed || !o.allFruit)) { offer(g, p.id, 'fruit'); return true; }
      if (p.treasure) { offer(g, p.id, 'treasure'); return true; }
      for (const q of here(g, 0)) if (q !== p && !q.placed && !q.fruit && p.fruit) { pass(g, p.id, q.id, 'fruit'); return true; }
    }
    if (g.turn.actions < 1) return false;
    const mySector = p.value;
    const sectorDone = s => o.trees[s];
    const needFruit = !p.placed && !p.fruit;
    // 0. a teammate is out of water: the nearest one who can spare it walks over
    const dryMate = g.players.find(q => q !== p && q.water === 0);
    if (dryMate && p.water >= 3 && p.water > 0) {
      const d = x => (path(g, x, x.pos, i => i === dryMate.pos) || { length: 99 }).length;
      const helpers = g.players.filter(q => q !== dryMate && q.water >= 3).sort((a, b) => d(a) - d(b));
      if (helpers[0] === p && goToward(g, p, i => i === dryMate.pos)) return true;
    }
    // 1. water low: find a spring
    if (p.water <= 1) {
      if (t && t.up && t.kind === 'spring') return tryDo(() => drink(g));
      const sp = g.tiles.findIndex(x => x && x.up && x.kind === 'spring');
      if (sp > 0 && p.water > 0 && goToward(g, p, i => i === sp)) return true;
    }
    // 2. carrying treasure or my fruit: go to the World Tree
    if (p.treasure || (p.fruit && !p.placed)) return goToward(g, p, i => i === 0);
    // 3. treasure revealed and nobody carrying it: take it
    if (t && t.up && t.treasure && !p.treasure) return tryDo(() => take(g));
    // 4. grow a Big Tree in my sector (or the neediest sector)
    const target = !sectorDone(mySector) ? mySector : [0, 1, 2, 3, 4, 5].find(s => !sectorDone(s) && g.players.filter(q => q.value === s).length === 0);
    if (target !== undefined && target !== null) {
      const inT = i => i > 0 && g.hexes[i].sector === target;
      if (inT(p.pos) && t.leaves < 2) {
        if (t.stage === 3) return tryDo(() => tend(g));
        if ((t.stage === 1 || t.stage === 2) && p.water >= 2 && wx(g, p.pos) !== 'rain') return tryDo(() => water(g));
        if (sowable(t) && !g.tiles.some((x, i) => x && inT(i) && x.stage > 0 && x.leaves < 2)) return tryDo(() => sow(g));
        if (!t.up && !g.tiles.some((x, i) => x && inT(i) && (x.stage > 0 || sowable(x)) && x.leaves < 2)) return tryDo(() => explore(g));
      }
      const growing = i => inT(i) && g.tiles[i].stage > 0 && g.tiles[i].stage < 4 && g.tiles[i].leaves < 2;
      const waiting = growing(p.pos) && (wx(g, p.pos) === 'rain' || p.water < 2); // the rain grows it, or I need water first
      if (!waiting) {
        const goal = g.tiles.some((x, i) => growing(i)) ? growing
          : g.tiles.some((x, i) => inT(i) && sowable(x)) ? (i => inT(i) && sowable(g.tiles[i]))
          : (i => inT(i) && !g.tiles[i].up && g.tiles[i].leaves < 2);
        if (!goal(p.pos) && goToward(g, p, goal)) return true;
      }
    }
    // 5. fruit: harvest at a Big Tree in sun or fog
    if (needFruit || (p.placed && !o.allFruit && p.fruit < 1)) {
      const ripe = i => i > 0 && g.tiles[i].stage === 4 && g.tiles[i].leaves < 2 && !g.tiles[i].harvested && wx(g, i) !== 'rain';
      if (ripe(p.pos)) return tryDo(() => harvest(g));
      const anyTree = i => i > 0 && g.tiles[i].stage === 4 && g.tiles[i].leaves < 2;
      if (goToward(g, p, g.tiles.some((x, i) => ripe(i)) ? ripe : anyTree)) return true;
    }
    // 6. treasures still hidden: explore
    const hidden = g.tiles.filter(x => x && x.kind === 'treasure' && !x.up).length;
    const loose = g.tiles.findIndex(x => x && x.up && x.treasure);
    const carried = g.players.filter(q => q.treasure).length;
    if (loose > 0 && !p.treasure) return goToward(g, p, i => i === loose);
    if (hidden && g.offered.treasure + carried < 3) {
      if (t && !t.up) return tryDo(() => explore(g));
      const claimed = new Set(g.players.filter(q => q !== p).map(q => q.pos));
      const open = i => i > 0 && !g.tiles[i].up && !claimed.has(i);
      const likely = i => open(i) && (!g.treasureRings || g.treasureRings.includes(g.hexes[i].ring));
      return goToward(g, p, g.tiles.some((x, i) => likely(i)) ? likely : open);
    }
    // 7. everything done for me: gather on the World Tree
    if (p.pos !== 0) return goToward(g, p, i => i === 0);
    return false;
  }
  function botTurn(g) {
    let guard = 0;
    while (g.phase === 'turn' && g.turn.pid === cur(g).id && guard++ < 12) {
      const before = g.turn.actions;
      let ok = false;
      try { ok = botAct(g); } catch (e) { ok = false; }
      if (!ok) break;
      if (g.turn.actions === before && guard > 8) break;
    }
    if (g.phase === 'turn') endTurn(g);
  }
  function botEnter(g, value) {
    const v = value ?? (g.turnIdx % 6), s = enterSpots(g, v);
    enter(g, v, s[Math.floor(g.rand() * s.length)]);
  }

  return { VALUES, STAGES, STAGE_ICON, WEATHER, TYPES, ROLE_TEXT, MAX_WATER, buildHexes, rng, newGame, cur, has, wx, sealed, here,
    enterSpots, enter, moveTargets, move, explore, sowable, sow, water, tend, clear, harvest, take, drink, offer, canPass, pass,
    investigate, endTurn, goals, breathLevel, botAct, botTurn, botEnter };
})();
if (typeof module !== 'undefined') module.exports = Forest;
