# WP-1.4 Spec: Database Backups

> Roadmap ref: master-plan §9 Phase 1, WP-1.4 (∥); §10 DoD. Scope: **ops/tooling only** —
> a new **root `Makefile`**, a `.gitignore` entry, a tracked `backups/.gitkeep`, and a README
> section. **No Go, no migrations, no openapi, no app/ changes.** This WP touches the database
> only through `pg_dump`/`pg_restore` run *inside* the existing Postgres container; it does not
> change the schema or any application behavior.

## 0. Why this WP exists (context)

The backend runs on a home/LAN machine and Postgres data lives in a single Docker named volume
(`postgres_data`, `docker-compose.yml:42`). That volume is the **only** copy of the family's
money ledger. A dead disk, a `docker volume rm`, or a bad `docker compose down -v` loses
everything with no recovery. This WP gives the operator a one-command, verified, rotating
backup so disk death is a nuisance, not a catastrophe. It is deliberately delivered in Phase 1
(long before any cloud deploy) because the risk exists the moment real family data is entered.

## 1. Files in scope

| File | Change | Notes |
|---|---|---|
| `Makefile` (repo **root**, new) | add `backup`, `restore`, `backup-verify`, `backup-prune` (+ `help`) | new file; see §3 for exact contents |
| `.gitignore` | ignore dumps, keep the dir | see §2 |
| `backups/.gitkeep` (new, empty) | tracked placeholder so `backups/` exists on clone | 0 bytes |
| `README.md` | add a "Backups & Restore" section | see §6 |

Do **not** modify `backend/Makefile`, `docker-compose.yml`, or any code. The backend already
runs migrations on startup, so a restored DB needs no extra migrate step.

## 2. Design decisions (and the alternatives rejected)

### 2.1 Where backups live — a **root** `Makefile`, not `backend/Makefile`
The plan calls for `make backup` / `make restore` (bare, run from the repo root). Backups are a
*deployment* concern that operates on the **root** `docker-compose.yml` Postgres (service
`postgres`, container `pocket_money_db`), not a backend build concern. `backend/Makefile` is
about Go build/test/lint and already reaches *up* to `../docker-compose.yml` for its dev DB.
Putting backup targets in a new **root `Makefile`** means the documented command is exactly
`make backup` (as the plan says), it sits next to the compose file it operates on, and it
doesn't pollute the Go makefile. Rejected: adding to `backend/Makefile` (would force
`make -C backend backup` and misplace an ops concern inside the build tool).

### 2.2 Dump format — **custom (`-Fc`)**, not plain SQL
Use `pg_dump -Fc` (PostgreSQL custom archive). Reasons:
- **Compressed** by default (zlib) → small nightly files, cheap to keep 14 of them.
- **Restorable with `pg_restore --clean --if-exists`**, which drops and recreates objects
  transactionally-ish and lets us restore into either the live DB or a scratch DB from the
  *same* artifact — this is what makes `backup-verify` (§5) trivial.
- Supports `--no-owner` and selective/ordered restore, and `pg_restore -l`/`-e` for inspection
  and fail-fast.

Rejected: **plain SQL (`-Fp` → `psql`)**. It is human-diffable but uncompressed (larger),
restores only via `psql` (no `--clean --if-exists` object handling, no fail-fast archive), and
gives no verification affordances. A money ledger doesn't need to be eyeballed as SQL; it needs
to restore reliably. (If an operator ever wants a readable dump, `pg_dump -Fp` is a one-off;
we don't ship a target for it.)

### 2.3 Run tools **inside the container**, not on the host
All `pg_dump`/`pg_restore`/`psql` run via `docker compose exec -T postgres …`. This (a) requires
no Postgres client tools on the host, and (b) **guarantees the client major version matches the
server** (`postgres:15-alpine`) — a host `pg_dump 16` against a v15 server, or vice-versa, can
error. `-T` disables TTY allocation so stdin/stdout redirection works under `cron` and in CI.

### 2.4 No silent empty dumps (fail loudly)
The naive `docker compose exec -T postgres pg_dump … > out.dump` **creates/truncates `out.dump`
before `pg_dump` runs**, so a failure (DB down, wrong creds) leaves a truncated or empty file
that looks like a backup. The `backup` target therefore writes to a `*.dump.tmp`, and **only
`mv`s it to the final `*.dump` name on success**; on any failure it removes the temp file and
exits non-zero. So a listed `*.dump` is always a completed dump, and the target's exit code is
honest (cron/CI can alert on it).

### 2.5 Restore is guarded; verify is isolated
`restore` targets the **live** DB and is destructive, so it **prompts for confirmation** (typed
`yes`) unless `FORCE=1`. `backup-verify` never touches the live DB — it restores into a
throwaway `pocket_money_verify` database in the same container, sanity-checks it, and drops it.

### 2.6 Parameterized, overridable
Credentials/paths are `make` variables with defaults matching `docker-compose.yml`
(`DB_USER=pocket`, `DB_NAME=pocket_money`, service `postgres`). An operator with different
production creds overrides on the command line (`make backup DB_USER=… DB_NAME=…`) or via
environment. Defaults are the dev values, so nothing extra is needed for the common case.

## 3. Makefile targets (exact)

Create `Makefile` at the repo root with the following. Indentation inside recipes **must be
tabs**. (`?=` lets every variable be overridden by env or command line.)

```makefile
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

.PHONY: help backup restore backup-verify backup-prune

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-16s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

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
```

Notes for the implementer:
- Each recipe **line** runs in its own shell, so any multi-step logic (the `if`/`read`/loops) is
  written as **one** logical line with `\` continuations and `;` — do not split it into separate
  `@` lines.
- Keep `-T` on every `exec` (redirection under cron/CI). Keep `--exit-on-error` on both
  `pg_restore` calls (fail-fast, honest exit code).
- `du`/`mv`/`ls`/`xargs` run on the **host** against the redirected file — that's intended; only
  `pg_dump`/`pg_restore`/`psql` run in the container.
- `xargs -r` (GNU) skips running `rm` when there's nothing to prune. On BusyBox/macOS `-r` may be
  absent; the surrounding `ls … | tail` yields empty input harmlessly, but if targeting macOS the
  implementer may drop `-r`. Home server is Linux, so `-r` stays.

## 4. Cron + rotation

Rotation is **built into `backup`** (it calls `backup-prune`, keeping the newest `KEEP=14`), so
the cron line is just the backup itself. `cron` runs with a minimal `PATH`, so give `make` (and
via it `docker`) an explicit `PATH`, and set an absolute repo path. Nightly at 02:00:

```cron
# m h dom mon dow   nightly Pocket Money DB backup (keeps newest 14), logs to backups/backup.log
0 2 * * *  cd /home/<user>/pocket-money && PATH=/usr/local/bin:/usr/bin:/bin /usr/bin/make backup >> /home/<user>/pocket-money/backups/backup.log 2>&1
```

- Replace `/home/<user>/pocket-money` with the real deploy path; confirm `which make` / `which
  docker` match the `PATH` above.
- Because `backup` exits non-zero on failure and the redirect keeps no empty dump, a failed night
  is visible in `backup.log` (and to any cron-mail/monitor) rather than a silent empty file.
- Retention knob: `KEEP=30 make backup` (or edit the cron line) keeps 30. Files are timestamped
  `pocket_money_YYYYmmdd_HHMMSS.dump`, so lexical == chronological order and `ls -1t` prunes the
  oldest.
- Optional weekly verify (belt-and-suspenders, alerts if the newest dump won't restore):
  ```cron
  30 2 * * 0  cd /home/<user>/pocket-money && PATH=/usr/local/bin:/usr/bin:/bin /usr/bin/make backup-verify >> /home/<user>/pocket-money/backups/verify.log 2>&1
  ```

## 5. Verification procedure (proves restorability — required by acceptance)

Automated path (the deliverable), with the stack up (`docker compose up -d`) and at least one
group/chore/ledger entry created so tables have rows:

1. `make backup` → prints `OK (<size>): backups/pocket_money_<stamp>.dump`; the file exists and
   is non-empty; no `.tmp` remains.
2. `make backup-verify` → restores the newest dump into scratch `pocket_money_verify`, prints
   `VERIFY OK: restored cleanly, N public table(s).` with `N > 0`, then drops the scratch db.
   The **live** `pocket_money` db is untouched (confirm the app still reads normally).
3. Full round-trip against a scratch DB (manual, documents the restore path without risking live
   data): `make restore BACKUP=backups/pocket_money_<stamp>.dump DB_NAME=pocket_money_verify
   FORCE=1` after creating that db — restores without error. (For the DoD, step 2 already proves
   restorability into a scratch DB; this step demonstrates the `restore` target's flags.)

Failure-mode checks (prove no silent empty dumps / honest exit codes):

4. Stop the DB (`docker compose stop postgres`) then `make backup` → prints `BACKUP FAILED`, exits
   non-zero (`echo $?` → non-zero), and **no** new `*.dump` or `*.dump.tmp` is left behind.
   Restart with `docker compose start postgres`.
5. `make backup-verify BACKUP=/dev/null` (or a truncated file) → prints `VERIFY FAILED`, exits
   non-zero, and leaves no `pocket_money_verify` db behind.

Rotation check:

6. Create 16 dummy `backups/pocket_money_*.dump` files (varying timestamps) and run
   `make backup-prune` → only the newest 14 remain.

State in the PR exactly which of the above you ran and their output.

## 6. README delta

Add a top-level **"Backups & Restore"** section (e.g. after "Database Migrations"), covering:

- One-liner intent: the DB volume is the only copy of the ledger; back it up.
- `make backup` — creates a compressed, timestamped dump in `./backups/` and prunes to the
  newest `KEEP` (default 14). Runs `pg_dump` inside the `postgres` container.
- `make backup-verify [BACKUP=…]` — proves the newest (or given) dump restores, using a throwaway
  db; leaves the live db untouched.
- `make restore BACKUP=backups/<file>.dump` — **destructive**, restores into the live db; prompts
  for `yes` (bypass with `FORCE=1`).
- The cron one-liner from §4 (nightly, keeps 14), with the note to fix the absolute path and
  `PATH`.
- A one-line note that `./backups/*.dump` is gitignored (dumps contain family financial data —
  never commit them) and that credential overrides are `make backup DB_USER=… DB_NAME=…`.

## 7. .gitignore + placeholder

Add to `.gitignore` (dumps are private financial data and can be large — never commit them, but
keep the directory on clone):

```gitignore
# Database backups (WP-1.4) — family financial data, never commit dumps
/backups/*.dump
/backups/*.dump.tmp
/backups/*.log
```

Add an empty tracked file `backups/.gitkeep` so `./backups/` exists after clone (the `backup`
target also `mkdir -p`s it, so this is belt-and-suspenders and gives `.gitignore` a directory to
sit next to). Do **not** add a broad `/backups/` ignore that would also drop `.gitkeep`.

## 8. Out of scope

- **Off-machine / offsite copies** (rsync to NAS, cloud object storage, encryption at rest).
  This WP makes local, verified dumps; shipping them elsewhere is a later ops task and belongs
  with cloud deploy (§8 backlog #10). Note it in the README as a follow-up, don't build it.
- **Point-in-time recovery / WAL archiving.** Overkill for a single-family home server; nightly
  logical dumps are the right altitude.
- **Backing up the app or uploaded assets** — there are none; all durable state is in Postgres.
- **Any schema, API, Go, or app/ change.** Migrations still run on server start, so a restored DB
  needs no extra step from this WP.
- **A `make seed` / demo-data target** — that's WP-4.4.

## 9. Acceptance criteria

- [ ] Root `Makefile` exists with `backup`, `restore`, `backup-verify`, `backup-prune`, `help`,
      matching §3 (tabs, `-T` on every `exec`, `--exit-on-error` on restores, tmp-then-`mv`).
- [ ] `make backup` produces a timestamped `-Fc` dump in `./backups/`; a valid dump is restorable.
- [ ] `make backup-verify` restores that dump into a **scratch** db, confirms `>0` public tables,
      drops the scratch db, exits 0 — and exits non-zero on a bad/empty dump. **Live db untouched.**
- [ ] `make restore BACKUP=<file>` confirms before overwriting the live db (or `FORCE=1` skips),
      and restores a real dump without error.
- [ ] Failure is loud: with the DB down, `make backup` exits non-zero and leaves **no** dump or
      `.tmp` file (no silent empty dumps).
- [ ] Rotation keeps the newest `KEEP` (default 14); `make backup` prunes automatically.
- [ ] Documented cron one-liner (nightly, rotating) present in README §Backups.
- [ ] `.gitignore` ignores `./backups/*.dump*` and `*.log`; `backups/.gitkeep` is tracked; no
      real dump is committed.
- [ ] README has a "Backups & Restore" section per §6.
- [ ] No changes outside the files in §1.
```
