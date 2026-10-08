// A sealed find (find.go): a one-off picture the forest turns up for one player.
// Every screen shows it over everything else: first the shock and the forest's
// question to that player, then, when the Keeper moves it on, the celebration.
// The Keeper's screen and the phones call HWFind.view(v) with every view; the
// Keeper's screen also passes how long its own animations still run, so the
// tile has flipped before the lights go out. Only the Keeper's screen plays
// sound (it is the one on the big screen); phones buzz.
(() => {
  const RM = matchMedia('(prefers-reduced-motion: reduce)');
  const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const keeper = () => !!HW.view?.isHost;
  let root = null, shown = '', stage = '', seen = false, timer = 0;

  const CSS = `
#hw-find { position: fixed; inset: 0; z-index: 1000; overflow: hidden; color: #fff; font-family: Nunito, system-ui, sans-serif; background: #050203; -webkit-user-select: none; user-select: none; }
#hw-find > i { position: absolute; inset: 0; pointer-events: none; }
.hf-red { background: radial-gradient(ellipse at 50% 46%, transparent 38%, rgba(150, 0, 0, .5) 72%, rgba(90, 0, 0, .92)); animation: hf-pulse 1.25s ease-in-out infinite; transition: opacity 1.2s; }
.hf-scan { background: repeating-linear-gradient(0deg, rgba(255, 255, 255, .035) 0 2px, transparent 2px 5px); mix-blend-mode: screen; transition: opacity 1s; }
.hf-gold { opacity: 0; background: radial-gradient(ellipse at 50% 42%, #ffe08a 0%, #f2ad38 34%, #c25f10 78%, #6d2d06); transition: opacity 1.4s; }
.hf-rays { opacity: 0; background: repeating-conic-gradient(from 0deg at 50% 42%, rgba(255, 255, 255, .2) 0 7deg, transparent 7deg 18deg); transition: opacity 1.4s; }
.hf-flash { opacity: 0; background: #fff; }
.hf-in { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 1.4vmin; padding: 3vmin 4vmin 2vmin; text-align: center; }
.hf-eye { width: clamp(84px, 12vmin, 170px); filter: drop-shadow(0 0 14px rgba(255, 70, 40, .85)) drop-shadow(0 0 40px rgba(255, 40, 20, .5)); transition: transform .9s cubic-bezier(.5, 0, .3, 1), opacity .9s; }
.hf-eye svg { display: block; width: 100%; height: auto; overflow: visible; }
.hf-look { animation: hf-look 5.5s ease-in-out 2s infinite; }
.hf-alert { margin: 0; font-weight: 900; font-size: clamp(13px, 2.3vmin, 28px); letter-spacing: .2em; text-transform: uppercase; color: #ff5b47; text-shadow: 0 0 12px rgba(255, 50, 30, .9); }
.hf-pic { position: relative; margin: 2.6vmin 0 1vmin; transform: rotate(-2.2deg); transition: transform 1.2s cubic-bezier(.3, 1.4, .5, 1); }
.hf-pic img { display: block; max-width: min(88vw, 1100px); max-height: 46vh; width: auto; height: auto; border: clamp(6px, 1.1vmin, 14px) solid #f3eee2; border-bottom-width: clamp(14px, 2.6vmin, 34px); border-radius: 3px; background: #000;
  box-shadow: 0 0 0 1px rgba(0, 0, 0, .4), 0 18px 50px rgba(0, 0, 0, .8), 0 0 70px rgba(255, 40, 20, .35); transition: box-shadow 1.2s; }
.hf-tape { position: absolute; top: -2.2vmin; left: 50%; width: 16vmin; height: 4.4vmin; margin-left: -8vmin; transform: rotate(3deg); background: rgba(255, 244, 200, .55); box-shadow: 0 1px 4px rgba(0, 0, 0, .3); }
.hf-stamp { position: absolute; right: -2vmin; bottom: 3.4vmin; padding: .4vmin 1.8vmin; font: 900 clamp(14px, 3vmin, 40px)/1.1 Nunito, sans-serif; letter-spacing: .12em; text-transform: uppercase;
  color: #ff3b30; border: .55vmin solid currentColor; border-radius: 1vmin; transform: rotate(-12deg); background: rgba(20, 0, 0, .35); text-shadow: 0 0 8px rgba(255, 40, 20, .6); transition: color .6s; }
.hf-by { margin: 0; font-weight: 900; font-size: clamp(11px, 1.8vmin, 22px); letter-spacing: .32em; text-transform: uppercase; color: #ffcf6a; opacity: .9; }
.hf-q { margin: 0; font: clamp(26px, 5.2vmin, 76px)/1.12 'Lilita One', Nunito, sans-serif; text-shadow: 0 0 22px rgba(255, 70, 40, .7), 0 3px 0 #2a0505; }
.hf-q > span { display: inline-block; margin: 0 .18em; }
.hf-name { display: inline-flex !important; align-items: center; gap: .3em; color: #ffd35a; }
.hf-med { width: 1.15em; height: 1.15em; border-radius: 50%; border: .08em solid #fff3dc; background: var(--c, #8d8f8a) center/cover; box-shadow: 0 0 0 .05em rgba(0, 0, 0, .4), 0 0 24px rgba(255, 210, 90, .8); display: inline-grid; place-items: center; font-size: .8em; color: #fff; }
.hf-you { margin: .4vmin 0 0; font-weight: 900; font-size: clamp(15px, 2.6vmin, 30px); color: #ffcf6a; }
.hf-yay { display: none; }
.hf-yay h2 { margin: 0; font: clamp(34px, 7.4vmin, 110px)/1 'Lilita One', Nunito, sans-serif; color: #fff; text-shadow: 0 .07em 0 #6e2c04, .04em .04em 0 #6e2c04, -.04em .04em 0 #6e2c04, .04em -.03em 0 #6e2c04, -.04em -.03em 0 #6e2c04, 0 0 28px rgba(110, 44, 4, .45); }
.hf-yay p { margin: 1vmin 0 0; font: 800 clamp(18px, 3.4vmin, 46px)/1.25 Nunito, sans-serif; color: #4a1c02; text-shadow: 0 1px 0 rgba(255, 255, 255, .6); }
.hf-keep { min-height: 46px; display: flex; flex-wrap: wrap; gap: 10px; align-items: center; justify-content: center; margin-top: 1vmin; }
.hf-keep small { width: 100%; font-weight: 800; font-size: 14px; color: rgba(255, 255, 255, .6); }
.hf-btn { font: 20px/1 'Lilita One', Nunito, sans-serif; padding: 12px 22px; border-radius: 14px; border: 3px solid #2e1709; cursor: pointer; color: #2e1709; background: linear-gradient(#ffe39a, #f3b43a); box-shadow: 0 4px 0 #2e1709; }
.hf-btn.ghost { color: #fff; background: rgba(255, 255, 255, .1); border-color: rgba(255, 255, 255, .45); box-shadow: none; font-size: 17px; }
.hf-btn:active { transform: translateY(2px); box-shadow: 0 2px 0 #2e1709; }
.hf-confetti { overflow: hidden; }
.hf-c { position: absolute; top: 0; width: 1.1vmin; height: 1.7vmin; min-width: 7px; min-height: 11px; border-radius: 2px; will-change: transform; animation: hf-fall var(--t) linear var(--d) infinite; }
.hf-c.leaf { border-radius: 0 80% 0 80%; width: 1.6vmin; height: 1.6vmin; }

/* the celebration */
#hw-find.hf-cheer .hf-red, #hw-find.hf-cheer .hf-scan { opacity: 0; }
#hw-find.hf-cheer .hf-gold { opacity: 1; }
#hw-find.hf-cheer .hf-rays { opacity: 1; animation: hf-spin 40s linear infinite; }
#hw-find.hf-cheer .hf-eye { opacity: 0; transform: scaleY(.05); position: absolute; }
#hw-find.hf-cheer .hf-alert, #hw-find.hf-cheer .hf-ask { display: none; }
#hw-find.hf-cheer .hf-yay { display: block; }
#hw-find.hf-cheer .hf-pic { transform: rotate(0) scale(1.03); }
#hw-find.hf-cheer .hf-pic img { box-shadow: 0 0 0 1px rgba(0, 0, 0, .3), 0 18px 50px rgba(80, 30, 0, .55), 0 0 90px rgba(255, 255, 255, .8); }
#hw-find.hf-cheer .hf-stamp { color: #2f8f3a; background: rgba(255, 255, 255, .55); text-shadow: none; }
#hw-find.hf-cheer .hf-keep small { color: rgba(60, 20, 0, .6); }
#hw-find.hf-cheer .hf-btn.ghost { color: #4a1c02; border-color: rgba(74, 28, 2, .4); }

/* the reveal, played once as it happens (not on a reload) */
#hw-find.hf-play .hf-flash { animation: hf-flash .7s ease-out both; }
#hw-find.hf-play .hf-in { animation: hf-shake .6s .05s both, hf-shake .45s 1.45s both; }
#hw-find.hf-play .hf-alert { animation: hf-flick 1s .35s both; }
#hw-find.hf-play .hf-eye { animation: hf-open .7s .9s cubic-bezier(.3, 1.6, .5, 1) both; }
#hw-find.hf-play .hf-pic { animation: hf-slam .65s 1.4s cubic-bezier(.2, 1.3, .4, 1) both; }
#hw-find.hf-play .hf-pic img { animation: hf-glitch .55s 1.45s steps(1) both; }
#hw-find.hf-play .hf-stamp { animation: hf-stamp .35s 2.4s cubic-bezier(.3, 1.8, .5, 1) both; }
#hw-find.hf-play .hf-by { animation: hf-up .6s 2.9s both; }
#hw-find.hf-play .hf-q > span { animation: hf-word .7s var(--d) cubic-bezier(.2, 1.4, .4, 1) both; }
#hw-find.hf-play .hf-you { animation: hf-up .6s 6s both; }
#hw-find.hf-play .hf-keep { animation: hf-up .6s 6.4s both; }
#hw-find.hf-yay-in .hf-flash { animation: hf-goldflash 1s ease-out both; }
#hw-find.hf-yay-in .hf-yay h2 { animation: hf-pop .9s .2s cubic-bezier(.2, 1.6, .4, 1) both; }
#hw-find.hf-yay-in .hf-yay p { animation: hf-up .7s .8s both; }
#hw-find.hf-out { animation: hf-fadeout .6s forwards; }

@keyframes hf-pulse { 0%, 100% { opacity: .75; } 18% { opacity: 1; } 34% { opacity: .82; } 48% { opacity: 1; } }
@keyframes hf-flash { 0% { opacity: 1; } 100% { opacity: 0; } }
@keyframes hf-goldflash { 0% { opacity: .95; background: #fff8d8; } 100% { opacity: 0; } }
@keyframes hf-shake { 0%, 100% { transform: none; } 10% { transform: translate(-14px, 6px) rotate(-.6deg); } 25% { transform: translate(12px, -8px) rotate(.5deg); }
  40% { transform: translate(-9px, -4px); } 55% { transform: translate(8px, 6px) rotate(.3deg); } 70% { transform: translate(-5px, 2px); } 85% { transform: translate(3px, -2px); } }
@keyframes hf-flick { 0% { opacity: 0; } 8% { opacity: 1; } 14% { opacity: .1; } 22% { opacity: 1; } 30% { opacity: .2; } 38%, 100% { opacity: 1; } }
@keyframes hf-open { 0% { opacity: 0; transform: scaleY(.04); } 100% { opacity: 1; transform: none; } }
@keyframes hf-look { 0%, 16%, 100% { transform: none; } 24%, 40% { transform: translateX(-13px); } 50%, 64% { transform: translateX(13px); } 72% { transform: none; } }
@keyframes hf-slam { 0% { opacity: 0; transform: scale(1.9) rotate(-9deg); } 60% { opacity: 1; } 100% { opacity: 1; transform: rotate(-2.2deg); } }
@keyframes hf-glitch { 0% { clip-path: inset(10% 0 62% 0); transform: translate(-18px, 0); filter: hue-rotate(90deg) saturate(4); } 15% { clip-path: inset(55% 0 12% 0); transform: translate(16px, 0); }
  30% { clip-path: inset(30% 0 40% 0); transform: translate(-8px, 0); filter: invert(1); } 45% { clip-path: inset(0 0 80% 0); transform: translate(10px, 0); filter: none; }
  60% { clip-path: inset(70% 0 0 0); transform: translate(-6px, 0); filter: hue-rotate(-60deg) saturate(3); } 75%, 100% { clip-path: inset(0); transform: none; filter: none; } }
@keyframes hf-stamp { 0% { opacity: 0; transform: rotate(-12deg) scale(3); } 100% { opacity: 1; transform: rotate(-12deg); } }
@keyframes hf-up { 0% { opacity: 0; transform: translateY(14px); } 100% { opacity: 1; transform: none; } }
@keyframes hf-word { 0% { opacity: 0; transform: scale(1.8); filter: blur(10px); } 100% { opacity: 1; transform: none; filter: none; } }
@keyframes hf-pop { 0% { opacity: 0; transform: scale(.3) rotate(-6deg); } 100% { opacity: 1; transform: none; } }
@keyframes hf-spin { to { transform: rotate(360deg); } }
@keyframes hf-fall { 0% { transform: translate3d(0, -6vh, 0) rotate(0); } 100% { transform: translate3d(var(--dx), 108vh, 0) rotate(var(--r)); } }
@keyframes hf-fadeout { to { opacity: 0; } }
@media (prefers-reduced-motion: reduce) { #hw-find *, #hw-find { animation: none !important; transition: none !important; } .hf-c { display: none; } }
@media (orientation: portrait) { .hf-pic img { max-height: 36vh; } .hf-in { gap: 1.8vmin; } }
`;

  const EYE = `<svg viewBox="-66 -36 132 72" aria-hidden="true"><defs>
    <radialGradient id="hfIris"><stop offset="0" stop-color="#fff6c8"/><stop offset=".42" stop-color="#ffcf3a"/><stop offset="1" stop-color="#b3330f"/></radialGradient>
    <clipPath id="hfLid"><path d="M-60 0Q0-58 60 0Q0 58-60 0Z"/></clipPath></defs>
    <path d="M-60 0Q0-58 60 0Q0 58-60 0Z" fill="#170606"/>
    <g clip-path="url(#hfLid)"><g class="hf-look"><circle r="21" fill="url(#hfIris)"/><circle r="8.5" fill="#0d0303"/><circle cx="-6" cy="-7" r="4" fill="#fff"/></g></g>
    <path d="M-60 0Q0-58 60 0Q0 58-60 0Z" fill="none" stroke="#ff6a4a" stroke-width="3.5"/>
    ${[-50, -25, 0, 25, 50].map(x => `<path d="M${x * .9} ${-30 + Math.abs(x) * .28}l${x * .1} -8" stroke="#ff6a4a" stroke-width="3" stroke-linecap="round"/>`).join('')}</svg>`;

  function medal(p) {
    if (!p) return '';
    const photo = p.photo > 0 ? `background-image:url('/api/games/${encodeURIComponent(HW.code)}/photo/${encodeURIComponent(p.id)}?v=${p.photo}');` : '';
    return `<span class="hf-med" style="--c:${esc(p.color)};${photo}">${photo ? '' : esc((p.name || '?')[0].toUpperCase())}</span>`;
  }

  function markup(v, f) {
    const p = (v.players || []).find(x => x.id === f.player), first = (p?.name || '').trim().split(/\s+/)[0] || 'You';
    const lines = String(f.question || '').split(/(?<=[?.!])\s+/).filter(Boolean);
    const words = [`<span class="hf-name" style="--d:3.3s">${medal(p)}${esc(first)}…</span><br>`,
      ...lines.map((l, k) => `<span style="--d:${(4.2 + k * 1.1).toFixed(1)}s">${esc(l)}</span>${k < lines.length - 1 ? '<br>' : ''}`)];
    const me = v.me?.id === f.player;
    return `<i class="hf-gold"></i><i class="hf-rays"></i><i class="hf-red"></i><i class="hf-scan"></i><i class="hf-confetti"></i>
      <div class="hf-in">
        <div class="hf-eye">${EYE}</div>
        <p class="hf-alert">⚠ Something that does not belong in the forest</p>
        <figure class="hf-pic"><img src="${esc(f.image)}" alt="The picture ${esc(first)} found"><span class="hf-tape"></span><span class="hf-stamp"></span></figure>
        <div class="hf-ask"><p class="hf-by">The forest asks</p><p class="hf-q">${words.join('')}</p>
          ${me ? '<p class="hf-you">Yes, you. Everyone is waiting.</p>' : ''}</div>
        <div class="hf-yay"><h2>Congratulations, ${esc(first)}!</h2>${f.cheer ? `<p>${esc(f.cheer)}</p>` : ''}</div>
        <div class="hf-keep"></div>
      </div><i class="hf-flash"></i>`;
  }

  function keeperButtons(st) {
    if (!keeper()) return '';
    return st === 'ask'
      ? '<small>Keeper: when the explaining is done</small><button class="hf-btn" data-hf="foundCheer">Celebrate 🎉</button><button class="hf-btn ghost" data-hf="foundClose">Close</button>'
      : '<button class="hf-btn" data-hf="foundClose">Back to the forest</button>';
  }

  function confetti() {
    const box = root?.querySelector('.hf-confetti');
    if (!box || box.childElementCount || RM.matches) return;
    const colors = ['#ffd34a', '#ff7a59', '#7ad36b', '#59b8ff', '#c58bff', '#ff9fd0', '#ffffff', '#3fae5a'];
    let h = '';
    for (let k = 0; k < 110; k++) {
      const leaf = k % 5 === 0, c = leaf ? '#4fb04a' : colors[k % colors.length];
      h += `<span class="hf-c${leaf ? ' leaf' : ''}" style="left:${(Math.random() * 100).toFixed(1)}%;background:${c};--t:${(3.2 + Math.random() * 3).toFixed(2)}s;--d:${(-Math.random() * 6).toFixed(2)}s;--dx:${((Math.random() - .5) * 30).toFixed(1)}vw;--r:${Math.round((Math.random() - .5) * 1440)}deg"></span>`;
    }
    box.innerHTML = h;
  }

  function setStage(v, f, live) {
    stage = f.stage;
    root.classList.toggle('hf-cheer', stage === 'cheer');
    root.querySelector('.hf-stamp').textContent = stage === 'cheer' ? 'Explained ♥' : 'Unexplained';
    root.querySelector('.hf-keep').innerHTML = keeperButtons(stage);
    if (stage === 'cheer') {
      confetti();
      if (live) {
        root.classList.remove('hf-play'); void root.offsetWidth; root.classList.add('hf-yay-in');
        sound(cheerSound); buzz([70, 50, 70, 50, 70, 50, 220]);
      }
    } else if (live) { sound(shockSound); buzz([400, 120, 400, 120, 900]); }
  }

  function build(f, instant) {
    const v = HW.view;
    root?.remove();
    root = document.createElement('div');
    root.id = 'hw-find';
    root.setAttribute('role', 'dialog');
    root.setAttribute('aria-modal', 'true');
    root.setAttribute('aria-label', 'Something was found in the forest');
    root.innerHTML = markup(v, f);
    if (!instant && !RM.matches) root.classList.add('hf-play');
    root.addEventListener('click', async e => {
      const b = e.target.closest('[data-hf]');
      if (!b || b.disabled) return;
      b.disabled = true;
      if (!await HW.act(HW.creds, b.dataset.hf)) b.disabled = false;
    });
    document.body.append(root);
    setStage(v, f, !instant);
  }

  function close() {
    clearTimeout(timer);
    shown = ''; stage = '';
    const el = root;
    root = null;
    if (!el) return;
    el.classList.add('hf-out');
    setTimeout(() => el.remove(), RM.matches ? 0 : 600);
  }

  // view(v, wait): call with every view; wait is how long the screen's own animations still run (ms).
  function view(v, wait = 0) {
    const instant = !seen;
    seen = true;
    const f = v?.found;
    if (!f) { if (shown) close(); return; }
    const id = `${f.player}|${f.image}`;
    if (id !== shown) {
      if (!document.getElementById('hw-find-css')) document.head.insertAdjacentHTML('beforeend', `<style id="hw-find-css">${CSS}</style>`);
      clearTimeout(timer);
      shown = id; stage = '';
      root?.remove(); root = null;
      const go = () => {
        // the picture is ready before the lights go out
        const img = new Image();
        img.src = f.image;
        const ready = img.decode ? img.decode().catch(() => {}) : Promise.resolve();
        Promise.race([ready, new Promise(r => setTimeout(r, 2500))]).then(() => {
          const now = HW.view?.found;
          if (shown === id && now) build(now, instant);
        });
      };
      if (instant || wait <= 60) go(); else timer = setTimeout(go, wait);
      return;
    }
    if (root && f.stage !== stage) setStage(v, f, true);
  }

  // ---- sound (the Keeper's screen only) and buzz (phones) ----
  let ac = null, out = null;
  function audio() {
    try {
      if (!ac) {
        ac = new (window.AudioContext || window.webkitAudioContext)();
        const comp = ac.createDynamicsCompressor();
        out = ac.createGain(); out.gain.value = .8;
        out.connect(comp).connect(ac.destination);
      }
      if (ac.state === 'suspended') ac.resume();
    } catch { ac = null; }
    return ac;
  }
  // browsers let a page play sound once someone has tapped it: the Keeper taps all game long
  addEventListener('pointerdown', () => { if (keeper() && !ac) audio(); }, true);
  function sound(fn) { if (keeper() && !RM.matches) { const c = audio(); if (c) try { fn(c, c.currentTime + .03); } catch {} } }
  function buzz(p) { if (!keeper() && navigator.userActivation?.hasBeenActive !== false) try { navigator.vibrate?.(p); } catch {} }

  function tone(c, t, { type = 'sawtooth', f, f2, dur, vol = .3, cut = 1600, vib = 0 }) {
    const o = c.createOscillator(), g = c.createGain(), lp = c.createBiquadFilter();
    o.type = type; o.frequency.setValueAtTime(f, t);
    if (f2) o.frequency.exponentialRampToValueAtTime(f2, t + dur);
    if (vib) { const l = c.createOscillator(), lg = c.createGain(); l.frequency.value = 5.5; lg.gain.value = vib; l.connect(lg).connect(o.frequency); l.start(t + .25); l.stop(t + dur + .1); }
    lp.type = 'lowpass'; lp.frequency.value = cut;
    g.gain.setValueAtTime(0, t); g.gain.linearRampToValueAtTime(vol, t + .025);
    g.gain.setValueAtTime(vol, t + Math.max(.03, dur - .12)); g.gain.exponentialRampToValueAtTime(.0005, t + dur + .35);
    o.connect(lp).connect(g).connect(out); o.start(t); o.stop(t + dur + .4);
  }
  function shockSound(c, t) {
    // the hit: a burst of noise and a falling boom
    const n = c.sampleRate * 1.4, buf = c.createBuffer(1, n, c.sampleRate), d = buf.getChannelData(0);
    for (let i = 0; i < n; i++) d[i] = (Math.random() * 2 - 1) * Math.pow(1 - i / n, 4);
    const src = c.createBufferSource(), lp = c.createBiquadFilter(), g = c.createGain();
    src.buffer = buf; lp.type = 'lowpass'; lp.frequency.setValueAtTime(2400, t); lp.frequency.exponentialRampToValueAtTime(90, t + 1.2); g.gain.value = .9;
    src.connect(lp).connect(g).connect(out); src.start(t);
    tone(c, t, { type: 'sine', f: 120, f2: 30, dur: 1.2, vol: .9, cut: 400 });
    // dun, dun, DUNNN as the picture lands
    const s = t + 1.35;
    [[0, 196, .2], [.3, 185, .2], [.62, 155.6, 1.7]].forEach(([at, f, dur], k) => {
      tone(c, s + at, { f, dur, vol: .26, cut: 1500, vib: k === 2 ? 5 : 0 });
      tone(c, s + at, { f: f * 1.006, dur, vol: .2, cut: 1300, vib: k === 2 ? 5 : 0 });
      tone(c, s + at, { type: 'sine', f: f / 2, dur, vol: .45, cut: 600 });
    });
  }
  function cheerSound(c, t) {
    [523.25, 659.25, 783.99, 1046.5, 1318.5].forEach((f, k) => tone(c, t + k * .09, { type: 'triangle', f, dur: .5, vol: .22, cut: 6000 }));
    [523.25, 659.25, 783.99].forEach(f => tone(c, t + .5, { type: 'triangle', f, dur: 1.2, vol: .13, cut: 5000 }));
    for (let k = 0; k < 14; k++) tone(c, t + .4 + Math.random() * 1.6, { type: 'sine', f: 1800 + Math.random() * 2200, dur: .06, vol: .06, cut: 9000 });
  }

  window.HWFind = { view };
})();
