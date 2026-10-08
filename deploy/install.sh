#!/bin/sh
# Install or update Heartwood on the Axon EC2 (Ubuntu). `make deploy` pulls the repo on the server,
# builds deploy/heartwood there and runs: sudo sh deploy/install.sh
# Re-running it snapshots the database, replaces the binary, the unit and the nginx site, and
# restarts the service. Settings below are the single source of truth for the unit and the site.
set -e
cd "$(dirname "$0")"

ADDR=127.0.0.1:7779
SITE=heartwood.zera.edu.my
BIN=/opt/heartwood/bin
DATA=/var/lib/heartwood
SNAPSHOTS=$DATA/pre-deploy
KEEP_SNAPSHOTS=10
AXON_ENV=/home/ubuntu/.axon/.env   # the Axon bot: TELEGRAM_BOT_TOKEN and El's TELEGRAM_OWNER_ID (admin login codes)
ENV_FILE=/etc/heartwood/heartwood.env

render() { sed -e "s|@ADDR@|$ADDR|g" -e "s|@SITE@|$SITE|g" -e "s|@BIN@|$BIN|g" -e "s|@DATA@|$DATA|g" "$1"; }

[ -f ./heartwood ] || { echo "no ./heartwood binary beside install.sh (run make deploy)"; exit 1; }

id -u heartwood >/dev/null 2>&1 || useradd -r -s /usr/sbin/nologin -d "$DATA" heartwood
mkdir -p "$BIN" "$DATA" "$SNAPSHOTS"
chown -R heartwood:heartwood "$DATA"
chmod 750 "$DATA"

# Copy the live database before touching it (the service is stopped, so WAL is checkpointed).
if [ -f "$DATA/heartwood.db" ]; then
  systemctl stop heartwood >/dev/null 2>&1 || true
  cp "$DATA/heartwood.db" "$SNAPSHOTS/pre-deploy-$(date -u +%Y%m%dT%H%M%SZ).db"
  ls -1t "$SNAPSHOTS"/pre-deploy-*.db | tail -n +$((KEEP_SNAPSHOTS + 1)) | xargs -r rm -f
fi

# The admin's login codes go to El's Telegram through the Axon bot (as blessed does). Copy only those two
# values out of ~/.axon/.env on every deploy, so a rotated token reaches the service and it never sees
# the rest of that file. Without them the game runs as before and the admin login says it isn't set up.
token=$(sed -n 's/^TELEGRAM_BOT_TOKEN=//p' "$AXON_ENV" 2>/dev/null | tr -d "\"'\r" | head -n1)
owner=$(sed -n 's/^TELEGRAM_OWNER_ID=//p' "$AXON_ENV" 2>/dev/null | tr -d "\"'\r" | head -n1)
if [ -n "$token" ] && [ -n "$owner" ]; then
  mkdir -p "$(dirname "$ENV_FILE")"
  umask 077
  printf 'HEARTWOOD_TG_TOKEN=%s\nHEARTWOOD_TG_CHAT=%s\n' "$token" "$owner" > "$ENV_FILE.new"
  umask 022
  mv -f "$ENV_FILE.new" "$ENV_FILE"
else
  echo "TELEGRAM_BOT_TOKEN or TELEGRAM_OWNER_ID missing from $AXON_ENV: the admin login can't send codes"
fi

cp ./heartwood "$BIN/heartwood.new"
chmod 755 "$BIN/heartwood.new"
mv -f "$BIN/heartwood.new" "$BIN/heartwood"

render heartwood.service > /etc/systemd/system/heartwood.service
systemctl daemon-reload
systemctl enable heartwood >/dev/null 2>&1
systemctl restart heartwood

tries=0
until curl -fsS "http://$ADDR/healthz" >/dev/null 2>&1; do
  tries=$((tries + 1))
  if [ "$tries" -ge 20 ]; then
    echo "heartwood did not become healthy; nginx left unchanged"
    journalctl -u heartwood -n 30 --no-pager
    exit 1
  fi
  sleep 0.5
done

render nginx.conf > "/etc/nginx/sites-available/$SITE"
ln -sf "/etc/nginx/sites-available/$SITE" "/etc/nginx/sites-enabled/$SITE"
nginx -t
systemctl reload nginx

echo "heartwood is serving https://$SITE (journalctl -u heartwood -f for logs)"
