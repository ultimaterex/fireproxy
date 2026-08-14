.PHONY: test agent server ui-install

test:
	go test fireproxy/pkg/... fireproxy/agent/... fireproxy/server/...

agent:
	@bash scripts/build-agent.sh || pwsh -File scripts/build-agent.ps1

server:
	cd server && go build -o ../dist/fireproxy-server ./cmd/fireproxy-server

ui-install:
	cd ui && npm install
