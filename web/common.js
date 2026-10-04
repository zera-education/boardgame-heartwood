// Shared helpers: API calls, live updates, the hex board and timers.
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
    v.log ||= []; v.order ||= []; v.alliances ||= [];
    v.alliances.forEach(a => { a.members ||= []; });
    v.players = (v.players || []).map(p => ({ ...p, allies: p.allies || [], partners: p.partners || [], cards: p.cards || [], types: p.types || [], used: p.used || {} }));
    if (v.turn) v.turn.followUps ||= [];
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

  // Sends an action and shows the caller's own result at once.
  async act(creds, type, extra) {
    try {
      const r = await HW.post(`/api/games/${HW.code}/action`, { ...creds, type, ...extra });
      HW.apply(r.view);
      return true;
    } catch (e) { HW.toast(e.message); return false; }
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

  toast(msg) {
    let t = document.getElementById('toast');
    if (!t) { t = document.createElement('div'); t.id = 'toast'; document.body.appendChild(t); }
    t.textContent = msg; t.classList.add('show');
    clearTimeout(HW._tt); HW._tt = setTimeout(() => t.classList.remove('show'), 3200);
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
  // An alliance's members and statement, for the board and the phone.
  allianceHTML(al) {
    if (!al) return '';
    return `<div class="alliance">${al.members.map(m => `${HW.chip(m)} <small>${HW.valueName(HW.player(m)?.value)}</small>`).join(' · ')}
      <div class="statement">${al.statement ? `“${HW.esc(al.statement)}”` : '<span class="muted">Writing their statement…</span>'}</div></div>`;
  },
  tokenIcons: { mushroom: '🍄', squirrel: '🐿️', campfire: '🔥', path: '🍃' },
  tokenNames: { mushroom: 'Mushroom patch', squirrel: 'Squirrel', campfire: 'Campfire', path: 'Hidden path' },
  tierNames: ['', '🌱 Seed', '🌿 Sapling', '🌳 Oak', '💛 Heartwood'],
  ringNames: ['Heartwood', 'Oak Circle', 'Sapling Path', 'Seedlands'],

  dist(a, b) {
    const dq = a.q - b.q, dr = a.r - b.r;
    return (Math.abs(dq) + Math.abs(dr) + Math.abs(dq + dr)) / 2;
  },

  ringOpen(ring) {
    const s = HW.view.season;
    if (HW.view.phase === 'plant') return ring === 3;
    return s <= 1 ? ring === 3 : s === 2 ? ring >= 2 : true;
  },

  // Draws the forest into an <svg>. opts: {highlight: Set of hex ids, onClick(i), peek: {i: kind}}
  renderBoard(svg, opts = {}) {
    const v = HW.view, size = 50, s3 = Math.sqrt(3);
    const px = h => [size * s3 * (h.q + h.r / 2), size * 1.5 * h.r];
    const pts = (cx, cy) => [...Array(6)].map((_, k) => {
      const a = Math.PI / 180 * (60 * k - 30);
      return `${(cx + (size - 2) * Math.cos(a)).toFixed(1)},${(cy + (size - 2) * Math.sin(a)).toFixed(1)}`;
    }).join(' ');
    // compact (phones): no rim labels, so the board can be bigger and easier to tap.
    const W = size * s3 * (opts.compact ? 7.1 : 10.6), H = size * 1.5 * (opts.compact ? 6.6 : 8.4);
    let out = '';
    // Region labels around the rim.
    for (let r = 0; r < 6 && !opts.compact; r++) {
      const outer = v.hexes.filter(h => h.ring === 3 && h.region === r).map(px);
      const x = outer.reduce((a, p) => a + p[0], 0) / outer.length, y = outer.reduce((a, p) => a + p[1], 0) / outer.length;
      // Push the label out past the rim, and anchor it away from the board so long names don't overlap it.
      const len = Math.hypot(x, y), ux = x / len, uy = y / len, R = len + size * 1.25;
      const anchor = ux > 0.3 ? 'start' : ux < -0.3 ? 'end' : 'middle';
      out += `<text class="rlabel" text-anchor="${anchor}" x="${ux * R}" y="${uy * R}" fill="${HW.regionColors[r]}">${HW.regionIcons[r]} ${v.regions[r]}</text>`;
    }
    v.hexes.forEach((h, i) => {
      const [x, y] = px(h);
      const base = h.region < 0 ? '#7a4b2a' : HW.regionColors[h.region];
      const op = [1, 1, 0.78, 0.55][h.ring];
      const open = HW.ringOpen(h.ring);
      const hl = opts.highlight?.has(i);
      out += `<g class="hex ${hl ? 'hl' : ''} ${open ? '' : 'closed'}" data-i="${i}">`;
      out += `<polygon points="${pts(x, y)}" fill="${base}" fill-opacity="${op}" />`;
      const tok = v.tokens[i];
      const peek = opts.peek?.[i];
      if (h.ring === 0) out += `<text x="${x}" y="${y - 12}" class="hlabel">HEARTWOOD</text><text x="${x}" y="${y + 6}" class="ticon">💛</text>`;
      else if (tok) out += `<text x="${x - 22}" y="${y - 18}" class="ticon small used">${HW.tokenIcons[tok] || ''}</text>`;
      else if (peek) out += `<text x="${x}" y="${y - 14}" class="ticon">${HW.tokenIcons[peek]}</text>`;
      else if (i in v.tokens) out += `<text x="${x}" y="${y - 16}" class="ticon hidden">✦</text>`; // face down; a clearing has none
      const trees = v.trees[i] || 0;
      if (trees) out += `<text x="${x + 22}" y="${y - 18}" class="ticon small">${'🌳'.repeat(Math.min(trees, 2))}${trees > 2 ? '+' : ''}</text>`;
      const here = v.players.filter(p => p.pos === i);
      here.forEach((p, k) => {
        const n = here.length, ang = (k / n) * 2 * Math.PI, rad = n > 1 ? 18 : 0;
        const cx = x + rad * Math.cos(ang), cy = y + 12 + rad * Math.sin(ang) * 0.7;
        const cur = v.current === p.id;
        out += `<circle cx="${cx}" cy="${cy}" r="${cur ? 13 : 11}" fill="${p.color}" class="pawn ${cur ? 'cur' : ''}"/>`;
        out += `<text x="${cx}" y="${cy + 4}" class="pinit">${HW.esc(p.name.slice(0, 2))}</text>`;
      });
      out += '</g>';
    });
    svg.setAttribute('viewBox', `${-W / 2 - 20} ${-H / 2 - 30} ${W + 40} ${H + 60}`);
    HW.patch(svg, out);
    svg.onclick = e => {
      const g = e.target.closest('.hex');
      if (g && opts.onClick) opts.onClick(+g.dataset.i);
    };
  },

  startTimers() {
    setInterval(() => {
      document.querySelectorAll('[data-timer]').forEach(el => {
        const end = HW.view?.timerEnd || 0;
        if (!end) { el.textContent = ''; el.classList.remove('over'); return; }
        const left = Math.round((end - (Date.now() + HW.clockOffset)) / 1000);
        const a = Math.abs(left);
        el.textContent = `${HW.view.timerLabel ? HW.view.timerLabel + ' · ' : ''}${left < 0 ? '-' : ''}${Math.floor(a / 60)}:${String(a % 60).padStart(2, '0')}`;
        el.classList.toggle('over', left < 0);
      });
    }, 250);
  },

  phaseTitle(v) {
    return {
      lobby: 'Gathering the expedition', plant: 'Plant your seed',
      turn: `Season ${v.season} · ${v.seasonName}`, dusk: `Dusk · Season ${v.season}`,
      stories: 'The Vision of a Forest', guess: 'The Vision of a Forest', chain: 'The Vision of a Forest', scores: 'The forest is grown',
    }[v.phase] || v.phase;
  },

  scoresTable(v) {
    let h = `<table class="scores"><tr><th></th><th>Player</th><th>Trust</th><th>Growth</th><th>Advocacy</th><th>Secret Owl</th><th>Total</th></tr>`;
    v.scores.forEach((s, i) => {
      h += `<tr class="${s.winner ? 'win' : ''}"><td>${s.winner ? '🏆' : i + 1}</td><td>${HW.chip(s.id)}</td><td>${s.trust} <small>(${s.givers} people)</small></td><td>${s.growth}</td><td>${s.advocacy} <small>${(s.advocated || []).map(x => v.values[x].icon).join('')}</small></td><td>${s.secret} <small>🦉 ${HW.esc(s.owlName)}</small></td><td><b>${s.total}</b></td></tr>`;
    });
    h += '</table>';
    return h;
  },
};
HW.startTimers();
