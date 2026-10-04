# Heartwood (digital)

The digital version of the board game in `../design-v0.md`. It runs as a Go server backed by SQLite, with a plain
HTML/JS/CSS frontend embedded in the binary. How to play, for players: `/howto.html`; one page per term: `/wiki.html`.

## Live

**https://heartwood.zera.edu.my**, on the Axon EC2: systemd unit `heartwood` on `127.0.0.1:7779`, binary in
`/opt/heartwood/bin`, SQLite at `/var/lib/heartwood/heartwood.db` (snapshotted to `pre-deploy/` on every deploy),
nginx site `/etc/nginx/sites-available/heartwood.zera.edu.my` (WebSocket upgrade on, `*.zera.edu.my` Origin cert).
DNS is Cloudflare, proxied.

## Run and deploy

```sh
cd ~/lab/heartwood
make run        # build and serve on http://localhost:8490 (data in heartwood.db)
make test       # full 4-player game + playtest rule fixes
make sim        # balance playtest: 4 × 2,000 bot games (~2 min)
make deploy     # test, push to GitHub, then on the EC2: git pull, build, run deploy/install.sh
make logs       # the service log on the EC2
```

Code: **git@github.com:zera-education/boardgame-heartwood.git** (`main`). `make deploy` needs a clean working tree and
an `axon` SSH host (`SERVER=…` to override). The EC2 keeps a clone at `~ubuntu/boardgame-heartwood`, pulls, builds there
(its Go 1.22 downloads the toolchain `go.mod` asks for) and runs `deploy/install.sh`, which snapshots the database first.

Locally, the server prints the address phones on the same Wi-Fi should use, and the board shows it with a QR code.

## Screens

- **`/` → Create a new game**: opens the **board** (`board.html?g=CODE`) for the projector. Whoever creates the game is
  the Keeper; the Keeper controls (Enneagram powers, start, next player, Dusk pairs, next season, tributes, timers) appear only on that
  browser. Under **Help a player rejoin · Keeper on another device** the Keeper can issue a 4-digit rejoin code
  (one use, 10 minutes) or get a link that moves the Keeper controls to another device.
- **Players say it, the Keeper taps it.** Every public action (plant, step, discover, share timer, draw again, lighter,
  pass, powers, Squirrel follow-ups, tribute confirm) is tapped by the Keeper on the board, acting for that player
  (`as` on the action; the server refuses `trust`, `duskChoice` and `guess` from the Keeper). The phone keeps only the
  private jobs: the Secret Owl name, Give trust, the Dusk choice and Bond card, the Secret Owl guess, and a reminder of
  your powers.
- **Players** scan the QR code or open `play.html?g=CODE`. They pick a name and a colour. In the lobby the Keeper gives
  each player **1 to 3 Enneagram types** (the game can't start until everyone has one); each type gives one power
  they can activate once per game. Their seat is kept in the URL (`&p=…`) and on the
  device, so a reload or a reopened browser goes straight back to it.
- Every screen shows a connection badge: **● Live**, **● Reconnecting…** or **● Offline**.

## Realtime

One WebSocket per screen (`/api/games/{code}/live`). After every change the server pushes each viewer their own view
(secrets stay server-side), in order, plus a heartbeat every 20 s. Actions are plain POSTs whose response carries the
actor's new view, so taps feel instant. Clients keep the newest view by version number, reconnect with backoff, treat
45 s without a heartbeat as a dead connection, open a fresh socket after a phone wakes up, and poll slowly while the
socket is down. Screens redraw only the parts that changed.

It used server-sent events before. Browsers allow only six open HTTP/1.1 connections per host, so a board plus five
player tabs on one computer froze every other request. WebSockets don't share that limit (tested with 12).

## Flow

Lobby → plant seeds → 3 seasons (each a round of turns, then Dusk) → Secret Owl guesses → tribute chain → scores.
On a turn the player speaks and the Keeper taps: neighbouring spaces to step → *Stop here and discover* → the prompt
shows on the board → *Start sharing* (timer) → *They're done sharing* → listeners tap *Give trust* on their phones →
*Next player*. A player may draw again once, take a lighter question, or pass the turn (no card, no points). At Dusk, a pair that asked for an alliance but didn't get
one is told why.

## Powers (one per Enneagram type, used once)

Balanced with the simulation: in 10-player games every power wins 8–13% of the time (fair share 10%).

| Type | Power | When | Effect |
|---|---|---|---|
| 1 Reformer | True North | your turn, before sharing | look at 3 cards, choose 1; +1 bonus point |
| 2 Helper | Open Hands | your turn | give anyone 2 ☀ or 2 💧; you both +1 bonus point |
| 3 Achiever | Momentum | while moving | +2 ☀ and +1 💧 |
| 4 Individualist | Deep Water | before sharing | answer the question one tier deeper (up to Oak); it scores at your ring's tier |
| 5 Investigator | Field Notes | while moving | see hidden discoveries next to you (open rings); claim one without moving |
| 6 Loyalist | Rope Team | while moving | jump to an ally's space free; you both +1 bonus point |
| 7 Enthusiast | Adventure | while moving | next 3 steps cost nothing; +1 bonus point |
| 8 Challenger | Champion | after someone else's share | give them 2 trust acorns at once; +1 bonus point |
| 9 Peacemaker | Common Ground | Dusk, before pairs | tonight's alliance works at any distance and costs no water |

## Files

- `game.go`: rules engine (map, turns, discoveries, trust, Dusk, powers, finale, scoring)
- `content.go`: card text and powers (edit prompts here)
- `main.go`: HTTP API, WebSocket live updates, rejoin codes, QR codes, SQLite (`games` holds each game's state as
  JSON; `events` logs every action)
- `game_test.go`, `sim_test.go`: rules tests and the bot balance simulation
- `web/`: `board.html` (projector), `play.html` (phone), `howto.html` (rules for players), `wiki.html` (one page per term, all content in its script; link with `[[id]]`), `common.js` (connection,
  board drawing, timers), `style.css`
