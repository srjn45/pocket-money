# ============================================================================
# Pocket Money — root Makefile (ops: database backups). See docs/specs/WP-1.4.md
# ============================================================================
.DEFAULT_GOAL := help

COMPOSE     ?= docker compose            # override e.g. `podman compose`
PG_SERVICE  ?= postgres                  # compose service name (docker-compose.yml)
DB_USER     ?= pocket                    # POSTGRES_USER
DB_NAME     ?= pocket_money              # POSTGRES_DB
BACKUP_DIR  ?= backups
VERIFY_DB   ?= pocket_money_verify       # throwaway db used by backup-verify
KEEP        ?= 14                        # nightly backups to retain
STAMP       := $(shell date +%Y%m%d_%H%M%S)
DUMP        := $(BACKUP_DIR)/pocket_money_$(STAMP).dump

.PHONY: help backup restore backup-verify backup-prune seed demo-up

SEED_DATABASE_URL ?= postgres://pocket:pocket@localhost:5432/pocket_money_dev?sslmode=disable
SEED_JWT_SECRET   ?= dev-secret-key
APP_BASE_URL      ?= http://localhost:8081

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-16s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

seed: ## Seed demo family into the dev database (SEED_DATABASE_URL)
	cd backend && \
	DATABASE_URL="$(SEED_DATABASE_URL)" \
	JWT_SECRET="$(SEED_JWT_SECRET)" \
	APP_BASE_URL="$(APP_BASE_URL)" \
	go run ./cmd/seed --reset

demo-up: seed ## Seed + boot the backend in the background (useful for local E2E)
	cd backend && \
	DATABASE_URL="$(SEED_DATABASE_URL)" \
	JWT_SECRET="$(SEED_JWT_SECRET)" \
	APP_BASE_URL="$(APP_BASE_URL)" \
	go run ./cmd/server &

backup: ## Dump the live DB to backups/ (custom format), then prune old dumps
	@mkdir -p $(BACKUP_DIR)
	@echo "Backing up '$(DB_NAME)' -> $(DUMP)"
	@if $(COMPOSE) exec -T $(PG_SERVICE) pg_dump -U $(DB_USER) -d $(DB_NAME) -Fc > $(DUMP).tmp; then \
		mv $(DUMP).tmp $(DUMP); \
		echo "OK ($$(du -h $(DUMP) | cut -f1)): $(DUMP)"; \
	else \
		rm -f $(DUMP).tmp; \
		echo "BACKUP FAILED — no dump written" >&2; \
		exit 1; \
	fi
	@$(MAKE) backup-prune

backup-prune: ## Delete all but the newest KEEP (=$(KEEP)) dumps
	@cd $(BACKUP_DIR) 2>/dev/null && \
		ls -1t pocket_money_*.dump 2>/dev/null | tail -n +$$(( $(KEEP) + 1 )) | xargs -r rm -f -- ; \
		echo "Retained newest $(KEEP) dump(s) in $(BACKUP_DIR)/"

restore: ## Restore a dump into the LIVE db. Usage: make restore BACKUP=backups/<file>.dump [FORCE=1]
	@test -n "$(BACKUP)" || { echo "Usage: make restore BACKUP=<file> [FORCE=1]" >&2; exit 1; }
	@test -f "$(BACKUP)" || { echo "No such file: $(BACKUP)" >&2; exit 1; }
	@if [ "$(FORCE)" != "1" ]; then \
		echo "WARNING: this DROPS and recreates objects in the LIVE db '$(DB_NAME)' from '$(BACKUP)'."; \
		printf "Type 'yes' to continue: "; read ans; [ "$$ans" = "yes" ] || { echo "Aborted."; exit 1; }; \
	fi
	@if $(COMPOSE) exec -T $(PG_SERVICE) pg_restore -U $(DB_USER) -d $(DB_NAME) \
			--clean --if-exists --no-owner --exit-on-error < "$(BACKUP)"; then \
		echo "Restore complete into '$(DB_NAME)'."; \
	else \
		echo "RESTORE FAILED" >&2; exit 1; \
	fi

backup-verify: ## Restore a dump into a throwaway db and sanity-check it (live db untouched)
	@f="$(BACKUP)"; \
	 if [ -z "$$f" ]; then f=$$(ls -1t $(BACKUP_DIR)/pocket_money_*.dump 2>/dev/null | head -1); fi; \
	 { test -n "$$f" && test -f "$$f"; } || { echo "No dump to verify (set BACKUP=<file>)" >&2; exit 1; }; \
	 echo "Verifying '$$f' via scratch db '$(VERIFY_DB)'"; \
	 $(COMPOSE) exec -T $(PG_SERVICE) psql -U $(DB_USER) -d postgres -v ON_ERROR_STOP=1 \
		-c "DROP DATABASE IF EXISTS $(VERIFY_DB);" -c "CREATE DATABASE $(VERIFY_DB);" \
		|| { echo "VERIFY FAILED: could not create scratch db" >&2; exit 1; }; \
	 if $(COMPOSE) exec -T $(PG_SERVICE) pg_restore -U $(DB_USER) -d $(VERIFY_DB) \
			--no-owner --exit-on-error < "$$f"; then \
		n=$$($(COMPOSE) exec -T $(PG_SERVICE) psql -U $(DB_USER) -d $(VERIFY_DB) -tAc \
			"SELECT count(*) FROM information_schema.tables WHERE table_schema='public';" | tr -d '[:space:]'); \
		$(COMPOSE) exec -T $(PG_SERVICE) psql -U $(DB_USER) -d postgres \
			-c "DROP DATABASE IF EXISTS $(VERIFY_DB);" >/dev/null; \
		[ "$$n" -gt 0 ] 2>/dev/null || { echo "VERIFY FAILED: restored 0 public tables" >&2; exit 1; }; \
		echo "VERIFY OK: restored cleanly, $$n public table(s)."; \
	 else \
		$(COMPOSE) exec -T $(PG_SERVICE) psql -U $(DB_USER) -d postgres \
			-c "DROP DATABASE IF EXISTS $(VERIFY_DB);" >/dev/null; \
		echo "VERIFY FAILED: pg_restore errored" >&2; exit 1; \
	 fi
