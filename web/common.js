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
  roleName(t) { return HW.view?.roles?.find(r => r.type === t)?.name || `Type ${t}`; },
  roleText(t) { return HW.view?.roles?.find(r => r.type === t)?.text || ''; },
  treasure(id) { return HW.view?.treasures?.find(x => x.id === id) || null; },
  treasureIcon(id) { return HW.treasure(id)?.icon || (id ? '💎' : ''); },
  weatherName(w) { return w ? HW.WEATHER[w].join(' ') : ''; },

  dist(a, b) {
    const dq = a.q - b.q, dr = a.r - b.r;
    return (Math.abs(dq) + Math.abs(dr) + Math.abs(dq + dr)) / 2;
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
      lobby: 'Gathering the team', enter: 'Entering the forest', turn: `Round ${v.round || 1}`,
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
HW.startTimers();
