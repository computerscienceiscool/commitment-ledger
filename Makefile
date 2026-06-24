SHELL := /bin/bash
.ONESHELL:

GO ?= go
APP := commitment-ledger
BIN_DIR ?= bin
BIN := $(BIN_DIR)/$(APP)
CONFIG ?= config/repos.json
DEMO_CONFIG ?= config/repos.demo.json
DEMO_ROOT ?= $(HOME)/lab/commitment-ledger-demo
GOCACHE ?= /tmp/gocache
RUN := GOCACHE=$(GOCACHE) $(GO) run ./cmd/commitment-ledger
VERSION ?= v0.1.0
SIGNER ?= commitment-ledger
ARGS ?=
STATUS_ARGS ?=
REPORT_ARGS ?=
COMMIT_ARGS ?=
EVIDENCE_ARGS ?=
ASSESS_ARGS ?=
INSPECT_ARGS ?=
VERIFY_ARGS ?=
EXPORT_ARGS ?=
IMPORT_ARGS ?=
PROVENANCE_ARGS ?=
SEND_ARGS ?=
RECEIVE_ARGS ?=
DOCTOR_ARGS ?=
REPAIR_ARGS ?=
IDENTITY_ARGS ?=
TODO_REPOS := alice-demo bob-demo dave-demo mallory-demo

.PHONY: help fmt test build check clean cli scan status report inspect verify export import provenance send receive doctor repair identity conformance conformance-update expire commit evidence assess \
	demo-init demo-seed demo-config demo-setup demo-scan demo-status demo-report

help:
	@echo "Commitment Ledger make targets"
	@echo
	@echo "Core development:"
	@echo "  make fmt"
	@echo "  make test"
	@echo "  make build"
	@echo "  make check"
	@echo "  make clean"
	@echo
	@echo "CLI wrappers:"
	@echo "  make cli ARGS='status'"
	@echo "  make scan CONFIG=config/repos.json"
	@echo "  make status STATUS_ARGS='--exchange --json'"
	@echo "  make report REPORT_ARGS='--promiser Alice'"
	@echo "  make report REPORT_ARGS='--imports --json'"
	@echo "  make inspect INSPECT_ARGS='--json COMMITMENT-...'"
	@echo "  make verify VERIFY_ARGS='--json COMMITMENT-...'"
	@echo "  make export EXPORT_ARGS='--out /tmp/bundle.json COMMITMENT-...'"
	@echo "  make import IMPORT_ARGS='--in /tmp/bundle.json'"
	@echo "  make provenance PROVENANCE_ARGS='--mode receive --receipt-signer commitment-ledger --json'"
	@echo "  make send SEND_ARGS='--outbox /tmp/peer-outbox COMMITMENT-...'"
	@echo "  make receive RECEIVE_ARGS='--inbox /tmp/peer-inbox --archive /tmp/peer-archive'"
	@echo "  make doctor DOCTOR_ARGS='--repairable --strict'"
	@echo "  make repair REPAIR_ARGS='--json --records --protocol-cas --import-artifacts --import-support --identity-lineage'"
	@echo "  make identity IDENTITY_ARGS='restore --in /tmp/identities.json Alice'"
	@echo "  make conformance VERSION=$(VERSION) SIGNER=$(SIGNER)"
	@echo "  make conformance-update VERSION=$(VERSION) SIGNER=$(SIGNER)"
	@echo "  make expire"
	@echo "  make commit COMMIT_ARGS='--promiser Alice --repo alice-demo --branch main --target alice-demo/main/TODO-ravud/1 --due 2026-07-01 --promise ...'"
	@echo "  make evidence EVIDENCE_ARGS='--commitment COMMITMENT-... --type manual_note --notes ...'"
	@echo "  make assess ASSESS_ARGS='--commitment COMMITMENT-... --assessor Alice --status kept --basis EVIDENCE-...'"
	@echo
	@echo "Demo helpers:"
	@echo "  make demo-setup"
	@echo "  make demo-scan"
	@echo "  make demo-status"
	@echo "  make demo-report REPORT_ARGS='--promiser Alice'"

fmt:
	@$(GO) fmt ./...

test:
	@$(GO) test ./...

build:
	@mkdir -p $(BIN_DIR)
	@$(GO) build -o $(BIN) ./cmd/commitment-ledger
	@echo "Built $(BIN)"

check: fmt test build

clean:
	@rm -f $(BIN)

cli:
	@$(RUN) $(ARGS)

scan:
	@$(RUN) scan --config $(CONFIG)

status:
	@$(RUN) status $(STATUS_ARGS)

report:
	@$(RUN) report $(REPORT_ARGS)

inspect:
	@$(RUN) inspect $(INSPECT_ARGS)

verify:
	@$(RUN) verify $(VERIFY_ARGS)

export:
	@$(RUN) export $(EXPORT_ARGS)

import:
	@$(RUN) import $(IMPORT_ARGS)

provenance:
	@$(RUN) provenance $(PROVENANCE_ARGS)

send:
	@$(RUN) send $(SEND_ARGS)

receive:
	@$(RUN) receive $(RECEIVE_ARGS)

doctor:
	@$(RUN) doctor $(DOCTOR_ARGS)

repair:
	@$(RUN) repair $(REPAIR_ARGS)

identity:
	@$(RUN) identity $(IDENTITY_ARGS)

conformance:
	@$(RUN) conformance --signer $(SIGNER) --version $(VERSION)

conformance-update:
	@$(RUN) conformance --signer $(SIGNER) --version $(VERSION) --write-changelog

expire:
	@$(RUN) expire

commit:
	@$(RUN) commit $(COMMIT_ARGS)

evidence:
	@$(RUN) evidence $(EVIDENCE_ARGS)

assess:
	@$(RUN) assess $(ASSESS_ARGS)

demo-init:
	@set -eu; \
	mkdir -p "$(DEMO_ROOT)"; \
	for repo in $(TODO_REPOS); do \
		mkdir -p "$(DEMO_ROOT)/$$repo"; \
		if [ ! -d "$(DEMO_ROOT)/$$repo/.git" ]; then \
			git -C "$(DEMO_ROOT)/$$repo" init -b main >/dev/null; \
			git -C "$(DEMO_ROOT)/$$repo" config user.name "Demo User"; \
			git -C "$(DEMO_ROOT)/$$repo" config user.email "demo@example.com"; \
		fi; \
		mkdir -p "$(DEMO_ROOT)/$$repo/TODO"; \
	done; \
	git -C "$(DEMO_ROOT)/dave-demo" checkout -B repair >/dev/null; \
	git -C "$(DEMO_ROOT)/mallory-demo" checkout -B jj >/dev/null; \
	echo "Initialized demo repos under $(DEMO_ROOT)"

demo-seed: demo-init
	@printf '%s\n' \
		'# TODO Index' \
		'' \
		'- [ ] TODO-ravud - Ship welcome flow (`TODO/TODO-ravud-ship-welcome-flow.md`)' \
		'- [ ] TODO-lomik - Write persistence note (`TODO/TODO-lomik-write-persistence-note.md`)' \
		>"$(DEMO_ROOT)/alice-demo/TODO/TODO.md"
	@printf '%s\n' \
		'# TODO-ravud: Ship welcome flow' \
		'' \
		'- [ ] 1. Add route' \
		'- [ ] 2. Add tests' \
		'- [ ] 3. Add docs' \
		>"$(DEMO_ROOT)/alice-demo/TODO/TODO-ravud-ship-welcome-flow.md"
	@printf '%s\n' \
		'# TODO-lomik: Write persistence note' \
		'' \
		'- [ ] 1. Describe storage layout' \
		>"$(DEMO_ROOT)/alice-demo/TODO/TODO-lomik-write-persistence-note.md"
	@printf '%s\n' \
		'# TODO Index' \
		'' \
		'- [ ] TODO-muban - Stabilize sync worker (`TODO/TODO-muban-stabilize-sync-worker.md`)' \
		>"$(DEMO_ROOT)/bob-demo/TODO/TODO.md"
	@printf '%s\n' \
		'# TODO-muban: Stabilize sync worker' \
		'' \
		'- [ ] 1. Handle retries' \
		'- [ ] 2. Add metrics' \
		>"$(DEMO_ROOT)/bob-demo/TODO/TODO-muban-stabilize-sync-worker.md"
	@printf '%s\n' \
		'# TODO Index' \
		'' \
		'- [ ] TODO-sivud - Repair Bob sync worker (`TODO/TODO-sivud-repair-bob-sync-worker.md`)' \
		>"$(DEMO_ROOT)/dave-demo/TODO/TODO.md"
	@printf '%s\n' \
		'# TODO-sivud: Repair Bob sync worker' \
		'' \
		'- [ ] 1. Add fallback path' \
		'- [ ] 2. Document repair' \
		>"$(DEMO_ROOT)/dave-demo/TODO/TODO-sivud-repair-bob-sync-worker.md"
	@printf '%s\n' \
		'# TODO Index' \
		'' \
		'- [ ] TODO-falun - Handle malformed packet report (`TODO/TODO-falun-handle-malformed-packet-report.md`)' \
		>"$(DEMO_ROOT)/mallory-demo/TODO/TODO.md"
	@printf '%s\n' \
		'# TODO-falun: Handle malformed packet report' \
		'' \
		'- [ ] 1. Add parser note' \
		'- [ ] two. Confusing numbering on purpose' \
		'- [x] 3. Misleading completed checkbox' \
		>"$(DEMO_ROOT)/mallory-demo/TODO/TODO-falun-handle-malformed-packet-report.md"
	@for repo in $(TODO_REPOS); do \
		git -C "$(DEMO_ROOT)/$$repo" add TODO >/dev/null; \
		if ! git -C "$(DEMO_ROOT)/$$repo" diff --cached --quiet; then \
			git -C "$(DEMO_ROOT)/$$repo" commit -m "Seed demo TODOs" >/dev/null; \
		fi; \
	done
	@echo "Seeded demo TODO content under $(DEMO_ROOT)"

demo-config:
	@mkdir -p "$$(dirname "$(DEMO_CONFIG)")"
	@cat >"$(DEMO_CONFIG)" <<-EOF
	{
	  "repos": [
	    {
	      "name": "alice-demo",
	      "provider": "local",
	      "url": "",
	      "local_path": "$(DEMO_ROOT)/alice-demo",
	      "branch": "main",
	      "todo_file": "TODO/TODO.md",
	      "enabled": true
	    },
	    {
	      "name": "bob-demo",
	      "provider": "local",
	      "url": "",
	      "local_path": "$(DEMO_ROOT)/bob-demo",
	      "branch": "main",
	      "todo_file": "TODO/TODO.md",
	      "enabled": true
	    },
	    {
	      "name": "dave-demo",
	      "provider": "local",
	      "url": "",
	      "local_path": "$(DEMO_ROOT)/dave-demo",
	      "branch": "repair",
	      "todo_file": "TODO/TODO.md",
	      "enabled": true
	    },
	    {
	      "name": "mallory-demo",
	      "provider": "local",
	      "url": "",
	      "local_path": "$(DEMO_ROOT)/mallory-demo",
	      "branch": "jj",
	      "todo_file": "TODO/TODO.md",
	      "enabled": true
	    }
	  ]
	}
	EOF
	@echo "Wrote $(DEMO_CONFIG)"

demo-setup: demo-seed demo-config
	@echo "Demo setup complete. Run 'make demo-scan' next."

demo-scan:
	@$(MAKE) scan CONFIG=$(DEMO_CONFIG)

demo-status:
	@$(MAKE) status

demo-report:
	@$(MAKE) report REPORT_ARGS="$(REPORT_ARGS)"
