# World Tree art pass — build contract

El's decisions (2026-10-06). This file is binding for every agent; the rules of play do not change
(see `docs/plan/world-tree/build-contract.md`), only names, art, animation and the photo.

## Direction

- **Theme: The Sleeping Forest.** The forest is asleep under a dusk shadow; the team wakes it ("The forest wakes!").
- **Style: shaded toon, like Plants vs Zombies.** Thick *coloured* outlines (a darker shade of each fill, never
  black), two-tone cel shading (dark body + lighter top-left layer) and one glossy white highlight, soft ground
  shadows, chunky round shapes. Motion: squash and stretch, springy overshoot (`cubic-bezier(.34,1.56,.64,1)`),
  gentle idle sway with a different delay per object. Fonts: **Lilita One** (titles, numbers, labels on cards
  and packets) and **Nunito** (reading text), from Google Fonts.
- **Plants have no faces** (they are not main characters) but each stage has character: shape, lean, leaf
  rustle, drifting petals. Nothing else on the board has a face either. No character design for players: a player
  is their **photo in a round medallion**.
- **No emoji on the board or the share cards.** Everything there is drawn by `web/art.js`. Emoji may stay in log
  lines and docs.

## Names (rename everywhere: Go, view, UI, docs)

- Growth stages (ints unchanged): 1 **Seeded** → 2 **Sprout** → 3 **Sapling** → 4 **Big Tree**
  (was Seeded, Grass, Shrub, Big Tree). Achiever: "After you Sow, the hex grows straight to Sprout."
- Ring card decks: Ring 3 **Light**, Ring 2 **Story**, Ring 1 **Deep** (was Seed, Sapling, Oak). Content is the same.
- View gains `stages: ["", "Seeded", "Sprout", "Sapling", "Big Tree"]` and
  `ringDecks: ["", "Deep", "Story", "Light"]` (index = ring) so clients don't hard-code them.

## The map: 2.5D, full screen (Keeper screen)

- **Full screen.** The board is an SVG filling the whole viewport (100vw × 100vh, no page scroll) and auto-fits
  on resize. Everything else floats over it as toon HUD panels (see Keeper layout).
- **2.5D projection.** Pointy-top hexes, axial (q, r) exactly as today. World → screen: `x = S·√3·(q + r/2)`,
  `y = S·1.5·r·TILT − elev(ring)`, with `TILT = 0.6`. A tile is a squashed hex top face plus a visible
  **side wall** (depth ≈ 0.42·S) under its lower edges: lower-left wall darker than lower-right (light from the
  top-left). Tiles are painted back to front (smaller screen y first).
- **The forest rises to the World Tree:** `elev(ring) = (4 − ring) · 0.16·S` (ring 4 lowest, centre highest),
  so inner rings sit on low terraces whose walls show.
- **Colour deepens toward the centre.** Ring 4 is fresh light grass; each ring inward is deeper and richer, Ring 1
  deep emerald, the World Tree hex dark mossy earth with roots. Face-down tiles are a mossy leaf blanket that
  deepens by ring the same way. A sector shows through a light tint of its value colour (≈15%) and a trim/banner
  in that colour on Ring 4's outer edge.
- **Objects stand up.** Plants, the World Tree, springs, treasures and leaves are drawn upright (not squashed),
  anchored at the tile's top-face centre, in one **object layer above all tiles**, sorted back to front, so a big
  tree may overlap the hexes behind it and spill a little onto neighbours. Sizes at hex size S:
  Seeded ≈ 0.5·S tall, Sprout ≈ 0.8·S, Sapling ≈ 1.3·S, Big Tree ≈ 2.1·S tall × 1.6·S wide, World Tree ≈ 4.5·S
  tall × 3.4·S wide (it may cover Ring 1 behind it).
- **Players are never hidden:** the medallion layer sits above the object layer (still sorted back to front
  among themselves). Several players on one hex fan out and shrink; 12 on the World Tree ring around its trunk.
- **Highlights** for legal targets (move, act) are a bright pulsing rim on the tile's top face plus a hit area
  covering top face + wall.

## Plants (web/art.js)

- **Seeded:** a dirt mound with a seed and one curled shoot.
- **Sprout:** two round leaves on a curled stem.
- **Sapling:** a thin, slightly leaning trunk with 3–4 leaf clusters.
- **Big Tree, one species per value**, in the value's colour family:
  Zealous = flame maple (red-orange), Excellence = golden ginkgo, Resilience = pine (bends, doesn't break),
  Authenticity = cherry blossom (pink), Open-mindedness = jacaranda (blue-violet), Sustainability = wide oak
  (teal-green). Harvestable: heart-shaped fruit hangs in the canopy; harvested since the last Tide: no fruit.
  Idle: canopy sway, an occasional leaf or petal drifts off.
- **World Tree** on the centre hex: huge trunk with roots over the hex, layered canopy, a heart-shaped **hollow**
  holding 3 treasure slots (empty / Compass / Lantern / Rope) and a fruit count. Asleep: drooping leaves, closed
  buds, dim. Awake (result `won`): blooms, glows, petals.

## Other elements (web/art.js)

- Dead leaves: 1 layer = a few scattered curled leaves; 2 = **sealed**: a leaf heap with purple brambles over the
  tile.
- Spring: a stone-ringed pond with ripples and a sparkle. Treasures: **Compass** (brass, needle wobbles),
  **Lantern** (glowing paper lantern, flicker), **Rope** (coiled rope with a knot); lying on a hex they sit on a
  small stump with a glint. Items: heart-fruit, water drop, trust acorn.
- Weather badges: Sun, Rain cloud, Fog puff (no faces). Weather overlays per sector: sun shafts + warm tint, rain
  streaks + cool tint, drifting fog.
- Value badges (6, letter Z E R A O S on the value colour), role badges (9, one symbol each: 1 rake, 2 watering
  can, 3 rising sprout, 4 eye, 5 spyglass, 6 shield, 7 double chevron, 8 boot, 9 linked rings).
- Action icons for seed-packet buttons: move, explore, sow, water, tend, clear, harvest, take, drink, pass, end;
  plus trust, owl, timer.

## The photo medallion

- Round photo clipped in a circle, inside a thick ring in the player's colour (outlined, glossy highlight),
  standing on a small wooden stump with a ground shadow.
- **Water crown:** 5 drop-shaped segments around the top of the ring; filled = water, empty = dim.
- Small badges: fruit (with count if 2), treasure icon.
- States: `turn` (golden glow + gentle bob), `dry` (0 water: photo greyscale, medallion droops), normal.
- No photo: the player's initial in Lilita One on their colour.

## Progress overlay (instead of a sky)

- A dusk shadow lies over the whole board, with slow drifting mist and a few fireflies.
- When a sector has its first Big Tree, the shadow **lifts from that sector** in a slow wave, and dappled light
  moves through it; asleep sectors stay dim. Win: dawn spreads from the centre outward and everything blooms.
  Lose (`lost`) / ended: dusk deepens, mist thickens.

## Animation per event (Keeper)

move = hop along the path with squash, dust puff on landing · enter = drop in + value-colour ring burst ·
flip = top face flips, moss blows off, reveal (spring sparkle) · grow = dirt burst, plant pops (overshoot) ·
harvest = tree shakes, fruit bounces to the medallion · clear = leaves spin away (a rake sweep) ·
drink = water arcs in, crown segments fill one by one · take = treasure rises spinning, flies to medallion ·
pass = item arcs between medallions (via the Peacemaker if chained) · place = items float into the hollow, the
heart pulses · treasure = screen dims, treasure rises in light rays with its name banner and meaning ·
tide = weather badges flip, rain → plants pop, sun → steam from medallions and crown segments empty, wind gusts
carry leaves `from → to`, brambles close over newly sealed tiles · wake = dawn wave, bloom · dry = dusk.
Idle loops (sway, ripples, fireflies) stop under `prefers-reduced-motion`. Keep it smooth on a school laptop:
CSS/WAAPI transforms and opacity only; blur filters only on a handful of overlay shapes; cap particles.

## Keeper layout (board.html)

Full-screen map; HUD panels in toon style (wooden/leafy frame, thick outline, chunky buttons with a pressed lip):
top-left title + round/Tide + Forest Breath; top-right **goal tracker** (6 sector trees, fruit placed n/N,
3 treasure slots, on the tree n/N); bottom centre **whose turn** (medallion + 2 action leaves) and the **action
tray of seed packets** (icon, name, cost leaves; available / unavailable with reason / selected / costs 2);
right edge a collapsible **drawer**: players (medallions, water, roles), Keeper tools (timer, rejoin, end early),
trail log. Lobby, entering, share cards and the finale are centred toon panels/cards over the dimmed board.
Share cards: Why (value colour), Ring card (Light / Story / Deep), Harvest (fruit), Heartwood (World Tree hollow),
Treasure moment (gold rays). Everything that works today keeps working (all actions, modes, pass, Challenger
bring, Keeper tools, timers, finale, End the game early).

## Photos (server + phone)

- Phone: after joining, a "Take your photo" step (`<input type=file accept="image/*" capture="user">`), drag to
  position + zoom inside a round mask, retake / use / skip; later changeable from the phone ("Your photo") until
  the game ends. The phone crops and scales to **320×320 JPEG (quality ≈ 0.82)** before sending.
- `POST /api/games/{code}/photo` JSON `{pid, secret, data: "data:image/jpeg;base64,…"}` → stores it; decoded size
  ≤ 300 KB, must decode as JPEG/PNG with both sides ≤ 1024 (Go `image.DecodeConfig`), else a plain-sentence
  error. Same route with `data: ""` removes it. The Keeper can remove a player's photo with action
  `removePhoto` (`host`, `target`).
- Stored outside the game state: table `photos(code TEXT, pid TEXT, data BLOB, PRIMARY KEY(code, pid))`.
  `Player.Photo int` (persisted; 0 = none) bumps on every change; view `players[].photo`.
- `GET /api/games/{code}/photo/{pid}?v=N` → the image (`Cache-Control: private, max-age=86400`), 404 if none.
- **Deleted after the game ends:** a sweep (at startup and every 10 minutes) deletes the photos of games whose
  result has been set for more than 2 hours, of games not updated for 24 hours, and of kicked players
  (immediately). Set `Player.Photo = 0` for deleted ones.

## File ownership

| Phase | Agent | Owns |
|---|---|---|
| 1 | Backend + docs | `*.go`, `web/howto.html`, `web/wiki.html`, `README.md` |
| 1 | Art | `web/art.js` (new), `web/art.html` (new: the element sheet) |
| 2 | Keeper | `web/board.html`, `web/style.css`, `web/common.js`, `web/index.html`, may extend `web/art.js` |
| 2 | Phone | `web/play.html` (reads `web/art.js`, doesn't edit it) |
