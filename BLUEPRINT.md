# sdgen — Synthetic Data Generator (Blueprint)

Repository: `github.com/mmrzaf/sdgen`  
Language: Go  
Interfaces: CLI + HTTP API + minimal HTML

---

## 1. Product goal and boundaries

### 1.1 Goal

Provide an internal service and toolchain that allows engineers to:

- Define **scenarios** (synthetic datasets) as versioned configs (YAML, Git-managed)
- Use **generators** to populate schema fields deterministically (seeded RNG)
- Execute **runs** that materialize scenario output into a **target** database
- Manage and test reusable **targets** via CLI/UI/API
- Observe runs (status, stats, logs) via CLI, API, and UI

### 1.2 Non-goals (v0)

- No non-database targets (files, Kafka, Elasticsearch) in v0
- No GUI “scenario editor” (HTML is minimal run control + status + target management)
- No multi-tenant auth/permissions (assume internal trusted network)
- No privacy-preserving generation or real-data transformation in v0
- No complex scheduling; runs are on-demand

---

## 2. Core concepts and invariants

### 2.1 Concepts

- **Generator**: value factory for a column (predefined + extendable via registry)
- **GeneratorSpec**: serialized configuration that describes a generator instance
- **Entity**: one table output with row count, columns, and relationships
- **Scenario**: named dataset recipe containing one or more entities (file-backed, read-only)
- **Target**: database destination with connection info and options (DB-backed, CRUD-managed)
- **Run**: a single execution of scenario → target, producing tables/rows

### 2.2 Must-hold invariants

- **Reproducibility**: a run must be reproducible from:
  - scenario (including version)
  - seed
  - resolved scenario config hash (after applying run-time overrides)
  - generator parameters

- **Deterministic seeding**:
  - if seed is provided (override or scenario default), it is used
  - if seed is not provided, one is generated and stored with the run

- **Config immutability per run**:
  - a run stores the exact config hash and resolved counts used

- **Target safety**:
  - v0 requires explicit behavior for table handling (create/truncate/append)
  - no silent destructive operations beyond the chosen mode

- **Validation first**:
  - invalid config fails before writing any rows
  - generator specs and foreign key references must validate
  - table/column/schema identifiers must be safe identifiers

---

## 3. Repository structure

Top-level layout:

- `cmd/`
  - `sdgen/` — CLI entrypoint
  - `sdgen-api/` — HTTP server entrypoint (API + HTML)

- `internal/`
  - `app/` — application services, orchestration (RunService)
  - `domain/` — core types (Scenario, Entity, GeneratorSpec, TargetConfig, Run, RunPlan)
  - `infra/` — implementations of repositories and targets
    - `repos/`
      - `scenarios/` — file-backed scenario repository
      - `targets/` — targets repository (runs DB)
      - `runs/` — run repository (runs DB)
    - `targets/`
      - `postgres/` — Postgres target implementation
  - `registry/` — generator registry
  - `generators/` — predefined generators
  - `validation/` — schema validation, dependency ordering, identifier safety checks
  - `exec/` — execution engine (batching, FK resolution, row building)
  - `api/` — HTTP handlers, request/response DTOs, routing
  - `web/` — HTML templates + static assets (minimal)

- `scenarios/` — default scenario configs (YAML)

Notes:

- `internal/domain` contains no knowledge of HTTP, CLI, YAML, DB drivers.
- `internal/infra` contains implementation details (file system scanning, DB drivers).
- `cmd/*` are thin wiring layers.

---

## 4. Configuration model

### 4.1 Scenario config file (YAML, read-only)

Required fields:

- `name`
- `entities[]` with at least one entity
- Each entity must have:
  - `name`
  - `target_table`
  - `rows`
  - `columns[]` with at least one column

Each column must have:

- `name`
- `type`
- `generator.type`

Recommended fields:

- `id` (stable identifier used as scenario ID)
- `version` (semantic version)
- `seed` (optional default seed)

### 4.2 Target model (DB-backed)

Targets are not file-backed. They are created/updated/deleted via UI/CLI/API and stored in the runs DB.

Fields:

- `id` (generated; stable)
- `name` (human label)
- `kind` in `{postgres}`
- `dsn` (stored raw internally; redacted in responses)
- `schema` (postgres only; optional; default `public`)
- `options` (future extension)

Inline targets:

- Run requests may supply an inline target object (not persisted) for ad-hoc usage.

---

## 5. Generator catalog (v0)

Generators are defined as `type` + `params`. All generators must:

- Validate params (missing/invalid fails validation)
- Be deterministic given the RNG and context
- Prefer stable output types matching declared column types

Must-have generators (v0):

- `const` (params: `value`)
- `uuid4`
- `uniform_int` (params: `min`, `max`)
- `uniform_float` (params: `min`, `max`)
- `normal` (params: `mean`, `std`)
- `choice` (params: `values`, optional `weights`)
- `faker_name` / `faker_city` / `faker_device_name`
- `time_series` (params: `start`, `step`, optional `jitter_seconds`)
- `fk` (params: `entity`, `column`)

---

## 6. Targets and write behavior

### 6.1 Supported targets

- Postgres

### 6.2 Table handling modes (run-level)

Runs must specify an explicit mode:

- `create` (create-if-missing; validate existing schema if present)
- `truncate` (truncate table then insert)
- `append` (append-only)

### 6.3 Schema creation strategy (v0)

Recommended v0:

- Create-if-missing based on scenario column list
- If table exists:
  - validate column presence / compatibility (fail early on mismatch)
- Do not attempt complex DDL diffs/migrations

### 6.4 Insert performance

- Use batching (configurable batch size; default sane)
- Transactions per entity (v0)

---

## 7. Foreign keys and dependency ordering

Entities referencing other entities via `fk` imply dependencies.

Rules:

- Cycles are rejected
- Missing referenced entity/column is rejected
- Topological sort determines execution order

---

## 8. Run execution model

### 8.1 Run-time row count overrides

Run-time row counts are controlled by the run request, not by editing scenarios.

Inputs:

- `scale` (float)
- optional per-entity `entity_counts` map

Resolution order:

1. scenario defaults
2. apply scale
3. apply explicit per-entity overrides

### 8.2 Execution pipeline

1. Resolve scenario:
   - if `scenario_id`: load from scenario repository
   - if inline scenario: validate directly

2. Resolve target:
   - if `target_id`: load from target repository
   - if inline target: validate directly

3. Determine seed (override > scenario default > generated)
4. Plan:
   - validate scenario + request
   - compute entity order (toposort)
   - resolve row counts with overrides
   - compute config hash (including resolved row counts)

5. Create run record:
   - status `running`
   - store seed, config hash, scenario identity, target identity
   - store resolved row counts (JSON)

6. Execute entities in order:
   - prepare target entity (DDL / checks)
   - generate rows in batches
   - write batches
   - update per-entity stats

7. Finalize run record:
   - success/failure
   - finished timestamp
   - error message if any
   - stats JSON

---

## 9. Persistence: runs DB schema

The runs DB stores:

- `runs` (run metadata + stats)
- `targets` (managed targets)
- `target_checks` (connection test history)

Target checks must record:

- `ok` (bool)
- `latency_ms` (int)
- `server_version` (string)
- `capabilities`:
  - `can_create`
  - `can_truncate`
  - `can_insert`
- `error` (string)

DSNs are stored raw internally, but redacted in all responses.

---

## 10. CLI spec

Binary: `sdgen`

### 10.1 Global flags

- `--scenarios-dir` (default `./scenarios`)
- `--db` (default from `SDGEN_DB` loaded via environment/.env; required when not passed)
- `--log-level` (info/debug)

### 10.2 Commands

#### Scenarios (read-only)

- `sdgen scenario list`
- `sdgen scenario show <id>`
- `sdgen scenario validate <id|path>`

#### Targets (DB-backed)

- `sdgen target add --name <name> --kind <postgres> [--schema <schema>] --dsn <dsn>`
- `sdgen target update <id> --name <name> --kind <postgres> [--schema <schema>] --dsn <dsn>`
- `sdgen target rm <id>`
- `sdgen target list` (DSNs redacted)
- `sdgen target show <id>` (DSNs redacted)
- `sdgen target test <id>` (structured output; records a target_check)

#### Runs

- `sdgen run start --scenario <id|path> (--target-id <id> | --target <dsn> --target-kind <kind> [--target-schema <schema>]) --mode <create|truncate|append> [--seed N] [--scale F] [--entity-count name=N ...] [--plan]`
- `sdgen run list [--limit N]`
- `sdgen run show <run_id>`

---

## 11. HTTP API spec (v0)

Base path: `/api/v1`

### 11.1 Scenarios (read-only)

- `GET /scenarios`
- `GET /scenarios/{id}`

### 11.2 Targets (DB-backed)

- `GET /targets` (DSN redacted)
- `POST /targets` (create)
- `GET /targets/{id}` (DSN redacted)
- `PUT /targets/{id}` (update)
- `DELETE /targets/{id}` (delete)
- `POST /targets/{id}/test` (test connection; records target_check)

Target test response:

```json
{
  "ok": true,
  "latency_ms": 12,
  "server_version": "15.6",
  "capabilities": {
    "can_create": true,
    "can_truncate": true,
    "can_insert": true
  },
  "error": ""
}
```

### 11.3 Runs

- `POST /runs`
  - accepts RunRequest with:
    - scenario_id OR scenario
    - target_id OR target
    - mode
    - optional seed
    - optional scale + entity_counts

- `POST /runs/plan`
  - returns execution order + resolved counts + warnings without executing

- `GET /runs`
- `GET /runs/{id}`

---

## 12. Minimal HTML UI

Routes:

- `/` (Run builder):
  - scenario dropdown (from `/api/v1/scenarios`)
  - target dropdown (from `/api/v1/targets`)
  - mode selection (create/truncate/append)
  - scale input
  - per-entity overrides editor
  - “Plan” button (calls `/api/v1/runs/plan`)
  - “Run” button (calls `/api/v1/runs`)
  - recent runs list with link to detail

- `/targets` (Target management):
  - list targets (redacted DSN)
  - create/update/delete target
  - test target connection (calls `/api/v1/targets/{id}/test`)

- `/runs/{id}`:
  - show status, stats, error message, timestamps
  - poll run detail until terminal state

Keep HTML/JS minimal and stable; avoid SPA complexity.

---

## 13. Validation rules (explicit)

Validation must fail fast and clearly.

### 13.1 Identifier safety (anti-injection)

Reject unsafe identifiers for:

- entity names (used in overrides)
- `target_table`
- column `name`
- postgres schema

Rule of thumb:

- allow only `[A-Za-z_][A-Za-z0-9_]*`
- reject SQL keywords
- never interpolate unchecked identifiers into SQL

### 13.2 Scenario validation

- scenario name not empty
- entities:
  - unique `name`
  - `rows > 0`
  - `target_table` is safe identifier

- columns:
  - unique per entity
  - `name` is safe identifier
  - `type` recognized
  - generator spec exists and validates

- foreign keys:
  - referenced entity/column exist
  - dependency graph is acyclic

### 13.3 Target validation

- kind supported
- dsn non-empty
- postgres schema is safe identifier if provided

### 13.4 Run request validation

- exactly one of `scenario_id` / `scenario`
- exactly one of `target_id` / `target`
- `scale` if provided must be > 0
- `entity_counts` values must be > 0
- `entity_counts` keys must be safe identifiers
- unknown entity overrides should produce plan warnings (not hard-fail)

---

## 14. Security posture (internal, still not careless)

- Do not log DSNs with credentials in plaintext.
- DSNs are redacted in API responses and UI, stored raw internally (v0).
- If HTML UI exists, restrict bind address by default (e.g., localhost unless configured).
- Avoid arbitrary file reads:
  - scenarios directory is configured; prevent path traversal.

---

## 15. Operational concerns

### 15.1 Runtime configuration

Environment variables and/or flags:

- scenario dir
- runs DB path
- bind address/port (api server)
- default batch size
- default mode (create/truncate/append)

### 15.2 Failure modes

Any failure must:

- stop the run
- mark run failed
- store error message
- leave partial data depending on mode

---

## 16. Deliverables for v0.1

Functional:

- Scenarios: file-backed YAML, read-only list/show/validate
- Targets: DB-backed CRUD + test connection (records checks)
- Runs:
  - start run with scenario id or path and target id or inline DSN
  - mode support create/truncate/append
  - run-time overrides via scale + entity_counts
  - plan endpoint that returns order + resolved counts + warnings

- UI:
  - run builder with plan + run
  - targets management page with test
  - run detail page with polling

Quality:

- deterministic runs with seed
- config hash stored
- clear validation errors
- basic logging and run stats

Forward-looking (planned, not implemented):

- new targets via registry (additional SQL engines, ClickHouse, file output)
- more generator types (seasonality, correlated fields)
- async workers / run queue
- scenario storage in DB + web editor
- data quality constraints in config (distribution assertions)
