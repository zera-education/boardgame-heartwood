// A sealed find (find.go): a one-off picture the forest turns up for one player.
// Every screen shows it over everything else: first a cute shock (a startled
// owl, the picture bouncing in, the forest's question to that player), then,
// when the Keeper moves it on, the celebration. The Keeper's screen and the
// phones call HWFind.view(v) with every view; the Keeper's screen also passes
// how long its own animations still run, so the tile has flipped first. Only
// the Keeper's screen plays sound (it is the one on the big screen); phones buzz.
(() => {
  const RM = matchMedia('(prefers-reduced-motion: reduce)');
  const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const keeper = () => !!HW.view?.isHost;
  let root = null, shown = '', stage = '', seen = false, timer = 0;

  const PLUM = '#5b2a6e', PINK = '#ff5f9a';
  const OUTLINE = (c, w = '.06em') => `0 ${w} 0 ${c}, ${w} 0 0 ${c}, -${w} 0 0 ${c}, 0 -${w} 0 ${c}, ${w} ${w} 0 ${c}, -${w} ${w} 0 ${c}, ${w} -${w} 0 ${c}, -${w} -${w} 0 ${c}`;
  const CSS = `
#hw-find { position: fixed; inset: 0; z-index: 1000; overflow: hidden; color: ${PLUM}; font-family: Nunito, system-ui, sans-serif; -webkit-user-select: none; user-select: none;
  background: radial-gradient(circle at 50% 45%, #fffaf3 0%, #ffe6ef 38%, #ffd1e3 68%, #e2cdf6 100%); }
#hw-find > i { position: absolute; inset: 0; pointer-events: none; }
.hf-lines { background: repeating-conic-gradient(from 0deg at 50% 45%, rgba(255, 255, 255, .75) 0 1.6deg, transparent 1.6deg 7deg);
  -webkit-mask: radial-gradient(circle at 50% 45%, transparent 28%, #000 62%); mask: radial-gradient(circle at 50% 45%, transparent 28%, #000 62%); transition: opacity 1s; }
.hf-sun { opacity: 0; background: radial-gradient(circle at 50% 42%, #fffbe8 0%, #ffeaa8 30%, #ffc9da 70%, #e9c8f5 100%); transition: opacity 1.2s; }
.hf-rays { opacity: 0; background: repeating-conic-gradient(from 0deg at 50% 42%, rgba(255, 255, 255, .45) 0 8deg, transparent 8deg 20deg); transition: opacity 1.2s; }
.hf-flash { opacity: 0; background: #fff4f8; }
.hf-floats span { position: absolute; bottom: -12vh; font: clamp(22px, 5vmin, 70px)/1 'Lilita One', Nunito, sans-serif; color: var(--c); opacity: .6; text-shadow: ${OUTLINE('#fff', '.05em')};
  animation: hf-float var(--t) linear var(--d) infinite; transition: opacity 1s; }
.hf-in { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 1vmin; padding: 2vmin 4vmin 1.6vmin; text-align: center; }
.hf-top { position: relative; display: flex; align-items: flex-end; justify-content: center; transition: transform .8s, opacity .8s; }
.hf-owl { width: clamp(64px, 9.5vmin, 130px); }
.hf-owl svg, .hf-bang svg { display: block; width: 100%; height: auto; overflow: visible; }
.hf-hop { animation: hf-hop 2.6s ease-in-out 3s infinite; transform-origin: 50% 100%; }
.hf-bang { position: absolute; left: 82%; top: -18%; width: clamp(34px, 5vmin, 70px); animation: hf-wiggle 1.4s ease-in-out infinite; }
.hf-alert { margin: 0; padding: .45em 1.2em; border-radius: 999px; background: #fff; border: .18em solid #ff8fb6; box-shadow: 0 .25em 0 #f2a6c4;
  font-weight: 900; font-size: clamp(14px, 2.2vmin, 28px); color: ${PLUM}; }
.hf-alert b { color: ${PINK}; }
.hf-pic { position: relative; margin: 2.2vmin 0 1vmin; transform: rotate(-2.5deg); transition: transform 1s cubic-bezier(.3, 1.5, .5, 1); }
.hf-pic img { display: block; max-width: min(86vw, 1060px); max-height: 38vh; width: auto; height: auto; border: clamp(7px, 1.1vmin, 14px) solid #fff; border-bottom-width: clamp(16px, 2.8vmin, 36px);
  border-radius: 1.2vmin; background: #000; box-shadow: 0 .8vmin 0 #f3b7cf, 0 2vmin 4.4vmin rgba(160, 60, 120, .35); }
.hf-tape { position: absolute; top: -2.2vmin; left: 50%; width: 15vmin; height: 4.2vmin; margin-left: -7.5vmin; transform: rotate(-4deg); opacity: .9;
  background: repeating-linear-gradient(45deg, #ffb3cc 0 .8vmin, #ffd3e1 .8vmin 1.6vmin); box-shadow: 0 1px 3px rgba(150, 50, 100, .25); }
.hf-stamp { position: absolute; right: -2.2vmin; bottom: 3vmin; padding: .35em .9em; border-radius: 999px; font: clamp(15px, 2.8vmin, 38px)/1.05 'Lilita One', Nunito, sans-serif; letter-spacing: .04em;
  color: #fff; background: ${PINK}; border: .14em solid #fff; box-shadow: 0 .18em 0 #c93a72, 0 .5em 1em rgba(160, 40, 90, .3); transform: rotate(-9deg); transition: background .5s, box-shadow .5s; }
.hf-spark { position: absolute; font: clamp(18px, 3.4vmin, 46px)/1 sans-serif; color: var(--c); text-shadow: 0 0 .3em #fff; animation: hf-twinkle 1.6s ease-in-out var(--d) infinite; }
.hf-by { margin: 0; font-weight: 900; font-size: clamp(11px, 1.8vmin, 22px); letter-spacing: .3em; text-transform: uppercase; color: #b0548a; }
.hf-q { margin: 0; font: clamp(26px, 4.8vmin, 70px)/1.12 'Lilita One', Nunito, sans-serif; color: ${PLUM}; text-shadow: ${OUTLINE('#fff')}, 0 .14em .3em rgba(160, 60, 120, .25); }
.hf-q > span { display: inline-block; margin: 0 .16em; }
.hf-name { display: inline-flex !important; align-items: center; gap: .3em; color: ${PINK}; }
.hf-name em { font-style: normal; display: inline-block; animation: hf-nervous 1.1s ease-in-out infinite; }
.hf-med { width: 1.15em; height: 1.15em; border-radius: 50%; border: .08em solid #fff; background: var(--c, #8d8f8a) center/cover; box-shadow: 0 0 0 .04em ${PINK}, 0 .1em .3em rgba(160, 60, 120, .35);
  display: inline-grid; place-items: center; font-size: .8em; color: #fff; text-shadow: none; }
.hf-you { margin: .3vmin 0 0; font-weight: 900; font-size: clamp(15px, 2.6vmin, 30px); color: #d6457a; }
.hf-yay { display: none; }
.hf-yay h2 { margin: 0; font: clamp(34px, 7.4vmin, 110px)/1 'Lilita One', Nunito, sans-serif; color: ${PINK}; text-shadow: ${OUTLINE('#fff', '.05em')}, 0 .12em .3em rgba(160, 60, 120, .3); }
.hf-yay p { margin: 1vmin 0 0; font: 800 clamp(18px, 3.4vmin, 46px)/1.25 Nunito, sans-serif; color: ${PLUM}; text-shadow: ${OUTLINE('#fff', '.04em')}; }
.hf-keep { min-height: 46px; display: flex; flex-wrap: wrap; gap: 10px; align-items: center; justify-content: center; margin-top: 1vmin; }
.hf-keep small { width: 100%; font-weight: 800; font-size: 14px; color: rgba(91, 42, 110, .6); }
.hf-btn { font: 20px/1 'Lilita One', Nunito, sans-serif; padding: 12px 22px; border-radius: 16px; border: 3px solid ${PLUM}; cursor: pointer; color: ${PLUM}; background: linear-gradient(#fff3b0, #ffc94a); box-shadow: 0 4px 0 ${PLUM}; }
.hf-btn.ghost { background: rgba(255, 255, 255, .6); border-color: rgba(91, 42, 110, .45); box-shadow: none; font-size: 17px; }
.hf-btn:active { transform: translateY(2px); box-shadow: 0 2px 0 ${PLUM}; }
.hf-confetti { overflow: hidden; }
.hf-c { position: absolute; top: 0; width: 1.1vmin; height: 1.7vmin; min-width: 7px; min-height: 11px; border-radius: 2px; will-change: transform; animation: hf-fall var(--t) linear var(--d) infinite; }
.hf-c.leaf { border-radius: 0 80% 0 80%; width: 1.6vmin; height: 1.6vmin; }
.hf-c.heart { background: none !important; width: auto; height: auto; color: var(--c); font: clamp(16px, 2.6vmin, 34px)/1 sans-serif; }

/* the celebration */
#hw-find.hf-cheer .hf-lines, #hw-find.hf-cheer .hf-floats span { opacity: 0; }
#hw-find.hf-cheer .hf-sun { opacity: 1; }
#hw-find.hf-cheer .hf-rays { opacity: 1; animation: hf-spin 40s linear infinite; }
#hw-find.hf-cheer .hf-top { opacity: 0; transform: translateY(-30vh); position: absolute; }
#hw-find.hf-cheer .hf-alert, #hw-find.hf-cheer .hf-ask { display: none; }
#hw-find.hf-cheer .hf-yay { display: block; }
#hw-find.hf-cheer .hf-pic { transform: rotate(0) scale(1.03); }
#hw-find.hf-cheer .hf-stamp { background: #3fbf80; box-shadow: 0 .18em 0 #23895a, 0 .5em 1em rgba(30, 120, 80, .3); }

/* the reveal, played once as it happens (not on a reload) */
#hw-find.hf-play .hf-flash { animation: hf-flash .6s ease-out both; }
#hw-find.hf-play .hf-lines { animation: hf-zoom .5s .05s cubic-bezier(.2, 1.6, .4, 1) both, hf-zoom .45s 1.4s cubic-bezier(.2, 1.6, .4, 1) both; }
#hw-find.hf-play .hf-in { animation: hf-boing .7s .05s both, hf-jolt .5s 1.5s both; }
#hw-find.hf-play .hf-owl { animation: hf-owlin .7s .25s cubic-bezier(.3, 1.6, .5, 1) both; }
#hw-find.hf-play .hf-bang { animation: hf-pop .45s .75s cubic-bezier(.3, 2, .5, 1) both, hf-wiggle 1.4s 1.2s ease-in-out infinite; }
#hw-find.hf-play .hf-alert { animation: hf-pop .5s .95s cubic-bezier(.3, 1.8, .5, 1) both; }
#hw-find.hf-play .hf-pic { animation: hf-drop .8s 1.25s cubic-bezier(.3, 1.35, .5, 1) both; }
#hw-find.hf-play .hf-stamp { animation: hf-sticker .45s 2.2s cubic-bezier(.3, 2, .5, 1) both; }
#hw-find.hf-play .hf-spark { animation: hf-pop .4s calc(2s + var(--d)) both, hf-twinkle 1.6s calc(2.4s + var(--d)) ease-in-out infinite; }
#hw-find.hf-play .hf-by { animation: hf-up .5s 2.9s both; }
#hw-find.hf-play .hf-q > span { animation: hf-word .6s var(--d) cubic-bezier(.3, 1.8, .5, 1) both; }
#hw-find.hf-play .hf-you { animation: hf-up .5s 6s both; }
#hw-find.hf-play .hf-keep { animation: hf-up .5s 6.4s both; }
#hw-find.hf-yay-in .hf-flash { animation: hf-flash .9s ease-out both; }
#hw-find.hf-yay-in .hf-yay h2 { animation: hf-word .9s .2s cubic-bezier(.2, 1.6, .4, 1) both; }
#hw-find.hf-yay-in .hf-yay p { animation: hf-up .7s .8s both; }
#hw-find.hf-out { animation: hf-fadeout .6s forwards; }

@keyframes hf-flash { 0% { opacity: 1; } 100% { opacity: 0; } }
@keyframes hf-zoom { 0% { transform: scale(1.35); opacity: .3; } 100% { transform: none; opacity: 1; } }
@keyframes hf-boing { 0% { transform: scale(.9); } 40% { transform: scale(1.04); } 70% { transform: scale(.985); } 100% { transform: none; } }
@keyframes hf-jolt { 0%, 100% { transform: none; } 25% { transform: translateY(-1.4vmin) scale(1.01); } 50% { transform: translateY(.6vmin); } 75% { transform: translateY(-.3vmin); } }
@keyframes hf-owlin { 0% { opacity: 0; transform: translateY(8vmin) scale(.4, .6); } 60% { opacity: 1; transform: translateY(-2vmin) scale(1.1, .92); } 100% { transform: none; } }
@keyframes hf-hop { 0%, 70%, 100% { transform: none; } 78% { transform: scale(1.08, .9); } 86% { transform: translateY(-14%) scale(.95, 1.06); } 94% { transform: scale(1.04, .96); } }
@keyframes hf-wiggle { 0%, 100% { transform: rotate(-8deg); } 50% { transform: rotate(10deg) scale(1.08); } }
@keyframes hf-pop { 0% { opacity: 0; transform: scale(0); } 100% { opacity: 1; transform: none; } }
@keyframes hf-drop { 0% { opacity: 0; transform: translateY(-55vh) rotate(10deg); } 55% { opacity: 1; transform: translateY(1.5vmin) rotate(-4deg) scale(1.03, .96); } 80% { transform: translateY(-.8vmin) rotate(-2deg); } 100% { opacity: 1; transform: rotate(-2.5deg); } }
@keyframes hf-sticker { 0% { opacity: 0; transform: rotate(-30deg) scale(2.4); } 100% { opacity: 1; transform: rotate(-9deg); } }
@keyframes hf-twinkle { 0%, 100% { transform: scale(.7) rotate(0); opacity: .6; } 50% { transform: scale(1.15) rotate(20deg); opacity: 1; } }
@keyframes hf-up { 0% { opacity: 0; transform: translateY(14px); } 100% { opacity: 1; transform: none; } }
@keyframes hf-word { 0% { opacity: 0; transform: scale(0) rotate(-6deg); } 100% { opacity: 1; transform: none; } }
@keyframes hf-nervous { 0%, 100% { transform: rotate(-3deg); } 50% { transform: rotate(3deg) translateY(-.04em); } }
@keyframes hf-float { 0% { transform: translateY(0) rotate(-12deg); } 50% { transform: translate(var(--sx), -60vh) rotate(12deg); } 100% { transform: translateY(-125vh) rotate(-12deg); } }
@keyframes hf-spin { to { transform: rotate(360deg); } }
@keyframes hf-fall { 0% { transform: translate3d(0, -6vh, 0) rotate(0); } 100% { transform: translate3d(var(--dx), 108vh, 0) rotate(var(--r)); } }
@keyframes hf-fadeout { to { opacity: 0; } }
@media (prefers-reduced-motion: reduce) { #hw-find *, #hw-find { animation: none !important; transition: none !important; } .hf-c, .hf-floats { display: none; } }
@media (orientation: portrait) { .hf-pic img { max-height: 34vh; } .hf-in { gap: 1.8vmin; } }
`;

  // a startled little owl, and its "!?"
  const OWL = `<svg viewBox="-42 -40 84 82" aria-hidden="true">
    <path d="M-27,-2 Q-42,-14 -38,-27 Q-31,-15 -23,-12Z M27,-2 Q42,-14 38,-27 Q31,-15 23,-12Z" fill="#9b7fc0" stroke="#4e2f66" stroke-width="2.6" stroke-linejoin="round"/>
    <path d="M-26,30 C-34,8 -31,-14 -25,-30 L-12,-19 Q0,-23 12,-19 L25,-30 C31,-14 34,8 26,30 Q0,40 -26,30Z" fill="#b597d6" stroke="#4e2f66" stroke-width="3" stroke-linejoin="round"/>
    <path d="M-15,15 Q0,5 15,15 Q13,31 0,33 Q-13,31 -15,15Z" fill="#fff4e2" stroke="#4e2f66" stroke-width="2"/>
    <circle cx="-11.5" cy="-5" r="11.5" fill="#fff" stroke="#4e2f66" stroke-width="2.6"/><circle cx="11.5" cy="-5" r="11.5" fill="#fff" stroke="#4e2f66" stroke-width="2.6"/>
    <circle cx="-11" cy="-4.5" r="4.6" fill="#2a1a30"/><circle cx="11" cy="-4.5" r="4.6" fill="#2a1a30"/>
    <circle cx="-12.8" cy="-6.6" r="1.8" fill="#fff"/><circle cx="9.2" cy="-6.6" r="1.8" fill="#fff"/>
    <ellipse cx="-21" cy="8" rx="5" ry="2.8" fill="#ff8fb6" opacity=".8"/><ellipse cx="21" cy="8" rx="5" ry="2.8" fill="#ff8fb6" opacity=".8"/>
    <ellipse cx="0" cy="9" rx="3" ry="4" fill="#f6a640" stroke="#8a5418" stroke-width="1.3"/>
    <path d="M30,-30 q4,7 0,10 q-4,-3 0,-10Z" fill="#9fd8ff" stroke="#4e2f66" stroke-width="1.4"/></svg>`;
  const BANG = `<svg viewBox="0 0 60 50" aria-hidden="true"><g font-family="Lilita One, Nunito, sans-serif" font-size="46" stroke="#fff" stroke-width="7" paint-order="stroke" stroke-linejoin="round">
    <text x="4" y="42" fill="${PINK}" transform="rotate(-10 14 30)">!</text><text x="22" y="40" fill="#8e63d6" transform="rotate(12 34 28)">?</text></g></svg>`;

  function medal(p) {
    if (!p) return '';
    const photo = p.photo > 0 ? `background-image:url('/api/games/${encodeURIComponent(HW.code)}/photo/${encodeURIComponent(p.id)}?v=${p.photo}');` : '';
    return `<span class="hf-med" style="--c:${esc(p.color)};${photo}">${photo ? '' : esc((p.name || '?')[0].toUpperCase())}</span>`;
  }

  // question marks drifting up behind everything, and sparkles around the picture
  function floats() {
    const cs = ['#ff7aa8', '#a77be0', '#5cc8a8', '#ffb347', '#6fb6ff'];
    let h = '';
    for (let k = 0; k < 16; k++) h += `<span style="left:${(k * 6.3 + Math.random() * 4).toFixed(1)}%;--c:${cs[k % cs.length]};--t:${(7 + Math.random() * 5).toFixed(1)}s;--d:${(-Math.random() * 12).toFixed(1)}s;--sx:${((Math.random() - .5) * 8).toFixed(1)}vw">${k % 3 ? '?' : '!'}</span>`;
    return h;
  }
  const SPARKS = [[-4, 12, '#ffc83d', 0], [101, 6, '#ff7aa8', .3], [-5, 70, '#a77be0', .6], [102, 58, '#ffc83d', .15], [30, -9, '#5cc8a8', .45], [72, 104, '#ff7aa8', .75]]
    .map(([x, y, c, d]) => `<span class="hf-spark" style="left:${x}%;top:${y}%;--c:${c};--d:${d}s">✦</span>`).join('');

  function markup(v, f) {
    const p = (v.players || []).find(x => x.id === f.player), first = (p?.name || '').trim().split(/\s+/)[0] || 'You';
    const lines = String(f.question || '').split(/(?<=[?.!])\s+/).filter(Boolean);
    const words = [`<span class="hf-name" style="--d:3.3s">${medal(p)}<em>${esc(first)}…</em></span><br>`,
      ...lines.map((l, k) => `<span style="--d:${(4.2 + k * 1.1).toFixed(1)}s">${esc(l)}</span>${k < lines.length - 1 ? '<br>' : ''}`)];
    const me = v.me?.id === f.player;
    return `<i class="hf-sun"></i><i class="hf-rays"></i><i class="hf-lines"></i><i class="hf-floats">${floats()}</i><i class="hf-confetti"></i>
      <div class="hf-in">
        <div class="hf-top"><div class="hf-owl"><div class="hf-hop">${OWL}</div></div><div class="hf-bang">${BANG}</div></div>
        <p class="hf-alert"><b>!?</b> The forest found something… unexpected</p>
        <figure class="hf-pic"><img src="${esc(f.image)}" alt="The picture ${esc(first)} found"><span class="hf-tape"></span>${SPARKS}<span class="hf-stamp"></span></figure>
        <div class="hf-ask"><p class="hf-by">The forest asks</p><p class="hf-q">${words.join('')}</p>
          ${me ? '<p class="hf-you">Yes, you! Everyone is waiting 👀</p>' : ''}</div>
        <div class="hf-yay"><h2>Congratulations, ${esc(first)}!</h2>${f.cheer ? `<p>${esc(f.cheer)}</p>` : ''}</div>
        <div class="hf-keep"></div>
      </div><i class="hf-flash"></i>`;
  }

  function keeperButtons(st) {
    if (!keeper()) return '';
    return st === 'ask'
      ? '<small>Keeper: when the explaining is done</small><button class="hf-btn" data-hf="foundCheer" data-enter="2">Explained</button><button class="hf-btn ghost" data-hf="foundClose">Close</button>'
      : '<button class="hf-btn" data-hf="foundClose" data-enter="2">Back to the forest</button>';
  }

  function confetti() {
    const box = root?.querySelector('.hf-confetti');
    if (!box || box.childElementCount || RM.matches) return;
    const colors = ['#ffd34a', '#ff8fb6', '#7ad3a0', '#8cc8ff', '#c9a4ff', '#ffb38a', '#ffffff'];
    let h = '';
    for (let k = 0; k < 110; k++) {
      const kind = k % 7 === 0 ? 'heart' : k % 5 === 0 ? 'leaf' : '', c = kind === 'leaf' ? '#5cbf6a' : colors[k % colors.length];
      h += `<span class="hf-c${kind ? ' ' + kind : ''}" style="left:${(Math.random() * 100).toFixed(1)}%;background:${c};--c:${kind === 'heart' ? '#ff6f9f' : c};--t:${(3.4 + Math.random() * 3).toFixed(2)}s;--d:${(-Math.random() * 6).toFixed(2)}s;--dx:${((Math.random() - .5) * 30).toFixed(1)}vw;--r:${Math.round((Math.random() - .5) * 1080)}deg">${kind === 'heart' ? '♥' : ''}</span>`;
    }
    box.innerHTML = h;
  }

  function setStage(v, f, live) {
    stage = f.stage;
    root.classList.toggle('hf-cheer', stage === 'cheer');
    root.querySelector('.hf-stamp').textContent = stage === 'cheer' ? 'Explained ♥' : '???';
    root.querySelector('.hf-keep').innerHTML = keeperButtons(stage);
    if (stage === 'cheer') {
      confetti();
      if (live) {
        root.classList.remove('hf-play'); void root.offsetWidth; root.classList.add('hf-yay-in');
        sound(cheerSound); buzz([70, 50, 70, 50, 70, 50, 220]);
      }
    } else if (live) { sound(surpriseSound); buzz([120, 80, 120, 80, 260]); }
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
        // the picture is ready before it bounces in
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

  function tone(c, t, { type = 'sine', f, f2, dur, vol = .3, cut = 8000, vib = 0, rate = 5.5, decay = false }) {
    const o = c.createOscillator(), g = c.createGain(), lp = c.createBiquadFilter();
    o.type = type; o.frequency.setValueAtTime(f, t);
    if (f2) o.frequency.exponentialRampToValueAtTime(f2, t + dur);
    if (vib) { const l = c.createOscillator(), lg = c.createGain(); l.frequency.value = rate; lg.gain.value = vib; l.connect(lg).connect(o.frequency); l.start(t); l.stop(t + dur + .5); }
    lp.type = 'lowpass'; lp.frequency.value = cut;
    g.gain.setValueAtTime(0, t); g.gain.linearRampToValueAtTime(vol, t + .012);
    if (decay) g.gain.exponentialRampToValueAtTime(.0005, t + dur);
    else { g.gain.setValueAtTime(vol, t + Math.max(.02, dur - .08)); g.gain.exponentialRampToValueAtTime(.0005, t + dur + .25); }
    o.connect(lp).connect(g).connect(out); o.start(t); o.stop(t + dur + .3);
  }
  // a marimba note: the tone and its bright overtone, both ringing out
  const mallet = (c, t, f, dur = .45, vol = .32) => { tone(c, t, { f, dur, vol, decay: true }); tone(c, t, { f: f * 4, dur: .08, vol: vol * .35, decay: true }); };
  function surpriseSound(c, t) {
    // pop!, a slide whistle up, and a boing
    const n = Math.floor(c.sampleRate * .06), buf = c.createBuffer(1, n, c.sampleRate), d = buf.getChannelData(0);
    for (let i = 0; i < n; i++) d[i] = (Math.random() * 2 - 1) * (1 - i / n);
    const src = c.createBufferSource(), bp = c.createBiquadFilter(), g = c.createGain();
    src.buffer = buf; bp.type = 'bandpass'; bp.frequency.value = 1400; bp.Q.value = 1.5; g.gain.value = .9;
    src.connect(bp).connect(g).connect(out); src.start(t);
    tone(c, t + .05, { f: 520, f2: 1560, dur: .38, vol: .2, vib: 22, rate: 9 });
    tone(c, t + .6, { f: 180, f2: 260, dur: .55, vol: .35, vib: 60, rate: 15, decay: true });
    // dun, dun, duuun on the marimba as the picture lands: dramatic, but small
    const s = t + 1.35;
    mallet(c, s, 784); mallet(c, s + .26, 740);
    [0, .07, .14, .21, .28, .35, .42, .49].forEach((dt, k) => mallet(c, s + .56 + dt, 622.3, .5, .3 * (1 - k * .09)));
  }
  function cheerSound(c, t) {
    [523.25, 659.25, 783.99, 1046.5, 1318.5].forEach((f, k) => tone(c, t + k * .09, { type: 'triangle', f, dur: .5, vol: .22 }));
    [523.25, 659.25, 783.99].forEach(f => tone(c, t + .5, { type: 'triangle', f, dur: 1.2, vol: .13 }));
    for (let k = 0; k < 14; k++) tone(c, t + .4 + Math.random() * 1.6, { f: 1800 + Math.random() * 2200, dur: .06, vol: .06 });
  }

  window.HWFind = { view };
})();
