#!/bin/sh
# Install or update Heartwood on the Axon EC2 (Ubuntu). `make deploy` copies the linux binary and
# this folder to the server and runs: sudo sh install.sh
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
