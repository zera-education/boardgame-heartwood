# Heartwood: build, test, and deploy to the Axon EC2 (heartwood.zera.edu.my).
# Source: git@github.com:zera-education/boardgame-heartwood.git, cloned on the EC2 at ~/boardgame-heartwood.
SERVER ?= axon
SRC    := ~/boardgame-heartwood

.PHONY: build test sim run deploy logs

build:
	go build -o heartwood .

test:
	go test ./...

sim:
	HW_SIM=1 go test -run TestBalanceSim -v

run: build
	./heartwood

# Push to GitHub, then on the EC2: pull, build (Go fetches the toolchain go.mod asks for), install.
deploy: test
	git diff --quiet HEAD || { echo "commit your changes first"; exit 1; }
	git push origin main
	ssh $(SERVER) 'set -e; cd $(SRC); git pull --ff-only; GOTOOLCHAIN=auto CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o deploy/heartwood .; sudo sh deploy/install.sh; rm -f deploy/heartwood'

logs:
	ssh $(SERVER) 'journalctl -u heartwood -n 100 --no-pager'
