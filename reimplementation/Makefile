GO ?= go
VERSION ?= dev
DIST ?= dist
SOAK_DURATION ?= 4h
BEACON_LDFLAGS = -s -w -buildid= -X main.buildVersion=$(VERSION)

.PHONY: beacon-build beacon-matrix sbom checksums sign verify-fresh verify-reproducible verify-matrix-reproducible integration-binary soak clean

beacon-build:
	mkdir -p $(DIST)
	CGO_ENABLED=0 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/specter-beacon ./cmd/beacon

beacon-matrix:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/specter-beacon-linux-amd64 ./cmd/beacon
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/specter-beacon-linux-arm64 ./cmd/beacon
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/specter-beacon-windows-amd64.exe ./cmd/beacon
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/specter-beacon-darwin-arm64 ./cmd/beacon

sbom:
	mkdir -p $(DIST)
	python3 tools/generate_sbom.py --version $(VERSION) --output $(DIST)/specter-beacon.spdx.json

checksums:
	cd $(DIST) && sha256sum specter-beacon* > SHA256SUMS

sign:
	test -n "$(COSIGN_PRIVATE_KEY)"
	cosign sign-blob --yes --key env://COSIGN_PRIVATE_KEY --output-signature $(DIST)/SHA256SUMS.sig $(DIST)/SHA256SUMS

verify-fresh: beacon-build
	$(DIST)/specter-beacon -h >/dev/null

verify-reproducible:
	mkdir -p $(DIST)
	CGO_ENABLED=0 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/.repro-a ./cmd/beacon
	CGO_ENABLED=0 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/.repro-b ./cmd/beacon
	cmp $(DIST)/.repro-a $(DIST)/.repro-b
	find $(DIST) -maxdepth 1 -type f -name '.repro-*' -delete

verify-matrix-reproducible:
	mkdir -p $(DIST)/.matrix-a $(DIST)/.matrix-b
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/.matrix-a/specter-beacon-linux-amd64 ./cmd/beacon
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/.matrix-b/specter-beacon-linux-amd64 ./cmd/beacon
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/.matrix-a/specter-beacon-linux-arm64 ./cmd/beacon
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/.matrix-b/specter-beacon-linux-arm64 ./cmd/beacon
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/.matrix-a/specter-beacon-windows-amd64.exe ./cmd/beacon
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/.matrix-b/specter-beacon-windows-amd64.exe ./cmd/beacon
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/.matrix-a/specter-beacon-darwin-arm64 ./cmd/beacon
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -buildvcs=false -ldflags "$(BEACON_LDFLAGS)" -o $(DIST)/.matrix-b/specter-beacon-darwin-arm64 ./cmd/beacon
	cmp $(DIST)/.matrix-a/specter-beacon-linux-amd64 $(DIST)/.matrix-b/specter-beacon-linux-amd64
	cmp $(DIST)/.matrix-a/specter-beacon-linux-arm64 $(DIST)/.matrix-b/specter-beacon-linux-arm64
	cmp $(DIST)/.matrix-a/specter-beacon-windows-amd64.exe $(DIST)/.matrix-b/specter-beacon-windows-amd64.exe
	cmp $(DIST)/.matrix-a/specter-beacon-darwin-arm64 $(DIST)/.matrix-b/specter-beacon-darwin-arm64
	find $(DIST)/.matrix-a $(DIST)/.matrix-b -type f -delete
	rmdir $(DIST)/.matrix-a $(DIST)/.matrix-b

integration-binary: beacon-build
	$(GO) run ./tests/beacon_binary_integration.go -binary $(DIST)/specter-beacon -version $(VERSION)

soak:
	SPECTER_TEST_TIMEOUT=0 docker compose --profile test run --build --rm -e SPECTER_SOAK_DURATION=$(SOAK_DURATION) gateway-test

clean:
	find $(DIST) -maxdepth 1 -type f -name 'specter-beacon*' -delete
	find $(DIST) -maxdepth 1 -type f -name 'SHA256SUMS*' -delete
