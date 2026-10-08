/* Story recording on the Keeper screen. board.html calls HWRec.view(v) after every render.
 *
 * One clip per share: recording starts when a share opens (a new share.idx) and stops when it closes or changes.
 * The clip then goes to the server (recording.go, POST /api/games/CODE/recordings), where El's Mac mini turns it
 * into text (ops/transcribe). The Keeper listens and reads on stories.html.
 *
 * Who records: only a device with the Keeper secret, and of those only the one the game names (v.recDevice). The
 * first Keeper device to allow the microphone takes it; another can take over ("Record on this device"). In one
 * browser only one tab records (Web Locks).
 *
 * Never in the way: no microphone, a refused one or a lost network only mean fewer clips, never an error or a wait.
 * A clip is kept in IndexedDB (every 3 s while recording, whole when it ends) until the server has it, and sent
 * again with backoff, also after a reload; a clip cut short by a reload is sent as it was ("partial").
 *
 * The UI is injected: a recording line on the share card (with "Don't keep this one"), the microphone step in the
 * lobby, a Stories block in the Keeper tools and a Stories link in the finale.
 */
(() => {
  'use strict';
  const code = (new URLSearchParams(location.search).get('g') || '').toUpperCase();
  if (!code || typeof HW === 'undefined') { window.HWRec = null; return; } // HW: common.js (a global const, not on window)

  const MIN_MS = 1200;            // shorter: a share closed at once, nothing said
  const MAX_MS = 20 * 60 * 1000;  // a share left open (a break): keep the first 20 minutes
  const SLICE = 3000;             // a chunk to IndexedDB every 3 s: a reload or a crash loses at most that
  const AUDIO = { channelCount: 1, echoCancellation: false, noiseSuppression: false, autoGainControl: true };

  const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const secret = c => { try { return localStorage.getItem('hw-host-' + c) || ''; } catch { return ''; } };
  const rnd = n => {
    const a = new Uint8Array(n), abc = 'abcdefghijkmnpqrstuvwxyz23456789';
    crypto.getRandomValues(a);
    return Array.from(a, x => abc[x % abc.length]).join('');
  };
  const device = (() => {
    try {
      let d = localStorage.getItem('hw-rec-device');
      if (!d) { d = 'd' + rnd(15); localStorage.setItem('hw-rec-device', d); }
      return d;
    } catch { return 'd' + rnd(15); }
  })();
  const secure = window.isSecureContext !== false;
  const canRecord = !!(secure && navigator.mediaDevices?.getUserMedia && window.MediaRecorder);
  const MIME = canRecord ? (['audio/webm;codecs=opus', 'audio/mp4', 'audio/ogg;codecs=opus', 'audio/webm']
    .find(t => { try { return MediaRecorder.isTypeSupported(t); } catch { return false; } }) || '') : '';
  const fmt = ms => { const s = Math.max(0, Math.floor(ms / 1000)); return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`; };

  // ---------- storage: IndexedDB, or memory when the browser has none (private mode) ----------
  const Store = (() => {
    const mem = { parts: new Map(), queue: new Map() };
    const keyOf = (store, o) => store === 'parts' ? o.k : o.clip;
    let dbp = null;
    const open = () => dbp || (dbp = new Promise(res => {
      try {
        const r = indexedDB.open('hw-rec', 1);
        r.onupgradeneeded = () => { r.result.createObjectStore('parts', { keyPath: 'k' }); r.result.createObjectStore('queue', { keyPath: 'clip' }); };
        r.onsuccess = () => res(r.result);
        r.onerror = r.onblocked = () => res(null);
      } catch { res(null); }
    }));
    const tx = (db, store, mode, fn) => new Promise((res, rej) => {
      const t = db.transaction(store, mode), req = fn(t.objectStore(store));
      t.oncomplete = () => res(req?.result);
      t.onerror = t.onabort = () => rej(t.error);
    });
    // any IndexedDB failure (quota, private mode): carry on in memory
    async function run(store, mode, idb, memFn) {
      const db = await open();
      if (db) { try { return await tx(db, store, mode, idb); } catch { dbp = Promise.resolve(null); } }
      return memFn(mem[store]);
    }
    return {
      put: (store, o) => run(store, 'readwrite', s => s.put(o), m => { m.set(keyOf(store, o), o); }),
      all: store => run(store, 'readonly', s => s.getAll(), m => [...m.values()]),
      del: (store, k) => run(store, 'readwrite', s => s.delete(k), m => { m.delete(k); }),
      delParts: clip => run('parts', 'readwrite', s => s.delete(IDBKeyRange.bound(clip + ':', clip + ';')),
        m => { for (const k of [...m.keys()]) if (k.startsWith(clip + ':')) m.delete(k); }),
    };
  })();

  // ---------- state ----------
  const S = {
    v: null, inited: false,
    tab: null,                 // this tab holds the recording lock (null: not known yet)
    perm: 'unknown',           // the browser's microphone permission: granted | prompt | denied | unknown
    mic: canRecord ? 'off' : secure ? 'none' : 'insecure', // off | asking | ready | denied | none | insecure
    stream: null, opening: null, retryMicAt: 0,
    cur: null, starting: false, startAfter: 0, // no new clip before this (after the recorder failed)
    skip: new Map(),           // share idx → 'dropped' | 'capped': not recorded again here
    sent: new Map(),           // clip id → {id, share} uploaded in this session (to delete on "Don't keep")
    claimAt: 0, pending: 0, flushing: false, backoff: 0, retryT: 0, lastErr: '', ticks: 0,
  };
  const here = v => v.recording !== false && S.tab === true && v.recDevice === device;
  const micUsable = () => !!S.stream?.active || ((S.mic === 'ready' || S.perm === 'granted') && S.mic !== 'denied' && Date.now() >= S.retryMicAt);

  // ---------- the microphone ----------
  function openMic(userAsked) {
    if (S.stream?.active) return Promise.resolve(true);
    if (!canRecord) return Promise.resolve(false);
    if (S.opening) return S.opening;
    if (!userAsked && !micUsable()) return Promise.resolve(false);
    if (userAsked) { S.mic = 'asking'; paint(); }
    S.opening = navigator.mediaDevices.getUserMedia({ audio: AUDIO }).then(st => {
      S.stream = st; S.mic = 'ready';
      if (S.perm !== 'denied') S.perm = 'granted';
      for (const t of st.getAudioTracks()) t.addEventListener('ended', micLost);
      return true;
    }, e => {
      S.retryMicAt = Date.now() + 15000;
      const n = e?.name || '';
      S.mic = n === 'NotAllowedError' || n === 'SecurityError' ? 'denied' : n === 'NotFoundError' || n === 'OverconstrainedError' ? 'none' : 'off';
      return false;
    }).finally(() => { S.opening = null; paint(); });
    return S.opening;
  }
  function closeMic() {
    if (!S.stream) return;
    for (const t of S.stream.getTracks()) { t.removeEventListener('ended', micLost); t.stop(); }
    S.stream = null;
    if (S.mic === 'ready') S.mic = 'off';
  }
  // unplugged, or taken away by the system: keep what was recorded, try again in a moment
  function micLost() {
    S.stream = null;
    S.mic = 'off';
    S.retryMicAt = Date.now() + 5000;
    if (S.cur) stopClip(S.cur, true);
    paint();
  }

  // ---------- clips ----------
  function valueOf(v, s) {
    if (s.kind === 'why') { const p = (v.players || []).find(x => x.id === s.player); return p && p.value >= 0 ? p.value : ''; }
    if (s.kind === 'harvest') { const i = (v.values || []).findIndex(x => x.tagline && x.tagline === s.sub); return i >= 0 ? i : ''; }
    return '';
  }

  async function startClip(s) {
    S.starting = true;
    try {
      if (!await openMic(false)) return;
      const v = S.v; // the newest view: the share may have moved on while the microphone opened
      if (!v?.share || v.share.idx !== s.idx || !here(v) || S.cur || S.skip.has(s.idx)) return;
      const opts = { audioBitsPerSecond: MIME.startsWith('audio/mp4') ? 64000 : 32000 };
      if (MIME) opts.mimeType = MIME;
      let rec;
      try { rec = new MediaRecorder(S.stream, opts); } catch { rec = new MediaRecorder(S.stream); }
      const t0 = Date.now();
      const clip = {
        id: 'c' + rnd(20), idx: s.idx, rec, chunks: [], n: 0, t0, keep: true, discard: false, done: false,
        meta: {
          code, share: s.idx, kind: s.kind, player: s.player, prompt: s.prompt || '', sub: String(s.sub ?? ''),
          seq: s.seq || 0, of: s.of || 0, value: valueOf(v, s), mime: rec.mimeType || MIME || 'audio/webm', t0,
          round: s.kind === 'why' ? 0 : v.round || 0, // the last "why" comes after round 1 has begun
        },
      };
      rec.ondataavailable = e => {
        if (!e.data?.size || clip.discard) return;
        clip.chunks.push(e.data);
        const n = clip.n++;
        Store.put('parts', { k: clip.id + ':' + String(n).padStart(5, '0'), clip: clip.id, n, at: Date.now(), meta: clip.meta, blob: e.data }).catch(() => {});
      };
      rec.onstop = () => finish(clip);
      rec.onerror = () => { if (S.cur === clip) S.cur = null; S.startAfter = Date.now() + 15000; finish(clip); };
      rec.start(SLICE);
      clip.meta.mime = rec.mimeType || clip.meta.mime;
      S.cur = clip;
    } catch (e) {
      S.lastErr = `The recorder could not start (${e?.name || e?.message || 'error'}).`;
      S.startAfter = Date.now() + 15000;
    } finally {
      S.starting = false;
      const v = S.v;
      if (S.cur && v && (!v.share || v.share.idx !== S.cur.idx || !here(v))) view(v);
      else paint();
    }
  }

  // keep: send it (the share ended); otherwise it is dropped
  function stopClip(clip, keep) {
    if (S.cur === clip) S.cur = null;
    clip.ms = Date.now() - clip.t0;
    clip.keep = keep && !clip.discard;
    if (!clip.keep) clip.discard = true;
    try { if (clip.rec.state !== 'inactive') { clip.rec.stop(); return; } } catch { /* finish below */ }
    finish(clip);
  }

  async function finish(clip) {
    if (clip.done) return;
    clip.done = true;
    clip.ms ??= Date.now() - clip.t0;
    if (clip.keep && !clip.discard && clip.ms >= MIN_MS && clip.chunks.length) {
      const blob = new Blob(clip.chunks, { type: clip.meta.mime });
      await Store.put('queue', { clip: clip.id, meta: { ...clip.meta, ms: clip.ms }, blob, at: Date.now() }).catch(() => {});
    }
    await Store.delParts(clip.id).catch(() => {});
    flush();
  }

  // "Don't keep this one": the clip of the open share goes, with any earlier part of it
  async function drop() {
    const v = S.v, idx = S.cur?.idx ?? v?.share?.idx;
    if (idx == null) return;
    S.skip.set(idx, 'dropped');
    if (S.cur) { S.cur.discard = true; stopClip(S.cur, false); }
    HW.toast("This story won't be kept.", 'ok');
    paint();
    for (const it of await Store.all('queue').catch(() => [])) {
      if (it.meta.code === code && it.meta.share === idx) await Store.del('queue', it.clip).catch(() => {});
    }
    for (const [clip, x] of S.sent) {
      if (x.share !== idx) continue;
      S.sent.delete(clip);
      fetch(`/api/games/${code}/recordings/${x.id}`, { method: 'DELETE', headers: { 'X-Keeper': secret(code) } }).catch(() => {});
    }
  }

  // A reload or a crash mid-story leaves chunks behind: send what was recorded. Chunks of another game are left
  // to that game's tab, unless they are older than half an hour (no recording runs that long without a new chunk).
  async function recover() {
    const parts = await Store.all('parts').catch(() => []);
    const queued = new Set((await Store.all('queue').catch(() => [])).map(q => q.clip));
    const by = new Map();
    for (const p of parts) {
      if (p.clip === S.cur?.id) continue;
      if (!by.has(p.clip)) by.set(p.clip, []);
      by.get(p.clip).push(p);
    }
    for (const [clip, ps] of by) {
      ps.sort((a, b) => a.n - b.n);
      const last = ps[ps.length - 1].at || 0;
      if (ps[0].meta?.code !== code && Date.now() - last < 30 * 60 * 1000) continue;
      const ms = last - (ps[0].meta?.t0 || ps[0].at - SLICE);
      if (!queued.has(clip) && ps[0].n === 0 && ms >= MIN_MS) {
        const blob = new Blob(ps.map(p => p.blob), { type: ps[0].meta.mime });
        await Store.put('queue', { clip, meta: { ...ps[0].meta, ms, partial: 1 }, blob, at: Date.now() }).catch(() => {});
      }
      await Store.delParts(clip).catch(() => {});
    }
  }

  // ---------- sending ----------
  async function send(it) {
    const m = it.meta, sec = secret(m.code);
    if (!sec) return 'drop'; // this browser is no longer the Keeper of that game
    const q = new URLSearchParams({
      clip: it.clip, share: m.share, kind: m.kind, player: m.player, value: m.value ?? '', prompt: m.prompt, sub: m.sub,
      seq: m.seq || 0, of: m.of || 0, round: m.round || 0, ms: Math.round(m.ms || 0), partial: m.partial ? 1 : 0,
    });
    try {
      const r = await fetch(`/api/games/${encodeURIComponent(m.code)}/recordings?${q}`, {
        method: 'POST', headers: { 'Content-Type': it.blob.type || m.mime, 'X-Keeper': sec }, body: it.blob,
      });
      const j = await r.json().catch(() => ({}));
      if (r.ok) { if (m.code === code) S.sent.set(it.clip, { id: j.id, share: m.share }); return 'ok'; }
      if (r.status >= 500 || r.status === 408 || r.status === 429) return 'retry';
      S.lastErr = `A story couldn't be saved: ${j.error || 'refused (' + r.status + ')'}.`;
      return 'drop';
    } catch { return 'retry'; }
  }

  async function flush() {
    if (S.flushing) { S.flushAgain = true; return; }
    S.flushing = true;
    clearTimeout(S.retryT);
    let wait = 0;
    try {
      do {
        S.flushAgain = false;
        const items = (await Store.all('queue')).sort((a, b) => a.at - b.at);
        S.pending = items.length; paint();
        for (const it of items) {
          const r = await send(it);
          if (r === 'retry') { S.backoff = Math.min((S.backoff || 2500) * 2, 120000); wait = S.backoff; break; }
          S.backoff = 0;
          await Store.del('queue', it.clip);
          S.pending = Math.max(0, S.pending - 1); paint();
        }
      } while (S.flushAgain && !wait);
    } catch { wait = 30000; }
    S.flushing = false;
    S.pending = (await Store.all('queue').catch(() => [])).length;
    if (wait && S.pending) S.retryT = setTimeout(flush, wait);
    paint();
  }

  // ---------- the game setting ----------
  async function claim(force) {
    if (!force && Date.now() - S.claimAt < 8000) return;
    S.claimAt = Date.now();
    try {
      const r = await HW.post(`/api/games/${code}/action`, { host: secret(code), type: 'recordHere', text: device });
      HW.apply(r.view);
    } catch (e) { if (force) HW.toast(e.message, 'bad'); }
  }
  async function setRecording(on) {
    const was = !!S.cur;
    S.switching = true;
    const ok = await HW.act({ host: secret(code) }, 'record', { n: on ? 1 : 0 }).finally(() => { S.switching = false; });
    if (ok) HW.toast(on ? 'Recording on: each story is recorded and turned into text.' : was ? "Recording off: this story isn't kept." : 'Recording off for this game.', 'ok');
  }
  async function allow() {
    if (!await openMic(true)) {
      HW.toast(S.mic === 'denied' ? "The microphone is blocked: stories won't be recorded." : S.mic === 'insecure'
        ? 'Recording needs https:// or localhost.' : "No microphone found: stories won't be recorded.", 'bad');
      return;
    }
    let v = S.v;
    if (v && !v.recDevice) { await claim(true); v = S.v; }
    HW.toast(v && v.recDevice && v.recDevice !== device ? 'Microphone ready. The stories are recorded on the other Keeper device.'
      : 'Microphone ready: each story is recorded here.', 'ok');
    if (S.v) view(S.v);
  }
  async function takeOver() {
    if (!S.stream?.active && !await openMic(true)) { HW.toast("This device can't record: no microphone.", 'bad'); return; }
    await claim(true);
    if (S.v?.recDevice === device) HW.toast('The stories are recorded on this device now.', 'ok');
  }

  // ---------- what the Keeper sees ----------
  function status(v) {
    if (v.recording === false) return 'off';
    if (S.mic === 'insecure') return 'insecure';
    if (S.mic === 'none') return 'none';
    if (S.tab === false) return 'tab';
    if (v.recDevice && v.recDevice !== device) return 'elsewhere';
    if (S.mic === 'asking') return 'asking';
    if (S.cur && S.cur.idx === v.share?.idx) return 'rec';
    if (v.share && S.skip.has(v.share.idx)) return S.skip.get(v.share.idx);
    if (S.mic === 'denied' || S.perm === 'denied') return 'denied';
    if (S.mic === 'ready' || S.perm === 'granted') return 'ready';
    return 'need';
  }
  const mic = h => window.ART?.inline ? ART.inline('icon:keeper', { height: h }) : '';
  const btn = (act, label, cls = 'cream sm') => `<button type="button" class="k-btn ${cls}" data-hwrec="${act}">${label}</button>`;

  const CARD = {
    rec: () => `<span class="hwrec-dot"></span><b>Recording</b><span class="hwrec-t">${fmt(Date.now() - (S.cur?.t0 || Date.now()))}</span>${btn('drop', "Don't keep this one")}`,
    ready: () => `<span class="hwrec-dot"></span><b>Recording</b>`,
    dropped: () => '<span class="hwrec-dot off"></span>Not kept',
    capped: () => '<span class="hwrec-dot off"></span>Recorded the first 20 minutes',
    elsewhere: () => `<span class="hwrec-dot"></span>Recorded on the other Keeper device${btn('here', 'Record here')}`,
    tab: () => '<span class="hwrec-dot"></span>Recorded in another tab',
    off: () => '<span class="hwrec-dot off"></span>Recording off',
    need: () => `<span class="hwrec-dot off"></span>Not recorded${btn('allow', 'Allow the microphone')}`,
    asking: () => '<span class="hwrec-dot off"></span>Allow the microphone in the browser…',
    denied: () => '<span class="hwrec-dot off"></span>Not recorded: the microphone is blocked',
    none: () => '<span class="hwrec-dot off"></span>Not recorded: no microphone',
    insecure: () => '<span class="hwrec-dot off"></span>Not recorded: needs https',
  };
  const LOBBY = {
    need: ['Record the stories', 'Each story is recorded on this laptop and turned into text for the team. You can switch it off, or drop a clip.', btn('allow', 'Allow the microphone', 'sm')],
    asking: ['Allow the microphone', "Choose Allow in the browser's question, so the first story isn't interrupted."],
    ready: ['Stories are recorded here', 'The microphone is ready. Each story is recorded on this laptop and turned into text for the team.', btn('off', 'Switch off')],
    rec: ['Stories are recorded here', 'The microphone is ready.', btn('off', 'Switch off')],
    off: ['Recording is off', "This game's stories are not recorded.", btn('on', 'Switch on')],
    elsewhere: ['Recorded on the other Keeper device', 'Another Keeper screen records the stories.', btn('here', 'Record here instead')],
    tab: ['Recorded in another tab', 'Another tab of this browser records the stories.'],
    denied: ['The microphone is blocked', "Stories won't be recorded. To record, allow the microphone in the site settings (left of the address bar), then reload."],
    none: ['No microphone', "No microphone was found: stories won't be recorded."],
    insecure: ['Recording needs a secure page', 'Open the game at https:// or on localhost to record the stories.'],
  };
  const TOOLS = {
    rec: 'Recording this story', ready: 'Each story is recorded here', need: 'Allow the microphone to record the stories',
    asking: 'Waiting for the microphone…', off: 'Off for this game', elsewhere: 'Recorded on the other Keeper device',
    tab: 'Recorded in another tab of this browser', denied: 'The microphone is blocked', none: 'No microphone on this device',
    insecure: 'Needs https:// or localhost', dropped: "This story isn't kept", capped: 'Stopped after 20 minutes',
  };

  function toolsHTML(v, st) {
    const b = [];
    if (st === 'need' || ((st === 'denied' || st === 'none') && canRecord)) b.push(btn('allow', 'Allow the microphone', 'sm'));
    if (st === 'elsewhere' || st === 'tab') b.push(btn('here', 'Record on this device'));
    if (st === 'rec') b.push(btn('drop', "Don't keep this one"));
    b.push(v.recording === false ? btn('on', 'Switch recording on', 'sm') : btn('off', 'Switch recording off'));
    const note = [S.pending ? `${S.pending} ${S.pending === 1 ? 'story' : 'stories'} waiting to upload${S.backoff ? ' (the network is down: trying again)' : '…'}` : '', S.lastErr]
      .filter(Boolean).map(esc).join(' · ');
    return `<div class="k-row"><span class="k-lab">Stories</span><span class="hwrec-st"><span class="hwrec-dot${st === 'rec' || st === 'ready' ? '' : ' off'}"></span>${TOOLS[st] || ''}</span></div>
      <div class="k-row">${b.join('')}</div>${note ? `<p class="hwrec-note">${note}</p>` : ''}
      <p class="hwrec-link"><a href="stories.html?g=${encodeURIComponent(code)}" target="_blank" rel="noopener">Stories and transcripts</a></p>`;
  }

  // board.html redraws its panels with innerHTML: put the injected parts back whenever they go missing
  function inject(v, st) {
    const card = document.querySelector('#k-modal .k-card .k-card-btns');
    if (card) {
      let el = card.querySelector('.hwrec-c');
      if (!el) { el = document.createElement('div'); el.className = 'hwrec-c'; card.appendChild(el); }
      const key = st + '|' + (v.share?.idx ?? '');
      if (el.dataset.key !== key) { el.dataset.key = key; el.innerHTML = (CARD[st] || CARD.off)(); }
    }
    const lob = v.phase === 'lobby' && document.querySelector('#k-modal .k-lob-r');
    if (lob) {
      let el = lob.querySelector('.hwrec-lob');
      if (!el) { el = document.createElement('div'); el.className = 'hwrec-lob'; lob.insertBefore(el, lob.querySelector('.k-start')); }
      const [t, small, b] = LOBBY[st] || LOBBY.ready;
      const key = st;
      if (el.dataset.key !== key) { el.dataset.key = key; el.dataset.st = st; el.innerHTML = `<span class="hwrec-ic">${mic('2.4em')}</span><div><b>${esc(t)}</b><small>${esc(small)}</small></div>${b || ''}`; }
    }
    const fin = document.querySelector('#k-modal .k-fin');
    if (fin && !fin.querySelector('.hwrec-fin')) {
      const el = document.createElement('div');
      el.className = 'hwrec-fin';
      el.innerHTML = `<a class="k-btn cream" href="stories.html?g=${encodeURIComponent(code)}" target="_blank" rel="noopener">${mic('1.1em')} The stories</a><small>Listen again and read every story told today. Transcripts arrive within minutes.</small>`;
      fin.appendChild(el);
    }
  }

  function paint() {
    const v = S.v;
    if (!S.inited || !v) return;
    const host = document.getElementById('k-host');
    let tools = document.getElementById('hwrec-tools');
    if (!v.isHost) { if (tools) tools.hidden = true; return; }
    const st = status(v);
    if (host && !tools) {
      tools = document.createElement('div');
      tools.id = 'hwrec-tools'; tools.className = 'hwrec-tools';
      const timer = host.querySelector('.k-row');
      host.insertBefore(tools, timer ? timer.nextSibling : null);
    }
    if (tools) {
      tools.hidden = false;
      const h = toolsHTML(v, st);
      if (tools._h !== h) { tools._h = h; tools.innerHTML = h; }
    }
    inject(v, st);
  }

  // ---------- the loop ----------
  function view(v) {
    try {
      S.v = v;
      if (!v || !v.isHost || (v.code && v.code !== code)) return;
      init();
      const s = v.share;
      if (S.cur && (!s || s.idx !== S.cur.idx || !here(v))) {
        const off = v.recording === false;
        // a card swapped for another ("not this one"): its clip holds no story, so it is dropped
        stopClip(S.cur, !off && !(s && s.was === S.cur.idx));
        if (off && !S.switching) HW.toast("Recording is off: this story isn't kept.");
      }
      if (here(v) && s && !S.cur && !S.starting && !S.skip.has(s.idx) && micUsable() && Date.now() >= S.startAfter) startClip(s);
      if (v.recording !== false && S.tab === true && !v.recDevice && (S.mic === 'ready' || S.perm === 'granted')) claim();
      // the microphone stays open from the first "Allow" (no second question), and closes when nothing more is recorded here
      if (S.stream && !S.cur && !S.starting && (!here(v) || (['chain', 'end'].includes(v.phase) && !s))) closeMic();
      paint();
    } catch { /* recording must never break the board */ }
  }

  function tick() {
    const v = S.v;
    if (!v || !v.isHost) return;
    if (S.cur) {
      const ms = Date.now() - S.cur.t0;
      for (const el of document.querySelectorAll('.hwrec-t')) el.textContent = fmt(ms);
      if (ms > MAX_MS) { S.skip.set(S.cur.idx, 'capped'); stopClip(S.cur, true); }
    }
    if (++S.ticks % 3 === 0) view(v); // retry a start that failed, put back a panel board.html redrew
    if (S.ticks % 60 === 0 && S.pending && !S.flushing) flush();
  }

  function takeLock() {
    if (!navigator.locks?.request) { S.tab = true; return; }
    navigator.locks.request('hw-rec-' + code, { ifAvailable: true }, lock => {
      if (!lock) { S.tab = false; setTimeout(takeLock, 5000); paint(); return; }
      S.tab = true;
      recover().then(flush);
      if (S.v) view(S.v);
      return new Promise(() => {}); // held until this tab closes
    }).catch(() => { S.tab = true; });
  }

  function init() {
    if (S.inited) return;
    S.inited = true;
    const css = document.createElement('style');
    css.id = 'hwrec-css';
    css.textContent = `
.hwrec-c { margin-left: auto; display: flex; align-items: center; gap: 8px; font: 800 16px/1.2 var(--read, Nunito, sans-serif); color: #6f4f36; }
.hwrec-c b, .hwrec-st { font: 20px/1 var(--toon, 'Lilita One', sans-serif); color: #b3261e; }
.hwrec-c .k-btn { margin-left: 4px; }
.hwrec-t { font: 20px/1 var(--toon, 'Lilita One', sans-serif); color: #6f4f36; font-variant-numeric: tabular-nums; min-width: 3ch; }
.hwrec-dot { display: inline-block; flex: none; width: 14px; height: 14px; border-radius: 50%; background: #e0392b; border: 2px solid #8a1d14;
  box-shadow: inset 0 2px 0 rgba(255, 255, 255, .45); animation: hwrec-blink 1.4s ease-in-out infinite; vertical-align: -1px; }
.hwrec-dot.off { background: #c9b497; border-color: #8a6a4e; animation: none; }
@keyframes hwrec-blink { 50% { opacity: .35; } }
.hwrec-lob { display: flex; align-items: center; gap: 12px; margin-top: 10px; padding: 8px 12px; border: 3px solid #e2c48e; border-radius: 16px; background: #fff3d6; }
.hwrec-lob > div { flex: 1; min-width: 0; }
.hwrec-lob b { display: block; font: 20px/1.1 var(--toon, 'Lilita One', sans-serif); font-weight: 400; color: #5a3418; }
.hwrec-lob small { display: block; font: 700 13.5px/1.3 var(--read, Nunito, sans-serif); color: #8a6a4e; margin-top: 2px; }
.hwrec-lob[data-st=ready], .hwrec-lob[data-st=rec] { border-color: #a9d68f; background: #eef8e4; }
.hwrec-lob[data-st=denied], .hwrec-lob[data-st=none], .hwrec-lob[data-st=insecure] { border-color: #e7b0a4; background: #fde9e3; }
.hwrec-lob .k-btn { flex: none; }
.hwrec-ic { flex: none; line-height: 0; }
.hwrec-tools { margin: 8px 0 4px; display: flex; flex-direction: column; gap: 6px; }
.hwrec-st { display: inline-flex; align-items: center; gap: 6px; font-size: 16px; color: #5a3418; }
.hwrec-note { margin: 0; font: 700 13px/1.3 var(--read, Nunito, sans-serif); color: #a8661a; }
.hwrec-link { margin: 0; font: 800 14px var(--read, Nunito, sans-serif); }
.hwrec-link a { color: #8a5418; }
.hwrec-fin { display: flex; flex-wrap: wrap; align-items: center; justify-content: center; gap: 6px 14px; margin-top: 16px; padding-top: 12px; border-top: 3px dashed #e2c48e; }
.hwrec-fin .k-btn { text-decoration: none; display: inline-flex; align-items: center; gap: 8px; }
.hwrec-fin small { font: 700 14px/1.3 var(--read, Nunito, sans-serif); color: #8a6a4e; max-width: 26em; }
@media (prefers-reduced-motion: reduce) { .hwrec-dot { animation: none; } }`;
    document.head.appendChild(css);
    document.addEventListener('click', e => {
      const b = e.target.closest?.('[data-hwrec]');
      if (!b || b.disabled) return;
      const a = b.dataset.hwrec;
      if (a === 'allow') allow();
      else if (a === 'drop') drop();
      else if (a === 'off') setRecording(false);
      else if (a === 'on') setRecording(true);
      else if (a === 'here') takeOver();
    });
    const modal = document.getElementById('k-modal');
    if (modal && window.MutationObserver) new MutationObserver(() => { if (S.v?.isHost) inject(S.v, status(S.v)); }).observe(modal, { childList: true, subtree: true });
    try {
      navigator.permissions?.query({ name: 'microphone' }).then(p => {
        const set = () => {
          S.perm = p.state;
          if (p.state === 'denied') { S.mic = S.mic === 'insecure' ? S.mic : 'denied'; if (S.cur) stopClip(S.cur, true); closeMic(); }
          else if (S.mic === 'denied') S.mic = 'off';
          if (S.v) view(S.v);
        };
        set();
        p.onchange = set;
      }).catch(() => {});
    } catch { /* no permission API: ask with the button */ }
    addEventListener('online', () => flush());
    // a reload or closing the tab: hand the last seconds to IndexedDB (best effort)
    addEventListener('pagehide', () => { try { if (S.cur?.rec.state === 'recording') S.cur.rec.requestData(); } catch { } });
    setInterval(tick, 1000);
    takeLock();
  }

  window.HWRec = { view, get state() { return { ...S, device, mime: MIME, status: S.v ? status(S.v) : '' }; } };
})();
