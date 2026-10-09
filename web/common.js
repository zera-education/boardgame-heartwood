// Shared helpers: API calls, live updates, timers and small Heartwood lookups.
// The Keeper screen's board drawing lives in board.html; phones only use what is here.
const HW = {
  code: (new URLSearchParams(location.search).get('g') || '').toUpperCase(),
  view: null,
  clockOffset: 0,

  async post(url, body) {
    const r = await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body || {}) });
    const j = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(j.error || 'request failed');
    return j;
  },

  async fetchState(params) {
    const q = new URLSearchParams(params || {});
    const r = await fetch(`/api/games/${HW.code}/state?${q}`, { cache: 'no-store' });
    if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || 'no such game');
    return r.json();
  },

  // Go sends empty lists as null.
  normalise(v) {
    v.log ||= []; v.order ||= []; v.events ||= []; v.hexes ||= []; v.tiles ||= []; v.weather ||= [];
    v.values ||= []; v.roles ||= []; v.treasures ||= []; v.colors ||= []; v.recognition ||= [];
    v.goals ||= {}; v.goals.trees ||= []; v.goals.treasures ||= [];
    v.players = (v.players || []).map(p => ({ ...p, types: p.types || [], reached: p.reached || [] }));
    if (v.me) { v.me.peeks ||= []; v.me.breath ||= []; }
    return v;
  },

  // Applies a view unless it is older than the one on screen. Views can arrive
  // from the live socket, from an action's response and from the fallback poll,
  // so they may overlap; the version number keeps the newest.
  apply(v) {
    if (!v || v.error) return;
    if (HW.view && v.version < HW.view.version) return;
    HW.clockOffset = v.now - Date.now();
    HW.view = HW.normalise(v);
    HW.onView?.(HW.view);
  },

  // Sends an action and shows the caller's own result at once. Errors are
  // plain sentences from the server, shown as a toast.
  async act(creds, type, extra) {
    try {
      const r = await HW.post(`/api/games/${HW.code}/action`, { ...creds, type, ...extra });
      HW.apply(r.view);
      return true;
    } catch (e) { HW.toast(e.message, 'bad'); return false; }
  },

  // Live connection: one WebSocket per screen. The server pushes this viewer's
  // whole view after every change, plus a heartbeat every 20 s.
  connect(creds, onView, onFatal) {
    HW.onView = onView;
    HW.creds = creds || {};
    let ws = null, retry = null, backoff = 500, lastMsg = 0, hiddenAt = 0, downSince = Date.now(), openedAt = 0;
    const status = HW.statusBadge();

    const open = () => {
      clearTimeout(retry);
      if (ws && ws.readyState <= 1) { const old = ws; ws = null; old.close(); }
      const q = new URLSearchParams(HW.creds);
      const sock = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/games/${HW.code}/live?${q}`);
      ws = sock;
      openedAt = Date.now();
      sock.onopen = () => { if (sock !== ws) return; backoff = 500; lastMsg = Date.now(); downSince = 0; status('live'); };
      sock.onmessage = e => {
        if (sock !== ws) return;
        lastMsg = Date.now();
        const m = JSON.parse(e.data);
        if (m.ping) return;
        if (m.error) { onFatal?.(m.error); return; }
        HW.apply(m);
      };
      sock.onclose = () => {
        if (sock !== ws) return;
        ws = null;
        if (!downSince) downSince = Date.now();
        status('down');
        retry = setTimeout(open, backoff);
        backoff = Math.min(backoff * 2, 5000);
      };
    };
    HW.reconnect = (newCreds) => { if (newCreds) HW.creds = newCreds; backoff = 500; open(); };

    // A phone that slept may hold a socket that looks open but is dead:
    // after any real absence, start a fresh one.
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'hidden') { hiddenAt = Date.now(); return; }
      if (!ws || ws.readyState > 1 || Date.now() - hiddenAt > 5000) { backoff = 500; open(); }
      else ws.send('sync');
    });
    window.addEventListener('online', () => { backoff = 500; open(); });
    window.addEventListener('pageshow', e => { if (e.persisted) { backoff = 500; open(); } });

    setInterval(() => {
      // No heartbeat for 45 s: the connection is dead even if it says it's open.
      if (ws && ws.readyState === 1 && Date.now() - lastMsg > 45000) ws.close();
      // A handshake that hangs (server or Wi-Fi gone quiet) gets a fresh try.
      if (ws && ws.readyState === 0 && Date.now() - openedAt > 10000) { const old = ws; ws = null; old.close(); open(); }
      // Slow fallback while the live connection is down.
      if ((!ws || ws.readyState !== 1) && Date.now() - downSince > 4000) {
        HW.fetchState(HW.creds).then(HW.apply).catch(() => {});
      }
      status(ws && ws.readyState === 1 ? 'live' : (Date.now() - downSince > 15000 ? 'offline' : 'down'));
    }, 4000);
    open();
  },

  statusBadge() {
    let el = document.getElementById('conn');
    if (!el) { el = document.createElement('div'); el.id = 'conn'; document.body.appendChild(el); }
    let shownDown = null;
    return (state) => {
      // Wait a moment before showing "reconnecting", so a quick blip doesn't flash.
      if (state === 'live') { clearTimeout(shownDown); shownDown = null; el.className = 'live'; el.textContent = '● Live'; return; }
      if (state === 'offline') { el.className = 'offline'; el.textContent = '● Offline: check the Wi-Fi'; return; }
      if (!shownDown) shownDown = setTimeout(() => { el.className = 'down'; el.textContent = '● Reconnecting…'; }, 1500);
    };
  },

  // Replaces an element's HTML only when it changed, so a redraw doesn't
  // swallow a tap or reset a dropdown in a part of the screen that didn't change.
  patch(el, html) {
    if (!el || el._html === html) return false;
    el._html = html;
    el.innerHTML = html;
    return true;
  },

  // kind: 'ok' (green, a success) or 'bad' (red, a refusal); plain otherwise.
  toast(msg, kind) {
    let t = document.getElementById('toast');
    if (!t) { t = document.createElement('div'); t.id = 'toast'; document.body.appendChild(t); }
    t.textContent = msg; t.className = `show ${kind || ''}`;
    clearTimeout(HW._tt); HW._tt = setTimeout(() => t.classList.remove('show'), kind === 'ok' ? 1800 : 3600);
  },

  esc(s) { return String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c])); },

  player(id) { return (HW.view?.players || []).find(p => p.id === id); },
  name(id) { return HW.player(id)?.name || '?'; },
  chip(id) {
    const p = HW.player(id);
    if (!p) return '';
    return `<span class="chip"><i style="background:${p.color}"></i>${HW.esc(p.name)}</span>`;
  },

  // The six ZERAOS values come from the server (content.go), with icon and colour.
  get regionColors() { return (HW.view?.values || []).map(x => x.color); },
  get regionIcons() { return (HW.view?.values || []).map(x => x.icon); },
  valueName(i) {
    const x = HW.view?.values?.[i];
    return x ? `${x.icon} ${x.name}` : '';
  },

  // ---- Heartwood lookups ----
  WEATHER: { sun: ['☀️', 'Sun'], rain: ['🌧️', 'Rain'], fog: ['🌫️', 'Fog'] },
  // Names come from the view (view.stages / view.ringDecks); these are the fallbacks.
  STAGES: ['', 'Seeded', 'Sprout', 'Sapling', 'Big Tree'],
  RING_DECKS: ['', 'Deep', 'Story', 'Light'],
  stageName(s) { return HW.view?.stages?.[s] || HW.STAGES[s] || ''; },
  ringDeck(r) { return HW.view?.ringDecks?.[r] || HW.RING_DECKS[r] || ''; },
  STAGE_ICON: ['', '🫘', '🌱', '🌿', '🌳'],   // 🌰 means trust only
  MAX_WATER: 5, MAX_FRUIT: 2,
  // The Keeper's briefing slides after Start (view.slide is the index); phones follow along.
  SLIDES: [['welcome', 'Welcome'], ['goal', 'Our goal'], ['carry', 'What we have'], ['weather', 'Weather'],
    ['tide', 'Leaves and water'], ['powers', 'Our powers'], ['owl', 'Secret Owl'], ['pact', 'Forest Pact'], ['play', 'How we play']],
  slideId(v) { return v?.phase === 'brief' ? (HW.SLIDES[v.slide || 0] || HW.SLIDES[0])[0] : ''; },
  roleName(t) { return HW.view?.roles?.find(r => r.type === t)?.name || `Type ${t}`; },
  roleText(t) { return HW.view?.roles?.find(r => r.type === t)?.text || ''; },
  treasure(id) { return HW.view?.treasures?.find(x => x.id === id) || null; },
  treasureIcon(id) { return HW.treasure(id)?.icon || (id ? '💎' : ''); },
  weatherName(w) { return w ? HW.WEATHER[w].join(' ') : ''; },

  dist(a, b) {
    const dq = a.q - b.q, dr = a.r - b.r;
    return (Math.abs(dq) + Math.abs(dr) + Math.abs(dq + dr)) / 2;
  },

  // The hexes next to each hex (index → indices), from the view's board.
  adj(v) {
    if (HW._adj?.n === v.hexes.length) return HW._adj.a;
    const at = new Map(v.hexes.map((h, i) => [h.q + ',' + h.r, i]));
    const a = v.hexes.map(h => [[1, 0], [1, -1], [0, -1], [-1, 0], [-1, 1], [0, 1]].map(([dq, dr]) => at.get((h.q + dq) + ',' + (h.r + dr))).filter(j => j !== undefined));
    HW._adj = { n: v.hexes.length, a };
    return a;
  },
  // A player's turn this round (everyone plays at once): {actions, done, enthusiastUsed, investigated}.
  turnOf(v, p) { return (p && v.turns?.[p.id]) || null; },
  playing(v, p) { const t = HW.turnOf(v, p); return v.phase === 'turn' && !!t && !t.done; },

  startTimers() {
    setInterval(() => {
      document.querySelectorAll('[data-timer]').forEach(el => {
        const end = HW.view?.timerEnd || 0, wait = HW.view?.timerWait || 0;
        if (!end && !wait) { el.textContent = ''; el.classList.remove('over'); return; }
        // a share's timer waiting for its card on the big screen shows its full time, still
        const left = end ? Math.round((end - (Date.now() + HW.clockOffset)) / 1000) : wait;
        const a = Math.abs(left);
        el.textContent = `${HW.view.timerLabel ? HW.view.timerLabel + ' · ' : ''}${left < 0 ? '-' : ''}${Math.floor(a / 60)}:${String(a % 60).padStart(2, '0')}`;
        el.classList.toggle('over', left < 0);
      });
    }, 250);
  },

  phaseTitle(v) {
    return {
      lobby: 'Gathering the team', brief: 'The briefing', enter: 'Entering the forest', turn: `Round ${v.round || 1}`,
      guess: 'Who was your Secret Owl?', chain: 'The tribute chain',
      end: v.result === 'won' ? 'The forest is awake!' : v.result === 'lost' ? 'The forest sleeps on' : v.result === 'ended' ? 'The game is closed' : 'The end',
    }[v.phase] || v.phase;
  },

  // The end of the game: recognition, not points. No winner among players.
  recognitionTable(v) {
    const rows = (v.recognition || []).map(r => `<tr><td><span class="chip"><i style="background:${r.color}"></i>${HW.esc(r.name)}</span></td>
      <td>🌰 <b>${r.trust}</b> ${r.givers ? `<small>from ${r.givers} ${r.givers === 1 ? 'person' : 'people'}</small>` : ''}</td>
      <td>🦉 ${HW.esc(r.owlName || '')}</td>
      <td>${r.guessedRight ? '<span class="good">✓ guessed right</span>' : '<span class="muted">not this time</span>'}</td>
      <td>${r.owlHidden ? '🤫 stayed hidden' : '<span class="muted">spotted</span>'}</td></tr>`).join('');
    return `<table class="scores"><tr><th>Player</th><th>Trust received</th><th>Their Secret Owl</th><th>Their guess</th><th>As an Owl</th></tr>${rows}</table>`;
  },
  // Older name, kept so any page still calling it shows the recognition table.
  scoresTable(v) { return HW.recognitionTable(v); },
};

// ---------- the rules both screens check before offering a tap ----------
// The server checks again and its refusal is shown; these only decide what glows (the Keeper's board) or
// what a phone offers.
HW.rules = (() => {
  const has = (p, t) => (p?.types || []).includes(t);
  const wxOf = (v, i) => (i > 0 && v.hexes[i] ? v.weather[v.hexes[i].sector] : null);
  const tileOf = (v, i) => (i > 0 ? v.tiles[i] : null);
  const sealed = (v, i) => !!tileOf(v, i) && tileOf(v, i).leaves >= 2;
  const left = (v, p) => HW.turnOf(v, p)?.actions || 0;
  function moveTargets(v, p) {
    const t = HW.turnOf(v, p), adj = HW.adj(v);
    if (!p || p.pos < 0 || p.water <= 0 || !t || t.done || t.actions < 1) return { one: [], two: [] };
    const one = adj[p.pos].filter(j => !sealed(v, j) || has(p, 8));
    const two = [];
    if (has(p, 7) && !t.enthusiastUsed && wxOf(v, p.pos) !== 'fog')
      for (const m of adj[p.pos].filter(j => !sealed(v, j) && wxOf(v, j) !== 'fog'))
        for (const j of adj[m]) if (j !== p.pos && !sealed(v, j) && wxOf(v, j) !== 'fog' && !one.includes(j) && !two.includes(j)) two.push(j);
    return { one, two };
  }
  // Passing: until the giver ends their turn, on the same hex, or to a teammate on a neighbouring hex when both
  // are in a Peacemaker's chain (teammates linked hex by hex, each on or next to the next one's hex, a Peacemaker
  // among them).
  function canPass(v, a, b) {
    if (!a || !b || a.id === b.id || a.pos < 0 || b.pos < 0) return false;
    if (a.pos === b.pos) return true;
    return HW.adj(v)[a.pos].includes(b.pos) && !!chainOf(v, a);
  }
  // the Peacemaker that p is linked to hex by hex through teammates (p themself, maybe), or null
  function chainOf(v, p) {
    const adj = HW.adj(v), seen = new Set([p.id]), todo = [p];
    while (todo.length) {
      const a = todo.shift();
      if (has(a, 9)) return a;
      for (const b of v.players) if (!seen.has(b.id) && b.pos >= 0 && (b.pos === a.pos || adj[a.pos]?.includes(b.pos))) { seen.add(b.id); todo.push(b); }
    }
    return null;
  }
  const reachable = (v, p) => [p.pos, ...(has(p, 2) ? HW.adj(v)[p.pos] : [])].filter(i => i > 0);
  const clearCost = (v, p, i) => (wxOf(v, i) === 'rain' ? 2 : 1);
  function targetsFor(v, p, mode) {
    if (mode === 'water') return reachable(v, p).filter(i => [1, 2].includes(v.tiles[i].stage) && v.tiles[i].leaves < 2);
    if (mode === 'tend') return reachable(v, p).filter(i => v.tiles[i].stage === 3 && v.tiles[i].leaves < 2);
    if (mode === 'clear') return [p.pos, ...HW.adj(v)[p.pos]].filter(i => i > 0 && v.tiles[i].leaves > 0)
      .filter(i => left(v, p) >= clearCost(v, p, i));
    return [];
  }
  // What a player can do now, with the reason when they can't.
  function turnOptions(v, p) {
    const n = left(v, p), pos = p.pos, tile = tileOf(v, pos), w = wxOf(v, pos), adj = HW.adj(v);
    const onTree = pos === 0, helper = has(p, 2);
    const o = {};
    const need = (k, ok, why) => (n < k ? { ok: false, why: n ? `needs ${k} actions` : 'no actions left' } : ok ? { ok: true } : { ok: false, why });
    const exploreCost = w === 'fog' ? 2 : 1;
    o.explore = need(exploreCost, tile && !tile.up, onTree ? 'nothing to explore here' : 'already explored');
    o.explore.cost = exploreCost; o.explore.wx = w;
    o.sow = need(1, tile && tile.up && tile.kind !== 'spring' && !tile.treasure && !tile.stage && tile.leaves < 2,
      onTree ? 'not on the World Tree' : !tile.up ? 'explore it first' : tile.kind === 'spring' ? 'a spring' : tile.treasure ? 'a treasure lies here'
        : tile.stage ? 'already growing' : 'sealed by leaves');
    const wt = targetsFor(v, p, 'water');
    o.water = need(1, p.water > 0 && wt.length, p.water <= 0 ? 'no water to give' : helper ? 'no Seeded or Sprout here or next door' : 'not Seeded or Sprout');
    o.water.targets = wt;
    const tt = targetsFor(v, p, 'tend');
    o.tend = need(1, tt.length, helper ? 'no Sapling here or next door' : 'no Sapling here');
    o.tend.targets = tt;
    const near = [pos, ...(adj[pos] || [])].filter(i => i > 0 && v.tiles[i].leaves > 0);
    const ct = targetsFor(v, p, 'clear');
    o.clear = need(1, ct.length, !near.length ? 'no leaves nearby' : 'needs 2 actions in rain');
    o.clear.targets = ct;
    o.clear.cost = near.length && near.every(i => clearCost(v, p, i) === 2) ? 2 : 1;
    o.clear.wx = o.clear.cost > 1 ? 'rain' : '';
    if (has(p, 1)) {
      // a Reformer's Clear sweeps a layer off their hex and every hex around it, in one go (2 actions in the rain)
      const cost = w === 'rain' ? 2 : 1;
      o.clear = Object.assign(need(cost, near.length, 'no leaves here or around'), { targets: near.length ? [pos] : [], cost, wx: cost > 1 ? 'rain' : '' });
    }
    o.harvest = need(1, tile && tile.stage === 4 && tile.leaves < 2 && w !== 'rain' && !tile.harvested && p.fruit < HW.MAX_FRUIT,
      !tile || tile.stage !== 4 ? 'no Big Tree here' : tile.leaves >= 2 ? 'sealed by leaves' : w === 'rain' ? 'not in the rain'
        : tile.harvested ? 'picked since the Tide' : 'carrying 2 fruit');
    o.take = need(1, tile && tile.up && tile.treasure && !p.treasure, tile && tile.up && tile.treasure ? 'carrying a treasure' : 'no treasure here');
    o.drink = need(1, tile && tile.up && tile.kind === 'spring' && p.water < HW.MAX_WATER, tile && tile.up && tile.kind === 'spring' ? 'already full' : 'no spring here');
    return o;
  }
  return { has, wxOf, tileOf, sealed, moveTargets, canPass, chainOf, reachable, clearCost, targetsFor, turnOptions };
})();

HW.startTimers();
