// Game report: one page for a finished game, in the forest's look (web/art.js). The team with their photos and what
// each stood for, a short memory for each player, what we said together for each treasure, and the Secret Owls. Made on the Mini from the game's state, the player
// photos and the Mini's own copy of each story (hw-transcribe -archive). The page holds people's photos and stories:
// it goes to El's private dashboard, never into this public repo.
//
//   node ops/report/report.mjs CODE --out FILE [--server URL] [--archive DIR] [--photos DIR] [--state FILE]
//        [--memories FILE] [--transcripts] [--fallback STORIES.json] [--story [--pdf FILE]]
//
// --photos keeps the photos it fetched (the server deletes them 2 hours after the game ends; a photo already in the
// folder is used as it is). --fallback is the Stories page's JSON download, for a story the archive doesn't have.
// --memories is the summary someone wrote from the stories (a summary, not a transcript: the recordings are of a busy
// room): {intro, people: {name: {quote, memory}}, together: [{treasure, answers: [[name, answer]]}]}.
// --transcripts adds every story word for word, round by round.
// --story makes Instagram story pages instead (1080 × 1920, printable to PDF at that size): a cover with everyone round
// the World Tree, a page for each player's memory, a page for each treasure. --pdf puts a Download PDF button on it.
import { createRequire } from 'node:module';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const require = createRequire(import.meta.url);
const ART = require('../../web/art.js');

// ---------- arguments ----------
const args = process.argv.slice(2);
const opt = (name, dflt) => { const i = args.indexOf('--' + name); return i >= 0 ? args[i + 1] : dflt; };
const SWITCHES = new Set(['--transcripts', '--story']);
const CODE = (args.filter((a, i) => !a.startsWith('--') && !(i > 0 && args[i - 1].startsWith('--') && !SWITCHES.has(args[i - 1])))[0] || '').toUpperCase();
const TRANSCRIPTS = args.includes('--transcripts');
const STORY = args.includes('--story');
const SERVER = opt('server', process.env.HW_SERVER || 'https://heartwood.zera.edu.my').replace(/\/$/, '');
const ARCHIVE = opt('archive', path.join(os.homedir(), '.local/share/heartwood/stories'));
const PHOTOS = opt('photos', '');
const OUT = opt('out', '');
if (!/^[A-Z0-9]{2,12}$/.test(CODE) || !OUT) {
  console.error('usage: node ops/report/report.mjs CODE --out FILE [--server URL] [--archive DIR] [--photos DIR] [--state FILE] [--fallback FILE]');
  process.exit(4);
}

const getJSON = async url => { const r = await fetch(url); if (!r.ok) throw new Error(`${url}: ${r.status}`); return r.json(); };
const state = opt('state') ? JSON.parse(fs.readFileSync(opt('state'), 'utf8')) : await getJSON(`${SERVER}/api/games/${CODE}/state`);
let cards = {};
try { cards = await getJSON(`${SERVER}/api/cards`); } catch { /* the state carries values, roles and treasures */ }
const VALUES = state.values || cards.values || [];
const ROLES = state.roles || cards.roles || [];
const TREASURES = state.treasures || cards.treasures || [];
const DECKS = cards.ringDeckNames || ['', 'Deep', 'Story', 'Light'];
const DECK_COLOR = { 1: '#1f6e5a', 2: '#b8742a', 3: '#7cbf3a' };

// ---------- players and photos ----------
const order = (state.order?.length ? state.order : state.players.map(p => p.id));
const players = order.map(id => state.players.find(p => p.id === id)).filter(Boolean);
const byName = new Map(players.map(p => [p.name, p]));
const recog = new Map((state.recognition || []).map(r => [r.id, r]));
const photo = {};
if (PHOTOS) fs.mkdirSync(PHOTOS, { recursive: true, mode: 0o700 });
for (const p of players) {
  const file = PHOTOS && path.join(PHOTOS, p.id + '.jpg');
  let b = file && fs.existsSync(file) ? fs.readFileSync(file) : null;
  if (!b && p.photo) {
    const r = await fetch(`${SERVER}/api/games/${CODE}/photo/${p.id}`);
    if (r.ok) { b = Buffer.from(await r.arrayBuffer()); if (file) fs.writeFileSync(file, b, { mode: 0o600 }); }
  }
  if (b) photo[p.id] = `data:${b[0] === 0x89 ? 'image/png' : 'image/jpeg'};base64,${b.toString('base64')}`;
}

// ---------- stories ----------
const stories = [];
const dir = path.join(ARCHIVE, CODE);
if (fs.existsSync(dir)) for (const f of fs.readdirSync(dir).filter(f => f.endsWith('.json'))) stories.push(JSON.parse(fs.readFileSync(path.join(dir, f), 'utf8')));
if (opt('fallback')) {
  const have = new Set(stories.map(s => s.share));
  for (const x of JSON.parse(fs.readFileSync(opt('fallback'), 'utf8')).stories || []) {
    if (have.has(x.share)) continue;
    stories.push({ share: x.share, round: x.round, kind: x.kind, playerName: x.player, prompt: x.question, title: x.title,
      detail: x.detail, seq: +x.seq || 0, of: +x.of || 0, durationMs: x.duration_s * 1000, createdAt: Date.parse(x.recorded_at) / 1000,
      text: x.transcript, fallback: true });
  }
}
stories.sort((a, b) => a.share - b.share || a.createdAt - b.createdAt || (a.id || 0) - (b.id || 0));

// ---------- drawing ----------
const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const art = (spec, h) => ART.inline(spec, { height: h });
// A photo is drawn once (in the page's defs) and every medallion uses it.
const photoDefs = {};
function med(name, size, color) {
  const p = byName.get(name);
  const svg = ART.svg(ART.medallion({ photo: p && photo[p.id] ? `ph:${p.id}` : '', name, color: color || p?.color || '#8d8f8a', water: 5, r: 20, stump: false }),
    { viewBox: '-42 -44 84 86', size });
  return svg.replace(/<image ([^>]*?)href="ph:([^"]+)"([^>]*?)\/?>(<\/image>)?/, (m, a, id, b) => {
    const attrs = (a + b).trim(), clip = (attrs.match(/clip-path="[^"]*"/) || [''])[0];
    photoDefs[id] ??= `<image id="ph-${id}" href="${photo[id]}" ${attrs.replace(/clip-path="[^"]*"|filter="[^"]*"/g, '').trim()}/>`;
    return `<use href="#ph-${id}" ${clip}/>`;
  });
}
const valueOf = i => VALUES[i];
function label(s) {
  if (s.title && s.fallback) return { text: s.title, icon: s.kind === 'treasure' ? 'treasure' : s.kind === 'harvest' ? 'fruit' : 'icon:talk', color: '#5cb84a' };
  const v = valueOf(s.value);
  switch (s.kind) {
    case 'why': return { text: v ? `Why ${v.name}` : 'Why this value', icon: v ? `value:${s.value}` : 'icon:talk', color: v?.color || '#5cb84a' };
    case 'ring': { const n = +s.sub || 0; return { text: `${DECKS[n] || 'Ring'} · Ring ${n} card`, icon: n ? `icon:deck${n}` : 'icon:cards', color: DECK_COLOR[n] || '#b8742a' }; }
    case 'harvest': return { text: v ? `Harvest story · ${v.name}` : 'Harvest story', icon: 'fruit', color: '#e8344f' };
    case 'heartwood': return { text: 'Heartwood question', icon: 'icon:heart', color: '#8a5a36' };
    case 'treasure': { const t = TREASURES.find(x => x.id === s.sub); return { text: `${t ? t.name : 'Treasure'} · answer ${s.seq} of ${s.of}`, icon: t ? `treasure:${t.id}` : 'treasure', color: '#c98a1a' }; }
  }
  return { text: s.kind, icon: 'icon:talk', color: '#5cb84a' };
}
const subOf = s => s.fallback ? (s.kind === 'why' || s.kind === 'harvest' ? s.detail : '') : s.kind === 'why' || s.kind === 'harvest' ? s.sub : '';
const textHTML = s => (s.text || '').trim()
  ? `<div class="text">${s.text.trim().split(/\n{2,}/).map(p => `<p>${esc(p)}</p>`).join('')}</div>`
  : '<div class="text empty">Nothing could be made out in this recording.</div>';
const dur = ms => { const t = Math.round((ms || 0) / 1000); return `${Math.floor(t / 60)}:${String(t % 60).padStart(2, '0')}`; };
const TZ = 'Asia/Kuala_Lumpur';
const clock = sec => new Intl.DateTimeFormat('en-GB', { timeZone: TZ, hour: 'numeric', minute: '2-digit', hour12: true }).format(sec * 1000);
const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

function storyCard(s) {
  const k = label(s), sub = subOf(s);
  return `<article class="story" style="--kc:${k.color}">
  <div class="med">${med(s.playerName, 72)}</div>
  <div><div class="who">${esc(s.playerName)}</div><div class="kind"><span class="ic">${art(k.icon, '1.6em')}</span>${esc(k.text)}</div></div>
  <div class="q">${esc(s.prompt)}${sub ? `<div class="sub">${esc(sub)}</div>` : ''}</div>
  <div class="body">${textHTML(s)}<div class="meta"><span>${dur(s.durationMs)}</span>${s.createdAt ? `<span>${clock(s.createdAt)}</span>` : ''}</div></div>
</article>`;
}
function answerCard(s) {
  return `<article class="answer"><div class="med">${med(s.playerName, 52)}</div>
  <div><div class="who">${esc(s.playerName)}</div>${textHTML(s)}</div></article>`;
}

// ---------- the page ----------
const won = state.result === 'won';
const goals = state.goals || {};
const trees = (goals.trees || []).filter(Boolean).length;
const acorns = [...recog.values()].reduce((n, r) => n + (r.trust || 0), 0);
const first = stories.find(s => s.createdAt)?.createdAt || state.now / 1000;
const day = new Intl.DateTimeFormat('en-GB', { timeZone: TZ, weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' }).format(first * 1000);
const foundBy = id => [...recog.values()].find(r => (r.found || []).includes(id))?.name;
const MEM = opt('memories') ? JSON.parse(fs.readFileSync(opt('memories'), 'utf8')) : {};
const memOf = name => MEM.people?.[name];

const teamCards = players.map(p => {
  const r = recog.get(p.id) || {}, v = valueOf(p.value), role = ROLES.find(x => x.type === p.types?.[0]);
  const told = stories.filter(s => s.playerName === p.name).length;
  const facts = [
    `<li>${art('acorn', '1.3em')}<span><b>${r.trust || 0}</b> acorns of trust, from ${plural(r.givers || 0, 'person', 'people')}</span></li>`,
    `<li>${art('icon:talk', '1.3em')}<span>Shared <b>${told}</b> ${told === 1 ? 'story' : 'stories'}</span></li>`,
    ...(r.found || []).map(id => { const t = TREASURES.find(x => x.id === id); return `<li>${art(`treasure:${id}`, '1.3em')}<span>Found <b>${esc(t?.name || id)}</b></span></li>`; }),
    r.targetName ? `<li>${art('icon:owl', '1.3em')}<span>Secret Owl watching over <b>${esc(r.targetName)}</b></span></li>` : '',
  ].join('');
  return `<article class="mate" style="--pc:${p.color}">
  <div class="mate-top"><div class="med big">${med(p.name, 104)}</div>
    <div><h3>${esc(p.name)}</h3>
      ${v ? `<div class="stood">${art(`value:${p.value}`, '1.5em')}<span>Stood for <b>${esc(v.name)}</b><small>${esc(v.tagline)}</small></span></div>` : ''}
      ${role ? `<div class="role">${art(`role:${role.type}`, '1.3em')}<span>${esc(role.name)} · ${esc(role.gift)}</span></div>` : ''}</div></div>
  ${memOf(p.name)?.quote ? `<blockquote style="--vc:${v?.color || p.color}">${esc(memOf(p.name).quote)}</blockquote>` : ''}
  ${memOf(p.name)?.memory ? `<p class="memory">${esc(memOf(p.name).memory)}</p>` : ''}
  <ul class="facts">${facts}</ul>
</article>`;
}).join('\n');

// what we said together: each treasure's question and everyone's answer, in a few words
const togetherHTML = (MEM.together || []).map(g => {
  const t = TREASURES.find(x => x.id === g.treasure), finder = foundBy(g.treasure);
  return `<section class="treasure">
  <div class="t-head">${art(t ? `treasure:${t.id}` : 'treasure', '3.4em')}<div>
    <div class="t-name">${esc(t?.name || 'A treasure')}${finder ? ` <small>found by ${esc(finder)}</small>` : ''}</div>
    ${t?.meaning ? `<div class="t-mean">${esc(t.meaning)}</div>` : ''}
    <div class="q">${esc((t?.question || '').replace(/^Everyone, one sentence: /, '').replace(/^./, c => c.toUpperCase()))}</div></div></div>
  <ul class="said">${g.answers.map(([name, a]) => `<li><span class="m">${med(name, 46)}</span><span><b>${esc(name)}</b>${esc(a)}</span></li>`).join('')}</ul>
</section>`;
}).join('\n');

// stories by round; a treasure's answers sit together under the treasure
const rounds = new Map();
for (const s of stories) { if (!rounds.has(s.round)) rounds.set(s.round, []); rounds.get(s.round).push(s); }
const roundHTML = [...rounds.entries()].map(([n, list]) => {
  const parts = [];
  for (let i = 0; i < list.length; i++) {
    const s = list[i];
    if (s.kind !== 'treasure' || s.fallback && !s.title?.includes('answer')) { parts.push(storyCard(s)); continue; }
    const id = s.sub, t = TREASURES.find(x => x.id === id) || TREASURES.find(x => s.title?.startsWith(x.name));
    const group = [];
    while (i < list.length && list[i].kind === 'treasure' && (list[i].sub === s.sub || list[i].fallback && list[i].title?.split(' · ')[0] === s.title?.split(' · ')[0])) group.push(list[i++]);
    i--;
    const finder = t && foundBy(t.id);
    parts.push(`<section class="treasure">
  <div class="t-head">${art(t ? `treasure:${t.id}` : 'treasure', '3.4em')}<div>
    <div class="t-name">${esc(t?.name || 'A treasure')}${finder ? ` <small>found by ${esc(finder)}</small>` : ''}</div>
    ${t?.meaning ? `<div class="t-mean">${esc(t.meaning)}</div>` : ''}
    <div class="q">${esc(s.prompt)}</div></div></div>
  <div class="answers">${group.map(answerCard).join('')}</div>
</section>`);
  }
  return `<section class="round"><h2>${n ? `Round ${n}` : 'Entering the forest'}</h2>${n ? '' : '<p class="lead">Each of us stepped in at the edge of the forest and said why we stand for our value.</p>'}
${parts.join('\n')}</section>`;
}).join('\n');

// the Secret Owls: who watched over whom, round the circle
const owlRows = [];
{
  const seen = new Set();
  for (const start of players) {
    let r = recog.get(start.id);
    while (r && !seen.has(r.id)) {
      seen.add(r.id);
      if (r.targetName) owlRows.push(`<li><span class="m">${med(r.name, 44)}</span><b>${esc(r.name)}</b><span class="arrow">${art('icon:owl', '1.4em')} watched over</span><span class="m">${med(r.targetName, 44)}</span><b>${esc(r.targetName)}</b></li>`);
      const next = players.find(p => p.name === r.targetName);
      r = next && recog.get(next.id);
    }
  }
}

const INTRO = MEM.intro ? `<section class="brief"><h2>The night in brief</h2>${MEM.intro.trim().split(/\n{2,}/).map(p => `<p>${esc(p.trim())}</p>`).join('')}</section>` : '';
const html = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Heartwood · ${esc(CODE)} report</title>
<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22%3E%3Ctext y=%22.9em%22 font-size=%2290%22%3E🌳%3C/text%3E%3C/svg%3E">
<link rel="preconnect" href="https://fonts.googleapis.com"><link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Lilita+One&family=Nunito:wght@400;600;700;800;900&display=swap">
<style>
:root {
  --toon: 'Lilita One', 'Nunito', system-ui, sans-serif; --read: 'Nunito', system-ui, -apple-system, 'Segoe UI', sans-serif;
  --bg: #13241b; --paper: #fff6df; --paper2: #fbecc8; --line: #e2c48e; --wood: #5a3418; --wood-d: #3e220f; --ink: #2a1a0c;
  --ink2: #6f4f36; --muted: #8a6a4e; --gold: #e0a32e; --good: #2c6a1f;
  color-scheme: light;
}
* { box-sizing: border-box; }
html { -webkit-text-size-adjust: 100%; scroll-behavior: smooth; }
body { margin: 0; min-height: 100vh; font: 600 16px/1.5 var(--read); color: var(--ink);
  background: radial-gradient(120% 70% at 50% 0%, #2f5e40 0%, #183424 45%, #0a1710 85%) fixed, var(--bg); }
.wrap { max-width: 980px; margin: 0 auto; padding: 18px 16px 60px; }
h1, h2, h3 { font-family: var(--toon); font-weight: 400; margin: 0; line-height: 1.1; }
svg { overflow: visible; }

.sign { display: flex; align-items: center; gap: 18px; padding: 16px 22px; background: var(--paper);
  border: 5px solid var(--wood); border-radius: 28px; box-shadow: 0 7px 0 var(--wood-d), 0 18px 40px rgba(0, 0, 0, .45); }
.sign .tree { flex: none; line-height: 0; }
.sign h1 { font-size: clamp(34px, 7vw, 56px); color: #8fe06a; letter-spacing: .01em;
  text-shadow: 0 4px 0 #1f4a18, 3px 3px 0 #1f4a18, -3px 3px 0 #1f4a18, 3px -3px 0 #1f4a18, -3px -3px 0 #1f4a18, 3px 0 0 #1f4a18, -3px 0 0 #1f4a18, 0 -3px 0 #1f4a18; }
.sign h1.slept { color: #c9d3c4; }
.sign p { margin: 6px 0 0; color: var(--ink2); font-weight: 700; }
.sign .code { font: 20px var(--toon); letter-spacing: .12em; color: #8a5418; }
.chips { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 10px; }
.chip { display: inline-flex; align-items: center; gap: 6px; padding: 4px 12px 4px 8px; border-radius: 999px; background: var(--paper2);
  border: 2px solid var(--line); font-weight: 800; font-size: 15px; color: var(--ink2); }
.chip b { font: 19px var(--toon); font-weight: 400; color: var(--wood); }
.brief { margin-top: 20px; padding: 16px 22px 8px; background: var(--paper); border: 4px solid var(--wood); border-radius: 24px; box-shadow: 0 6px 0 var(--wood-d), 0 14px 30px rgba(0, 0, 0, .35); }
.brief h2 { font-size: 26px; color: var(--wood); margin-bottom: 6px; }
.brief p { margin: 0 0 10px; font: 600 17px/1.6 var(--read); }
nav { display: flex; flex-wrap: wrap; gap: 10px; margin: 20px 0 4px; }
nav a { font: 18px/1 var(--toon); color: var(--wood); text-decoration: none; background: linear-gradient(#fffbf2, #ecd8ad); border: 3px solid #8a5a2c;
  border-radius: 14px; padding: 9px 14px 10px; box-shadow: inset 0 3px 0 #fff, 0 5px 0 #8a5a2c, 0 9px 14px rgba(0, 0, 0, .22); }
.band { margin-top: 34px; }
.band > h2 { font-size: 34px; color: #fff3d6; text-shadow: 0 3px 0 rgba(0, 0, 0, .45); margin: 0 0 6px 4px; }
.band > p.about { color: #d9ccae; font-weight: 700; margin: 0 0 16px 4px; }

/* the team */
.team { display: grid; grid-template-columns: repeat(auto-fill, minmax(290px, 1fr)); gap: 18px; }
.mate { position: relative; padding: 16px 16px 14px; background: var(--paper); border: 4px solid var(--wood); border-radius: 24px;
  box-shadow: 0 6px 0 var(--wood-d), 0 14px 30px rgba(0, 0, 0, .35); overflow: hidden; }
.mate::before { content: ''; position: absolute; inset: 0 0 auto 0; height: 10px; background: var(--pc); }
.mate-top { display: flex; gap: 14px; align-items: center; margin-top: 4px; }
.med { line-height: 0; flex: none; }
.mate h3 { font-size: 27px; color: var(--ink); margin-bottom: 4px; }
.stood, .role { display: flex; align-items: center; gap: 6px; font-weight: 700; color: var(--ink2); line-height: 1.25; }
.stood small { display: block; font-style: italic; color: var(--muted); font-size: 13.5px; }
.stood b { color: var(--ink); }
.role { margin-top: 4px; font-size: 14px; }
.team.mem { grid-template-columns: repeat(auto-fill, minmax(420px, 1fr)); }
.mate blockquote { margin: 14px 0 0; padding: 10px 14px; border-left: 6px solid var(--vc); background: var(--paper2); border-radius: 0 14px 14px 0;
  font: 21px/1.3 var(--toon); color: var(--wood); }
.mate blockquote::before { content: '“'; } .mate blockquote::after { content: '”'; }
.memory { margin: 12px 0 0; font: 600 17px/1.6 var(--read); color: var(--ink); }
.facts { list-style: none; margin: 12px 0 0; padding: 10px 0 0; border-top: 2px dashed var(--line); display: grid; gap: 4px; }
.facts li { display: flex; align-items: center; gap: 8px; font-weight: 700; color: var(--ink2); }
.facts li .art-inline { flex: none; }
.facts b { color: var(--ink); }

/* rounds and stories */
.round { margin-top: 26px; }
.round > h2 { font-size: 26px; color: #fff3d6; text-shadow: 0 3px 0 rgba(0, 0, 0, .45); margin: 0 0 10px 4px; }
.round > p.lead { color: #d9ccae; font-weight: 700; margin: -4px 0 12px 4px; }
.story { position: relative; display: grid; grid-template-columns: 76px 1fr; gap: 6px 16px; padding: 14px 18px 16px; margin: 0 0 16px;
  background: var(--paper); border: 4px solid var(--wood); border-radius: 22px; box-shadow: 0 6px 0 var(--wood-d), 0 14px 30px rgba(0, 0, 0, .35); }
.story::before { content: ''; position: absolute; left: 0; top: 0; bottom: 0; width: 12px; border-radius: 18px 0 0 18px; background: var(--kc, #5cb84a); }
.story .med { grid-row: span 2; padding-left: 4px; }
.who { font: 22px/1.1 var(--toon); color: var(--ink); }
.kind { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; font: 800 13px/1.2 var(--read); letter-spacing: .06em; text-transform: uppercase; color: var(--kc, #5cb84a); filter: brightness(.82); }
.kind .ic { line-height: 0; }
.q { font: 800 19px/1.3 var(--read); color: var(--ink); }
.story .q { grid-column: 2; }
.sub { font: italic 700 15px/1.3 var(--read); color: var(--ink2); margin-top: 2px; }
.body { grid-column: 1 / -1; }
.text { margin-top: 8px; padding: 12px 14px; background: #fffdf6; border: 3px dashed var(--line); border-radius: 14px; font: 600 17px/1.55 var(--read); color: var(--ink); overflow-wrap: anywhere; }
.text p { margin: 0 0 .6em; } .text p:last-child { margin: 0; }
.text.empty { color: var(--muted); font-style: italic; }
.meta { display: flex; gap: 14px; margin-top: 6px; font-size: 13px; font-weight: 700; color: var(--muted); }

/* a treasure and everyone's answer */
.treasure { margin: 0 0 18px; padding: 16px 18px 6px; background: linear-gradient(#fff3cf, #fbe5b0); border: 4px solid #8a5a2c; border-radius: 24px;
  box-shadow: 0 6px 0 #6b4116, 0 14px 30px rgba(0, 0, 0, .35); }
.t-head { display: flex; gap: 14px; align-items: center; margin-bottom: 12px; }
.t-head .art-inline { flex: none; }
.t-name { font: 28px/1.1 var(--toon); color: #7a4a12; }
.t-name small { font: 800 14px var(--read); color: var(--ink2); letter-spacing: .02em; }
.t-mean { font: italic 700 15px var(--read); color: var(--ink2); margin: 2px 0 4px; }
.answers { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 12px; margin-bottom: 12px; }
.answer { display: grid; grid-template-columns: 56px 1fr; gap: 10px; padding: 10px 12px; background: var(--paper); border: 3px solid var(--line); border-radius: 18px; }
.answer .who { font-size: 19px; }
.answer .text { font-size: 16px; margin-top: 6px; padding: 8px 10px; }

.said { list-style: none; margin: 0 0 12px; padding: 0; display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 10px; }
.said li { display: flex; align-items: center; gap: 10px; padding: 8px 12px 8px 8px; background: var(--paper); border: 3px solid var(--line); border-radius: 18px;
  font: 700 16.5px/1.35 var(--read); color: var(--ink); }
.said .m { line-height: 0; flex: none; }
.said b { display: block; font: 18px/1.1 var(--toon); font-weight: 400; color: var(--ink2); margin-bottom: 2px; }

/* the Secret Owls */
.owls { list-style: none; margin: 0; padding: 18px; display: grid; grid-template-columns: repeat(auto-fill, minmax(400px, 1fr)); gap: 10px 24px; background: var(--paper); border: 4px solid var(--wood); border-radius: 24px;
  box-shadow: 0 6px 0 var(--wood-d), 0 14px 30px rgba(0, 0, 0, .35); }
.owls li { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; font: 20px var(--toon); color: var(--ink); }
.owls .m { line-height: 0; }
.owls .arrow { display: inline-flex; align-items: center; gap: 4px; font: 800 14px var(--read); color: var(--ink2); margin: 0 6px; }
footer { margin-top: 34px; color: #bfb397; font-size: 13.5px; font-weight: 700; text-align: center; }

@media (max-width: 560px) {
  .sign { flex-direction: column; text-align: center; padding: 14px; }
  .chips { justify-content: center; }
  .story { grid-template-columns: 52px 1fr; padding: 12px 12px 14px 16px; gap: 4px 10px; }
  .story .med svg { width: 52px; height: 52px; }
  .story .q { grid-column: 1 / -1; font-size: 17px; }
  .mate-top .med svg { width: 84px; height: 84px; }
  .answers, .owls, .said, .team.mem { grid-template-columns: 1fr; }
}
${ART.css}
</style>
</head>
<body>
<svg width="0" height="0" style="position:absolute;width:0;height:0;overflow:hidden" aria-hidden="true">${ART.defs()}<defs>@@PHOTOS@@</defs></svg>
<div class="wrap">
  <header class="sign">
    <span class="tree">${art(won ? 'worldTree:awake' : 'worldTree', '120px')}</span>
    <div>
      <h1${won ? '' : ' class="slept"'}>${won ? 'The forest woke' : 'The forest slept'}</h1>
      <p>Game <span class="code">${esc(CODE)}</span> · ${esc(day)} · ${plural(players.length, 'player', 'players')} · ${plural(state.round || 0, 'round', 'rounds')}</p>
      <div class="chips">
        <span class="chip">${art('stage:4', '1.5em')}<b>${trees}</b> of 6 trees grown</span>
        <span class="chip">${art('fruit', '1.3em')}<b>${goals.onTree ?? 0}</b> fruit on the World Tree</span>
        <span class="chip">${art('treasure', '1.4em')}<b>${(goals.treasures || []).length}</b> of 3 treasures</span>
        <span class="chip">${art('acorn', '1.3em')}<b>${acorns}</b> acorns of trust</span>
        <span class="chip">${art('icon:talk', '1.3em')}<b>${stories.length}</b> stories shared</span>
      </div>
    </div>
  </header>
  ${INTRO}
  <nav><a href="#team">${MEM.people ? 'For each of you' : 'The team'}</a>${togetherHTML ? '<a href="#together">What we said together</a>' : ''}<a href="#owls">The Secret Owls</a>${TRANSCRIPTS ? '<a href="#stories">Every story</a>' : ''}</nav>

  <section class="band" id="team"><h2>${MEM.people ? 'For each of you' : 'The team'}</h2>
    <p class="about">${MEM.people ? 'Something to remember from what each of us shared tonight.' : 'Who played, the value each one stood for, and the trust the others gave them.'}</p>
    <div class="team${MEM.people ? ' mem' : ''}">${teamCards}</div></section>

  ${togetherHTML ? `<section class="band" id="together"><h2>What we said together</h2>
    <p class="about">Each treasure asked all of us one question.</p>
    ${togetherHTML}</section>` : ''}

  ${owlRows.length ? `<section class="band" id="owls"><h2>The Secret Owls</h2>
    <p class="about">Each of us quietly watched over one other player all game, then honoured them in the tribute chain.</p>
    <ul class="owls">${owlRows.join('')}</ul></section>` : ''}

  ${TRANSCRIPTS ? `<section class="band" id="stories"><h2>Every story</h2>
    <p class="about">Every story told in the forest, word for word as the recording heard it.</p>
    ${roundHTML}</section>` : ''}

  <footer>Heartwood · World Tree · written from the stories we told during the game.</footer>
</div>
</body>
</html>
`;
// ---------- the story deck: Instagram story pages (1080 × 1920), one per screen and per printed page ----------
// A cover with everyone around the World Tree, a page for each player (their memory), a page for each treasure.
function storyHTML() {
  const shortDay = new Intl.DateTimeFormat('en-GB', { timeZone: TZ, day: 'numeric', month: 'long', year: 'numeric' }).format(first * 1000);
  const foot = `<div class="foot">Heartwood · World Tree · ${esc(shortDay)}</div>`;
  const ring = players.map((p, i) => {
    const a = (-90 + i * 360 / players.length) * Math.PI / 180, R = 400;
    return `<div class="g-mate" style="left:${(540 + R * Math.cos(a)).toFixed(1)}px;top:${(500 + R * Math.sin(a)).toFixed(1)}px">${med(p.name, 196)}<span>${esc(p.name)}</span></div>`;
  }).join('');
  const chip = (icon, n, what) => `<span class="chip">${art(icon, '1.25em')}<b>${n}</b> ${what}</span>`;
  const cover = `<section class="page cover">
  <div class="kicker">Heartwood · World Tree</div>
  <h1${won ? '' : ' class="slept"'}>${won ? 'The forest woke' : 'The forest slept'}</h1>
  <div class="date">${esc(day)}</div>
  <div class="group"><div class="g-tree">${art(won ? 'worldTree:awake' : 'worldTree', '540px')}</div>${ring}</div>
  <div class="chips">${chip('icon:round', state.round || 0, 'rounds')}${chip('stage:4', trees, 'trees grown')}${chip('fruit', goals.onTree ?? 0, 'fruit on the World Tree')}${chip('treasure', (goals.treasures || []).length, 'treasures')}${chip('icon:talk', stories.length, 'stories shared')}</div>
  ${foot}
</section>`;
  const person = p => {
    const r = recog.get(p.id) || {}, v = valueOf(p.value), role = ROLES.find(x => x.type === p.types?.[0]), m = memOf(p.name) || {};
    const told = stories.filter(s => s.playerName === p.name).length;
    const found = (r.found || []).map(id => { const t = TREASURES.find(x => x.id === id); return `<span>${art(`treasure:${id}`, '1.25em')} Found <b>${esc(t?.name || id)}</b></span>`; }).join('');
    return `<section class="page person" style="--pc:${p.color};--vc:${v?.color || p.color}">
  <div class="p-med">${med(p.name, 470)}</div>
  <h2>${esc(p.name)}</h2>
  ${v ? `<div class="p-val">${art(`value:${p.value}`, '1.3em')}<span>Stood for <b>${esc(v.name)}</b></span></div><div class="p-tag">${esc(v.tagline)}</div>` : ''}
  ${role ? `<div class="p-role">${art(`role:${role.type}`, '1.2em')}<span>${esc(role.name)} · ${esc(role.gift)}</span></div>` : ''}
  ${m.quote ? `<blockquote>${esc(m.quote)}</blockquote>` : ''}
  ${m.memory ? `<div class="p-mem fit">${esc(m.memory)}</div>` : ''}
  <div class="p-facts"><span>${art('acorn', '1.25em')} <b>${r.trust || 0}</b> acorns of trust</span>${found}<span>${art('icon:talk', '1.25em')} <b>${told}</b> stories shared</span></div>
  ${foot}
</section>`;
  };
  const treasure = g => {
    const t = TREASURES.find(x => x.id === g.treasure), finder = foundBy(g.treasure);
    const q = (t?.question || '').replace(/^Everyone, one sentence: /, '').replace(/^./, c => c.toUpperCase());
    return `<section class="page treasure-p">
  <div class="t-art">${art(t ? `treasure:${t.id}` : 'treasure', '210px')}</div>
  <h2>${esc(t?.name || 'A treasure')}</h2>
  <div class="t-found">${t?.meaning ? esc(t.meaning) : ''}${finder ? ` · found by ${esc(finder)}` : ''}</div>
  <div class="t-q">${esc(q)}</div>
  <ul class="t-ans fit${g.answers.length > 6 ? ' many' : ''}">${g.answers.map(([n, a]) => `<li><span class="m">${med(n, g.answers.length > 6 ? 92 : 118)}</span><span><b>${esc(n)}</b>${esc(a)}</span></li>`).join('')}</ul>
  ${foot}
</section>`;
  };
  const pdf = opt('pdf') ? `<a class="dl" download="heartwood-${esc(CODE)}-story.pdf" href="data:application/pdf;base64,${fs.readFileSync(opt('pdf')).toString('base64')}">Download PDF</a>` : '';
  return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Heartwood · ${esc(CODE)} story</title>
<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22%3E%3Ctext y=%22.9em%22 font-size=%2290%22%3E🌳%3C/text%3E%3C/svg%3E">
<link rel="preconnect" href="https://fonts.googleapis.com"><link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Lilita+One&family=Nunito:wght@600;700;800;900&display=block">
<style>
@page { size: 1080px 1920px; margin: 0; }
:root { --toon: 'Lilita One', 'Nunito', system-ui, sans-serif; --read: 'Nunito', system-ui, sans-serif;
  --paper: #fff6df; --paper2: #fbecc8; --line: #e2c48e; --wood: #5a3418; --wood-d: #3e220f; --ink: #2a1a0c; --ink2: #6f4f36; --cream: #fff3d6;
  color-scheme: light; -webkit-print-color-adjust: exact; print-color-adjust: exact; }
* { box-sizing: border-box; }
html, body { margin: 0; background: #07120c; }
svg { overflow: visible; }
h1, h2 { font-family: var(--toon); font-weight: 400; margin: 0; line-height: 1.05; }
.page { position: relative; width: 1080px; height: 1920px; overflow: hidden; display: flex; flex-direction: column; align-items: center;
  padding: 150px 80px 150px; text-align: center; break-after: page; page-break-after: always;
  background: radial-gradient(120% 60% at 50% 0%, #3a7550 0%, #1d3f2b 42%, #0b1a11 88%), #13241b; color: var(--cream); font-family: var(--read); }
.page:last-child { break-after: auto; page-break-after: auto; }
.foot { position: absolute; left: 0; right: 0; bottom: 70px; font: 800 24px var(--read); letter-spacing: .08em; color: rgba(255, 243, 214, .55); }
.outline { text-shadow: 0 6px 0 #1f4a18, 4px 4px 0 #1f4a18, -4px 4px 0 #1f4a18, 4px -4px 0 #1f4a18, -4px -4px 0 #1f4a18, 4px 0 0 #1f4a18, -4px 0 0 #1f4a18, 0 -4px 0 #1f4a18; }

/* cover */
.kicker { font: 34px var(--toon); letter-spacing: .14em; text-transform: uppercase; color: #cfe9b4; }
.cover h1 { margin-top: 14px; font-size: 128px; color: #8fe06a;
  text-shadow: 0 7px 0 #1f4a18, 5px 5px 0 #1f4a18, -5px 5px 0 #1f4a18, 5px -5px 0 #1f4a18, -5px -5px 0 #1f4a18, 5px 0 0 #1f4a18, -5px 0 0 #1f4a18, 0 -5px 0 #1f4a18; }
.cover h1.slept { color: #c9d3c4; }
.date { margin-top: 18px; font: 800 36px var(--read); color: var(--cream); }
.group { position: relative; width: 1080px; height: 1000px; margin: 40px -80px 0; flex: none; }
.g-tree { position: absolute; left: 50%; top: 50%; transform: translate(-50%, -54%); line-height: 0; }
.g-mate { position: absolute; transform: translate(-50%, -50%); display: flex; flex-direction: column; align-items: center; line-height: 0; }
.g-mate span { margin-top: 6px; font: 34px/1 var(--toon); color: #fff; white-space: nowrap;
  text-shadow: 0 4px 0 #142a1c, 3px 3px 0 #142a1c, -3px 3px 0 #142a1c, 3px -3px 0 #142a1c, -3px -3px 0 #142a1c; }
.chips { display: flex; flex-wrap: wrap; justify-content: center; gap: 14px; margin-top: 30px; }
.chip { display: inline-flex; align-items: center; gap: 10px; padding: 10px 22px 10px 14px; border-radius: 999px; background: var(--paper);
  border: 4px solid var(--wood); box-shadow: 0 5px 0 var(--wood-d); font: 800 28px var(--read); color: var(--ink2); }
.chip b { font: 36px var(--toon); font-weight: 400; color: var(--wood); }

/* a player's page */
.p-med { line-height: 0; }
/* a player's or a treasure's page sits in the middle of the screen (auto margins: never cut off at the top) */
.person > :first-child, .treasure-p > :first-child { margin-top: auto; }
.person > .p-facts, .treasure-p > .t-ans { margin-bottom: auto; }
.person h2 { margin-top: 8px; font-size: 104px; color: #fff;
  text-shadow: 0 7px 0 var(--wood-d), 5px 5px 0 var(--wood-d), -5px 5px 0 var(--wood-d), 5px -5px 0 var(--wood-d), -5px -5px 0 var(--wood-d), 5px 0 0 var(--wood-d), -5px 0 0 var(--wood-d), 0 -5px 0 var(--wood-d); }
.p-val { display: flex; align-items: center; gap: 12px; margin-top: 14px; font: 800 40px var(--read); color: var(--cream); }
.p-val b { color: #fff; }
.p-tag { font: italic 800 30px var(--read); color: #d9ccae; margin-top: 2px; }
.p-role { display: flex; align-items: center; gap: 10px; margin-top: 10px; font: 800 28px var(--read); color: #cfe9b4; }
blockquote { margin: 34px 0 0; padding: 26px 40px 30px; background: var(--paper); border: 6px solid var(--wood); border-radius: 36px;
  box-shadow: 0 8px 0 var(--wood-d), 0 18px 40px rgba(0, 0, 0, .4); font: 52px/1.18 var(--toon); color: var(--wood); position: relative; }
blockquote::before { content: '“'; } blockquote::after { content: '”'; }
blockquote { border-top: 14px solid var(--vc); }
.p-mem { margin-top: 30px; padding: 30px 38px; background: rgba(255, 246, 223, .96); border-radius: 30px; border: 4px solid var(--line);
  font: 600 35px/1.5 var(--read); color: var(--ink); text-align: left; }
.p-facts { display: flex; flex-wrap: wrap; justify-content: center; gap: 12px 28px; margin-top: 26px; font: 800 28px var(--read); color: var(--cream); }
.p-facts span { display: inline-flex; align-items: center; gap: 8px; }
.p-facts b { color: #fff; }

/* a treasure's page */
.t-art { line-height: 0; }
.treasure-p h2 { margin-top: 10px; font-size: 104px; color: #ffd36a;
  text-shadow: 0 7px 0 #6b4116, 5px 5px 0 #6b4116, -5px 5px 0 #6b4116, 5px -5px 0 #6b4116, -5px -5px 0 #6b4116, 5px 0 0 #6b4116, -5px 0 0 #6b4116, 0 -5px 0 #6b4116; }
.t-found { margin-top: 8px; font: italic 800 30px var(--read); color: #d9ccae; }
.t-q { margin-top: 26px; padding: 22px 34px 26px; background: var(--paper); border: 6px solid #8a5a2c; border-radius: 32px; box-shadow: 0 8px 0 #6b4116;
  font: 46px/1.2 var(--toon); color: #7a4a12; }
.t-ans { list-style: none; margin: 34px 0 0; padding: 0; width: 100%; display: grid; gap: 16px; font-size: 32px; }
.t-ans li { display: flex; align-items: center; gap: 18px; padding: 12px 26px 12px 12px; background: rgba(255, 246, 223, .96); border: 4px solid var(--line);
  border-radius: 30px; text-align: left; font: 700 1em/1.3 var(--read); color: var(--ink); }
.t-ans .m { line-height: 0; flex: none; }
.t-ans.many { gap: 12px; margin-top: 28px; }
.t-ans.many li { padding: 6px 22px 6px 8px; }
.t-ans b { display: block; font: 1.05em/1.1 var(--toon); font-weight: 400; color: var(--ink2); margin-bottom: 4px; }

.dl { position: fixed; top: 14px; right: 14px; z-index: 5; font: 20px/1 var(--toon); color: #fff; text-decoration: none; background: linear-gradient(#88dc64, #4cab37);
  border: 3px solid #2c6a1f; border-radius: 14px; padding: 10px 16px 11px; box-shadow: inset 0 3px 0 rgba(255, 255, 255, .38), 0 5px 0 #2c6a1f; }
@media screen { body { padding: 16px 0 40px; } .page { zoom: var(--z, 1); margin: 0 auto 28px; box-shadow: 0 20px 60px rgba(0, 0, 0, .6); } }
@media print { .dl { display: none; } body { padding: 0; } .page { margin: 0; zoom: 1; box-shadow: none; } }
${ART.css}
</style>
</head>
<body>
<svg width="0" height="0" style="position:absolute;width:0;height:0;overflow:hidden" aria-hidden="true">${ART.defs()}<defs>@@PHOTOS@@</defs></svg>
${pdf}
${cover}
${players.map(person).join('\n')}
${(MEM.together || []).map(treasure).join('\n')}
<script>
// A long memory shrinks until its page fits; then the pages scale to the screen (never in print).
function fit() {
  for (const pg of document.querySelectorAll('.page')) {
    const box = pg.querySelector('.fit');
    if (!box) continue;
    let size = parseFloat(getComputedStyle(box).fontSize);
    while (pg.scrollHeight > pg.clientHeight + 1 && size > 18) { size -= 1; box.style.fontSize = size + 'px'; }
  }
}
const zoom = () => document.documentElement.style.setProperty('--z', Math.min(1, (innerWidth - 24) / 1080));
document.fonts.ready.then(() => { fit(); zoom(); document.documentElement.dataset.ready = '1'; });
addEventListener('resize', zoom);
</script>
</body>
</html>
`;
}

const out = STORY ? storyHTML() : html;
fs.writeFileSync(OUT, out.replace('@@PHOTOS@@', Object.values(photoDefs).join('')), { mode: 0o600 });
console.log(`${OUT}: ${players.length} players, ${Object.keys(photo).length} photos, ${stories.length} stories (${stories.filter(s => s.fallback).length} from the fallback), ${Math.round(fs.statSync(OUT).size / 1024)} KB`);
