# Heartwood (digital)

The digital **Heartwood** board game: a cooperative forest game for 6 to 12 school
leaders, played in one room around an animated Keeper screen. The team explores a face-down forest of 61 hexes, grows a
Big Tree in each of the six ZERAOS value sectors, harvests fruit while telling stories, finds three treasures and
gathers on the World Tree in the centre. Its purpose: leaders get to know each other while working as one team. There
are no points and no winner among the players; trust acorns and the Secret Owl stay as recognition.

It runs as a Go server backed by SQLite, with a plain HTML/JS/CSS frontend embedded in the binary. How to play, for
players: `/howto.html`; one page per term: `/wiki.html`. The binding rules are in
`docs/plan/world-tree/build-contract.md` (the approved proposal: `docs/plan/world-tree/ui-proposal.html`).

## Live

**https://heartwood.zera.edu.my**, on the Axon EC2: systemd unit `heartwood` on `127.0.0.1:7779`, binary in
`/opt/heartwood/bin`, SQLite at `/var/lib/heartwood/heartwood.db` (snapshotted to `pre-deploy/` on every deploy),
nginx site `/etc/nginx/sites-available/heartwood.zera.edu.my` (WebSocket upgrade on, `*.zera.edu.my` Origin cert).
DNS is Cloudflare, proxied.

## Run and deploy

```sh
cd ~/lab/heartwood
make run        # build and serve on http://localhost:8490 (data in heartwood.db)
make test       # rules tests
make sim        # balance playtest with team bots
make deploy     # test, push to GitHub, then on the EC2: git pull, build, run deploy/install.sh
make logs       # the service log on the EC2
```

Code: **git@github.com:zera-education/boardgame-heartwood.git** (`main`). `make deploy` needs a clean working tree and
an `axon` SSH host (`SERVER=…` to override). The EC2 keeps a clone at `~ubuntu/boardgame-heartwood`, pulls, builds there
(its Go 1.22 downloads the toolchain `go.mod` asks for) and runs `deploy/install.sh`, which snapshots the database first.

Locally, the server prints the address phones on the same Wi-Fi should use, and the board shows it with a QR code.

## Screens

- **`/` → Create a new game**: opens the **Keeper screen** (`board.html?g=CODE`) for the projector. Whoever creates
  the game is the Keeper; the Keeper controls (Enneagram types, start, acting for players, Done sharing, End turn,
  timers, the tribute chain) appear only on that browser. In the lobby, **Deal 1 random type each** gives every
  player one Enneagram type, all different up to 9 players (the Keeper can still change any of them). Under **Help a player rejoin · Keeper on another device** the
  Keeper can issue a 4-digit rejoin code (one use, 10 minutes) or get a link that moves the Keeper controls to another
  device.
- **The Keeper screen is animated.** It shows the forest (61 hexes, each sector in its value's colour, face-down hexes
  covered in moss and explored ones dug to dirt, weather tint per sector, plants, dead leaves,
  each player's photo medallion), the three goals (once fruit starts arriving, the fruit goal names who still has to bring one:
  a treasure alone doesn't count), each player's water, fruit and treasure, the open share card and the trail log. Everything
  that happens is played as animation from the view's `events`. An Explore always says what it found on the hex,
  "Nothing here" too. **Tap a hex** for its card: value, ring, weather, what lies there, dead leaves, who stands there
  and whether its plant is safe at the next Forest Tide (and **Move here** when the current player can go there).
  Nothing moves on a plain tap: the Keeper picks the action first (**Move**, Water, Tend, Clear), then the hexes it can
  reach glow with numbers 1, 2, 3… in reading order, so the team can say "move to 3"; the number keys pick them too.
  Keys do the commonest taps: **M** Move, **E** Explore, **Enter** End turn (each packet shows its key), so a turn can be
  M, 3, E, Enter. While someone is sharing, **Enter** is Done sharing (one share per press).
  The drawer (**Tools · log**: Keeper tools and the trail log) opens over the map without moving it. **Undo** at the
  top of the trail log (or Ctrl+Z) takes back the Keeper's last tap for a player, newest first, until the turn ends.
  **Turn the map** (in the top-left sign) shows the forest from any of its six sides: the ground turns like a table,
  trees and medallions ride on it upright, the value signs travel with their sectors. It is this screen's view only,
  kept per game.
- **Players say it, the Keeper taps it. Phones down.** Every public action (enter, move, explore, sow, water, tend,
  clear, harvest, take, drink, pass, end turn) is tapped by the Keeper, acting for that player (`host` + `as`). The
  server refuses `trust`, `investigate` and `guess` from the Keeper. The phone keeps only private things: the Secret
  Owl name, what you carry, rings reached, your roles, private looks (Investigator forecast and weather look), Give
  trust, **Not this one** on your own ring card or Heartwood question, and the Secret Owl guess.
- **Players (6 to 12)** scan the QR code or open `play.html?g=CODE`, pick a name and one of 12 colours. Join refuses a
  13th player; Start needs at least 6 and every player needs at least one Enneagram type (the Keeper gives 1 to 3 in
  the lobby). Their seat is kept in the URL (`&p=…`) and on the device, so a reload or a reopened browser goes straight
  back to it. After joining, each player may take a photo (or skip); it shows in their round medallion on the board.
  With no photo, the medallion shows their initial.
- Every screen shows a connection badge: **● Live**, **● Reconnecting…** or **● Offline**.

## Realtime

One WebSocket per screen (`/api/games/{code}/live`). After every change the server pushes each viewer their own view
(secrets stay server-side), in order, plus a heartbeat every 20 s. Actions are plain POSTs whose response carries the
actor's new view, so taps feel instant. Clients keep the newest view by version number, reconnect with backoff, treat
45 s without a heartbeat as a dead connection, open a fresh socket after a phone wakes up, and poll slowly while the
socket is down. Screens redraw only the parts that changed.

Each view also carries `events` (the last 80, with increasing ids: `move`, `flip`, `grow`, `harvest`, `clear`,
`take`, `drink`, `pass`, `place`, `treasure`, `tide`, `wake`, `dry`, `hint`, `sweep`). The Keeper screen plays the events it hasn't seen
yet as animation: medallions hop, tiles flip, plants grow, leaves drift in at the Forest Tide. Private results (the
Investigator's look) go only into that player's `me`, never into events. An Individualist's hint is public: a face-down
tile carries `hint: "something" | "nothing"`, never its kind.

It used server-sent events before. Browsers allow only six open HTTP/1.1 connections per host, so a board plus five
player tabs on one computer froze every other request. WebSockets don't share that limit (tested with 12).

## Flow

Lobby → **brief** (the Keeper's briefing slides on every screen: the goal, limits, obstacles and how to prevent them,
the roles, the Secret Owl side quest, the Forest Pact, how we play; phones show your roles and your Owl) → **enter**
(in turn order, each player chooses the value they stand for and a Ring 4 hex of its sector, and says why) → **turns** (clockwise, up to 2 actions each; after the last player the **Forest Tide** runs by itself: new
weather per sector, rain grows Seeded/Sprout, sun dries players not on a Big Tree, Forest Breath drifts dead leaves one
ring inward, and from beyond the edge onto Ring 4; a plant someone stands on keeps its stage) → the game ends the moment the forest **wakes** (a Big Tree in every sector, every player has placed a
fruit, all 3 treasures placed, everyone on the World Tree) or every player is at 0 water → **guess** (Secret Owl, on
the phones) → **chain** (tribute chain around the Secret Owl loop) → **end** (result and recognition: trust received
and from how many people, the Secret Owl, guessed right, stayed hidden, the value each one stood for, fruit harvested, springs
found, treasures found and picked up; the Keeper taps a card to spotlight that person).

Shares queue up and pause play until the Keeper taps Done sharing: *why* (entering), *ring* cards (the first time a
player reaches Ring 3, 2 and 1: the Light, Story and Deep decks), *harvest* stories (about the value of the tree's sector),
*Heartwood* questions (one per placing that includes fruit) and *treasure* group moments (everyone answers; no trust).
Listeners may give one trust acorn per share from their 10. Springs are the only way to refill water and never run
dry; placing on the World Tree is automatic.

Plants grow in four stages: 🫘 Seeded → 🌱 Sprout → 🪴 Sapling → 🌳 Big Tree. The view carries the names as
`stages` (by stage number) and the ring decks as `ringDecks` (`["", "Deep", "Story", "Light"]`, by ring), so screens
don't hard-code them; `/api/cards` has the same as `stages` and `ringDeckNames`.

## Photos

- `POST /api/games/{code}/photo` with `{pid, secret, data}`, where `data` is a data URL
  (`data:image/jpeg;base64,…`; the phone sends a 320×320 JPEG). The photo must be a JPEG or PNG, at most 300 KB and
  1024 pixels a side. `data: ""` removes it. Players can change it until the game ends and remove it any time. The
  Keeper removes one with the action `removePhoto` (`host`, `target`).
- Photos are kept outside the game state, in the `photos` table. `players[].photo` in the view is 0 for no photo, and a
  new number on every change; every change bumps the game version and is pushed live.
- `GET /api/games/{code}/photo/{pid}?v=N` serves it (`Cache-Control: private, max-age=86400`), or 404.
- **Deleted after the game:** kicking a player deletes their photo at once. A sweep at startup and every 10 minutes
  deletes the photos of games whose result was set more than 2 hours ago (`endedAt`), of games not updated for 24
  hours, and of players or games that are gone.

## Story recordings

- **On the Keeper screen** (`web/recorder.js`, injected into `board.html`): one clip per share, from the moment a
  share opens until it closes (Opus in WebM, mono, 32 kbit/s; MP4 on Safari). The lobby asks for the microphone first
  (**Allow the microphone**), so the browser's question doesn't interrupt the first story. The share card shows
  **● Recording** and **Don't keep this one**; Keeper tools have **Switch recording off/on** (stored on the game:
  `recording` in every view, so the phone's join screen says it) and the **Stories and transcripts** link (also in
  the finale). Only one Keeper device records: `recDevice` in the Keeper's view, taken by the first device that
  allows the microphone, or with **Record on this device**; in one browser only one tab (Web Locks). Clips wait in
  IndexedDB until the server has them (retry with backoff, also after a reload; a reload mid-story sends what was
  recorded, `partial`). No microphone, a refusal or a plain-http page only means no clips; the game never waits.
- **Server** (`recording.go`, table `recordings`, Keeper secret in `X-Keeper` or `?host=`):
  `POST /api/games/{code}/recordings?clip&share&kind&player&value&prompt&sub&seq&of&round&ms&partial` (raw audio
  body, WebM/MP4/Ogg checked by type and magic bytes, 25 MB at most; the same `clip` again is stored once),
  `GET /api/games/{code}/recordings` (metadata and transcripts, in story order), `GET …/recordings/{id}/audio`,
  `DELETE …/recordings/{id}`, `POST …/recordings/{id}/retry`. Keeper actions `record` (n=1/0) and `recordHere`
  (text = device id). Recordings are the team's story record: the photo sweep never touches them; the audio alone
  is deleted 30 days after its transcript (same 10-minute sweep).
- **Transcription on the Mac mini** (`ops/transcribe`, offline Whisper through `steward run transcribe text`): the
  lanes job `heartwood/stories/transcribe` runs `~/.local/bin/hw-transcribe` every 5 minutes. It asks
  `GET /api/transcribe/queue`, downloads `GET /api/transcribe/{id}/audio`, posts `POST /api/transcribe/{id}`
  `{text, language, duration}` or `{error}` (3 tries, then the Keeper can retry). Bearer token in
  `~/.config/heartwood/transcribe-token` on the Mini only; the server has its SHA-256 (`transcribeTokenHash`).
  Quiet with nothing to do; it fails (one lanes alert) when Whisper is missing, the token is refused, or the server
  is unreachable for 30 minutes. Install: `ops/transcribe/install.sh` (after a deploy).
- **Stories page** `stories.html?g=CODE` (Keeper only): every clip by round, with the question, the player, an
  audio player and the transcript; downloads as CSV and JSON.

## Sealed find (one-off surprises)

A private picture the forest turns up for one named player (a team surprise). What it is and whom it waits for never
enter this public repo: the server keeps them in `sealed-find.json` beside its database, and the Mini puts it there
or takes it away with the transcription worker's token: `PUT /api/found` `{name, question, cheer, image (base64)}`,
`DELETE /api/found`. The Mini's copy is `~/.config/heartwood/sealed-find.json`:

    curl -fsS -X PUT https://heartwood.zera.edu.my/api/found -H "Authorization: Bearer $(cat ~/.config/heartwood/transcribe-token)" \
      -H 'Content-Type: application/json' --data-binary @$HOME/.config/heartwood/sealed-find.json

When that player (first name, any case) explores for the third time, once per game, the hex holds the find instead
of nothing (their first two explores find what is there; if the third is a spring or a treasure, their next empty hex
does). `Found` and `FoundTries` are in the game, so Undo takes it back with the explore. Every screen covers over with it (`web/find.js`,
`found` in every view; the picture at `GET /api/games/{code}/found`, only for a game that turned it up): a cute surprise and
the forest's question to them, then the Keeper's **Explained** (`foundCheer`, to the celebration) and **Close** (`foundClose`). The
Keeper's screen plays the sound; phones buzz. Without the file, explore is as it always was.

## Admin

- **`/admin`**: El's page for every game on this server (Live / Ended / All, newest activity first). A game's page shows
  its players and progress and lets him run it as Keeper on this device (the `?k=` Keeper link), add a bot seat (lobby:
  the next free "Bot N" and colour), remove a player (lobby), end the game early (during play) and delete it. A game
  still being played needs its code typed. Deleting keeps the story recordings: the game stays as **Stories only**
  (table `deleted_games`, which keeps its Keeper secret, so its Stories page still opens) until El deletes them too.
- **Login:** a 6-digit code sent to El's Telegram through the Bot API: 5 minutes, works once, one per 30 s and at most
  6 an hour, 5 wrong tries void it; only its hash is kept. A right code sets `hw_admin` (HttpOnly, Secure,
  SameSite=Strict, 30 days, stored hashed in `admin_sessions`). A login from a new place sends an alert; **Log out on
  every browser** ends every session. Every `/api/admin` route except `me`, `code` and `login` needs it; the session
  also opens any game's stories.
- **Setup:** the bot token and El's chat id come from the env vars named by `-tg-token-var` (default
  `HEARTWOOD_TG_TOKEN`) and `-tg-chat-var` (default `HEARTWOOD_TG_CHAT`); `-tg-api` sets the Bot API URL. On the EC2,
  `deploy/install.sh` copies just `TELEGRAM_BOT_TOKEN` and `TELEGRAM_OWNER_ID` out of `~/.axon/.env` (the Axon bot,
  as blessed does) into `/etc/heartwood/heartwood.env` (root only) on every deploy. Without them the login says
  Telegram isn't set up, and the game works as before.

## Roles (Enneagram types, always on)

The Keeper gives each player 1 to 3 types in the lobby; several players may hold the same type.

| Type | Role | Effect |
|---|---|---|
| 1 | Reformer | One Clear sweeps a layer of dead leaves off your hex and all 6 around it (2 actions when your hex is in rain) |
| 2 | Helper | Water or Tend a hex next to you without standing on it |
| 3 | Achiever | After you Sow, the hex grows straight to Sprout |
| 4 | Individualist | When you Explore, the board shows which face-down hexes around you hold something, not what (`Tile.Hint`) |
| 5 | Investigator | Always sees the next Forest Breath; once a round, on their turn, sees one sector's next weather |
| 6 | Loyalist | Plants on your hex and the 6 hexes around you (7 in all) don't lose a stage to dead leaves |
| 7 | Enthusiast | Once a turn, one Move may go 2 hexes (no fog, no sealed hexes) |
| 8 | Challenger | Everyone on your hex moves with you (the Keeper may leave someone behind; nobody is taken off the World Tree unless picked); may enter sealed hexes |
| 9 | Peacemaker | Everyone in your chain (teammates linked to you hex by hex) can pass to a teammate on a neighbouring hex |

## Files

- `game.go`: rules engine (board, tiles, turns and actions, weather, Forest Tide, shares, trust, Secret Owl, finale)
- `content.go`: values, ring decks, Heartwood questions, the harvest prompt, treasures, roles, colours (edit prompts here)
- `main.go`: HTTP API, WebSocket live updates, rejoin codes, QR codes, `/api/cards`, SQLite (`games` holds each game's
  state as JSON; `events` logs every action; `photos` holds the players' photos)
- `photo.go`: photo upload, serving, and the sweep that deletes them after the game
- `admin.go`: the admin page's API (Telegram login code, sessions, games list and actions); `web/admin.html`: the page
- `recording.go`: story recordings, the Keeper's and the transcription worker's API, the audio sweep;
  `ops/transcribe/`: the worker on the Mac mini, its lanes seed and install script
- `find.go`: the sealed find (its file, its API, turning it up); `web/find.js`: how every screen shows it
- `game_test.go`, `photo_test.go`, `sim_test.go`: rules tests, photo tests and the bot balance simulation
- `web/`: `board.html` (Keeper screen), `play.html` (phone), `index.html` (create a game), `howto.html` (rules for
  players), `wiki.html` (one page per term, all content in its script; link with `[[id]]`; card pages filled from
  `/api/cards`), `common.js` (connection, timers, helpers), `style.css`
- `docs/plan/world-tree/`: build contract, approved UI proposal, and the JS rules engine and sim it was designed with
