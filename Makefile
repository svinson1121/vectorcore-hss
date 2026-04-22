.PHONY: build clean deps install uninstall ui dev-ui

BINARY=hss
APP_VERSION=0.4.0B
API_VERSION=1.4.0
VERSION_PKG=github.com/svinson1121/vectorcore-hss/internal/version
GO_LDFLAGS=-X $(VERSION_PKG).AppVersion=$(APP_VERSION) -X $(VERSION_PKG).APIVersion=$(API_VERSION)
PREFIX=/opt/vectorcore
BINDIR=$(PREFIX)/bin
ETCDIR=$(PREFIX)/etc
LOGDIR=$(PREFIX)/log
SYSTEMD=/lib/systemd/system/

build: ui deps
	mkdir -p bin
	go build -ldflags "$(GO_LDFLAGS)" -o bin/$(BINARY) ./cmd/hss

ui: ## Build the React UI (requires Node.js / npm)
	cd web && npm install && npm run build

dev-ui: ## Start Vite dev server (proxies API to localhost:8080)
	cd web && npm install && npm run dev

deps:
	go mod tidy

install: build
	install -d $(BINDIR)
	install -d $(ETCDIR)
	install -d $(LOGDIR)

	install -m755 bin/$(BINARY) $(BINDIR)/$(BINARY)

	if [ ! -f $(ETCDIR)/hss.yaml ]; then \
		install -m644 config/hss.yaml $(ETCDIR)/hss.yaml; \
	fi

	touch $(LOGDIR)/hss.log
	chmod 644 $(LOGDIR)/hss.log

	install -m644 systemd/vectorcore-hss.service $(SYSTEMD)/vectorcore-hss.service

	systemctl daemon-reload
	systemctl enable vectorcore-hss
	systemctl start vectorcore-hss

clean:
	rm -rf bin/

uninstall:
	systemctl stop vectorcore-hss || true
	systemctl disable vectorcore-hss || true

	rm -f $(BINDIR)/$(BINARY)
	rm -f $(SYSTEMD)/vectorcore-hss.service

	systemctl daemon-reload
