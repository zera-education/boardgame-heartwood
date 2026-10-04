# Heartwood: build, test, and deploy to the Axon EC2 (heartwood.zera.edu.my).
SERVER ?= axon
TMP    := /tmp/heartwood-deploy

.PHONY: build test sim run deploy logs

build:
	go build -o heartwood .

test:
	go test ./...

sim:
	HW_SIM=1 go test -run TestBalanceSim -v

run: build
	./heartwood

# Cross-compile here (the server's Go is too old), ship the binary and deploy/, install.
deploy: test
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o deploy/heartwood .
	ssh $(SERVER) 'rm -rf $(TMP) && mkdir -p $(TMP)'
	scp -q deploy/heartwood deploy/install.sh deploy/heartwood.service deploy/nginx.conf $(SERVER):$(TMP)/
	ssh $(SERVER) 'sudo sh $(TMP)/install.sh && rm -rf $(TMP)'
	rm -f deploy/heartwood

logs:
	ssh $(SERVER) 'journalctl -u heartwood -n 100 --no-pager'
