/* Heartwood art: "The Sleeping Forest" sprite library (shaded toon, PvZ-like).
 *
 * Plain script, no modules: defines window.ART. Every drawing function returns an SVG markup string that can be
 * dropped into the Keeper's board SVG or wrapped with ART.svg() for small inline pictures on phones.
 * Style rules (docs/plan/toon-art/build-contract.md): coloured outlines (a darker shade of each fill, never
 * black), two-tone cel shading (outline silhouette, dark body, lighter layer up-left, one glossy highlight),
 * soft ground shadows, chunky round shapes, no faces.
 *
 * Setup
 *   ART.install()                 inject the <style> (ART.css) and one hidden <svg> holding ART.defs(); safe to
 *                                 call many times. ART.svg() calls it for you in a browser.
 *   ART.defs()                    the shared <defs> markup (ids prefixed art-), if you prefer to embed it yourself.
 *   ART.css                       the idle animation CSS (incl. prefers-reduced-motion).
 *   ART.svg(inner, {size, width, height, viewBox, cls, title})  → standalone <svg> string (default viewBox -24 -24 48 48).
 *
 * Geometry (pointy-top hexes, axial q,r; S = hex corner radius, default ART.S = 40)
 *   ART.TILT = 0.6, ART.S = 40
 *   ART.ringOf(q, r)              hex distance from the centre
 *   ART.elev(ring, S)             (4 − ring)·0.16·S  — the forest rises toward the World Tree
 *   ART.project(q, r, S)          → [x, y] screen centre of the tile's top face (x = S√3(q + r/2), y = 1.5·S·r·TILT − elev)
 *   ART.hexes(R = 4)              → [{q, r, ring, sector}] in engine buildHexes order (index 0 = World Tree)
 *   ART.facePoints(S, k = 0.965)  → "x,y …" squashed top-face polygon (for highlight rims)
 *   ART.tileOutline(ring, S)      → "x,y …" top face + walls polygon (for hit areas)
 *   ART.depth(ring, S)            wall depth below the top face (0.42·S + elev, so terraces reach the base)
 *   ART.sortKey(q, r, S)          screen y used to paint back to front
 *
 * Tiles (origin = top-face centre; the face is √3·0.965·S ≈ 1.67·S wide × 1.16·S tall, the walls hang ART.depth below)
 *   Colour deepens toward the centre in clear steps: Ring 4 fresh spring green → Ring 1 deep emerald, the World Tree
 *   hex darkest (mossy earth); face-down leaf blankets deepen the same way (ART.palette.up / .down, index = ring).
 *   ART.tile({ring, sector, up, kind, world, S, q, r, outer, seed})
 *     ring 1..4 (0 or world:true = World Tree hex), sector 0..5 (value tint 15%), up = face-up,
 *     kind 'empty'|'spring'|'treasure' (spring adds wet flowers), q/r → deterministic decoration + the Ring 4
 *     outer edges (value-colour ribbon + pennants), outer: direction indices 0..5 (DIRS order) to force the trim,
 *     seed: decoration variant when q/r are not given.
 *
 * Objects (upright, anchored at (0,0) = ground point = tile top-face centre)
 *   ART.plant(stage, sector, {harvestable, seed, S})   0 → '', 1 Seeded ≈0.5S, 2 Sprout ≈0.8S, 3 Sapling ≈1.3S,
 *       4 Big Tree ≈2.1S tall × 1.6S wide (oak 2.1S wide): Z flame maple, E ginkgo, R pine, A cherry, O jacaranda,
 *       S oak (ART.species). harvestable: three heart fruits hang in the canopy. Saplings take a hint of the sector.
 *   ART.worldTree({awake, treasures: [ids], fruit, seed, S})  ≈4.6S tall × 3.5S wide over roots that spill onto
 *       Ring 1; heart hollow at (0, −1.33S) with a shelf holding compass, lantern, rope side by side — empty
 *       slots show a faint dot; fruit = count placed (up to 12 hang in the crown + a count sign by the roots).
 *       Asleep: weeping leaf strands, closed buds, dim teal. Awake: green crown, blossoms in all six value colours,
 *       glow halo, petals.
 *   ART.leaves(n, {seed, S})      1 scattered dead leaves, 2 sealed heap + purple brambles.
 *   ART.spring({seed, S})         stone-ringed pond with ripples, reeds and a sparkle (≈1.4S wide, lies flat).
 *   ART.treasure(id, {onTile, S, size, seed})  compass | lantern | rope. Plain: centred at (0,0) in a ≈30-unit box
 *       (size rescales that box). onTile: on a little stump with a glint, anchored at the ground (≈1.1S tall).
 *
 * Items and badges (centred at (0,0))
 *   ART.fruit({size}), ART.drop({size}), ART.acorn({size})   ≈24 units tall at size 24 (default).
 *   ART.weatherBadge('sun'|'rain'|'fog', {size}), ART.valueBadge(i, {size}), ART.roleBadge(type 1..9, {size})
 *       48-unit badges (size = diameter in units, default 48).
 *   ART.icon(name, {size, disc})  move explore sow water tend clear harvest take drink pass end trust owl timer
 *       in a 48-unit box; disc:true puts it on a round cream button.
 *
 * Players
 *   ART.medallion({photo, name, color, water, fruit, treasure, state: ''|'turn'|'dry', r, stump, label})
 *       r = photo radius (default 17 ≈ 0.42·S); everything (ring, crown, badges, stump) scales with r.
 *       With stump (default) (0,0) is the ground point, the photo centre is at (0, −1.65·r), the top of the water
 *       crown at ≈ −3.5·r. stump:false → photo centred at (0,0), outer radius ≈ 1.9·r incl. crown.
 *       photo = image URL ('' → the name's initial in Lilita One). label:true adds a name tag under the stump.
 *
 * Ambient
 *   ART.weatherFx('sun'|'rain'|'fog', {S, seed})  per-hex overlay at the top-face centre (tint on the face + sun
 *       shaft / 4 rain streaks + splash / 2 fog puffs); pointer-events none, put it in a layer above the tiles.
 *   ART.sectorWeather(kind, centres, {S, seed})  one sector's weather: centres = [[x, y] …] projected top-face centres;
 *       a tint face per hex plus a capped number of shafts / streaks / ripples / fog puffs spread over the sector.
 *   ART.dapple(centres, {S, seed})   warm light spots drifting over an awake sector's ground (class art-dapple).
 *   ART.mist(centres, {S, seed, n})  n slow mist banks (soft gradient, no blur filter; class art-mist).
 *
 * Particles and decorations (centred at (0,0), for the Keeper's animations and HUD)
 *   ART.leaf({size, color, empty})   a toon leaf (empty: a dashed outline, e.g. a spent action), 24-unit box.
 *   ART.deadLeaf({size, seed, color}) a curled dead leaf standing up (wind gusts, the Clear sweep), 16-unit box.
 *   ART.petal(color, {size}), ART.puff({size, color}) (dust / steam), ART.rays({size, n, color, opacity}) (sunburst,
 *       100-unit box), ART.sprig({size}) (a leafy twig for panel corners, 40 units wide).
 *   ART.firefly(x, y, {seed}), ART.fireflies(n, {w, h, x, y, seed})   glowing motes (class art-firefly).
 *
 * Animation classes (CSS in ART.css; every element carries its own transform-box/origin and delay inline, and
 * never a transform attribute, so wrap it to position it):
 *   art-sway (canopies, leaves, reeds), art-flicker (lantern glow), art-needle (compass needle), art-glow
 *   (turn ring, awake halo), art-ripple (pond rings), art-twinkle (sparkles, glints), art-firefly, art-drift
 *   (falling petal/leaf), art-bob (medallion on its turn), art-rain, art-fog, art-shaft (weatherFx), art-dapple, art-mist.
 *   All stop under prefers-reduced-motion.
 *
 * Data: ART.values {Z,E,R,A,O,S → colour}, ART.VALUES [6 colours], ART.palette, ART.species, ART.letters, ART.icons,
 *   ART.DIRS. Helpers: ART.mix(a, b, t), ART.dark(c, k), ART.light(c, k), ART.tone(c) → {o outline, d dark body,
 *   l light, h}, ART.rng(seed) → () ⇒ [0,1), ART.sparkle(x, y, r, delay).
 */
(function () {
  'use strict';
  const S0 = 40, TILT = 0.6, R3 = Math.sqrt(3), RAD = Math.PI / 180;
  const VAL = ['#e0703a', '#e0a32e', '#c8463f', '#cf6f97', '#4f74b0', '#3f9e9a'];
  const N = v => Math.round(v * 100) / 100;
  const D = (s, ...v) => s.reduce((a, x, i) => a + x + (i < v.length ? N(v[i]) : ''), '');

  // ---------- markup ----------
  function attrs(o) {
    let s = '';
    for (const k in o) {
      const v = o[k];
      if (v === undefined || v === null || v === false) continue;
      s += ' ' + k + '="' + (typeof v === 'number' ? N(v) : v) + '"';
    }
    return s;
  }
  const el = (t, o, inner) => inner === undefined ? '<' + t + attrs(o) + '/>' : '<' + t + attrs(o) + '>' + inner + '</' + t + '>';
  const G = (o, ...kids) => el('g', o || {}, kids.join(''));
  const Pa = (d, o) => el('path', Object.assign({ d }, o));
  const Ci = (cx, cy, r, o) => el('circle', Object.assign({ cx, cy, r }, o));
  const El = (cx, cy, rx, ry, o) => el('ellipse', Object.assign({ cx, cy, rx, ry }, o));
  const T = (x, y, s, r) => 'translate(' + N(x) + ',' + N(y) + ')' + (r ? ' rotate(' + N(r) + ')' : '') + (s != null && s !== 1 ? ' scale(' + N(s) + ')' : '');
  const scaleTo = (S, inner) => (S && S !== S0 ? G({ transform: 'scale(' + N(S / S0) + ')' }, inner) : inner);
  const esc = s => String(s).replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
  // inline style for an animated element: its own pivot and a per-object delay
  const anim = (cls, origin, delay, extra) => ({ class: cls, style: 'transform-box:fill-box;transform-origin:' + origin + ';animation-delay:' + N(-delay) + 's' + (extra ? ';' + extra : '') });
  let uid = 0;

  // ---------- deterministic randomness ----------
  function rng(seed) {
    let a = (seed >>> 0) || 0x9e3779b9;
    return () => { a = (a + 0x6D2B79F5) | 0; let t = Math.imul(a ^ (a >>> 15), 1 | a); t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t; return ((t ^ (t >>> 14)) >>> 0) / 4294967296; };
  }
  const hashQR = (q, r, k) => (Math.imul(q | 0, 73856093) ^ Math.imul(r | 0, 19349663) ^ Math.imul(k | 0, 83492791)) >>> 0;

  // ---------- colour ----------
  function rgb(c) { c = c.replace('#', ''); if (c.length === 3) c = c.replace(/./g, '$&$&'); const n = parseInt(c, 16); return [n >> 16 & 255, n >> 8 & 255, n & 255]; }
  const hex = a => '#' + a.map(v => Math.max(0, Math.min(255, Math.round(v))).toString(16).padStart(2, '0')).join('');
  const mix = (a, b, t) => { const x = rgb(a), y = rgb(b); return hex(x.map((v, i) => v + (y[i] - v) * t)); };
  function toHsl(c) {
    const [r, g, b] = rgb(c).map(v => v / 255), mx = Math.max(r, g, b), mn = Math.min(r, g, b), l = (mx + mn) / 2;
    let h = 0, s = 0;
    if (mx !== mn) { const d = mx - mn; s = l > .5 ? d / (2 - mx - mn) : d / (mx + mn); h = 60 * (mx === r ? (g - b) / d + (g < b ? 6 : 0) : mx === g ? (b - r) / d + 2 : (r - g) / d + 4); }
    return [h, s, l];
  }
  function fromHsl(h, s, l) {
    h = (((h % 360) + 360) % 360) / 30;
    const a = s * Math.min(l, 1 - l), f = n => { const k = (n + h) % 12; return l - a * Math.max(-1, Math.min(k - 3, 9 - k, 1)); };
    return hex([f(0), f(8), f(4)].map(v => v * 255));
  }
  const toward = (h, t, k) => h + (((t - h + 540) % 360) - 180) * k;
  // Shadows lean toward violet-blue and richer saturation, lights toward warm yellow: painterly, never grey.
  function dark(c, k) { const [h, s, l] = toHsl(c); return fromHsl(toward(h, 250, k * .16), Math.min(1, s * (1 + k * .3) + k * .06), l * (1 - k)); }
  function light(c, k) { const [h, s, l] = toHsl(c); return fromHsl(toward(h, 55, k * .14), Math.min(1, s * (1 + k * .08)), l + (1 - l) * k); }
  const tone = (c, o = .52, d = .16, l = .14) => ({ o: dark(c, o), d: dark(c, d), l: light(c, l), h: light(c, .6), c });

  // ---------- toon primitives: each returns [outline, fill] so groups can draw all outlines first ----------
  const OW = 2.2;
  const stack = parts => parts.map(p => p[0]).join('') + parts.map(p => p[1]).join('');
  const seq = parts => parts.map(p => p[0] + p[1]).join('');
  const gloss = (x, y, rx, ry, rot = -35, op = .8) => El(0, 0, rx, ry, { fill: '#fff', opacity: op, transform: T(x, y, 1, rot) });
  const shrink = (cx, cy, k, dx, dy) => 'translate(' + N(cx * (1 - k) + dx) + ',' + N(cy * (1 - k) + dy) + ') scale(' + k + ')';
  function blob(x, y, r, t, o = {}) {
    const ow = o.ow ?? OW;
    let body = Ci(x, y, r, { fill: t.d }) + Ci(x - r * .16, y - r * .19, r * .8, { fill: t.l });
    if (o.hl) body += gloss(x - r * .36, y - r * .44, r * .3, r * .16, -35, o.hl === true ? .75 : o.hl);
    return [Ci(x, y, r + ow, { fill: t.o }), body];
  }
  // a cel-shaded path; c = [cx, cy, size] gives the light layer's pivot
  function shape(d, t, o = {}) {
    const ow = o.ow ?? OW, c = o.c || [0, 0, 10];
    let body = Pa(d, { fill: t.d });
    if (o.light !== false) body += Pa(o.lightD || d, { fill: t.l, transform: o.lightD ? o.lightT : shrink(c[0], c[1], o.k || .8, -c[2] * .12, -c[2] * .15) });
    if (o.hl) body += o.hl;
    if (o.extra) body += o.extra;
    return [Pa(d, { fill: t.o, stroke: t.o, 'stroke-width': ow * 2, 'stroke-linejoin': 'round' }), body];
  }
  // a stroked limb (stem, branch, vine): outline, body, then a thin light streak up-left
  function limb(d, w, t, o = {}) {
    const ow = o.ow ?? OW * .8, cap = { fill: 'none', 'stroke-linecap': 'round', 'stroke-linejoin': 'round' };
    return [Pa(d, Object.assign({ stroke: t.o, 'stroke-width': w + ow * 2 }, cap)),
      Pa(d, Object.assign({ stroke: t.d, 'stroke-width': w }, cap)) + (o.light === false ? '' : Pa(d, Object.assign({ stroke: t.l, 'stroke-width': w * .42, transform: T(-w * .16, -w * .14) }, cap)))];
  }
  const leafD = (L, W) => D`M0,0 C${L * .22},${-W} ${L * .68},${-W} ${L},0 C${L * .68},${W} ${L * .22},${W} 0,0Z`;
  const halfLeafD = (L, W, s) => D`M0,0 C${L * .22},${s * W} ${L * .68},${s * W} ${L},0 C${L * .62},${s * W * .1} ${L * .3},${s * W * .08} 0,0Z`;
  // an almond leaf from (x,y) pointing at angle ang (deg, 0 = right), lit half + midrib
  function leaf(x, y, L, W, ang, t, o = {}) {
    const a = ang * RAD, upper = (-.6 * Math.sin(a) + .8 * Math.cos(a)) >= 0, tr = T(x, y, 1, ang), ow = o.ow ?? 1.5;
    const body = G({ transform: tr }, Pa(leafD(L, W), { fill: t.d }), Pa(halfLeafD(L, W, upper ? -1 : 1), { fill: t.l }),
      o.rib === false ? '' : Pa(D`M${L * .1},0 Q${L * .5},${upper ? W * .1 : -W * .1} ${L * .84},0`, { fill: 'none', stroke: t.o, 'stroke-width': .75, opacity: .5, 'stroke-linecap': 'round' }));
    return [Pa(leafD(L, W), { fill: t.o, stroke: t.o, 'stroke-width': ow * 2, 'stroke-linejoin': 'round', transform: tr }), body];
  }
  const heartD = (x, y, s) => D`M${x},${y + .92 * s} C${x - .14 * s},${y + .78 * s} ${x - s},${y + .3 * s} ${x - s},${y - .22 * s} C${x - s},${y - .8 * s} ${x - .34 * s},${y - 1.02 * s} ${x},${y - .52 * s} C${x + .34 * s},${y - 1.02 * s} ${x + s},${y - .8 * s} ${x + s},${y - .22 * s} C${x + s},${y + .3 * s} ${x + .14 * s},${y + .78 * s} ${x},${y + .92 * s}Z`;
  const dropD = (x, y, s) => D`M${x},${y - 1.35 * s} C${x + .3 * s},${y - .8 * s} ${x + s},${y - .28 * s} ${x + s},${y + .3 * s} A${s},${s} 0 0 1 ${x - s},${y + .3 * s} C${x - s},${y - .28 * s} ${x - .3 * s},${y - .8 * s} ${x},${y - 1.35 * s}Z`;
  const starD = (x, y, R, r, n = 4, rot = 0) => { let d = ''; for (let i = 0; i < n * 2; i++) { const a = (rot - 90 + i * 180 / n) * RAD, k = i % 2 ? r : R; d += (i ? 'L' : 'M') + N(x + k * Math.cos(a)) + ',' + N(y + k * Math.sin(a)); } return d + 'Z'; };
  const shadow = (rx, ry, x = 0, y = 0, op) => El(x, y, rx, ry ?? rx * .34, { fill: 'url(#art-shadow)', opacity: op });
  const sparkle = (x, y, r, delay = 0, col = '#fffbe0') => G({ transform: T(x, y) }, G(anim('art-twinkle', '50% 50%', delay), Pa(starD(0, 0, r, r * .28, 4), { fill: col, stroke: '#ffd75a', 'stroke-width': .6 }), Ci(0, 0, r * .25, { fill: '#fff' })));
  // tapered trunk from base (x0,y0, width w0) to top (x1,y1, width w1), bowed by `bend`, with a root flare
  function trunkD(x0, y0, x1, y1, w0, w1, bend = 0, flare = 0) {
    const h = y0 - y1, cx = (x0 + x1) / 2 + bend, cy = (y0 + y1) / 2, wm = (w0 + w1) / 4;
    return D`M${x0 - w0 / 2 - flare},${y0} C${x0 - w0 / 2},${y0 - h * .1} ${cx - wm},${cy + h * .1} ${x1 - w1 / 2},${y1} L${x1 + w1 / 2},${y1} C${cx + wm},${cy + h * .1} ${x0 + w0 / 2},${y0 - h * .1} ${x0 + w0 / 2 + flare},${y0} Q${x0},${y0 + 2.2} ${x0 - w0 / 2 - flare},${y0}Z`;
  }
  function trunk(x0, y0, x1, y1, w0, w1, bend, flare, t, o = {}) {
    const d = trunkD(x0, y0, x1, y1, w0, w1, bend, flare);
    const lit = trunkD(x0 - w0 * .2, y0 - 1, x1 - w1 * .2, y1, w0 * .42, w1 * .45, bend, 0);
    let grain = '';
    if (o.grain !== false) {
      const h = y0 - y1;
      grain = Pa(D`M${x0 + w0 * .12},${y0 - h * .12} Q${(x0 + x1) / 2 + bend * .8 + w0 * .1},${(y0 + y1) / 2} ${x1 + w1 * .1},${y1 + h * .1}`, { fill: 'none', stroke: t.o, 'stroke-width': .9, opacity: .45, 'stroke-linecap': 'round' })
        + Pa(D`M${x0 + w0 * .26},${y0 - h * .35} q${w0 * .05},${-h * .12} ${-w0 * .02},${-h * .22}`, { fill: 'none', stroke: t.o, 'stroke-width': .8, opacity: .4, 'stroke-linecap': 'round' });
    }
    return [Pa(d, { fill: t.o, stroke: t.o, 'stroke-width': (o.ow ?? OW) * 2, 'stroke-linejoin': 'round' }), Pa(d, { fill: t.d }) + Pa(lit, { fill: t.l }) + grain];
  }

  // ---------- geometry ----------
  const ringOf = (q, r) => (Math.abs(q) + Math.abs(r) + Math.abs(q + r)) / 2;
  const elev = (ring, S = ART.S) => (4 - ring) * 0.16 * S;
  const depth = (ring, S = ART.S) => 0.42 * S + elev(ring, S);
  const project = (q, r, S = ART.S) => [S * R3 * (q + r / 2), S * 1.5 * r * TILT - elev(ringOf(q, r), S)];
  const corner = (i, k) => { const a = (60 * i - 30) * RAD; return [k * Math.cos(a), k * Math.sin(a) * TILT]; };
  const facePts = (k, dy = 0) => [0, 1, 2, 3, 4, 5].map(i => { const c = corner(i, k); return [c[0], c[1] + dy]; });
  const ptsStr = a => a.map(p => N(p[0]) + ',' + N(p[1])).join(' ');
  const DIRS = [[1, 0], [1, -1], [0, -1], [-1, 0], [-1, 1], [0, 1]];
  const EDGE = [[0, 1], [5, 0], [4, 5], [3, 4], [2, 3], [1, 2]];   // direction index → top-face corner pair
  function hexes(R = 4) {
    const hs = [{ q: 0, r: 0, ring: 0, sector: -1 }];
    for (let k = 1; k <= R; k++) {
      let q = DIRS[4][0] * k, r = DIRS[4][1] * k;
      for (let side = 0; side < 6; side++) for (let s = 0; s < k; s++) { hs.push({ q, r, ring: k, sector: (3 - side + 6) % 6 }); q += DIRS[side][0]; r += DIRS[side][1]; }
    }
    return hs;
  }

  // ---------- tiles ----------
  // Colour deepens toward the centre (El, 2026-10-06): Ring 4 fresh spring green → Ring 1 deep rich emerald,
  // the World Tree hex darkest (mossy earth). Big steps on purpose, so the rings read from across the room.
  const UP = ['#3f3322', '#0b6b47', '#21944a', '#58bb3e', '#a9df48'];     // ring 0..4 face-up
  const DOWN = ['#332c1f', '#173f30', '#2b6038', '#4d8442', '#7fa94e'];   // mossy leaf blanket, deepening inward the same way
  const FACE_L = [.06, .05, .08, .1, .13];                                // light layer: deep rings stay deep
  const SOIL = ['#9c6b42', '#7a4a2c', '#4a2a17'];                         // lower-right wall, lower-left wall, outline

  function tuft(x, y, s, t) {
    const d = D`M${x - 3 * s},${y} Q${x - 3.4 * s},${y - 4 * s} ${x - 4.6 * s},${y - 6.2 * s} Q${x - 1.6 * s},${y - 4.4 * s} ${x - .8 * s},${y - 1.6 * s} Q${x - .2 * s},${y - 6 * s} ${x + .6 * s},${y - 8 * s} Q${x + 1.6 * s},${y - 5 * s} ${x + 1 * s},${y - 1.4 * s} Q${x + 2.6 * s},${y - 4.6 * s} ${x + 5 * s},${y - 5.6 * s} Q${x + 3.6 * s},${y - 2.6 * s} ${x + 3 * s},${y}Z`;
    return Pa(d, { fill: t.o, stroke: t.o, 'stroke-width': 2, 'stroke-linejoin': 'round' }) + Pa(d, { fill: t.d }) + Pa(D`M${x - .8 * s},${y - 1.6 * s} Q${x - .2 * s},${y - 6 * s} ${x + .6 * s},${y - 8 * s} Q${x + .4 * s},${y - 4.4 * s} ${x + .2 * s},${y - 1.4 * s}Z`, { fill: t.l });
  }
  function flower(x, y, s, col, mid = '#ffd23f') {
    let p = '';
    for (let i = 0; i < 5; i++) { const a = (i * 72 - 90) * RAD; p += El(x + Math.cos(a) * 1.7 * s, y + Math.sin(a) * 1.7 * s * TILT, 1.5 * s, 1.5 * s * .8, { fill: col }); }
    return El(x, y + .4, 3.6 * s, 3.6 * s * TILT, { fill: dark(col, .55) }) + p + Ci(x, y, .9 * s, { fill: mid });
  }
  function pebble(x, y, s, c = '#a8a29a') {
    const t = tone(c);
    return El(x, y, 3.2 * s, 2 * s, { fill: t.o }) + El(x, y - .3, 2.5 * s, 1.4 * s, { fill: t.d }) + El(x - .5 * s, y - .6 * s, 1.6 * s, .8 * s, { fill: t.l }) + El(x - .9 * s, y - .9 * s, .6 * s, .3 * s, { fill: '#fff', opacity: .7 });
  }
  function mushroom(x, y, s, c = '#d8473f') {
    const t = tone(c), st = tone('#f3e6c8');
    return El(x, y + .4, 3 * s, 1 * s, { fill: 'url(#art-shadow)' }) + stack([shape(D`M${x - 1.2 * s},${y} L${x - 1 * s},${y - 3 * s} L${x + 1 * s},${y - 3 * s} L${x + 1.2 * s},${y}Z`, st, { ow: 1, light: false }),
      shape(D`M${x - 3.4 * s},${y - 2.6 * s} Q${x},${y - 7.4 * s} ${x + 3.4 * s},${y - 2.6 * s}Z`, t, { ow: 1.1, c: [x, y - 3.6 * s, 3 * s] })])
      + Ci(x - 1.2 * s, y - 4.2 * s, .6 * s, { fill: '#fff' }) + Ci(x + 1.1 * s, y - 3.4 * s, .45 * s, { fill: '#fff' });
  }
  // a fallen leaf lying flat (squashed into the ground plane)
  function flatLeaf(x, y, L, W, ang, t) {
    return G({ transform: 'translate(' + N(x) + ',' + N(y) + ') scale(1,' + TILT + ')' }, stack([leaf(-L / 2 * Math.cos(ang * RAD), -L / 2 * Math.sin(ang * RAD), L, W, ang, t, { ow: 1.1 })]));
  }

  function tile(o = {}) {
    const S = o.S || ART.S, world = !!o.world || o.ring === 0, ring = world ? 0 : (o.ring ?? 4);
    const R = rng(o.seed ?? (o.q != null ? hashQR(o.q, o.r, ring + 7) : (ring * 977 + (o.sector ?? 0) * 131 + (o.up ? 7 : 3))));
    const vc = o.sector != null && o.sector >= 0 ? VAL[o.sector] : null;
    let top = world ? UP[0] : (o.up ? UP : DOWN)[ring];
    if (vc) top = mix(top, vc, .15);
    const tt = tone(top, .5, .12, FACE_L[ring]), k = S0 * .965, Dp = depth(ring, S0);
    const c = facePts(k), down = p => [p[0], p[1] + Dp];
    const deep = Math.max(0, 4 - ring) * .05;
    const sR = mix(SOIL[0], '#3a2418', deep), sL = mix(SOIL[1], '#2a160c', deep);
    const out = [];
    // silhouette outline, then the two visible walls (lower-left darker)
    out.push(el('polygon', { points: ptsStr([c[5], c[0], c[1], down(c[1]), down(c[2]), down(c[3]), c[3], c[4]]), fill: SOIL[2], stroke: SOIL[2], 'stroke-width': 2.6, 'stroke-linejoin': 'round' }));
    out.push(el('polygon', { points: ptsStr([c[1], c[2], down(c[2]), down(c[1])]), fill: sR }));
    out.push(el('polygon', { points: ptsStr([c[2], c[3], down(c[3]), down(c[2])]), fill: sL }));
    // strata line, embedded pebbles, a root, ambient shade toward the base
    const sy = Math.min(Dp * .5, 9);
    out.push(Pa(D`M${c[1][0]},${c[1][1] + sy} L${c[2][0]},${c[2][1] + sy} L${c[3][0]},${c[3][1] + sy}`, { fill: 'none', stroke: dark(sR, .35), 'stroke-width': 1.1, opacity: .7, 'stroke-dasharray': '7 3 12 4' }));
    for (let i = 0; i < 2; i++) {
      const u = .15 + R() * .7, onR = R() < .5, A = onR ? c[1] : c[2], B = onR ? c[2] : c[3], yy = A[1] + (B[1] - A[1]) * u + sy + 2 + R() * 3;
      if (yy < Math.max(A[1], B[1]) + Dp - 2) out.push(El(A[0] + (B[0] - A[0]) * u, yy, 1.8, 1.1, { fill: mix(onR ? sR : sL, '#d8cbb8', .35), stroke: dark(sR, .4), 'stroke-width': .6 }));
    }
    out.push(el('polygon', { points: ptsStr([c[1], c[2], c[3], down(c[3]), down(c[2]), down(c[1])]), fill: 'url(#art-wallshade)' }));
    // grass / moss lip hanging over the front edges
    const lipT = tone(world ? '#4f6b38' : top, .55, .26);
    let lip = D`M${c[1][0]},${c[1][1]} L${c[2][0]},${c[2][1]} L${c[3][0]},${c[3][1]}`;
    const sc = [[c[3], c[2]], [c[2], c[1]]];
    for (const [A, B] of sc) {
      const n = 4;
      for (let j = 0; j < n; j++) {
        const x0 = A[0] + (B[0] - A[0]) * j / n, y0 = A[1] + (B[1] - A[1]) * j / n, x1 = A[0] + (B[0] - A[0]) * (j + 1) / n, y1 = A[1] + (B[1] - A[1]) * (j + 1) / n;
        lip += D` Q${(x0 + x1) / 2},${(y0 + y1) / 2 + 5.2 + (j % 2) * 1.2} ${x1},${y1}`;
      }
    }
    out.push(Pa(lip + 'Z', { fill: lipT.d, stroke: lipT.o, 'stroke-width': 1.3, 'stroke-linejoin': 'round' }));
    // top face: dark body, lighter inner layer up-left, soft highlight along the back edges
    out.push(el('polygon', { points: ptsStr(c), fill: tt.d, stroke: tt.o, 'stroke-width': 1.6, 'stroke-linejoin': 'round' }));
    out.push(el('polygon', { points: ptsStr(facePts(k * .9).map(p => [p[0] - 1.6, p[1] - 1.1])), fill: tt.l }));
    out.push(el('polygon', { points: ptsStr(c), fill: 'url(#art-topsheen)' }));
    const hi = facePts(k * .9);
    out.push(Pa(D`M${hi[3][0] - 1},${hi[3][1] - .6} L${hi[4][0] - 1},${hi[4][1] - .6} L${hi[5][0]},${hi[5][1] - .9}`, { fill: 'none', stroke: '#fff', 'stroke-width': 1.3, opacity: .35, 'stroke-linecap': 'round', 'stroke-linejoin': 'round' }));
    // surface detail
    const inHex = (rad) => { for (;;) { const x = (R() * 2 - 1) * rad, y = (R() * 2 - 1) * rad; if (Math.abs(x) < rad * .866 && Math.abs(y) + Math.abs(x) * .577 < rad) return [x, y * TILT]; } };
    if (world) {
      const moss = tone('#557a3c');
      for (let i = 0; i < 5; i++) { const [x, y] = inHex(30); out.push(El(x, y, 6 + R() * 5, (3 + R() * 2.5), { fill: moss.d, opacity: .8 }) + El(x - 1, y - .8, 4 + R() * 3, 2 + R(), { fill: moss.l, opacity: .7 })); }
      for (let i = 0; i < 4; i++) { const [x, y] = inHex(30); out.push(mushroom(x, y + 2, .55, '#7fd6c8')); }
    } else if (o.up) {
      for (let i = 0; i < 7; i++) { const [x, y] = inHex(32); out.push(Pa(D`M${x},${y} q.8,-2.4 2.2,-3.4`, { fill: 'none', stroke: i % 2 ? tt.h : tt.o, 'stroke-width': 1, opacity: i % 2 ? .6 : .35, 'stroke-linecap': 'round' })); }
      const decoN = 2 + Math.floor(R() * 3);
      const deco = [];
      for (let i = 0; i < decoN; i++) {
        const a = R() * 360 * RAD, rr = 18 + R() * 12, x = Math.cos(a) * rr, y = Math.sin(a) * rr * TILT;
        const pick = R();
        if (o.kind === 'spring' && i < 2) deco.push([y, flower(x, y, .8, '#9fd8ff', '#fff3a8')]);
        else if (pick < .42) deco.push([y, tuft(x, y + 1, .8 + R() * .3, tone(mix(top, '#2f7a32', .45), .5, .1, .05))]);
        else if (pick < .72) deco.push([y, flower(x, y, .7 + R() * .2, R() < .5 ? '#ffffff' : (vc ? light(vc, .45) : '#fff6b0'))]);
        else if (pick < .88) deco.push([y, pebble(x, y, .8 + R() * .3)]);
        else deco.push([y, mushroom(x, y, .7)]);
      }
      deco.sort((a, b) => a[0] - b[0]).forEach(d => out.push(d[1]));
    } else {
      const base = top, lts = [tone(mix(base, '#c7a24c', .3), .5, .12, .12), tone(mix(base, '#a8623a', .22), .5, .12, .12), tone(light(base, .14), .5, .12, .12), tone(dark(base, .08), .5, .12, .12)];
      const spots = [[-14, -9], [6, -12], [20, -4], [-24, 2], [-6, 0], [13, 7], [-14, 12], [2, 13], [24, 9], [-4, -20]];
      for (const [sx, sy2] of spots) {
        if (R() < .12) continue;
        const L = 9 + R() * 4;
        out.push(flatLeaf(sx + (R() - .5) * 5, (sy2 + (R() - .5) * 4) * TILT * 1.05, L, L * .42, R() * 360, lts[Math.floor(R() * lts.length)]));
      }
      for (let i = 0; i < 3; i++) { const [x, y] = inHex(26), m = tone(mix(base, '#3f7a3a', .4)), r = 2.2 + R() * 1.2; out.push(El(x, y, r * 1.5, r, { fill: m.d }) + El(x - .5, y - .4, r * 1.1, r * .65, { fill: m.l }) + Ci(x - r * .5, y - r * .3, r * .22, { fill: '#fff', opacity: .35 })); }
      if (R() < .3) { const [x, y] = inHex(24); out.push(mushroom(x, y, .6, R() < .5 ? '#c9564c' : '#d9a441')); }
    }
    // Ring 4's outer edges: a ribbon in the sector's value colour, and pennants on the front walls
    let outer = o.outer;
    if (!outer && ring === 4 && o.q != null) outer = DIRS.map(([dq, dr], d) => ringOf(o.q + dq, o.r + dr) > 4 ? d : -1).filter(d => d >= 0);
    if (vc && outer && outer.length) {
      const vt = tone(vc, .5, .1, .18);
      for (const d of outer) {
        const [a, b] = EDGE[d], A = c[a], B = c[b], ki = .84, Ai = [A[0] * ki, A[1] * ki], Bi = [B[0] * ki, B[1] * ki];
        out.push(el('polygon', { points: ptsStr([A, B, Bi, Ai]), fill: vt.d, stroke: vt.o, 'stroke-width': 1.2, 'stroke-linejoin': 'round' }));
        out.push(Pa(D`M${(A[0] + Ai[0]) / 2 + (B[0] - A[0]) * .06},${(A[1] + Ai[1]) / 2 + (B[1] - A[1]) * .06} L${(B[0] + Bi[0]) / 2 - (B[0] - A[0]) * .06},${(B[1] + Bi[1]) / 2 - (B[1] - A[1]) * .06}`, { stroke: vt.l, 'stroke-width': 1.6, 'stroke-linecap': 'round', opacity: .9 }));
        if (d === 4 || d === 5) {
          out.push(el('polygon', { points: ptsStr([A, B, down(B).map((v, i) => i ? B[1] + 4.5 : v), [A[0], A[1] + 4.5]]), fill: vt.d, stroke: vt.o, 'stroke-width': 1.1, 'stroke-linejoin': 'round' }));
          for (let j = 0; j < 3; j++) {
            const u0 = .12 + j * .29, u1 = u0 + .2, x0 = A[0] + (B[0] - A[0]) * u0, y0 = A[1] + (B[1] - A[1]) * u0 + 4.5, x1 = A[0] + (B[0] - A[0]) * u1, y1 = A[1] + (B[1] - A[1]) * u1 + 4.5;
            out.push(Pa(D`M${x0},${y0} L${x1},${y1} L${(x0 + x1) / 2},${(y0 + y1) / 2 + 6.5}Z`, { fill: j % 2 ? vt.l : light(vc, .5), stroke: vt.o, 'stroke-width': 1, 'stroke-linejoin': 'round' }));
          }
        }
      }
    }
    return scaleTo(S, G({ class: 'art-tile' }, ...out));
  }

  // ---------- plants ----------
  const BARK = tone('#8a5636', .55, .18, .12);
  const GREEN = '#5cb84a';
  const sway = (R, amp) => anim('art-sway', '50% 100%', R() * 4.6, amp ? 'animation-duration:' + N(3.6 + R() * 2) + 's' : '');

  function seeded(R) {
    const soil = tone('#93603a', .55, .16, .12), seed = tone('#c98b3e', .55, .14, .2), st = tone('#6cc24a', .55, .14, .16);
    const mound = D`M-13,0 C-11,-8.5 11,-8.5 13,0 Q0,2.6 -13,0Z`;
    return shadow(15, 4.2, 0, .8) + stack([shape(mound, soil, { c: [0, -3, 12] })])
      + Ci(-6, -3, .9, { fill: soil.o, opacity: .5 }) + Ci(6.5, -2.2, .8, { fill: soil.o, opacity: .5 }) + Ci(3, -5, .6, { fill: soil.o, opacity: .4 })
      + G(sway(R), stack([limb('M0.5,-6 C1,-11 7,-13 5.5,-17.5 C4.6,-20.5 1,-19.8 2,-17.4', 2.1, st), leaf(5, -13.5, 6.5, 3.4, -20, st, { ow: 1.2, rib: false })]))
      + stack([shape(D`M-7.5,-5.5 C-8,-9.5 -2,-11.5 1.2,-8.6 C3.4,-6.4 1.6,-3.6 -2,-3.4 C-5,-3.2 -7.2,-3.8 -7.5,-5.5Z`, seed, { c: [-3, -7, 5], ow: 1.6, hl: gloss(-4.6, -8.2, 1.6, .8, -25, .9) })]);
  }
  function sprout(R, vc) {
    const soil = tone('#93603a', .55, .16, .12), st = tone(mix(GREEN, vc, .08), .55, .14, .2);
    const lt = tone(mix('#66c64e', vc, .1), .52, .15, .16);
    return shadow(12, 3.6, 0, .6) + stack([shape(D`M-9,0 C-7,-4 7,-4 9,0 Q0,2 -9,0Z`, soil, { c: [0, -1.5, 8], ow: 1.6 })])
      + G(sway(R, 1), stack([limb('M0,-1 C-4,-8 5,-14 1,-23', 2.4, st)]),
        stack([leaf(1, -23, 14, 9.5, -158, lt), leaf(1, -23, 14, 9.5, -24, lt), limb('M1,-23 c0,-3.4 3.8,-4.6 4.2,-1.8', 1.5, st, { light: false })]),
        gloss(-7.5, -27.5, 2.2, 1, -20, .55), gloss(7.6, -29, 2.2, 1, -35, .55));
  }
  function sapling(R, vc) {
    const st = tone(mix('#8a5a38', vc, .05), .55, .16, .14), lf = tone(mix(GREEN, vc, .3), .52, .16, .16), lf2 = tone(mix('#4aa443', vc, .3), .52, .16, .14);
    const cl = (x, y, s, t, hl) => [blob(x - 4 * s, y + 1 * s, 5 * s, t), blob(x + 4 * s, y + 1.4 * s, 4.6 * s, t), blob(x, y - 2.6 * s, 5.6 * s, t, { hl })];
    return shadow(16, 4.6, 1, .6)
      + stack([trunk(0, 0, 5, -34, 5.2, 3, -3.5, 1.5, st, { grain: false }), limb('M2,-17 Q-4,-21 -8,-28', 2, st), limb('M3.6,-25 Q8,-28 11,-35', 2, st)])
      + G(sway(R, 1), stack([...cl(-9, -29, .9, lf2), ...cl(12, -36, .85, lf2)]), stack([...cl(5, -45, 1, lf, true), ...cl(-2, -38, .8, lf)]));
  }
  // flame lobe for the maple: round base, licking tip
  const flameD = (r, tip) => D`M${-r},0 A${r},${r} 0 0 0 ${r},0 C${r},${-r * .7} ${r * .45},${-r * 1.05} ${r * .18},${-r * tip} C${-r * .05},${-r * (tip - .5)} ${-r * .55},${-r * 1.05} ${-r},0Z`;
  function lobes(list, t) {
    return seq(list.map(([x, y, r, ang, tip, tt, hl]) => {
      const tn = tt || t, d = flameD(r, tip || 1.6), tr = T(x, y, 1, ang);
      return [Pa(d, { fill: tn.o, stroke: tn.o, 'stroke-width': OW * 2, 'stroke-linejoin': 'round', transform: tr }),
        G({ transform: tr }, Pa(d, { fill: tn.d }), Pa(d, { fill: tn.l, transform: shrink(0, -r * .3, .76, -r * .14, -r * .12) }),
          Pa(D`M0,${r * .55} Q${r * .02},${-r * .4} ${r * .16},${-r * (tip - .3)}`, { fill: 'none', stroke: tn.o, 'stroke-width': .8, opacity: .4, 'stroke-linecap': 'round' }),
          hl ? gloss(-r * .4, -r * .45, r * .28, r * .14, -40, .7) : '')];
    }));
  }
  const fanD = (L, half = 50) => { const a = half * RAD, sx = Math.sin(a) * L, sy = -Math.cos(a) * L; return D`M0,0 L${-sx},${sy} Q${-L * .52},${-L * 1.06} ${-L * .1},${-L * .99} L0,${-L * .82} L${L * .1},${-L * .99} Q${L * .52},${-L * 1.06} ${sx},${sy}Z`; };
  function fans(list, t0) {
    return seq(list.map(([x, y, L, ang, tt]) => {
      const t = tt || t0, d = fanD(L), tr = T(x, y, 1, ang);
      return [Pa(d, { fill: t.o, stroke: t.o, 'stroke-width': 3, 'stroke-linejoin': 'round', transform: tr }),
        G({ transform: tr }, Pa(d, { fill: t.d }), Pa(d, { fill: t.l, transform: shrink(0, -L * .6, .8, -L * .1, -L * .07) }),
          Pa(D`M0,${-L * .12} L${-L * .42},${-L * .72} M0,${-L * .12} L${-L * .16},${-L * .8} M0,${-L * .12} L${L * .16},${-L * .8} M0,${-L * .12} L${L * .42},${-L * .72}`, { stroke: t.o, 'stroke-width': .7, opacity: .3, fill: 'none', 'stroke-linecap': 'round' }),
          gloss(-L * .38, -L * .66, L * .1, L * .05, -50, .55))];
    }));
  }
  function tierD(cx, ty, by, hw, lean = 0) {
    const n = 4, step = hw * 2 / n;
    let d = D`M${cx + lean},${ty} C${cx + lean - hw * .2},${ty + (by - ty) * .4} ${cx - hw * .7},${by - (by - ty) * .15} ${cx - hw},${by - 3}`;
    for (let i = 0; i < n; i++) { const x0 = cx - hw + i * step, x1 = x0 + step; d += D` Q${(x0 + x1) / 2},${by + 5.5} ${x1},${i === n - 1 ? by - 3 : by}`; }
    return d + D` C${cx + hw * .7},${by - (by - ty) * .15} ${cx + lean + hw * .2},${ty + (by - ty) * .4} ${cx + lean},${ty}Z`;
  }
  function hangFruit(x, y, s = 1) {
    const t = tone('#e8344f', .5, .14, .16), lf = tone('#5cb84a');
    return stack([limb(D`M${x},${y - 7 * s} Q${x + 1.2 * s},${y - 4 * s} ${x},${y - 2.6 * s}`, 1.1 * s, tone('#7a4a2a'), { light: false, ow: .8 }),
      leaf(x + .3 * s, y - 5.6 * s, 4.6 * s, 2.2 * s, -30, lf, { ow: 1, rib: false }),
      shape(heartD(x, y + 1.2 * s, 4.6 * s), t, { c: [x, y + .6 * s, 4.6 * s], ow: 1.6, hl: gloss(x - 2 * s, y - .6 * s, 1.3 * s, .7 * s, -40, .9) })]);
  }
  function blossom(x, y, s, col = '#fff2f7', mid = '#e2588a') {
    let p = '';
    for (let i = 0; i < 5; i++) { const a = (i * 72 - 90) * RAD; p += Ci(x + Math.cos(a) * 1.9 * s, y + Math.sin(a) * 1.9 * s, 1.6 * s, { fill: col, stroke: dark(mid, .35), 'stroke-width': .55 }); }
    return p + Ci(x, y, 1.05 * s, { fill: mid });
  }
  const drifter = (x, y, R, body) => G({ transform: T(x, y) }, G(anim('art-drift', '50% 50%', R() * 9, 'animation-duration:' + N(8 + R() * 5) + 's'), body));

  const SPECIES = ['maple', 'ginkgo', 'pine', 'cherry', 'jacaranda', 'oak'];
  const FRUIT_AT = {
    maple: [[-15, -40], [14, -44], [-3, -55]], ginkgo: [[-12, -42], [13, -46], [1, -60]], pine: [[-12, -26], [15, -40], [3, -54]],
    cherry: [[-17, -42], [17, -45], [2, -52]], jacaranda: [[-15, -47], [14, -48], [0, -56]], oak: [[-24, -38], [22, -40], [0, -48]],
  };
  function bigTree(sector, R, harvestable) {
    const sp = SPECIES[((sector % 6) + 6) % 6];
    let back = '', canopy = '', front = '';
    if (sp === 'maple') {
      const A = tone('#d6452b', .52, .16, .1), B = tone('#f07a2d', .5, .14, .14), C = tone('#f7a23a', .5, .14, .16);
      back = shadow(30, 9, 0, 1) + stack([trunk(0, 0, -1, -36, 11, 6, -2.5, 3, BARK), limb('M-1,-28 Q-8,-34 -13,-45', 3.6, BARK), limb('M0,-30 Q7,-36 11,-47', 3.6, BARK)]);
      canopy = lobes([[-21, -46, 10.5, -48, 1.55, A], [21, -47, 10.5, 46, 1.55, A], [-11, -60, 12, -20, 1.6, A], [11, -61, 12, 20, 1.6, A], [0, -66, 12, 2, 1.6, A],
        [-14, -44, 10, -26, 1.5, B], [14, -44, 10, 24, 1.5, B], [-5, -55, 10.5, -8, 1.55, B, true], [6, -54, 10, 10, 1.5, B], [0, -45, 10, 0, 1.4, C]], A);
      front = drifter(16, -40, R, Pa(D`M0,-4 L1.2,-1.2 4,-1.8 2.6,.8 3.6,3.4 0,2.2 -3.6,3.4 -2.6,.8 -4,-1.8 -1.2,-1.2Z`, { fill: '#f07a2d', stroke: '#9a2f17', 'stroke-width': .9, 'stroke-linejoin': 'round' }));
    } else if (sp === 'ginkgo') {
      const bark = tone('#937660', .55, .18, .12), A = tone('#e39a1f', .52, .14, .1), B = tone('#f4bb2c', .5, .12, .14), F = tone('#ffd546', .48, .1, .18);
      back = shadow(28, 8.5, 0, 1) + stack([trunk(0, 0, 0, -40, 10, 5.5, 1.2, 2.5, bark), limb('M0,-30 Q-7,-36 -10,-44', 3, bark), limb('M.5,-34 Q7,-40 9,-48', 3, bark)]);
      canopy = fans([[0, -58, 26, 0, A], [-14, -49, 23, -16, A], [14, -49, 23, 16, A], [-19, -38, 19, -30, B], [0, -40, 22, 0, B], [19, -38, 19, 30, B], [-9, -31, 15, -12, F], [9, -31, 15, 12, F]]);
      front = drifter(-14, -36, R, Pa(fanD(5.5), { fill: '#ffd94f', stroke: '#a8661a', 'stroke-width': .9, 'stroke-linejoin': 'round' }));
    } else if (sp === 'pine') {
      const bark = tone('#7c3a29', .55, .18, .12);
      const tiers = [[1, -50, -22, 31, 1.5, '#a8323a'], [3, -62, -36, 25, 2, '#bd3c3d'], [5.5, -74, -50, 18.5, 2.5, '#cf4b41'], [8, -83, -63, 12, 2, '#de5e47']];
      back = shadow(30, 9, 0, 1) + stack([trunk(0, 0, 2.5, -26, 9, 5, 1.2, 2.5, bark)]);
      canopy = seq(tiers.map(([cx, ty, by, hw, lean, col], i) => {
        const t = tone(col, .52, .14, .14);
        return shape(tierD(cx, ty, by, hw, lean), t, { c: [cx, (ty + by) / 2 + 3, hw * .6], k: .8, hl: i === 3 ? gloss(cx + lean * .4 - 3.2, ty + 9, 2, 1.1, -55, .7) : (i === 1 ? gloss(cx - 9, by - 7, 2.6, 1.2, -30, .45) : '') });
      }));
      front = drifter(-18, -28, R, Pa('M0,-3.5 L.7,3.5 M-1.2,-2 L1.4,2.6', { stroke: '#cf4b41', 'stroke-width': 1.4, 'stroke-linecap': 'round' }));
    } else if (sp === 'cherry') {
      const bark = tone('#6b3a3c', .55, .18, .14), A = tone('#e57fae', .52, .14, .12), B = tone('#f6a2c6', .48, .12, .16);
      back = shadow(29, 9, 0, 1) + stack([trunk(0, 0, -2, -28, 10, 6, -3.5, 3, bark), limb('M-2,-24 Q-12,-30 -17,-42', 3.6, bark), limb('M-1,-26 Q8,-32 13,-44', 3.4, bark), limb('M-2,-27 Q-1,-38 -3,-50', 3, bark)]);
      const blobsA = [[-22, -52, 10.5, A], [22, -53, 10.5, A], [-10, -66, 12.5, A], [12, -66, 12.5, A], [1, -70, 11, A],
        [-15, -46, 11, B], [16, -47, 11, B], [0, -55, 13, B, true], [-25, -43, 7.5, B], [26, -45, 7, B]];
      canopy = stack(blobsA.map(([x, y, r, t, hl]) => blob(x, y, r, t, { hl })));
      for (const [x, y] of [[-20, -56], [-7, -71], [10, -68], [22, -51], [-13, -44], [6, -50], [18, -44], [-3, -60], [-26, -46]]) canopy += blossom(x, y, .8);
      front = drifter(12, -40, R, El(0, 0, 2.2, 1.4, { fill: '#ffd2e4', stroke: '#c0507e', 'stroke-width': .8 })) + drifter(-18, -38, R, El(0, 0, 2, 1.3, { fill: '#ffe4ef', stroke: '#c0507e', 'stroke-width': .8 }));
    } else if (sp === 'jacaranda') {
      const bark = tone('#7a6352', .55, .18, .12), A = tone('#5452c4', .5, .14, .08), B = tone('#6f73dc', .48, .12, .1), F = tone('#b89cf2', .5, .12, .12);
      back = shadow(30, 9, 0, 1) + stack([trunk(0, 0, 1, -32, 8.5, 5, 2, 2.5, bark), limb('M.5,-27 Q-9,-34 -18,-46', 3, bark), limb('M1,-29 Q10,-36 17,-48', 3, bark), limb('M1,-32 Q2,-42 1,-52', 2.6, bark)]);
      const cl = [[-23, -55, 11, A], [23, -55, 11, A], [-11, -66, 13, A], [12, -67, 13, A], [0, -70, 12, A], [-15, -51, 11, B], [16, -51, 11, B], [0, -57, 13, B, true], [-28, -49, 7, B], [28, -49, 7, B]];
      const bunch = (x, y) => mass([[x - 3.2, y, 2.9], [x + 3.2, y, 2.9], [x, y + 1.4, 3.1], [x - 1.8, y + 4.8, 2.5], [x + 1.8, y + 4.8, 2.5], [x, y + 8, 2.1], [x, y + 10.6, 1.4]], F, { ow: 1.2 })
        + Ci(x - 2.2, y + 1.6, .8, { fill: '#fff', opacity: .7 }) + Ci(x + 1.2, y + 5, .7, { fill: '#fff', opacity: .55 }) + Ci(x - .5, y + 8.4, .55, { fill: '#fff', opacity: .5 });
      canopy = stack(cl.slice(0, 5).map(([x, y, r, t]) => blob(x, y, r, t))) + bunch(-24, -41) + bunch(-8, -39) + bunch(8, -40) + bunch(23, -41)
        + stack(cl.slice(5).map(([x, y, r, t, hl]) => blob(x, y, r, t, { hl })));
      front = drifter(-10, -36, R, Ci(0, 0, 1.8, { fill: '#c4b0ff', stroke: '#5a46b0', 'stroke-width': .8 }));
    } else {
      const bark = tone('#7a5234', .55, .18, .12), A = tone('#1f8a78', .52, .14, .1), B = tone('#33a98a', .48, .12, .16);
      back = shadow(36, 10, 0, 1) + stack([trunk(0, 0, 0, -30, 15, 9, 0, 4, bark), limb('M-1,-24 Q-10,-28 -19,-40', 5, bark), limb('M1,-25 Q10,-29 19,-41', 5, bark)]);
      const cl = [[-28, -48, 12, A], [28, -48, 12, A], [-14, -61, 15, A], [14, -61, 15, A], [0, -65, 14, A],
        [-33, -40, 8.5, B], [33, -40, 8.5, B], [-18, -44, 12, B], [18, -44, 12, B], [0, -50, 14, B, true]];
      canopy = stack(cl.map(([x, y, r, t, hl]) => blob(x, y, r, t, { hl })));
      for (const [x, y, a] of [[-38, -46, -150], [38, -46, -30], [-24, -66, -120], [24, -66, -60]]) canopy += stack([leaf(x, y, 8, 4, a, B, { ow: 1.3 })]);
      front = drifter(20, -36, R, Pa(leafD(6, 3), { fill: '#33a98a', stroke: '#0f4f45', 'stroke-width': .9, transform: 'rotate(30)' }));
    }
    let fruit = '';
    if (harvestable) fruit = FRUIT_AT[sp].map(([x, y]) => hangFruit(x, y, 1)).join('');
    return back + G(sway(R, 1), canopy, fruit) + front;
  }

  function plant(stage, sector, o = {}) {
    const S = o.S || ART.S, R = rng(o.seed ?? (stage * 1013 + (sector ?? 0) * 7919 + 1)), vc = sector != null && sector >= 0 ? VAL[sector % 6] : GREEN;
    if (!stage) return '';
    const body = stage <= 1 ? seeded(R) : stage === 2 ? sprout(R, vc) : stage === 3 ? sapling(R, vc) : bigTree(sector ?? 5, R, o.harvestable);
    return scaleTo(S, G({ class: 'art-plant art-stage' + stage }, body));
  }

  // ---------- items ----------
  function fruitBody(s = 1) {
    const t = tone('#e8344f', .5, .14, .16), lf = tone('#62c04c', .55, .14, .14);
    return stack([limb(D`M${.6 * s},${-8.4 * s} Q${1.6 * s},${-10.6 * s} ${3.4 * s},${-11 * s}`, 1.5 * s, tone('#7a4a2a'), { light: false, ow: 1 }), leaf(1 * s, -9.6 * s, 7 * s, 3.4 * s, -28, lf, { ow: 1.1 }),
      shape(heartD(0, 0, 9 * s), t, { c: [0, -.8 * s, 9 * s], ow: 2.2, hl: gloss(-4.2 * s, -4.2 * s, 2.6 * s, 1.3 * s, -40, .9) + Ci(4.6 * s, -3.2 * s, .9 * s, { fill: '#fff', opacity: .55 }) })]);
  }
  function dropBody(s = 1) {
    const t = tone('#3aa6f0', .52, .14, .18);
    return stack([shape(dropD(0, 1.6 * s, 7.6 * s), t, { c: [0, 2.4 * s, 7.6 * s], ow: 2.2, hl: gloss(-3 * s, 0, 1.6 * s, 3.2 * s, 20, .85) + Ci(3.6 * s, 5 * s, 1 * s, { fill: '#fff', opacity: .5 }) })]);
  }
  function acornBody(s = 1) {
    const nut = tone('#c98a44', .52, .14, .16), cap = tone('#7d4f2c', .52, .14, .12);
    const capD = D`M${-9 * s},${-2 * s} C${-9.6 * s},${-8.6 * s} ${9.6 * s},${-8.6 * s} ${9 * s},${-2 * s} Q0,${.6 * s} ${-9 * s},${-2 * s}Z`;
    return stack([shape(D`M${-7.4 * s},${-2 * s} C${-7.6 * s},${5 * s} ${-3 * s},${9.4 * s} 0,${11 * s} C${3 * s},${9.4 * s} ${7.6 * s},${5 * s} ${7.4 * s},${-2 * s}Z`, nut, { c: [0, 3 * s, 7 * s], hl: gloss(-3.6 * s, 2.4 * s, 1.4 * s, 2.8 * s, 15, .75) }),
      shape(capD, cap, { c: [0, -4 * s, 8 * s], extra: Pa(D`M${-6 * s},${-5.6 * s} L${-3 * s},${-2 * s} M${-2 * s},${-6.8 * s} L${1 * s},${-2.4 * s} M${2.4 * s},${-6.8 * s} L${5.2 * s},${-2.6 * s} M${6 * s},${-5.2 * s} L${3.8 * s},${-2.2 * s} M${-5.6 * s},${-2.4 * s} L${-1.8 * s},${-6.8 * s} M${-1.2 * s},${-2.2 * s} L${3 * s},${-7 * s}`, { stroke: cap.o, 'stroke-width': .8 * s, opacity: .45 }) }),
      limb(D`M0,${-7 * s} Q${.6 * s},${-10 * s} ${2.6 * s},${-11 * s}`, 2 * s, cap, { light: false, ow: 1 })]);
  }
  const sized = (size, base, inner) => (size && size !== base ? G({ transform: 'scale(' + N(size / base) + ')' }, inner) : inner);

  // ---------- treasures ----------
  function compassBody(R) {
    const brass = tone('#e2a93b', .55, .16, .18), face = tone('#fbf1d8', .45, .06, .05);
    return stack([blob(0, -12.4, 2.8, brass, { ow: 1.4 }), blob(0, 0, 12, brass, { ow: 2.4 })]) + Ci(0, -12.4, 1.3, { fill: brass.o })
      + Ci(0, 0, 8.6, { fill: face.o }) + Ci(0, 0, 7.8, { fill: face.d }) + Ci(-.8, -.9, 6.6, { fill: face.l })
      + [0, 90, 180, 270].map(a => Pa(D`M0,-7.4 L0,-5.6`, { stroke: '#8a5a24', 'stroke-width': 1.3, transform: 'rotate(' + a + ')', 'stroke-linecap': 'round' })).join('')
      + Pa('M0,-7.8 L1.5,-5.4 -1.5,-5.4Z', { fill: '#d23a35' })
      + G(anim('art-needle', '50% 50%', R ? R() * 3 : 0), Pa('M0,-6.2 L2,0 0,.4 -2,0Z', { fill: '#e2453c', stroke: '#8a1f1a', 'stroke-width': .6, 'stroke-linejoin': 'round' }), Pa('M0,6.2 L2,0 0,-.4 -2,0Z', { fill: '#e8edf2', stroke: '#5a6470', 'stroke-width': .6, 'stroke-linejoin': 'round' }))
      + Ci(0, 0, 1.3, { fill: '#8a5a24' }) + Pa('M-6,-3.6 A6.8,6.8 0 0 1 -2.4,-6.6', { fill: 'none', stroke: '#fff', 'stroke-width': 1.5, opacity: .85, 'stroke-linecap': 'round' });
  }
  function lanternBody(R) {
    const paper = tone('#f0753a', .5, .16, .2), wood = tone('#5d3a26', .5, .14, .14), glow = '#ffd36e';
    return G(anim('art-flicker', '50% 50%', R ? R() * 2.6 : 0), Ci(0, 1, 19, { fill: 'url(#art-warm)' }))
      + stack([limb('M-5,-13 Q0,-19 5,-13', 1.4, wood, { light: false, ow: 1 }), shape(D`M-11,1 C-11,-9 11,-9 11,1 C11,11 -11,11 -11,1Z`, paper, { c: [0, 1, 10], light: false }),
        shape('M-6,-12.4 L6,-12.4 L5,-8 L-5,-8Z', wood, { c: [0, -10, 4], ow: 1.4 }), shape('M-5,10 L5,10 L6,13.6 L-6,13.6Z', wood, { c: [0, 12, 4], ow: 1.4 })])
      + G(anim('art-flicker', '50% 50%', R ? R() * 2.6 + .7 : .7), El(0, 1, 8.2, 8.4, { fill: glow, opacity: .85 }), El(-1.6, -.6, 4.6, 5, { fill: '#fff6cf', opacity: .9 }))
      + ['M-4.5,-7.4 C-7.2,-3 -7.2,5 -4.5,9.6', 'M4.5,-7.4 C7.2,-3 7.2,5 4.5,9.6', 'M0,-8 L0,10'].map(d => Pa(d, { fill: 'none', stroke: paper.o, 'stroke-width': .9, opacity: .55 })).join('')
      + Pa('M0,13.6 L0,17', { stroke: '#d23a35', 'stroke-width': 1.6, 'stroke-linecap': 'round' }) + Ci(0, 17.6, 1.5, { fill: '#d23a35', stroke: '#7a1f1a', 'stroke-width': .6 })
      + gloss(-6.6, -3, 1.6, 3, 20, .55);
  }
  function ropeBody() {
    const rope = tone('#cf9e5c', .55, .16, .16);
    const coil = (y, rx, ry) => { const d = D`M${-rx},${y} A${rx},${ry} 0 1 0 ${rx},${y} A${rx},${ry} 0 1 0 ${-rx},${y}`; return [Pa(d, { fill: 'none', stroke: rope.o, 'stroke-width': 6.6 }), Pa(d, { fill: 'none', stroke: rope.d, 'stroke-width': 4.4 }) + Pa(d, { fill: 'none', stroke: rope.l, 'stroke-width': 4.4, 'stroke-dasharray': '2.2 1.6', opacity: .9 })]; };
    return seq([coil(6, 12.5, 5.2), coil(2.5, 11.5, 4.8), coil(-1, 10.4, 4.4)])
      + stack([limb('M2,-1 C6,-8 -4,-13 -2,-6 C-.5,-1 8,-4 7,-10 C6.4,-14 1,-14 .6,-11', 4.2, rope)]) + Pa('M2,-1 C6,-8 -4,-13 -2,-6 C-.5,-1 8,-4 7,-10 C6.4,-14 1,-14 .6,-11', { fill: 'none', stroke: rope.o, 'stroke-width': 4.2, 'stroke-dasharray': '1 1.8', opacity: .35 })
      + stack([limb('M10,5.5 C14,8 15,11 12.5,13.5', 3.8, rope)]) + Ci(12.5, 13.6, 2.2, { fill: rope.l, stroke: rope.o, 'stroke-width': 1.2 })
      + gloss(-7, 0, 2.4, .9, -15, .55);
  }
  function stump(w = 12, h = 7, t = tone('#8f5d38', .55, .16, .14)) {
    const top = tone('#e2b47a', .45, .1, .1);
    return stack([shape(D`M${-w},${-h} L${-w},-1 Q${-w - 2.6},1.4 ${-w - 3.6},1.6 Q0,4.4 ${w + 3.6},1.6 Q${w + 2.6},1.4 ${w},-1 L${w},${-h}Z`, t, { c: [0, -h / 2, w], k: .82 })])
      + El(0, -h, w, w * .36, { fill: top.o }) + El(0, -h, w - 1.1, w * .36 - .9, { fill: top.d }) + El(-.6, -h - .3, w - 2.6, w * .36 - 1.5, { fill: top.l })
      + El(0, -h, w * .55, w * .19, { fill: 'none', stroke: top.o, 'stroke-width': .7, opacity: .55 }) + El(0, -h, w * .25, w * .09, { fill: 'none', stroke: top.o, 'stroke-width': .7, opacity: .55 })
      + Pa(D`M${-w + 2.4},${-h + 3.2} L${-w + 2.4},-2`, { stroke: '#fff', 'stroke-width': 1, opacity: .25, 'stroke-linecap': 'round' });
  }
  function treasureBody(id, R) { return id === 'compass' ? compassBody(R) : id === 'lantern' ? lanternBody(R) : ropeBody(); }
  function treasure(id, o = {}) {
    const S = o.S || ART.S, R = rng(o.seed ?? (id || '').length * 97);
    if (!o.onTile) return sized(o.size, 30, scaleTo(S, treasureBody(id, R)));
    const lift = id === 'lantern' ? -27 : id === 'rope' ? -15 : -20;
    const body = shadow(16, 5, 0, .6) + stump(11, 6.5) + G({ transform: T(0, lift, .82) }, treasureBody(id, R)) + sparkle(9, lift - 9, 4, R() * 3) + sparkle(-10, lift + 2, 2.6, R() * 3 + 1.2);
    return scaleTo(S, G({ class: 'art-treasure art-on-tile' }, body));
  }

  // ---------- leaves, spring ----------
  const DEAD = ['#b8643a', '#cf8a3e', '#8f4a35', '#a77a42', '#b5563f'];
  const deadLeafD = (L, W) => D`M0,0 C${L * .18},${-W * 1.05} ${L * .62},${-W * 1.1} ${L},0 C${L * .62},${W * .9} ${L * .2},${W * .95} 0,0Z`;
  function curledLeaf(x, y, L, ang, col, curl = 0) {
    const t = tone(col, .52, .12, .16), W = L * .4, d = deadLeafD(L, W);
    const roll = curl ? Pa(D`M${L * .18},${-W * .62} C${L * .45},${-W * .95} ${L * .75},${-W * .7} ${L * .92},${-W * .12} C${L * .6},${-W * .2} ${L * .35},${-W * .2} ${L * .18},${-W * .62}Z`, { fill: light(col, .35), stroke: t.o, 'stroke-width': .8, 'stroke-linejoin': 'round' }) : '';
    return G({ transform: 'translate(' + N(x) + ',' + N(y) + ') scale(1,' + (TILT + .15) + ') rotate(' + N(ang) + ') translate(' + N(-L / 2) + ',0)' },
      Pa(D`M0,0 l${-L * .16},${W * .12}`, { stroke: t.o, 'stroke-width': 2.4, 'stroke-linecap': 'round' }),
      Pa(d, { fill: t.o, stroke: t.o, 'stroke-width': 2.4, 'stroke-linejoin': 'round' }), Pa(d, { fill: t.d }), Pa(halfLeafD(L, W * 1.05, -1), { fill: t.l }),
      Pa(D`M${L * .04},0 Q${L * .5},${W * .08} ${L * .9},0 M${L * .3},${W * .04} l${L * .12},${-W * .5} M${L * .55},${W * .05} l${L * .1},${-W * .45} M${L * .38},${W * .05} l${L * .1},${W * .45}`, { fill: 'none', stroke: t.o, 'stroke-width': .7, opacity: .5, 'stroke-linecap': 'round' }), roll);
  }
  function leaves(n, o = {}) {
    const S = o.S || ART.S, R = rng(o.seed ?? 4242 + n);
    let s = '';
    if (n <= 1) {
      const spots = [[4, -12], [-16, -4], [13, -5], [-3, 5], [21, 6], [-23, 8]];
      for (const [x, y] of spots) s += curledLeaf(x + (R() - .5) * 4, y + (R() - .5) * 3, 10 + R() * 3, R() * 360, DEAD[Math.floor(R() * DEAD.length)], R() < .45);
      return scaleTo(S, G({ class: 'art-leaves art-leaves1' }, s));
    }
    // sealed: a heap of leaves under arching purple brambles
    s += shadow(32, 10, 0, 2);
    const heap = tone('#8a5434', .52, .14, .1);
    s += stack([shape('M-30,4 C-30,-8 -16,-17 0,-17 C16,-17 30,-8 30,4 Q0,11 -30,4Z', heap, { c: [0, -4, 24] })]);
    const lp = [[-22, -2], [-12, -8], [0, -12], [12, -9], [22, -2], [-17, 3], [-5, 0], [7, -1], [17, 3], [-8, -12], [8, 4], [-26, 3], [26, 3], [0, 5]];
    for (const [x, y] of lp) s += curledLeaf(x + (R() - .5) * 3, y + (R() - .5) * 2, 10 + R() * 3, R() * 360, DEAD[Math.floor(R() * DEAD.length)], R() < .4);
    const vine = tone('#6d3d95', .6, .12, .22);
    const arcs = [[[-32, 6], [-30, -24], [-6, -30], [4, -14]], [[30, 7], [30, -22], [6, -30], [-6, -16]], [[-20, 10], [-18, -15], [18, -17], [22, 8]], [[-34, -1], [-22, -10], [-8, -5], [-2, 4]], [[34, 0], [24, -8], [12, -4], [8, 5]]];
    const bez = (P, t) => { const u = 1 - t; return [0, 1].map(k => u * u * u * P[0][k] + 3 * u * u * t * P[1][k] + 3 * u * t * t * P[2][k] + t * t * t * P[3][k]); };
    s += stack(arcs.map(P => limb(D`M${P[0][0]},${P[0][1]} C${P[1][0]},${P[1][1]} ${P[2][0]},${P[2][1]} ${P[3][0]},${P[3][1]}`, 2.7, vine)));
    arcs.forEach((P, ai) => {
      for (let i = 1; i < 9; i++) {
        const t = i / 9, a0 = bez(P, t - .02), a1 = bez(P, t + .02), p = bez(P, t), ang = Math.atan2(a1[1] - a0[1], a1[0] - a0[0]) / RAD + ((i + ai) % 2 ? -90 : 90);
        s += Pa('M-1.5,0 L0,-4.2 L1.5,0Z', { fill: '#3e1d58', stroke: '#24102f', 'stroke-width': .6, 'stroke-linejoin': 'round', transform: T(p[0], p[1], 1, ang + 90) });
      }
    });
    s += stack([leaf(-20, -22, 6, 2.8, -140, tone('#5a4a7a'), { ow: 1 }), leaf(14, -24, 6, 2.8, -40, tone('#5a4a7a'), { ow: 1 }), leaf(2, -16, 5, 2.4, -80, tone('#4f5a6a'), { ow: 1 })]);
    s += Pa('M4,-14 c3,-1 4,3 1.4,4', { fill: 'none', stroke: vine.d, 'stroke-width': 1.8, 'stroke-linecap': 'round' }) + Pa('M-6,-16 c-3,-1 -4,3 -1.4,4', { fill: 'none', stroke: vine.d, 'stroke-width': 1.8, 'stroke-linecap': 'round' });
    for (const [x, y] of [[-21, -21], [11, -24], [-2, -9]]) s += Ci(x, y, 1.3, { fill: '#d6a8f0', opacity: .85 });
    return scaleTo(S, G({ class: 'art-leaves art-sealed' }, s));
  }
  function spring(o = {}) {
    const S = o.S || ART.S, R = rng(o.seed ?? 77);
    const stoneT = [tone('#a9a7a2', .5, .14, .14), tone('#8f979c', .5, .14, .14), tone('#b4a796', .5, .14, .14)];
    const rx = 21, ry = 11.5, st = [];
    for (let i = 0; i < 14; i++) { const a = (i / 14) * Math.PI * 2 + .1, x = Math.cos(a) * (rx + 2), y = Math.sin(a) * (ry + 1.8); st.push([y, x, 3.2 + R() * 1.6, stoneT[i % 3]]); }
    const backS = st.filter(s => s[0] < 0).sort((a, b) => a[0] - b[0]), frontS = st.filter(s => s[0] >= 0).sort((a, b) => a[0] - b[0]);
    const stone = ([y, x, r, t]) => [El(x, y, r + 1.6, r * .8 + 1.4, { fill: t.o }), El(x, y, r, r * .8, { fill: t.d }) + El(x - r * .2, y - r * .22, r * .72, r * .52, { fill: t.l }) + El(x - r * .4, y - r * .4, r * .28, r * .13, { fill: '#fff', opacity: .6 })];
    const reed = tone('#4f9a3a', .55, .14, .14), cat = tone('#8a5230', .5, .14, .14);
    const reeds = G({ transform: T(17, -8) }, G(sway(R, 1), stack([limb('M0,0 Q-1,-10 1,-19', 1.4, reed), limb('M3,0 Q4,-8 6,-14', 1.3, reed), limb('M-3,1 Q-6,-5 -8,-9', 1.2, reed, { light: false }),
      shape('M-.8,-19 Q1,-26 2.6,-19 Q1,-17 -.8,-19Z', cat, { ow: 1, light: false }), shape('M5,-14 Q7,-20 8,-13.6 Q6.4,-12 5,-14Z', cat, { ow: 1, light: false })])));
    let s = shadow(28, 9, 0, 3) + stack(backS.map(stone)) + reeds;
    s += El(0, 0, rx + 1.4, ry + 1.4, { fill: '#1a4f7a' }) + El(0, 0, rx, ry, { fill: 'url(#art-water)' }) + El(0, 1.6, rx - 2, ry - 2.6, { fill: '#0c3a64', opacity: .18 })
      + Pa(D`M${-rx + 5},-2 A${rx - 5},${ry - 4} 0 0 1 ${-4},${-ry + 3.6}`, { fill: 'none', stroke: '#fff', 'stroke-width': 1.6, opacity: .75, 'stroke-linecap': 'round' });
    for (let i = 0; i < 2; i++) s += G({ transform: T(-3 + i * 7, 1 + i * .6) }, El(0, 0, 9, 4.4, Object.assign({ fill: 'none', stroke: '#e8fbff', 'stroke-width': .9 }, anim('art-ripple', '50% 50%', i * 1.6 + R()))));
    const pad = tone('#5fb84a', .52, .14, .14);
    s += stack([shape('M-8,4 A4.6,2.6 0 1 1 -8.2,4.1 L-11.6,3.4Z', pad, { ow: 1.2, light: false })]) + El(-12.4, 3.6, 3.4, 1.7, { fill: pad.l }) + blossom(-11.4, 2.4, .55, '#ffd6e8', '#ffcc3a');
    s += stack(frontS.map(stone));
    s += sparkle(7, -5, 4.2, R() * 3) + sparkle(-6, -1, 2.4, R() * 3 + 1.4);
    return scaleTo(S, G({ class: 'art-spring' }, s));
  }

  // ---------- the World Tree ----------
  // a smooth tapered tube along a centreline (roots, boughs): pts [[x,y]…], ws half-widths
  function taperD(pts, ws) {
    const n = pts.length, L = [], Rr = [];
    for (let i = 0; i < n; i++) {
      const a = pts[Math.max(0, i - 1)], b = pts[Math.min(n - 1, i + 1)];
      let tx = b[0] - a[0], ty = b[1] - a[1]; const m = Math.hypot(tx, ty) || 1; tx /= m; ty /= m;
      L.push([pts[i][0] - ty * ws[i], pts[i][1] + tx * ws[i]]); Rr.push([pts[i][0] + ty * ws[i], pts[i][1] - tx * ws[i]]);
    }
    const smooth = P => { let d = ''; for (let i = 1; i < P.length - 1; i++) d += D` Q${P[i][0]},${P[i][1]} ${(P[i][0] + P[i + 1][0]) / 2},${(P[i][1] + P[i + 1][1]) / 2}`; return d + D` L${P[P.length - 1][0]},${P[P.length - 1][1]}`; };
    const we = ws[n - 1], e = Rr[n - 1];
    return D`M${L[0][0]},${L[0][1]}` + smooth(L) + D` A${we},${we} 0 0 0 ${e[0]},${e[1]}` + smooth(Rr.slice().reverse()) + 'Z';
  }
  function tube(pts, ws, t, o = {}) {
    const d = taperD(pts, ws), lit = taperD(pts.map(([x, y], i) => [x - ws[i] * .22, y - ws[i] * .34]), ws.map(w => w * .44));
    return [Pa(d, { fill: t.o, stroke: t.o, 'stroke-width': (o.ow ?? OW) * 2, 'stroke-linejoin': 'round' }), Pa(d, { fill: t.d }) + Pa(lit, { fill: t.l })];
  }
  // a foliage mass: one silhouette, one dark body, one light layer up-left, seams on the front bumps
  function mass(list, t, o = {}) {
    const ow = o.ow ?? OW;
    let s = list.map(([x, y, r]) => Ci(x, y, r + ow, { fill: t.o })).join('') + list.map(([x, y, r]) => Ci(x, y, r, { fill: t.d })).join('')
      + list.map(([x, y, r]) => Ci(x - r * .2, y - r * .25, r * .76, { fill: t.l })).join('');
    for (const i of o.seams || []) { const [x, y, r] = list[i]; s += Pa(D`M${x + r * .98},${y - r * .2} A${r},${r} 0 0 1 ${x - r * .35},${y + r * .94}`, { fill: 'none', stroke: t.o, 'stroke-width': 1.3, opacity: .45, 'stroke-linecap': 'round' }); }
    for (const i of o.hl || []) { const [x, y, r] = list[i]; s += gloss(x - r * .42, y - r * .5, r * .26, r * .12, -35, .65); }
    return s;
  }
  // the three treasure slots sit side by side on one shelf in the hollow (a triangle of marks would read as a face)
  const TSLOT = { compass: [-11, -55.5], lantern: [0, -57], rope: [11, -54.5] };
  function worldTree(o = {}) {
    const S = o.S || ART.S, R = rng(o.seed ?? 31), awake = !!o.awake, placed = o.treasures || [], fruitN = o.fruit || 0;
    const bark = awake ? tone('#8e5b3a', .56, .18, .14) : tone('#6f5444', .56, .18, .1);
    const can = awake ? [tone('#1f7a48', .55, .14, .08), tone('#2f9a50', .52, .14, .14), tone('#46b356', .5, .12, .16)]
      : [tone('#26433f', .5, .12, .06), tone('#2f5a52', .5, .14, .1), tone('#3a675d', .5, .14, .1)];
    let s = '';
    if (awake) s += G({ transform: T(0, -112) }, G(anim('art-glow', '50% 50%', 0, 'animation-duration:3.2s'), Ci(0, 0, 100, { fill: 'url(#art-dawn)' })));
    s += shadow(70, 22, 0, 4);
    // buttress roots creeping over the hex; the ones behind the trunk first
    const roots = [[200, 56, 9], [238, 42, 7.5], [302, 42, 7.5], [340, 56, 9], [155, 54, 10], [114, 46, 9.5], [68, 48, 9.5], [27, 56, 10]];
    const root = ([deg, L, w]) => {
      const a = deg * RAD, c = Math.cos(a), sn = Math.sin(a) * TILT, pts = [], ws = [];
      for (let i = 0; i <= 6; i++) { const t = i / 6, rr = 15 + (L - 15) * t; pts.push([c * rr, sn * rr - (1 - t) * (1 - t) * 15 - Math.sin(t * Math.PI) * 2.2]); ws.push(w * (1 - t * .8)); }
      return tube(pts, ws, bark);
    };
    s += stack(roots.filter(r => Math.sin(r[0] * RAD) < 0).map(root));
    // canopy, back layer
    const back = [[-44, -124, 22], [44, -126, 22], [-25, -150, 26], [25, -152, 26], [0, -157, 24], [-58, -108, 13], [58, -108, 13]];
    s += G(sway(R, 1), mass(back, can[0]));
    // boughs and trunk
    s += stack([tube([[-5, -90], [-18, -103], [-34, -113], [-48, -118]], [8.5, 6.5, 4.6, 3], bark), tube([[5, -90], [19, -104], [35, -115], [49, -121]], [8.5, 6.5, 4.6, 3], bark),
      tube([[-2, -96], [-9, -114], [-15, -132]], [6.5, 4.6, 3], bark), tube([[2, -96], [9, -116], [14, -136]], [6.5, 4.6, 3], bark),
      trunk(0, 2, 0, -100, 56, 30, 0, 10, bark, { grain: false })]);
    s += ['M-17,-6 C-21,-40 -14,-72 -11,-98', 'M17,-6 C21,-34 15,-72 11,-98', 'M-6,-14 C-8,-24 -6,-30 -7,-34', 'M7,-80 C9,-88 7,-94 8,-100', 'M23,-8 Q20,-20 22,-28']
      .map(d => Pa(d, { fill: 'none', stroke: bark.o, 'stroke-width': 1.4, opacity: .5, 'stroke-linecap': 'round' })).join('')
      + El(-13, -24, 3, 4, { fill: bark.d, stroke: bark.o, 'stroke-width': 1.1, opacity: .9 }) + El(-13, -24, 1.2, 1.8, { fill: bark.o, opacity: .6 })
      + Pa('M-22,-12 Q-24,-46 -18,-86', { fill: 'none', stroke: bark.l, 'stroke-width': 3.2, opacity: .55, 'stroke-linecap': 'round' });
    s += stack(roots.filter(r => Math.sin(r[0] * RAD) >= 0).map(root));
    // the heart-shaped hollow and its three treasure slots
    const hy = -53, hs = 17;
    s += Pa(heartD(0, hy, hs + 3.6), { fill: bark.o }) + Pa(heartD(0, hy - .8, hs + 2.2), { fill: light(bark.c, .28) }) + Pa(heartD(0, hy, hs), { fill: awake ? 'url(#art-hollow-lit)' : 'url(#art-hollow)', stroke: bark.o, 'stroke-width': 1.4 });
    if (awake || placed.length) s += G(anim('art-glow', '50% 50%', R() * 2, 'animation-duration:2.6s'), Pa(heartD(0, hy, hs * .8), { fill: 'url(#art-gold)', opacity: awake ? .9 : .22 * placed.length }));
    const shelf = tone(light(bark.c, .12), .55, .12, .16);
    s += stack([shape('M-15.5,-49.5 Q0,-51.5 15.5,-49.5 L14.5,-46.6 Q0,-48.4 -14.5,-46.6Z', shelf, { c: [0, -48.5, 8], ow: 1.1 })]);
    for (const id of ['compass', 'lantern', 'rope']) {
      const [x, y] = TSLOT[id], has = placed.includes(id), k = id === 'lantern' ? .3 : .36;
      if (has) s += G({ transform: T(x, y, k) }, treasureBody(id, R)) + sparkle(x + 4.6, y - 5, 2.2, R() * 3);
      else s += Ci(x, -51.2, 1.1, { fill: '#e8d2a8', opacity: .35 });
    }
    // canopy, middle and front layers
    const mid = [[-48, -104, 18], [48, -105, 18], [-29, -122, 22], [29, -124, 22], [0, -132, 24], [-12, -148, 17], [13, -150, 17]];
    const skirt = [[-56, -96, 11], [56, -96, 11], [-35, -95, 15], [35, -96, 15], [-12, -99, 16], [12, -100, 16]];
    let fc = '';
    if (!awake) {
      // drooping leaf strands hang under the canopy
      const dl = [];
      for (const [x, y, n] of [[-62, -88, 2], [-48, -84, 3], [-34, -83, 2], [-20, -86, 3], [-6, -86, 2], [8, -86, 3], [22, -86, 2], [36, -83, 3], [50, -84, 2], [63, -88, 2]])
        for (let j = 0; j < n; j++) dl.push(leaf(x + (j % 2 ? 1.2 : -1.2), y + j * 7, 8.6 - j, 3.4, 90 + (x < 0 ? 10 : -10) + (j % 2 ? 8 : -8), can[1], { ow: 1.2 }));
      fc += stack(dl);
    } else {
      const up = [];
      for (const [x, y, a] of [[-66, -104, -165], [66, -104, -15], [-60, -126, -140], [60, -128, -40], [-40, -160, -120], [40, -162, -60], [-18, -176, -100], [18, -177, -80], [0, -180, -90]]) up.push(leaf(x, y, 12, 5, a, can[2], { ow: 1.4 }));
      fc += stack(up);
    }
    fc += mass(mid, can[2], { hl: [4, 2], seams: [0, 1, 4] }) + mass(skirt, can[1], { seams: [2, 3, 4, 5], hl: [2] });
    if (!awake) {
      for (const [x, y] of [[-40, -130], [-14, -150], [22, -146], [44, -116], [-50, -106], [6, -128], [-26, -114], [30, -110], [-2, -100], [-46, -92], [42, -94]])
        fc += stack([shape(D`M${x},${y - 6} C${x + 3.2},${y - 3} ${x + 3},${y + 1.6} ${x},${y + 2} C${x - 3},${y + 1.6} ${x - 3.2},${y - 3} ${x},${y - 6}Z`, tone('#a68aa8', .5, .12, .1), { c: [x, y - 2, 3], ow: 1.1 }), leaf(x, y + 2, 4, 2, -150, can[2], { ow: .9, rib: false }), leaf(x, y + 2, 4, 2, -30, can[2], { ow: .9, rib: false })]);
    } else {
      const fl = [[-44, -128, 0], [-20, -146, 1], [12, -152, 2], [38, -134, 3], [-56, -104, 4], [52, -106, 5], [-6, -122, 0], [22, -116, 1], [-30, -100, 2], [30, -98, 3], [-12, -162, 4], [46, -118, 5], [-40, -112, 1], [6, -138, 3], [0, -100, 5], [-24, -126, 4]];
      for (const [x, y, v] of fl) fc += blossom(x, y, 1.25, light(VAL[v], .5), dark(VAL[v], .05));
      for (let i = 0; i < 5; i++) fc += sparkle(-60 + i * 30, -152 + (i % 2) * 44, 3.4, R() * 3);
    }
    // fruit placed on the tree hangs from the canopy
    const FA = [[-44, -92], [40, -93], [-20, -90], [18, -91], [-58, -102], [57, -102], [-31, -116], [30, -118], [-6, -124], [12, -134], [-44, -126], [44, -126]];
    let fr = '';
    for (let i = 0; i < Math.min(fruitN, FA.length); i++) fr += hangFruit(FA[i][0], FA[i][1] + 6, 1.05);
    s += G(sway(R, 1), fc, fr);
    if (awake) for (let i = 0; i < 4; i++) s += drifter(-50 + i * 32, -104 + (i % 2) * 18, R, El(0, 0, 2.2, 1.4, { fill: light(VAL[i + 1], .5), stroke: dark(VAL[i + 1], .3), 'stroke-width': .7 }));
    if (fruitN > 0) {
      const t = tone('#f3dfb8', .5, .08, .06);
      s += G({ transform: T(36, -6) }, stack([shape('M-11,-8 L11,-8 Q13,-8 13,-6 L13,6 Q13,8 11,8 L-11,8 Q-13,8 -13,6 L-13,-6 Q-13,-8 -11,-8Z', t, { c: [0, 0, 10], ow: 1.6 })]),
        stack([shape(heartD(-5.4, 0, 4.6), tone('#e8344f'), { c: [-5.4, 0, 4.6], ow: 1.2 })]),
        el('text', { x: 4.6, y: 4.4, 'text-anchor': 'middle', 'font-family': 'Lilita One, Nunito, sans-serif', 'font-size': 12, fill: '#7a3a1a' }, String(fruitN)));
    }
    return scaleTo(S, G({ class: 'art-worldtree ' + (awake ? 'art-awake' : 'art-asleep') }, s));
  }

  // ---------- medallion ----------
  function medallion(o = {}) {
    const r = 17, k = (o.r || 17) / 17, col = o.color || '#43a047', st = o.state || '', dry = st === 'dry', turn = st === 'turn', water = Math.max(0, Math.min(5, o.water ?? 0));
    const ringCol = dry ? mix(col, '#8d8f8a', .55) : col, t = tone(ringCol, .5, .16, .16), id = 'art-ph' + (++uid);
    const RW = 5, cy = o.stump === false ? 0 : -(r + 11);
    let m = '';
    if (turn) m += G({ transform: T(0, cy) }, G(anim('art-glow', '50% 50%', 0), Ci(0, 0, r + RW + 13, { fill: 'url(#art-gold)' })));
    let disc = '';
    // ring: outline, dark body, light layer up-left, gloss
    disc += Ci(0, 0, r + RW + 1.6, { fill: t.o }) + Ci(0, 0, r + RW, { fill: t.d }) + Ci(-.9, -1.1, r + RW - 1.4, { fill: t.l });
    if (turn) disc += Ci(0, 0, r + RW + 3.4, { fill: 'none', stroke: '#ffd84a', 'stroke-width': 2.6 }) + Ci(0, 0, r + RW + 5, { fill: 'none', stroke: '#b9781a', 'stroke-width': 1 });
    disc += el('clipPath', { id }, Ci(0, 0, r));
    if (o.photo) {
      disc += Ci(0, 0, r, { fill: dark(col, .3) }) + el('image', { href: esc(o.photo), x: -r, y: -r, width: r * 2, height: r * 2, preserveAspectRatio: 'xMidYMid slice', 'clip-path': 'url(#' + id + ')', filter: dry ? 'url(#art-grey)' : null });
      if (dry) disc += Ci(0, 0, r, { fill: '#2a2f3a', opacity: .28 });
    } else {
      const bg = tone(dry ? mix(col, '#8d8f8a', .6) : col, .5, .12, .3);
      disc += Ci(0, 0, r, { fill: bg.d }) + Ci(-r * .12, -r * .14, r * .86, { fill: bg.c }) + el('text', { x: 0, y: r * .38, 'text-anchor': 'middle', 'font-family': 'Lilita One, Nunito, sans-serif', 'font-size': r * 1.15, fill: '#fff', stroke: bg.o, 'stroke-width': r * .14, 'paint-order': 'stroke', 'stroke-linejoin': 'round' }, esc((o.name || '?').trim().charAt(0).toUpperCase() || '?'));
    }
    disc += Ci(0, 0, r, { fill: 'none', stroke: t.o, 'stroke-width': 1.4 }) + Pa(D`M${Math.cos(200 * RAD) * (r + 2.6)},${Math.sin(200 * RAD) * (r + 2.6)} A${r + 2.6},${r + 2.6} 0 0 1 ${Math.cos(255 * RAD) * (r + 2.6)},${Math.sin(255 * RAD) * (r + 2.6)}`, { fill: 'none', stroke: '#fff', 'stroke-width': 1.8, opacity: .85, 'stroke-linecap': 'round' })
      + Pa(D`M${-r * .55},${-r * .62} A${r * .85},${r * .85} 0 0 1 ${r * .1},${-r * .84}`, { fill: 'none', stroke: '#fff', 'stroke-width': 1.4, opacity: .35, 'stroke-linecap': 'round' });
    // water crown: five drops around the top of the ring
    for (let i = 0; i < 5; i++) {
      const a = (-150 + i * 30) * RAD, R2 = r + RW + 5.4, x = Math.cos(a) * R2, y = Math.sin(a) * R2, full = i < water && !dry;
      const dt = full ? tone('#3aa6f0', .52, .12, .2) : tone('#3b4d5a', .4, .1, .05);
      disc += G({ transform: T(x, y, 1, a / RAD + 90) }, Pa(dropD(0, 0, 3.5), { fill: dt.o, stroke: dt.o, 'stroke-width': 2.4, 'stroke-linejoin': 'round' }), Pa(dropD(0, 0, 3.5), { fill: dt.d }), full ? Pa(dropD(-.6, -.5, 2.4), { fill: dt.l }) + gloss(-1.2, -.4, .7, 1.4, 15, .9) : '');
    }
    const badge = (deg, inner) => { const a = deg * RAD, x = Math.cos(a) * (r + 3), y = Math.sin(a) * (r + 3); return G({ transform: T(x, y) }, Ci(0, 0, 7.6, { fill: '#7a4a24' }) + Ci(0, 0, 6.4, { fill: '#f6e6c4' }) + Ci(-.8, -.9, 5, { fill: '#fff6e0' }) + inner); };
    if (o.fruit > 0) disc += badge(140, G({ transform: T(0, .6, .5) }, fruitBody(.9)) + (o.fruit > 1 ? G({ transform: T(-6, -5.6) }, Ci(0, 0, 4.2, { fill: '#b8233d', stroke: '#fff', 'stroke-width': 1 }) + el('text', { y: 2.6, 'text-anchor': 'middle', 'font-family': 'Lilita One, Nunito, sans-serif', 'font-size': 7, fill: '#fff' }, String(o.fruit))) : ''));
    if (o.treasure) disc += badge(40, G({ transform: T(0, 0, o.treasure === 'lantern' ? .3 : .36) }, treasureBody(o.treasure)));
    disc = G({ transform: T(0, cy) }, disc);
    if (dry) disc = G({ transform: 'rotate(-11 0 ' + N(cy + r) + ') translate(0,2.5)' }, disc);
    else if (turn) disc = G(anim('art-bob', '50% 100%', 0), disc);
    m += disc;
    if (o.stump !== false) {
      let base = shadow(15, 4.4, 0, .6) + stump(8.6, 9.5, tone('#8a5a36', .55, .16, .14));
      if (dry) base += stack([leaf(7, -5, 7, 2.6, 70, tone('#a3a45a'), { ow: 1 })]);
      m = base + m;
      if (o.label && o.name) {
        const nm = esc(o.name), w = Math.max(22, nm.length * 5.6 + 10);
        m += G({ transform: T(0, 9) }, el('rect', { x: -w / 2, y: -6.5, width: w, height: 13, rx: 6.5, fill: '#2a1a10', stroke: col, 'stroke-width': 1.6 }) + el('text', { y: 3.6, 'text-anchor': 'middle', 'font-family': 'Lilita One, Nunito, sans-serif', 'font-size': 9.5, fill: '#fff6e0' }, nm));
      }
    }
    return G({ class: 'art-medallion' + (st ? ' art-' + st : '') }, k !== 1 ? G({ transform: 'scale(' + N(k) + ')' }, m) : m);
  }

  // ---------- badges & icons ----------
  function disc(r, fill, o = {}) {
    const t = tone(fill, o.ol ?? .5, .14, .2);
    return Ci(0, 0, r + 2.6, { fill: t.o }) + Ci(0, 0, r, { fill: t.d }) + Ci(-r * .08, -r * .1, r * .9, { fill: t.l }) + Pa(D`M${-r * .62},${-r * .46} A${r * .78},${r * .78} 0 0 1 ${-r * .1},${-r * .78}`, { fill: 'none', stroke: '#fff', 'stroke-width': 2.2, opacity: .6, 'stroke-linecap': 'round' });
  }
  function cloudD(x, y, s) { return D`M${x - 13 * s},${y + 6 * s} C${x - 19 * s},${y + 6 * s} ${x - 19 * s},${y - 3 * s} ${x - 12 * s},${y - 3.4 * s} C${x - 11 * s},${y - 11 * s} ${x - 1 * s},${y - 12 * s} ${x + 1.6 * s},${y - 6.4 * s} C${x + 5 * s},${y - 10.6 * s} ${x + 13 * s},${y - 8 * s} ${x + 12.4 * s},${y - 1.6 * s} C${x + 19 * s},${y - 1 * s} ${x + 18 * s},${y + 6 * s} ${x + 12 * s},${y + 6 * s}Z`; }
  function sunBody(s = 1) {
    const ray = tone('#ffb52e', .5, .12, .14), core = tone('#ffd23a', .48, .1, .2);
    return stack([shape(starD(0, 0, 17 * s, 9.6 * s, 10, 0), ray, { c: [0, 0, 14 * s], k: .82, ow: 2 })]) + stack([blob(0, 0, 9.6 * s, core, { hl: true })]);
  }
  function weatherBadge(kind, o = {}) {
    let inner;
    if (kind === 'sun') inner = disc(21, '#8fd3f4') + sunBody(1);
    else if (kind === 'rain') {
      const cl = tone('#e9f1f8', .5, .1, .06), dr = tone('#3aa6f0', .5, .12, .2);
      inner = disc(21, '#5b7fa6') + stack([shape(dropD(-7, 10, 2.6), dr, { c: [-7, 10, 2.6], ow: 1.4 }), shape(dropD(1, 13, 2.6), dr, { c: [1, 13, 2.6], ow: 1.4 }), shape(dropD(9, 10, 2.6), dr, { c: [9, 10, 2.6], ow: 1.4 })])
        + stack([shape(cloudD(0, -1, .95), cl, { c: [0, -3, 14] })]) + gloss(-8, -7, 3.4, 1.6, -25, .8);
    } else {
      const f1 = tone('#dfe6ea', .45, .08, .04), f2 = tone('#c3cdd4', .45, .08, .04);
      inner = disc(21, '#8e9cab') + stack([shape(cloudD(3, -4, .7), f2, { c: [3, -5, 10] })]) + stack([shape(cloudD(-3, 2, .85), f1, { c: [-3, 0, 12] })])
        + ['M-16,9 L6,9', 'M-10,14 L14,14', 'M-4,4.4 L16,4.4'].map((d, i) => Pa(d, { stroke: i === 2 ? '#f4f8fa' : '#eef3f6', 'stroke-width': 3.2, 'stroke-linecap': 'round', opacity: .95 })).join('');
    }
    return sized(o.size, 48, G({ class: 'art-weather art-weather-' + kind }, inner));
  }
  const LETTERS = 'ZERAOS';
  function valueBadge(i, o = {}) {
    const c = VAL[i] || '#888', t = tone(c, .5, .14, .2), p = k => ptsStr([0, 1, 2, 3, 4, 5].map(j => { const a = (60 * j - 30) * RAD; return [k * Math.cos(a), k * Math.sin(a)]; }));
    const inner = el('polygon', { points: p(22.5), fill: t.o, stroke: t.o, 'stroke-width': 4, 'stroke-linejoin': 'round' }) + el('polygon', { points: p(20.5), fill: t.d, stroke: t.d, 'stroke-width': 2, 'stroke-linejoin': 'round' })
      + el('polygon', { points: p(17.6), fill: t.l, stroke: t.l, 'stroke-width': 3, 'stroke-linejoin': 'round', transform: 'translate(-1.2,-1.6)' })
      + Pa('M-15,-6 L-15,-9.4 L-6,-14.6', { fill: 'none', stroke: '#fff', 'stroke-width': 2.4, opacity: .6, 'stroke-linecap': 'round', 'stroke-linejoin': 'round' })
      + el('text', { y: 8.6, 'text-anchor': 'middle', 'font-family': 'Lilita One, Nunito, sans-serif', 'font-size': 25, fill: '#fff', stroke: t.o, 'stroke-width': 4, 'paint-order': 'stroke', 'stroke-linejoin': 'round' }, LETTERS[i] || '?');
    return sized(o.size, 48, G({ class: 'art-value' }, inner));
  }
  // role symbols, drawn in a ~30-unit box
  const WOOD = tone('#9a6338', .52, .14, .16), STEEL = tone('#b9c6cf', .5, .12, .12), LEAFT = tone('#5cb84a', .52, .14, .16), GOLD = tone('#f2b32e', .52, .12, .16);
  const ROLE = {
    1: () => { const tines = [-8, -4, 0, 4, 8].map(k => limb(D`M${k},-5 L${k * 1.15},4`, 2, STEEL, { light: false, ow: 1.1 })); return G({ transform: 'translate(3,-3) rotate(38)' }, stack([limb('M0,5 L0,26', 3.4, WOOD), ...tines, limb('M-10,-5 L10,-5', 3.2, STEEL)]), Pa('M-9,-5.6 L9,-5.6', { stroke: '#fff', 'stroke-width': .9, opacity: .7, 'stroke-linecap': 'round' })); },
    2: () => { const can = tone('#4fa0d8', .52, .14, .16); return stack([limb('M8,-2 L17,-10', 3.4, can), shape('M-12,-6 L8,-6 L7,10 Q7,12 5,12 L-9,12 Q-11,12 -11,10Z', can, { c: [-2, 3, 9], hl: gloss(-7, -1, 1.4, 3, 0, .6) }), limb('M-12,-2 Q-19,0 -16,7 Q-14,10 -11,8', 2.4, can, { light: false })]) + Ci(17.6, -10.6, 2.6, { fill: can.o }) + [[19, -14], [21.6, -9], [22, -14.4]].map(([x, y]) => Ci(x + 2, y + 3, 1.2, { fill: '#8fd3ff' })).join(''); },
    3: () => stack([shape('M-11,8 C-9,3 9,3 11,8 Q0,11 -11,8Z', tone('#93603a'), { c: [0, 6, 8], ow: 1.6 }), limb('M0,6 Q-2,-2 1,-8', 2.4, LEAFT), leaf(1, -7, 9, 5.6, -150, LEAFT, { ow: 1.4 }), leaf(1, -7, 9, 5.6, -30, LEAFT, { ow: 1.4 })])
      + stack([shape('M10,-4 L15,-12 L20,-4 L17,-4 L17,4 L13,4 L13,-4Z', GOLD, { c: [15, -3, 5], ow: 1.5, light: false })]),
    4: () => stack([shape('M-16,0 Q0,-15 16,0 Q0,15 -16,0Z', tone('#f6efe0', .45, .06, .03), { c: [0, 0, 12], light: false })]) + stack([blob(0, 0, 7.4, tone('#3f9e9a'))]) + Ci(0, 0, 3.4, { fill: '#173c3a' }) + gloss(-2.4, -2.6, 1.8, 1.1, -30, .95),
    5: () => G({ transform: 'translate(-2,3) rotate(-32)' }, stack([shape('M-17,-3 L-5,-3.6 L-5,3.6 L-17,3Z', tone('#8a5230'), { c: [-11, 0, 4], ow: 1.6 }), shape('M-5,-4.8 L5,-5.4 L5,5.4 L-5,4.8Z', GOLD, { c: [0, 0, 5], ow: 1.6 }), shape('M5,-6.6 L15,-7.4 L15,7.4 L5,6.6Z', tone('#d9952a'), { c: [10, 0, 6], ow: 1.6, hl: gloss(9, -4, 3, .9, 0, .6) })]))
      + G({ transform: 'translate(-2,3) rotate(-32)' }, el('rect', { x: 13, y: -7.8, width: 3, height: 15.6, rx: 1.2, fill: '#7a4a14' }) + El(16.4, 0, 2.2, 6.6, { fill: '#9fe0ff', stroke: '#2a5a7a', 'stroke-width': 1.1 }) + El(15.8, -2.4, .7, 2, { fill: '#fff', opacity: .9 }) + el('rect', { x: -18.6, y: -2.6, width: 2.4, height: 5.2, rx: 1, fill: '#3a2414' })),
    6: () => stack([shape('M0,-15 L13,-10 L12,3 Q10,11 0,16 Q-10,11 -12,3 L-13,-10Z', tone('#4f74b0'), { c: [0, 0, 12], hl: gloss(-6, -7, 2.4, 1.2, -30, .7) })]) + Pa('M0,-10 L0,11 M-8,-3 L8,-3', { stroke: '#f6e6c4', 'stroke-width': 3, 'stroke-linecap': 'round' }),
    7: () => stack([shape('M-12,-2 L0,-14 L12,-2 L7,2 L0,-5 L-7,2Z', tone('#ff8a3c'), { c: [0, -6, 8], ow: 1.8 }), shape('M-12,10 L0,-2 L12,10 L7,14 L0,7 L-7,14Z', tone('#ffb03c'), { c: [0, 6, 8], ow: 1.8 })]),
    8: () => stack([shape('M-8,-14 L4,-14 L4,2 L14,4 Q17,5 16,10 L15,12 L-10,12 Q-11,12 -11,10 L-9,-1Z', tone('#8a5230'), { c: [0, 0, 11], hl: gloss(-4, -8, 1.4, 3, 5, .5) })]) + Pa('M-11,9 L16,9', { stroke: '#4a2614', 'stroke-width': 2.6 }) + Pa('M-9,-10 L4,-10', { stroke: '#c58a4e', 'stroke-width': 2, 'stroke-linecap': 'round' }),
    9: () => { const a = tone('#e0a32e'), b = tone('#3f9e9a'); return Ci(-5.5, 0, 9, { fill: 'none', stroke: a.o, 'stroke-width': 6.4 }) + Ci(-5.5, 0, 9, { fill: 'none', stroke: a.l, 'stroke-width': 3.6 }) + Ci(5.5, 0, 9, { fill: 'none', stroke: b.o, 'stroke-width': 6.4 }) + Ci(5.5, 0, 9, { fill: 'none', stroke: b.l, 'stroke-width': 3.6 }) + Pa('M-5.5,-9 A9,9 0 0 1 3.5,-1', { fill: 'none', stroke: a.o, 'stroke-width': 6.4 }) + Pa('M-5.5,-9 A9,9 0 0 1 3.5,-1', { fill: 'none', stroke: a.l, 'stroke-width': 3.6 }); },
  };
  function roleBadge(type, o = {}) {
    const f = ROLE[type];
    const inner = disc(21, '#f2dcae', { ol: .55 }) + (f ? G({ transform: 'scale(.95)' }, f()) : '')
      + G({ transform: T(15, -15) }, Ci(0, 0, 7.4, { fill: '#5a3418' }) + Ci(0, 0, 6, { fill: '#7a4a24' }) + el('text', { y: 3.6, 'text-anchor': 'middle', 'font-family': 'Lilita One, Nunito, sans-serif', 'font-size': 10, fill: '#fff6e0' }, String(type)));
    return sized(o.size, 48, G({ class: 'art-role' }, inner));
  }
  const ICON = {
    move: () => stack([limb('M-15,10 C-14,-12 8,-16 12,-2', 5, tone('#7bd35a'))]) + stack([shape('M5,-4 L19,-6 L13,7Z', tone('#7bd35a'), { c: [12, -1, 6], ow: 2, light: false })]) + [[-16, 17], [-6, 18], [4, 17]].map(([x, y]) => El(x, y, 2.6, 1.6, { fill: '#cfe9b8', stroke: '#4a7a2a', 'stroke-width': 1 })).join(''),
    explore: () => stack([limb('M7,7 L16,16', 5.4, tone('#8a5230')), blob(-3, -3, 12, tone('#9fe0ff', .5, .1, .2))]) + Ci(-3, -3, 12, { fill: 'none', stroke: '#e2a93b', 'stroke-width': 3.6 }) + Ci(-3, -3, 13.6, { fill: 'none', stroke: '#8a5a1a', 'stroke-width': 1 }) + Ci(-3, -3, 10.2, { fill: 'none', stroke: '#8a5a1a', 'stroke-width': 1 }) + gloss(-8, -8, 3.4, 1.8, -40, .9) + sparkle(-3, -3, 4.4, 0, '#fffbe0'),
    sow: () => stack([shape('M-17,10 C-13,0 13,0 17,10 Q0,15 -17,10Z', tone('#93603a'), { c: [0, 7, 14] })]) + stack([shape(D`M-2,-16 C-6,-14 -6,-8 -2,-6 C2,-8 2,-14 -2,-16Z`, tone('#d99a48'), { c: [-2, -11, 4], ow: 1.6 }), shape(D`M6,-6 C2,-4 2,2 6,4 C10,2 10,-4 6,-6Z`, tone('#d99a48'), { c: [6, -1, 4], ow: 1.6 })]) + Pa('M-9,-14 L-9,-8 M12,-4 L12,1', { stroke: '#fff', 'stroke-width': 1.6, opacity: .7, 'stroke-linecap': 'round' }),
    water: () => stack([shape('M-12,14 C-9,10 9,10 12,14 Q0,17 -12,14Z', tone('#93603a'), { c: [0, 13, 8], ow: 1.6 }), limb('M0,13 Q-1,9 0,6', 2, LEAFT), leaf(0, 7, 6, 3.6, -150, LEAFT, { ow: 1.1 }), leaf(0, 7, 6, 3.6, -30, LEAFT, { ow: 1.1 })]) + G({ transform: T(0, -6) }, dropBody(1.05)),
    tend: () => stack([trunk(0, 15, 0, 0, 6, 4, 0, 2, BARK, { grain: false }), blob(-7, -2, 7, tone('#3fae5a')), blob(7, -2, 7, tone('#3fae5a')), blob(0, -9, 9, tone('#4cbc5a'), { hl: true })]) + sparkle(12, -14, 5, 0) + sparkle(-14, -10, 3, 1),
    clear: () => ['M-20,-4 Q-6,-14 10,-12', 'M-22,4 Q-4,-4 16,-1', 'M-18,12 Q-2,6 12,9'].map(d => Pa(d, { fill: 'none', stroke: '#2a5a7a', 'stroke-width': 4.6, 'stroke-linecap': 'round' }) + Pa(d, { fill: 'none', stroke: '#e8f6ff', 'stroke-width': 2.4, 'stroke-linecap': 'round' })).join('')
      + G({ transform: T(4, 6, 1.5, -24) }, curledLeaf(0, 0, 14, 0, '#c8692f', 1).replace(/scale\(1,[^)]*\)/, 'scale(1,1)'))
      + G({ transform: T(12, -12, 1.3, 28) }, curledLeaf(0, 0, 11, 0, '#dc9a3e', 0).replace(/scale\(1,[^)]*\)/, 'scale(1,1)')),
    harvest: () => { const bk = tone('#c58a4e', .5, .14, .14); return G({ transform: T(0, -6) }, fruitBody(.95)) + stack([shape('M-16,2 L16,2 L12,16 Q11.4,17.4 10,17.4 L-10,17.4 Q-11.4,17.4 -12,16Z', bk, { c: [0, 9, 12], k: .85 })]) + Pa('M-14,7 L14,7 M-13,12 L13,12', { stroke: bk.o, 'stroke-width': 1, opacity: .5 }) + Pa('M-6,3 L-5,17 M2,3 L2,17 M9,3 L7,17', { stroke: bk.o, 'stroke-width': 1, opacity: .45 }); },
    take: () => G({ transform: T(0, 7, .62) }, compassBody()) + stack([shape('M-4,-3 L4,-3 L4,-11 L9.5,-11 L0,-20 L-9.5,-11 L-4,-11Z', tone('#7bd35a'), { c: [0, -11, 6], ow: 1.8, light: false })]) + Pa('M-2.4,-5 L-2.4,-11.6', { stroke: '#fff', 'stroke-width': 1.4, opacity: .6, 'stroke-linecap': 'round' }) + sparkle(14, 2, 3.6, 0) + sparkle(-13, 12, 2.6, 1),
    drink: () => { const cup = tone('#b5773a', .52, .14, .16); return stack([shape('M-14,-4 L14,-4 L11,13 Q10.4,15 8,15 L-8,15 Q-10.4,15 -11,13Z', cup, { c: [0, 5, 12], hl: gloss(-8, 3, 1.4, 3.4, 10, .5) }), limb('M14,0 Q21,2 14,10', 2.6, cup, { light: false })]) + El(0, -4, 14, 3.4, { fill: cup.o }) + El(0, -4, 12.6, 2.6, { fill: '#3aa6f0' }) + El(-3, -4.6, 6, 1, { fill: '#bfe8ff' }) + G({ transform: T(2, -14, .62) }, dropBody(1)); },
    pass: () => { const a = tone('#7bd35a'), b = tone('#ffb03c'); return stack([limb('M-14,-4 C-12,-14 6,-16 12,-8', 4, a), shape('M8,-14 L18,-6 L7,-3Z', a, { c: [11, -8, 4], ow: 1.6, light: false }), limb('M14,4 C12,14 -6,16 -12,8', 4, b), shape('M-8,14 L-18,6 L-7,3Z', b, { c: [-11, 8, 4], ow: 1.6, light: false })]); },
    end: () => stack([blob(0, 0, 16, tone('#5cb84a'), { hl: true })]) + Pa('M-8,0 L-2,7 L9,-7', { fill: 'none', stroke: '#245a1a', 'stroke-width': 7.2, 'stroke-linecap': 'round', 'stroke-linejoin': 'round' }) + Pa('M-8,0 L-2,7 L9,-7', { fill: 'none', stroke: '#fff', 'stroke-width': 4, 'stroke-linecap': 'round', 'stroke-linejoin': 'round' }),
    trust: () => G({ transform: 'scale(1.25)' }, acornBody(1)),
    owl: () => { const b = tone('#8a6a9a', .52, .14, .14), w = tone('#f2e6c8', .45, .08, .05); return stack([shape('M-13,16 C-17,4 -15,-8 -12,-15 L-6,-9 Q0,-11 6,-9 L12,-15 C15,-8 17,4 13,16 Q0,20 -13,16Z', b, { c: [0, 2, 13], hl: gloss(-8, -2, 1.6, 3.4, 10, .5) })]) + stack([shape('M-7,8 Q0,2 7,8 Q6,15 0,16 Q-6,15 -7,8Z', w, { c: [0, 10, 6], light: false, ow: 1.2 })]) + Ci(-5.6, -2, 5.4, { fill: '#f9f1da', stroke: b.o, 'stroke-width': 1.6 }) + Ci(5.6, -2, 5.4, { fill: '#f9f1da', stroke: b.o, 'stroke-width': 1.6 }) + Ci(-5.6, -2, 2.4, { fill: '#2a1a30' }) + Ci(5.6, -2, 2.4, { fill: '#2a1a30' }) + Pa('M-1.6,3 L0,6 L1.6,3Z', { fill: '#f2a43a', stroke: '#8a5418', 'stroke-width': .8 }); },
    timer: () => { const c = tone('#e0a32e', .52, .14, .16); return stack([shape('M-3,-19 L3,-19 L3,-14 L-3,-14Z', c, { c: [0, -16, 3], ow: 1.4, light: false }), blob(0, 2, 15, c)]) + Ci(0, 2, 11.4, { fill: '#fff8e6', stroke: c.o, 'stroke-width': 1.4 }) + Pa('M0,2 L0,-6 M0,2 L5.6,5.4', { stroke: '#5a3418', 'stroke-width': 2.4, 'stroke-linecap': 'round' }) + Ci(0, 2, 1.6, { fill: '#c8463f' }) + Pa('M0,-7.6 L0,-9.4 M8.6,2 L10.4,2 M0,11.6 L0,13.4 M-8.6,2 L-10.4,2', { stroke: c.o, 'stroke-width': 1.4, 'stroke-linecap': 'round' }) + gloss(-7, -6, 2.4, 1.2, -40, .8); },
  };
  function icon(name, o = {}) {
    const f = ICON[name];
    const inner = (o.disc ? disc(21, '#f2dcae', { ol: .55 }) : '') + (f ? G({ transform: o.disc ? 'scale(.82)' : null }, f()) : '');
    return sized(o.size, 48, G({ class: 'art-icon art-icon-' + name }, inner));
  }

  // ---------- weather overlay (per hex, origin = top-face centre; put it in a layer above the tiles) ----------
  function weatherFx(kind, o = {}) {
    const S = o.S || ART.S, R = rng(o.seed ?? 11), face = ptsStr(facePts(S0 * .965));
    let s = '';
    if (kind === 'sun') {
      s += el('polygon', { points: face, fill: '#ffd76b', opacity: .16 });
      s += G(anim('art-shaft', '50% 100%', R() * 6), Pa('M-12,-56 L-3,-56 L14,4 L-2,6Z', { fill: 'url(#art-shaft)', opacity: .8 }));
      s += sparkle(-8 + R() * 16, -6 - R() * 6, 2.2, R() * 3, '#fff3b0');
    } else if (kind === 'rain') {
      s += el('polygon', { points: face, fill: '#4f8fe0', opacity: .15 });
      for (let i = 0; i < 4; i++) s += G({ transform: T(-24 + i * 14 + R() * 6, -44 + R() * 18) }, Pa('M0,0 l-2.2,7', Object.assign({ stroke: '#cfe8ff', 'stroke-width': 1.4, 'stroke-linecap': 'round' }, anim('art-rain', '50% 50%', R() * 1.2))));
      s += G({ transform: T(-8 + R() * 16, 2 + R() * 4) }, El(0, 0, 4, 1.6, Object.assign({ fill: 'none', stroke: '#e8f6ff', 'stroke-width': .8 }, anim('art-ripple', '50% 50%', R() * 3, 'animation-duration:1.6s'))));
    } else if (kind === 'fog') {
      s += el('polygon', { points: face, fill: '#d9e2dc', opacity: .2 });
      for (let i = 0; i < 2; i++) s += G({ transform: T(i ? 10 : -12, i ? 4 : -6) }, G(anim('art-fog', '50% 50%', R() * 14, 'animation-duration:' + N(12 + R() * 6) + 's'), Pa(cloudD(0, 0, .9 + R() * .3), { fill: '#eef3f1', opacity: .42 })));
    }
    return scaleTo(S, G({ class: 'art-wx art-wx-' + kind, 'pointer-events': 'none' }, s));
  }

  // ---------- sector-wide weather, dappled light, mist (the Keeper's progress overlay) ----------
  // centres: [[x, y] …] top-face centres of the sector's hexes (already projected, at size S). One tint face per hex
  // plus a capped number of particles spread over the whole sector, so six sectors stay light on a school laptop.
  function sectorWeather(kind, centres, o = {}) {
    const S = o.S || ART.S, k = S / S0, R = rng(o.seed ?? 13), face = ptsStr(facePts(S * .965)), n = centres.length;
    const pick = (m) => { const out = [], used = new Set(); for (let i = 0; i < Math.min(m, n); i++) { let j = Math.floor(R() * n); while (used.has(j) && used.size < n) j = (j + 1) % n; used.add(j); out.push(centres[j]); } return out; };
    const tint = { sun: ['#ffd76b', .14], rain: ['#3f7fd6', .14], fog: ['#dfe7e2', .17] }[kind] || ['#fff', 0];
    let s = G({ opacity: tint[1] }, centres.map(([x, y]) => el('polygon', { points: face, fill: tint[0], transform: T(x, y) })).join(''));
    if (kind === 'sun') {
      for (const [x, y] of pick(4)) s += G({ transform: T(x + (R() - .5) * 10 * k, y, k * 1.25) }, G(anim('art-shaft', '50% 100%', R() * 6), Pa('M-14,-62 L-3,-62 L15,4 L-3,7Z', { fill: 'url(#art-shaft)', opacity: .75 })));
      for (const [x, y] of pick(3)) s += sparkle(x + (R() - .5) * 30 * k, y - (4 + R() * 10) * k, 2.4 * k, R() * 3, '#fff3b0');
    } else if (kind === 'rain') {
      for (const [x, y] of centres) for (let i = 0; i < 2; i++)
        s += G({ transform: T(x + (R() - .5) * 52 * k, y - (34 + R() * 26) * k) }, Pa(D`M0,0 l${-2.6 * k},${8.4 * k}`, Object.assign({ stroke: '#d8edff', 'stroke-width': 1.5 * k, 'stroke-linecap': 'round' }, anim('art-rain', '50% 50%', R() * 1.2, 'animation-duration:' + N(.75 + R() * .35) + 's'))));
      for (const [x, y] of pick(4)) s += G({ transform: T(x + (R() - .5) * 24 * k, y + (R() - .3) * 8 * k) }, El(0, 0, 4.4 * k, 1.8 * k, Object.assign({ fill: 'none', stroke: '#e8f6ff', 'stroke-width': .9 * k }, anim('art-ripple', '50% 50%', R() * 3, 'animation-duration:1.7s'))));
    } else if (kind === 'fog') {
      centres.forEach(([x, y], i) => { if (i % 3 === 1) return; s += G({ transform: T(x + (R() - .5) * 14 * k, y - (2 + R() * 6) * k, k * (1 + R() * .3)) }, G(anim('art-fog', '50% 50%', R() * 14, 'animation-duration:' + N(12 + R() * 6) + 's'), Pa(cloudD(0, 0, 1.05), { fill: '#f1f5f3', opacity: .3 }))); });
    }
    return G({ class: 'art-wx art-wx-' + kind, 'pointer-events': 'none' }, s);
  }
  // warm light spots drifting over an awake sector's ground
  function dapple(centres, o = {}) {
    const S = o.S || ART.S, k = S / S0, R = rng(o.seed ?? 17);
    let s = '';
    centres.forEach(([x, y], i) => { if (i % 2 && R() < .5) return; s += G({ transform: T(x + (R() - .5) * 20 * k, y + (R() - .5) * 8 * k) }, El(0, 0, (14 + R() * 8) * k, (7 + R() * 3) * k, Object.assign({ fill: 'url(#art-dapple)' }, anim('art-dapple', '50% 50%', R() * 8, 'animation-duration:' + N(6 + R() * 4) + 's')))); });
    return G({ class: 'art-dapples', 'pointer-events': 'none' }, s);
  }
  // slow drifting mist banks (soft gradients, no blur filter)
  function mist(centres, o = {}) {
    const S = o.S || ART.S, k = S / S0, R = rng(o.seed ?? 19), n = o.n ?? 3;
    let s = '';
    for (let i = 0; i < n; i++) { const [x, y] = centres[Math.floor(R() * centres.length)]; s += G({ transform: T(x, y - 6 * k) }, El(0, 0, (46 + R() * 22) * k, (14 + R() * 6) * k, Object.assign({ fill: 'url(#art-mist)' }, anim('art-mist', '50% 50%', R() * 20, 'animation-duration:' + N(16 + R() * 8) + 's')))); }
    return G({ class: 'art-mists', 'pointer-events': 'none' }, s);
  }

  // ---------- particles and small decorations (Keeper animations, HUD) ----------
  // a green leaf centred at (0,0), pointing up-right; used for the action leaves and flying moss/petals
  function leafSprite(o = {}) {
    const t = tone(o.color || '#5cb84a', .55, .14, .16), L = 20, W = 9;
    if (o.empty) return sized(o.size, 24, G({ class: 'art-leaf art-leaf-empty' }, Pa(leafD(L, W), { fill: 'none', stroke: o.stroke || '#9a7a52', 'stroke-width': 2, 'stroke-dasharray': '3 2.4', transform: T(-L / 2 * .76, L / 2 * .64, 1, -40) })));
    return sized(o.size, 24, G({ class: 'art-leaf' }, stack([leaf(-L / 2 * .76, L / 2 * .64, L, W, -40, t, { ow: 1.6 })]), gloss(-2.6, -1.8, 2.4, 1, -40, .7)));
  }
  // a curled dead leaf standing up (not squashed), centred, for wind gusts and the Clear sweep
  function deadLeaf(o = {}) {
    const R = rng(o.seed ?? 3), col = o.color || DEAD[Math.floor(R() * DEAD.length)];
    return sized(o.size, 16, G({ class: 'art-deadleaf' }, curledLeaf(0, 0, 15, 0, col, R() < .5 ? 1 : 0).replace(/scale\(1,[^)]*\)/, 'scale(1,1)')));
  }
  const petal = (color = '#ffd2e4', o = {}) => sized(o.size, 8, G({ class: 'art-petal' }, El(0, 0, 3.6, 2.2, { fill: color, stroke: dark(color, .35), 'stroke-width': .8 }), El(-1, -.7, 1.4, .6, { fill: '#fff', opacity: .7 })));
  // a soft round puff: dust (cream), steam (white), smoke
  function puff(o = {}) {
    const t = tone(o.color || '#efe4c8', .3, .06, .2);
    return sized(o.size, 20, G({ class: 'art-puff' }, Ci(-4, 2, 6.4, { fill: t.o, opacity: .5 }) + Ci(4, 1.6, 5.6, { fill: t.o, opacity: .5 }) + Ci(0, -2, 7.2, { fill: t.o, opacity: .5 })
      + Ci(-4, 2, 5.4, { fill: t.d }) + Ci(4, 1.6, 4.6, { fill: t.d }) + Ci(0, -2, 6.2, { fill: t.l }) + Ci(-2, -4, 2, { fill: '#fff', opacity: .7 })));
  }
  // sunburst rays (treasure moment, dawn); size = diameter
  function rays(o = {}) {
    const n = o.n || 16, c = o.color || '#ffe28a', r = 50;
    let d = '';
    for (let i = 0; i < n; i++) { const a0 = (i / n) * 360 * RAD, a1 = a0 + (180 / n) * RAD; d += D`M0,0 L${Math.cos(a0) * r},${Math.sin(a0) * r} L${Math.cos(a1) * r},${Math.sin(a1) * r}Z`; }
    return sized(o.size, 100, G({ class: 'art-rays' }, Ci(0, 0, r, { fill: 'url(#art-gold)', opacity: .9 }) + Pa(d, { fill: c, opacity: o.opacity ?? .45 })));
  }
  // a leafy sprig for HUD frame corners (3 leaves on a twig), ≈ 40 units wide
  function sprig(o = {}) {
    const a = tone(o.color || '#5cb84a', .55, .14, .16), b = tone(o.color2 || '#3f9e4a', .55, .14, .14), tw = tone('#8a5a36', .55, .16, .12);
    return sized(o.size, 40, G({ class: 'art-sprig' }, stack([limb('M-18,6 Q-4,2 12,-8', 2.4, tw, { light: false }), leaf(-8, 4, 14, 6.4, -120, b, { ow: 1.3 }), leaf(2, -1, 15, 6.6, -60, a, { ow: 1.3 }), leaf(10, -7, 13, 6, 10, a, { ow: 1.3 }), leaf(-14, 6, 11, 5.4, 160, b, { ow: 1.3 })])));
  }

  // ---------- fireflies ----------
  function firefly(x, y, o = {}) {
    const R = rng(o.seed ?? (x * 31 + y * 17));
    return G({ transform: T(x, y) }, G(anim('art-firefly', '50% 50%', R() * 7, 'animation-duration:' + N(6 + R() * 4) + 's'), Ci(0, 0, 5, { fill: 'url(#art-fly)' }), Ci(0, 0, 1.3, { fill: '#fbffc8' })));
  }
  function fireflies(n, o = {}) {
    const R = rng(o.seed ?? 5), w = o.w ?? 400, h = o.h ?? 300, x0 = o.x ?? -w / 2, y0 = o.y ?? -h / 2;
    let s = '';
    for (let i = 0; i < n; i++) s += firefly(x0 + R() * w, y0 + R() * h, { seed: (o.seed ?? 5) * 100 + i });
    return G({ class: 'art-fireflies' }, s);
  }

  // ---------- defs, css, install ----------
  function defs() {
    const stop = (o, c, a) => el('stop', { offset: o, 'stop-color': c, 'stop-opacity': a });
    return '<defs>'
      + el('radialGradient', { id: 'art-shadow' }, stop(0, '#08140c', .5) + stop(.55, '#08140c', .3) + stop(1, '#08140c', 0))
      + el('linearGradient', { id: 'art-wallshade', x1: 0, y1: 0, x2: 0, y2: 1 }, stop(0, '#1a0d05', 0) + stop(.35, '#1a0d05', .05) + stop(1, '#1a0d05', .5))
      + el('linearGradient', { id: 'art-topsheen', x1: 0, y1: 0, x2: 1, y2: 1 }, stop(0, '#ffffff', .16) + stop(.45, '#ffffff', 0) + stop(1, '#0a2a10', .14))
      + el('radialGradient', { id: 'art-water', cx: .42, cy: .38, r: .68 }, stop(0, '#b8f6ff', 1) + stop(.5, '#46bde9', 1) + stop(1, '#1f6cb5', 1))
      + el('radialGradient', { id: 'art-gold' }, stop(0, '#fff6c2', .95) + stop(.45, '#ffd54a', .55) + stop(1, '#ffb300', 0))
      + el('radialGradient', { id: 'art-warm' }, stop(0, '#ffe08a', .9) + stop(.5, '#ffb347', .4) + stop(1, '#ff8a2a', 0))
      + el('radialGradient', { id: 'art-dawn' }, stop(0, '#fff3c4', .75) + stop(.5, '#ffd88a', .35) + stop(1, '#ffc46a', 0))
      + el('radialGradient', { id: 'art-fly' }, stop(0, '#f6ffb0', .95) + stop(.4, '#d8ff6a', .45) + stop(1, '#b8ff4a', 0))
      + el('radialGradient', { id: 'art-hollow', cx: .5, cy: .35, r: .65 }, stop(0, '#3e2412', 1) + stop(1, '#140904', 1))
      + el('radialGradient', { id: 'art-hollow-lit', cx: .5, cy: .45, r: .6 }, stop(0, '#ffe7a0', 1) + stop(.5, '#c97a2a', 1) + stop(1, '#4a2410', 1))
      + el('linearGradient', { id: 'art-shaft', x1: 0, y1: 0, x2: 0, y2: 1 }, stop(0, '#fff6c8', 0) + stop(.35, '#fff1b0', .32) + stop(1, '#ffe48a', .05))
      + el('radialGradient', { id: 'art-mist' }, stop(0, '#dfe9f2', .5) + stop(.6, '#cfdbe6', .22) + stop(1, '#cfdbe6', 0))
      + el('radialGradient', { id: 'art-dapple' }, stop(0, '#fff3b8', .55) + stop(.6, '#ffe58a', .2) + stop(1, '#ffe58a', 0))
      + el('filter', { id: 'art-grey' }, el('feColorMatrix', { type: 'saturate', values: 0 }))
      + el('filter', { id: 'art-blur', x: '-50%', y: '-50%', width: '200%', height: '200%' }, el('feGaussianBlur', { stdDeviation: 3 }))
      + '</defs>';
  }
  const CSS = [
    '.art-sway{animation:art-sway 4.6s ease-in-out infinite}',
    '@keyframes art-sway{0%,100%{transform:rotate(-1.6deg)}50%{transform:rotate(1.6deg)}}',
    '.art-flicker{animation:art-flicker 2.6s linear infinite}',
    '@keyframes art-flicker{0%,100%{opacity:.9}10%{opacity:.62}18%{opacity:1}42%{opacity:.78}58%{opacity:1}80%{opacity:.7}}',
    '.art-needle{animation:art-needle 3.4s ease-in-out infinite}',
    '@keyframes art-needle{0%,100%{transform:rotate(-16deg)}30%{transform:rotate(10deg)}55%{transform:rotate(-6deg)}78%{transform:rotate(4deg)}}',
    '.art-glow{animation:art-glow 1.8s ease-in-out infinite}',
    '@keyframes art-glow{0%,100%{opacity:.55;transform:scale(.93)}50%{opacity:1;transform:scale(1.06)}}',
    '.art-ripple{animation:art-ripple 3.2s ease-out infinite}',
    '@keyframes art-ripple{0%{transform:scale(.25);opacity:0}15%{opacity:.85}100%{transform:scale(1.2);opacity:0}}',
    '.art-twinkle{animation:art-twinkle 3s ease-in-out infinite}',
    '@keyframes art-twinkle{0%,55%,100%{transform:scale(.15) rotate(0deg);opacity:0}72%{transform:scale(1) rotate(45deg);opacity:1}}',
    '.art-firefly{animation:art-firefly 7s ease-in-out infinite}',
    '@keyframes art-firefly{0%,100%{transform:translate(0,0);opacity:.15}25%{transform:translate(7px,-9px);opacity:1}50%{transform:translate(-4px,-16px);opacity:.35}75%{transform:translate(-9px,-5px);opacity:.9}}',
    '.art-drift{animation:art-drift 9s ease-in infinite;opacity:0}',
    '@keyframes art-drift{0%,50%{transform:translate(0,0) rotate(0deg);opacity:0}56%{opacity:1}100%{transform:translate(18px,42px) rotate(280deg);opacity:0}}',
    '.art-bob{animation:art-bob 1.5s ease-in-out infinite}',
    '.art-rain{animation:art-rain .9s linear infinite}',
    '@keyframes art-rain{0%{transform:translate(0,-6px);opacity:0}20%{opacity:.9}100%{transform:translate(-5px,16px);opacity:0}}',
    '.art-fog{animation:art-fog 14s ease-in-out infinite}',
    '@keyframes art-fog{0%,100%{transform:translateX(-7px);opacity:.6}50%{transform:translateX(7px);opacity:1}}',
    '.art-shaft{animation:art-shaft 6s ease-in-out infinite}',
    '@keyframes art-shaft{0%,100%{opacity:.55}50%{opacity:1}}',
    '@keyframes art-bob{0%,100%{transform:translateY(0)}50%{transform:translateY(-3px)}}',
    '.art-dapple{animation:art-dapple 7s ease-in-out infinite}',
    '@keyframes art-dapple{0%,100%{transform:translate(-5px,1px) scale(.9);opacity:.45}50%{transform:translate(6px,-2px) scale(1.08);opacity:1}}',
    '.art-mist{animation:art-mist 18s ease-in-out infinite}',
    '@keyframes art-mist{0%,100%{transform:translateX(-16px);opacity:.55}50%{transform:translateX(16px);opacity:1}}',
    '@media (prefers-reduced-motion:reduce){.art-sway,.art-flicker,.art-needle,.art-glow,.art-ripple,.art-twinkle,.art-firefly,.art-drift,.art-bob,.art-rain,.art-fog,.art-shaft,.art-dapple,.art-mist{animation:none!important}.art-drift,.art-rain{opacity:0}}',
  ].join('\n');
  function install(doc) {
    doc = doc || (typeof document !== 'undefined' ? document : null);
    if (!doc || doc.getElementById('art-defs')) return;
    const st = doc.createElement('style');
    st.id = 'art-css'; st.textContent = CSS;
    (doc.head || doc.documentElement).appendChild(st);
    const box = doc.createElement('div');
    box.innerHTML = '<svg id="art-defs" xmlns="http://www.w3.org/2000/svg" width="0" height="0" style="position:absolute;width:0;height:0;overflow:hidden" aria-hidden="true">' + defs() + '</svg>';
    (doc.body || doc.documentElement).appendChild(box.firstChild);
  }
  function svg(inner, o = {}) {
    install();
    const vb = o.viewBox || '-24 -24 48 48', w = o.width ?? o.size ?? 48, h = o.height ?? o.size ?? 48;
    return '<svg xmlns="http://www.w3.org/2000/svg" viewBox="' + vb + '" width="' + w + '" height="' + h + '"' + (o.cls ? ' class="' + o.cls + '"' : '') + ' style="overflow:visible">' + (o.title ? '<title>' + esc(o.title) + '</title>' : '') + inner + '</svg>';
  }

  const ART = {
    TILT, S: 40, VALUES: VAL, values: { Z: VAL[0], E: VAL[1], R: VAL[2], A: VAL[3], O: VAL[4], S: VAL[5] },
    palette: { up: UP, down: DOWN, soil: SOIL, fruit: '#e8344f', water: '#3aa6f0', gold: '#f2b32e', bramble: '#7d47a6', bark: '#8a5636', ink: '#2a1a10', cream: '#f6e6c4' },
    species: SPECIES, letters: LETTERS,
    ringOf, elev, depth, project, hexes, DIRS,
    facePoints: (S = ART.S, k = .965) => ptsStr(facePts(S * k)),
    tileOutline: (ring = 4, S = ART.S, k = .965) => { const c = facePts(S * k), d = depth(ring, S), dn = p => [p[0], p[1] + d]; return ptsStr([c[5], c[0], c[1], dn(c[1]), dn(c[2]), dn(c[3]), c[3], c[4]]); },
    sortKey: (q, r, S = ART.S) => project(q, r, S)[1],
    tile, plant, worldTree, leaves, spring, treasure,
    fruit: (o = {}) => sized(o.size, 24, G({ class: 'art-fruit' }, G({ transform: 'translate(0,1.2) scale(1.12)' }, fruitBody(1)))),
    drop: (o = {}) => sized(o.size, 24, G({ class: 'art-drop' }, G({ transform: 'translate(0,-1) scale(1.12)' }, dropBody(1)))),
    acorn: (o = {}) => sized(o.size, 24, G({ class: 'art-acorn' }, G({ transform: 'translate(0,-.2) scale(1.02)' }, acornBody(1)))),
    medallion, weatherBadge, valueBadge, roleBadge, icon, icons: Object.keys(ICON),
    weatherFx, firefly, fireflies, sparkle: (x, y, r, d) => sparkle(x, y, r, d),
    sectorWeather, dapple, mist, leaf: leafSprite, deadLeaf, petal, puff, rays, sprig,
    defs, css: CSS, install, svg,
    mix, dark, light, tone, rng,
  };
  if (typeof window !== 'undefined') window.ART = ART;
  if (typeof module !== 'undefined') module.exports = ART;
})();
