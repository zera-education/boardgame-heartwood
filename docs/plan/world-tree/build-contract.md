# World Tree — build contract

The single source of truth for the rebuild. Backend, Keeper screen, phone and docs are built in parallel
against this file. The rules engine reference is `docs/plan/world-tree/sim/engine.js` (JS); where this file
and engine.js differ, **this file wins** (El's later decisions: 6–12 players, named treasures in Rings 1–2,
auto-placing, springs only, 2 actions).

The playable prototype (`docs/plan/world-tree/ui-proposal.html`, tab "▶ Keeper screen") shows the intended
Keeper UI and animation; reuse its drawing and animation code freely.

Everything else in today's game is **removed**: seasons, ring locks, discoveries/tokens, Squirrel, Campfire,
old powers, Dusk, pairs, alliances, Bond cards, statements, statement tips, points/scores, the stories phase.
Keep the house style: plain HTML/JS/CSS, no build step, no frameworks, dark forest tokens in `web/style.css`.

## Rules (final)

- **Players: 6 to 12.** Join refuses a 13th; Start needs at least 6 (and at most 12), and every player needs at
  least one Enneagram type. 12 colours (add `#00acc1` and `#7cb342` to `Colors`).
- **Board: 61 hexes.** Index 0 is the World Tree (centre, ring 0, sector -1). Rings 1–4 built exactly like
  `buildHexes(4)` in engine.js (same order and `sector = (3 - side + 6) % 6`, so ZERAOS reads clockwise from
  the top). Sector = value index 0..5 (Zealous, Excellence, Resilience, Authenticity, Open-mindedness,
  Sustainability). 10 hexes per sector.
- **Tiles (60, face down at start):** 3 treasures hidden on random hexes in **Rings 1–2 only**; 4 springs on
  random other hexes (any ring 1–4); 53 empty. Each tile: `up`, `kind` (empty|spring|treasure),
  `treasure` (which treasure still lies there, "" once taken), `stage` 0..4 (0 none, 1 Seeded, 2 Grass,
  3 Shrub, 4 Big Tree), `leaves` 0..2 (2 = sealed), `harvested` (since the last Tide).
- **The three treasures** (shuffled onto the 3 treasure hexes), each with a meaning and a group moment shown
  when revealed (kind `treasure` share, everyone answers in one sentence, no trust):
  - `compass` 🧭 **The Compass** — *Purpose: where we're going.* "Everyone, one sentence: where do you want
    this team to be a year from now?"
  - `lantern` 🏮 **The Lantern** — *Lighting the way for others.* "Everyone, one sentence: who lit the way
    for you when you were new?"
  - `rope` 🪢 **The Rope** — *Holding together.* "Everyone, one sentence: what holds this team together when
    things get hard?"
  When all three are on the World Tree, the log/board says: "Purpose, light and each other: the heart of the
  forest is complete."
- **Weather** per sector: sun / rain / fog, equally likely; drawn at game creation and again at every Forest
  Tide. The next weather per sector is pre-drawn (for the Investigator). The World Tree has no weather.
- **Water:** start 4, max 5. Only **springs** refill (Drink: back to 5, 1 action); springs never run dry.
  Rain does **not** refill. At 0 water a player cannot Move until someone passes them 1. No elimination.
- **Entering (phase `enter`):** in turn order each player chooses their value and an outer-ring (ring 4) hex of
  that sector (sharing a hex is allowed), then shares **why** (share kind `why`).
- **Turn (phase `turn`):** clockwise in join order; up to **2 actions**. Actions (1 action unless noted):
  - **Move** to a neighbouring hex that isn't sealed (Challenger may enter sealed). Not at 0 water.
    Enthusiast: once per turn a move may go 2 hexes (both steps unsealed, neither hex in fog, own hex not in
    fog). Challenger may bring one teammate standing on the same hex (`target`).
    **Ring card:** the first time a player enters Ring 3, Ring 2 or Ring 1 (moving inward), they draw a card
    from that ring's deck (share kind `ring`): Ring 3 = today's Seed prompts, Ring 2 = Sapling, Ring 1 = Oak,
    pooled across values (30 each). Applies to the brought teammate too.
  - **Explore** the face-down hex you stand on (2 actions in fog). Individualist also privately peeks at one
    face-down neighbour (goes to their phone). Revealing a treasure triggers that treasure's group moment.
  - **Sow** on an explored empty hex with no plant, no treasure lying there, not sealed → Seeded (Achiever →
    Grass).
  - **Water** (costs 1 water): Seeded → Grass, Grass → Shrub. Own hex, or a neighbour for the Helper.
  - **Tend:** Shrub → Big Tree. Own hex, or a neighbour for the Helper.
  - **Clear:** remove 1 layer of leaves from your hex or a neighbour (Reformer: 2 layers). 2 actions in rain
    (Reformer: still 1).
  - **Harvest:** on an unsealed Big Tree, not in rain, not harvested since the last Tide, carrying < 2 fruit →
    +1 fruit; share kind `harvest`: "Tell us a story from your own life about {Value}." with the value's
    tagline, where Value = the sector of the tree.
  - **Take** a revealed treasure lying on your hex (carry at most 1).
  - **Drink** at a revealed spring → water 5.
  - **Free, on the giver's own turn when no share is open** (El, 2026-10-06: only the current player gives):
    **pass** water / fruit / treasure to a player on the same hex; or between a Peacemaker and a teammate on a neighbouring hex, either way (El, 2026-10-06:
    the 2-hex chain is gone).
    Limits: water ≤ 5, fruit ≤ 2, treasure ≤ 1.
  - **Automatic placing:** whenever a player stands on the World Tree holding fruit or a treasure (after any
    move, pass, or Challenger carry), **everything they carry is placed at once**. If at least one fruit was
    placed, that player answers **one Heartwood question** (share kind `heartwood`), however many fruit.
  - **End turn** (Keeper). After the last player of the round, the **Forest Tide** runs automatically.
- **Forest Tide:** (1) each sector takes its pre-drawn weather and pre-draws the next; reset `harvested`.
  (2) Rain: every unsealed Seeded/Grass hex in a rainy sector grows one stage. Sun: every player in a sunny
  sector not standing on a Big Tree loses 1 water (not below 0). (3) **Forest Breath:** the pre-rolled list
  for this Tide (size = min(6, 2 + floor(tidesSoFar / 2)), each entry `{from,to}`: `from` = random hex 1..60,
  `to` = a random neighbour one ring closer to the centre, or `from` itself on Ring 1; never the World Tree;
  El, 2026-10-08: `from` may also be one of 30 spots beyond the edge, `from` = -1, `to` = a random Ring 4 hex)
  adds 1 leaf layer to `to` unless already sealed; a Seeded/Grass/Shrub there drops one stage (Seeded → none)
  unless a player stands on it (El, 2026-10-08) or a Loyalist stands next to it (7 hexes in all). Big Trees keep
  their stage. Then pre-roll the next Breath.
  A player standing on a hex that seals can still walk out.
- **Roles (Enneagram types 1–9, always on):** 1 Reformer, 2 Helper, 3 Achiever, 4 Individualist,
  5 Investigator, 6 Loyalist, 7 Enthusiast, 8 Challenger, 9 Peacemaker; texts in engine.js `ROLE_TEXT`
  (Peacemaker: "You and a teammate on a hex next to yours can pass things to each other.").
  The Keeper gives each player 1–3 types in the lobby (a player can't hold one twice; several players may
  hold the same type), or deals one random type to everyone (`randomTypes`: all different up to 9 players, no
  type twice before all 9 are dealt; El, 2026-10-08). Investigator: always sees the next Forest Breath list on their phone, and once per
  round (during their own turn) looks at one sector's next weather (phone action `investigate`, `n`=sector).
- **Win:** the moment all hold: every sector has ≥1 Big Tree; every player has placed ≥1 fruit; all 3
  treasures placed; every player stands on the World Tree → result `won`, log "The forest wakes!".
  **Lose:** every player at 0 water → result `lost`. Either way → phase `guess`.
- **Shares:** a queue. While a share is open, no play action is accepted (only trust, Keeper `doneShare`,
  timers). Each share: `{kind, player, prompt, sub}`; kinds `why`, `ring` (sub = ring number), `harvest`
  (sub = tagline), `heartwood`, `treasure` (group moment; player = finder; sub = treasure id).
  Listeners (not the sharer) may give **1 trust acorn** per share from their 10 (`trust`), except for
  `treasure` shares. Keeper taps **doneShare** to close it.
- **Finale:** `guess` (each player guesses their Secret Owl on the phone; Keeper `startChain` when ready) →
  `chain` (tribute chain as today: `nextTribute`) → `end` (result + recognition: per player trust acorns
  received and from how many people, their Secret Owl, whether they guessed right, whether their Owl stayed
  hidden). No points, no winner among players.
- **Secret Owl:** dealt at Start exactly as today (one loop through a shuffled order).

## Content (`content.go`)

Keep `Values`. Replace region cards with `RingDecks [4][]string` (index 1..3; build from today's
RegionCards: tier Oak → ring 1, Sapling → ring 2, Seed → ring 3), `HeartwoodCards` (today's 6 plus at least
8 new ones, e.g. "What has this team given you that you didn't expect?", "Which value do you want this team to
grow in most this year, and why?", "What will you do differently after today?", "Who here would you like to
know better, and what would you ask them?", "What makes you proud to work at ZERA?", "Who at this table do you
want to thank, and for what?", "What would a 'woken forest' look like at our school?", "When did you last see
this team at its best?"), `HarvestPrompt` template, `Treasures` (id, icon, name, meaning, question),
`Roles` (type, name, text), `Colors` (12). Delete Bond, Statement, Squirrel, Campfire, Powers, TokenMix,
StatementTips, RegionThreads (if the wiki needs themes, keep RegionThreads only for the wiki).

## HTTP API (unchanged routes)

`POST /api/games` create, `POST /api/games/{code}/join`, rejoin code routes, `POST /api/games/{code}/action`,
`GET /api/games/{code}/state`, `GET /api/games/{code}/live` (WebSocket), `GET /api/cards`, `/qr`. Unchanged
mechanics: versioned views, pushes after every change, Keeper secret, rejoin pins.

`Action` JSON: `{pid, secret, host, type, hex, target, n, text, types, as}` (`hex` defaults to -1).

**Keeper acts for a player** (`host` + `as`; keeperActs): `enter` (n=value, hex), `move` (hex, target=brought
teammate id or ""), `explore`, `sow`, `water` (hex), `tend` (hex), `clear` (hex), `harvest`, `take`, `drink`,
`pass` (target=receiver id, text=water|fruit|treasure), `endTurn`.
**Keeper only:** `start`, `assignPowers` (target, types), `kick`, `doneShare`, `startChain`, `nextTribute`,
`timer` (n seconds).
**Phone only (own pid+secret):** `trust`, `investigate` (n=sector), `guess` (target).
Errors are plain sentences shown to the user.

## View JSON (`buildView`) — exact shape

```
{
  code, phase: "lobby"|"enter"|"turn"|"guess"|"chain"|"end", version, now, isHost, lan, colors,
  values: Values, roles: [{type,name,text}], treasures: [{id,icon,name,meaning}],
  minPlayers: 6, maxPlayers: 12, actionsPerTurn: 2,
  round, tide, result: ""|"won"|"lost",
  hexes: [{q,r,ring,sector}],                      // 61, index 0 = World Tree
  tiles: [null, {up, kind, treasure, stage, leaves, harvested}, ...],
                                                   // kind and treasure are "" while face down
  weather: ["sun"|"rain"|"fog" x6], breathCount,
  players: [{id,name,color,value,types,pos,water,fruit,treasure,placed,reached:[r1,r2,r3 bools],
             trustLeft, guessed}],               // treasure = id or ""
  order: [ids], current: id (enter/turn only),
  turn: {player, actions, enthusiastUsed, investigated} | null,
  share: null | {idx, kind, player, prompt, sub, trustCount, iGave},
  goals: {trees:[6 bools], placedPlayers, players, treasures:[ids placed], onTree},
  events: [{id, type, ...}],                       // the last 80; ids increase
  log: [last 40 strings],
  timerEnd, timerLabel,
  tribute: {giver, receiver, idx, total}           // chain phase
  recognition: [{id,name,color,trust,givers,owlName,owlHidden,guessedRight}]   // end phase
  me: {id, target, targetName, guess, peeks:[...], breath:[{from,to}] (Investigator only),
       weatherPeek: {sector, weather} (Investigator, this round)}   // only with a valid pid+secret
}
```

**Events** (for the Keeper animation; each has `id` and `type`): `move {pid,from,to}`, `flip {hex}`,
`grow {hex,stage}`, `harvest {hex,pid}`, `clear {hex}`, `take {hex,pid,treasure}`, `drink {hex,pid}`,
`pass {from,to,item}`, `place {pid,fruit,treasure}`, `treasure {hex,treasure}` (revealed),
`tide {weather:[6], growth:[hexes], dry:[pids], leaves:[{from,to}], sealed:[hexes]}`, `wake {}`, `dry {}`.
Private results (Individualist peek, Investigator look) go into that player's `me.peeks`, never into events.

## File ownership (parallel agents — edit only your files)

| Agent | Owns | Must not edit |
|---|---|---|
| Backend | `game.go`, `content.go`, `main.go`, `game_test.go`, `sim_test.go` | `web/*` |
| Keeper screen | `web/board.html`, `web/common.js`, `web/style.css`, `web/index.html` | Go files, `play.html`, docs pages |
| Phone | `web/play.html` (put any new CSS in a `<style>` block inside play.html) | everything else |
| Docs | `web/howto.html`, `web/wiki.html`, `README.md` | everything else |

`web/common.js` keeps its connection layer (WebSocket, versions, reconnect, badge, `HW.act`, timers,
`HW.esc`, `HW.patch`); the Keeper agent replaces `renderBoard` and season helpers and must keep every
function `play.html` uses working (phone agent: only use `HW` connection/util functions; don't rely on board
drawing). Docs pages fetch `/api/cards` for card lists: shape
`{values, ringDecks:{1:[],2:[],3:[]}, heartwood:[], harvest: "...", treasures:[...], roles:[...]}`.
