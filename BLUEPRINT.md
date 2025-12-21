# sdgen — Synthetic Data Generator (Blueprint)

Repository: `github.com/mmrzaf/sdgen`
Language: Go
Interfaces: CLI + HTTP API + minimal HTML

---

## 1. Product goal and boundaries

### 1.1 Goal

Provide an internal service and toolchain that allows engineers to:

- Define **scenarios** (synthetic datasets) as versioned configs
- Use **generators** to populate schema fields deterministically (seeded RNG)
- Execute **runs** that materialize scenario output into a **target**
- Observe runs (status, stats, logs) via CLI and API
- Iterate on scenarios without rewriting backend code

### 1.2 Non-goals (v0)

- No non-database targets (files, Kafka, Elasticsearch) in v0
- No GUI “scenario editor” (HTML is minimal run control + status)
- No multi-tenant auth/permissions (assume internal trusted network)
- No privacy-preserving generation or real-data transformation in v0
- No complex scheduling; runs are on-demand

---

## 2. Core concepts and invariants

### 2.1 Concepts

- **Generator**: value factory for a column (predefined + extendable via registry)
- **GeneratorSpec**: serialized configuration that describes a generator instance
- **Schema**: entity definition consisting of columns and their generator specs
- **Entity**: one table output with row count, column definitions, relationships
- **Scenario**: named dataset recipe containing one or more entities
- **Target**: database destination with connection info and options
- **Run**: a single execution of scenario → target, producing tables/rows

### 2.2 Must-hold invariants

- **Reproducibility**: a run must be reproducible from:
  - scenario (including version)
  - seed
  - scenario config hash
  - generator parameters

- **Deterministic seeding**:
  - if seed is provided (override or scenario default), it is used
  - if seed is not provided, one is generated and stored with the run

- **Config immutability per run**:
  - a run stores the exact config hash and resolved scenario version used

- **Target safety**:
  - v0 requires explicit behavior for table handling (truncate/append/create)
  - no silent destructive operations beyond what is configured

- **Validation first**:
  - invalid config fails before writing any rows
  - generator specs and foreign key references must validate

---

## 3. Repository structure (proposed)

Top-level layout:

- `cmd/`
  - `sdgen/` — CLI entrypoint
  - `sdgen-api/` — HTTP server entrypoint (API + HTML)

- `internal/`
  - `app/` — application services, orchestration (RunService)
  - `domain/` — core types (Scenario, Entity, GeneratorSpec, TargetConfig, Run)
  - `infra/` — implementations of repositories and targets
    - `repos/`
      - `scenarios/` — scenario repository implementations
      - `targets/` — target repository implementations
      - `runs/` — run repository implementations

    - `targets/`
      - `postgres/` — Postgres target implementation
      - `sqlite/` — SQLite target implementation

  - `registry/` — generator and target registries
  - `generators/` — predefined generators (factory + parameter validation)
  - `validation/` — schema validation, dependency ordering, constraint checks
  - `exec/` — execution engine (batching, FK resolution, row building)
  - `api/` — HTTP handlers, request/response DTOs, routing
  - `web/` — HTML templates + static assets (minimal)
  - `config/` — config loading for sdgen itself (paths, defaults)
  - `logging/` — structured logging wrapper and run-scoped logs
  - `hashing/` — config hashing utilities
  - `timeutil/` — duration parsing (“-7d”, “1m”) for generators like time_series

- `scenarios/` — default scenario configs (YAML)
- `targets/` — default target configs (YAML) for dev environments
- `docs/`
  - `blueprint.md` — this document
  - `schema.md` — scenario config schema reference
  - `generators.md` — generator catalog reference
  - `api.md` — API reference

- `examples/`
  - `iot_demo_small.yaml`
  - `finance_demo.yaml`

- `Makefile` or `justfile` — build/test/run helpers

Notes:

- `internal/domain` contains **no knowledge** of HTTP, CLI, YAML, DB drivers.
- `internal/infra` contains implementation details (file system scanning, DB drivers).
- `cmd/*` are thin wiring layers.

---

## 4. Configuration model (YAML/JSON)

sdgen supports scenario and target definitions in YAML (primary) and JSON (secondary). Both map to the same domain structures.

### 4.1 Scenario config file

Required fields:

- `name`
- `entities[]` with at least one entity
- Each entity must have:
  - `name`
  - `target_table`
  - `rows`
  - `columns[]` with at least one column

- Each column must have:
  - `name`
  - `type`
  - `generator.type`

Recommended fields:

- `id` (stable identifier used for scenario ID execution)
- `version` (semantic version; required if scenarios are stored/served)
- `seed` (optional default seed)

Example conceptual schema:

- Scenario
  - id (string, optional)
  - name (string, required)
  - version (string, optional in v0 but strongly recommended)
  - description (string, optional)
  - seed (int64, optional)
  - entities (list, required)

- Entity
  - name (string, required)
  - target_table (string, required)
  - rows (int64, required, > 0)
  - columns (list, required)

- Column
  - name (string, required)
  - type (enum, required)
  - nullable (bool, optional, default false)
  - generator (GeneratorSpec, required)
  - fk (optional ForeignKey descriptor)

- GeneratorSpec
  - type (string, required)
  - params (map, optional)

### 4.2 Target config file

Required fields:

- `name`
- `kind` in {postgres, sqlite}
- `dsn` (postgres URL or sqlite path)

Optional:

- `id` (stable identifier)
- `schema` (postgres schema; default `public`)
- `options` (future extension: insert mode, batch size)

---

## 5. Generator catalog (v0)

Generators are defined as `type` + `params`. All generators must:

- Validate params (missing/invalid fails validation)
- Be deterministic given the RNG and context
- Prefer stable output types matching declared column types

### 5.1 Must-have generators (v0)

- `const`
  - params: `value`

- `uuid4`
  - params: none

- `uniform_int`
  - params: `min`, `max`

- `uniform_float`
  - params: `min`, `max`

- `normal`
  - params: `mean`, `std`

- `choice`
  - params: `values` (list), `weights` (optional list same length)

- `faker_name` / `faker_city` / `faker_device_name` (keep small set)
  - params: optional locale in the future

- `time_series`
  - params: `start` (relative like `-7d` or absolute), `step` (`1m`, `5s`), `jitter_seconds` (optional)

- `fk`
  - params: `entity`, `column`
  - semantics: select an existing value from referenced entity output

### 5.2 Generator extension rules

- Generators are registered via registry by `type` string.
- Adding a generator must not require changes to executor logic beyond registry.
- Generator docs must include:
  - supported column types
  - params schema
  - determinism guarantees
  - example usage

---

## 6. Targets (v0) and write behavior

### 6.1 Supported targets

- Postgres
- SQLite

### 6.2 Table handling policy (explicit, not implicit)

v0 should enforce an explicit per-entity policy, defaulting to safe behavior.

Per entity (recommended config extension):

- `table_mode` in:
  - `create_if_missing` (default)
  - `truncate_then_insert`
  - `append_only`

If you do not want to expand config yet, implement global defaults via sdgen runtime config:

- CLI flags / API options: `--mode create_if_missing|truncate|append`

### 6.3 Schema creation strategy (v0)

Options:

- Minimal approach: **require existing tables** (fastest to implement, safest)
- More useful approach: **create tables if missing** based on column types

Recommended v0.1:

- Create-if-missing based on column list
- Do not attempt complex DDL diffs/migrations
- If table exists:
  - if mode is `create_if_missing`, validate columns exist (or warn)
  - if mismatch, fail early (avoid silent incorrect schema)

### 6.4 Insert performance

- Use batching (configurable batch size, default e.g. 1,000–10,000)
- Use prepared statements / bulk insert mechanisms where practical
- Transactions per entity (or per batch if tables are huge)

---

## 7. Foreign keys and dependency ordering

### 7.1 Dependency DAG

- Entities referencing other entities via `fk` imply dependencies.
- A topological sort determines generation order.

Rules:

- Cycles are not allowed in v0; cycle detection must fail validation.
- If an entity references `fk` to an entity not present, fail validation.

### 7.2 FK value selection strategy (v0)

- Keep a per-entity in-memory cache of generated FK source values (e.g., `device.id` list).
- `fk` generator selects a random element from that cache.

This is simple, deterministic, and sufficient for v0 datasets.

---

## 8. Run execution model

### 8.1 Execution pipeline

1. Resolve scenario:
   - if `scenario_id`: load from scenario repository
   - if inline scenario: validate directly

2. Resolve target:
   - if `target_id`: load from target repository
   - if inline target: validate directly

3. Determine seed (override > scenario default > generated)
4. Compile scenario:
   - validate
   - build generator instances from specs
   - compute entity order (toposort)
   - compute config hash

5. Create run record:
   - status `running`
   - store seed, config hash, scenario name/version/ID (resolved)

6. Execute entities in order:
   - prepare target entity (DDL / checks)
   - generate rows in batches
   - write batch to target
   - update per-entity stats

7. Finalize run record:
   - success/failure
   - finished timestamp
   - error message if any

### 8.2 Synchronous vs asynchronous (v0 decision)

Pick one explicitly:

- **v0 recommended:** synchronous execution by default.
  - CLI runs block until done.
  - API call can either:
    - run synchronously (simple), or
    - create run and process in background goroutine (still single-process)

If you do background runs, then:

- API must return `run_id` immediately and provide polling via `GET /runs/{id}`

Both are acceptable; background is more “service-like.”

### 8.3 Observability (must-have)

- Structured logs with run_id prefix
- Run stats per entity:
  - requested rows
  - written rows
  - duration per entity (optional but valuable)

- Expose last N runs via CLI and UI

---

## 9. Persistence: repositories

### 9.1 ScenarioRepository (v0)

- Implementation: sqlite

Behavior:

- List scenarios
- Resolve by ID:
  - match `id` field first
  - fallback to filename stem if no id

### 9.2 TargetRepository (v0)

- Implementation: sqlite
- Resolve by `id`

### 9.3 RunRepository (v0)

- sdgen keeps its own internal SQLite database for run metadata
- Works regardless of user’s target DB
- Good for service mode (sdgen-api)

RunRepository must store:

- run id
- timestamps
- status
- seed
- config hash
- scenario identity (id/name/version)
- target identity (id/name/kind)
- stats JSON
- error message

---

## 10. CLI spec

Binary: `sdgen`

### 10.1 Global flags

- `--scenarios-dir` (default `./scenarios`)
- `--targets-dir` (default `./targets`)
- `--runs-db` (default `./sdgen-runs.sqlite` or similar)
- `--log-level` (info/debug)

### 10.2 Commands

#### Scenarios

- `sdgen scenario list [--format table|json]`
- `sdgen scenario show <id>`
- `sdgen scenario validate <id|path>`

#### Targets

- `sdgen target list [--format table|json]`
- `sdgen target show <id>`
- `sdgen target validate <id|path>`

#### Runs

- `sdgen run start --scenario <id|path> --target <id|path|dsn> [--target-kind postgres|sqlite] [--seed N] [--rows-override entity=rows ...] [--mode create_if_missing|truncate_then_insert|append_only]`
- `sdgen run list [--limit N] [--status running|success|failed]`
- `sdgen run show <run_id>`

Notes:

- If `--target` is a DSN/path rather than an ID, `--target-kind` becomes required.
- `--rows-override` may be repeated or accept comma-separated values.

---

## 11. HTTP API spec (v0)

Base path: `/api/v1`

### 11.1 Scenarios

- `GET /scenarios`
- `GET /scenarios/{id}`
- Optional in v0: `POST /scenarios` (if you want scenario write support later)

### 11.2 Targets

- `GET /targets`
- `GET /targets/{id}`

### 11.3 Runs

- `POST /runs`
  - Accepts a RunRequest that supports both ID-based and inline configs:
    - scenario_id OR scenario
    - target_id OR target
    - optional seed
    - optional row overrides
    - optional mode

  - Returns run object or run_id (depending on sync/async decision)

- `GET /runs`
- `GET /runs/{id}`

### 11.4 Minimal HTML UI

Routes:

- `/` page:
  - scenario dropdown (fetched from `/api/v1/scenarios`)
  - target dropdown (fetched from `/api/v1/targets`)
  - seed field (optional)
  - rows overrides text box
  - mode selection
  - “Start run” button
  - Recent runs list with status badges and link to detail

- `/runs/{id}` page:
  - show stats, status, error message, timestamps

Keep HTML/JS minimal and stable; avoid SPA complexity.

---

## 12. Validation rules (explicit)

Validation is a first-class subsystem. It must fail fast and clearly.

### 12.1 Scenario validation

- Scenario name not empty
- Entities:
  - unique `name`
  - unique `target_table` (recommended; or allow but warn)
  - `rows > 0`

- Columns:
  - unique per entity
  - `type` is recognized
  - generator spec exists and generator type is registered
  - generator params validate (per generator)

- Foreign keys:
  - referenced entity exists
  - referenced column exists
  - referenced entity must be generated earlier (acyclic)
  - cycles are rejected

- Optional: column nullable compatibility with generator (if generator can emit null)

### 12.2 Target validation

- kind is supported
- dsn is non-empty
- For sqlite:
  - path valid (or createable)

- For postgres:
  - parseable URL format (you can fail on obvious invalid DSNs early)

### 12.3 Run request validation

- Exactly one of scenario_id / scenario is provided
- Exactly one of target_id / target is provided
- rows overrides only for existing entity names
- seed fits int64

---

## 13. Determinism and hashing

### 13.1 Config hash

Compute a stable hash over:

- resolved scenario config (after applying overrides that affect generation)
- generator specs with params
- scenario version/id/name
- resolved target kind and schema options that affect behavior (not raw DSN credentials unless necessary)

Use a canonical JSON representation (sorted keys) before hashing to avoid hash drift.

### 13.2 Seed strategy

Seed resolution order:

1. RunRequest.OverrideSeed
2. ScenarioConfig.Seed
3. Generated seed (crypto/rand or time-based) — but once generated, store it

Seed must be stored in Run record.

---

## 14. Versioning strategy

Scenario versioning is critical once people rely on consistent datasets.

### 14.1 Scenario identity

A scenario is identified by:

- `id` (stable)
- `version` (semver)
- `name` (human-friendly)

### 14.2 How versions change

- Any generator param change, column change, entity change should bump version.
- v0 can treat `version` as “recommended but not enforced,” but the run record should store it.

---

## 15. Security posture (internal, still not careless)

- No auth in v0 is acceptable only if internal network and non-public.
- Do not log DSNs with credentials in plaintext.
- If HTML UI exists, restrict bind address by default (e.g., localhost unless configured).
- Avoid arbitrary file reads:
  - scenario/target directories are configured; prevent path traversal.

---

## 16. Operational concerns

### 16.1 Runtime configuration

- Environment variables and/or flags:
  - scenario dir
  - target dir
  - run DB path
  - bind address/port (api server)
  - default batch size
  - default mode (create/truncate/append)

### 16.2 Performance knobs

- batch size (rows per insert)
- transaction boundaries (per entity)
- optional concurrency:
  - v0 keep sequential for reliability
  - later: concurrent generation per entity if independent

### 16.3 Failure modes

- Any failure must:
  - stop the run
  - mark run failed
  - store error message
  - leave partial data depending on mode
    - optional future: “transactional run” across entities is complex; defer

---

## 17. Deliverables for v0.1

What “done” looks like.

### 17.1 Functional

- Can define scenarios and targets as YAML
- CLI:
  - list/show/validate scenarios
  - list/show/validate targets
  - start run with scenario id or file and target id or DSN
  - show run status/stats

- API:
  - list scenarios/targets
  - start run
  - list runs / show run

- Postgres + SQLite targets:
  - insert generated data in batches
  - table handling modes supported (at least truncate vs append vs create-if-missing)

- FK support with dependency ordering

### 17.2 Quality

- Deterministic runs with seed
- Config hash stored
- Clear validation errors
- Basic logging and run stats
- Docs:
  - generator catalog
  - scenario schema reference
  - quickstart for CLI and API

---

## 18. Recommended v0.1 documentation set

- `README.md`
  - What sdgen is
  - Quick start (CLI + API)
  - Config examples

- `docs/schema.md`
  - Scenario and target config fields + examples

- `docs/generators.md`
  - Generator list, params, compatible types, examples

- `docs/api.md`
  - API endpoints with example requests/responses

- `examples/`
  - iot demo scenario
  - simple finance scenario

---

## 19. Forward-looking (planned, not implemented)

Designed seams that should not require refactors later:

- New targets via target registry (MySQL, ClickHouse, ES, file output)
- More generator types (seasonality, trend, correlated fields, markov, etc.)
- “Profiles” (demo vs load vs qa) as scenario variants
- Async workers (separate process) and run queue
- Scenario storage in DB + web editor
- Data quality constraints in config (distribution assertions)
