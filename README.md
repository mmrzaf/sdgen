# sdgen — Synthetic Data Generator

`sdgen` generates deterministic synthetic datasets from versioned, file-backed **scenarios** and loads them into DB-backed **targets** (PostgreSQL, Elasticsearch). It ships with a CLI, an HTTP API, and a small web UI.

---

## What’s what

### Scenarios (read-only)

- Stored as YAML files (typically managed in Git).
- Exposed read-only via CLI/UI/API (list/show/validate).
- Scenario files define entities, columns, generators, and dependencies.

### Targets (managed via UI/CLI/API)

- Stored in the sdgen PostgreSQL **metadata DB**.
- Support create/update/delete and **test connection**.
- DSNs are stored raw internally, but are **redacted** in API responses and the UI.

### Runs

- Runs are created via API/CLI and tracked in the sdgen PostgreSQL **metadata DB**.
- Run-time row counts are controlled by the **run request**, not by editing scenarios:
  - `scale` (float), optional per-entity `entity_scales`, optional `entity_counts`
  - optional `include_entities` / `exclude_entities`
  - Resolution order:
    1. scenario defaults
    2. apply scale
    3. apply per-entity scale
    4. apply explicit per-entity counts
- `/api/v1/runs/plan` returns execution order + resolved counts + warnings without executing.

---

## Build

```bash
go mod download
go build -o bin/sdgen ./cmd/sdgen
go build -o bin/sdgen-api ./cmd/sdgen-api
```

---

## Run the API server + Web UI

```bash
./bin/sdgen-api
```

Web UI:

- Home / Run builder: [http://localhost:8080](http://localhost:8080)
- Targets management: [http://localhost:8080/targets](http://localhost:8080/targets)

---

## Configuration

Environment variables:

- `SDGEN_SCENARIOS_DIR` — Scenarios directory (default: `./scenarios`)
- `SDGEN_DB` — Runs/targets metadata database DSN (required)
- `SDGEN_LOG_LEVEL` — Log level (default: `info`)
- `SDGEN_BIND` — API bind address (default: `127.0.0.1:8080`)
- `SDGEN_BATCH_SIZE` — Insert batch size (default: `1000`)

`.env` is loaded automatically from the current working directory if present.
Use `.env.example` as the template for your local `.env`.

---

## CLI

### Scenarios (read-only)

List:

```bash
./bin/sdgen scenario list
```

Show:

```bash
./bin/sdgen scenario show example
```

Validate:

```bash
./bin/sdgen scenario validate example
```

### Targets

Add (postgres):

```bash
./bin/sdgen target add --name dev-pg --kind postgres --schema public --dsn "postgres://user:pass@localhost:5432/db?sslmode=disable"
```

Add (postgres, DSN helper flags):

```bash
./bin/sdgen target add --name dev-pg --kind postgres --host localhost --port 5432 --user user --password pass --database appdb --sslmode disable
```

Add (elasticsearch):

```bash
./bin/sdgen target add --name dev-es --kind elasticsearch --dsn http://localhost:9200
```

Update:

```bash
./bin/sdgen target update <target-id> --name dev-pg --kind postgres --schema public --dsn "postgres://user:pass@localhost:5432/db?sslmode=disable"
```

List (DSNs redacted):

```bash
./bin/sdgen target list
```

Show (DSN redacted):

```bash
./bin/sdgen target show <target-id>
```

Remove:

```bash
./bin/sdgen target rm <target-id>
```

Test connection (structured output):

```bash
./bin/sdgen target test <target-id>
```

### Runs

Start:

```bash
./bin/sdgen run start --scenario iot_demo_small --target-id <target-id> --mode create
```

Start with scale:

```bash
./bin/sdgen run start --scenario iot_demo_small --target-id <target-id> --mode create --scale 0.25
```

Start with explicit per-entity overrides:

```bash
./bin/sdgen run start --scenario iot_demo_small --target-id <target-id> --mode create \
  --entity-count users=1000 \
  --entity-count events=50000
```

Start with per-entity scale and include/exclude:

```bash
./bin/sdgen run start --scenario finance --target-id <target-id> --mode create \
  --scale 0.5 \
  --entity-scale payments=2.0 \
  --include-entity customers \
  --include-entity accounts \
  --include-entity payments \
  --exclude-entity fraud_alerts
```

Run against a different database on the same physical target:

```bash
./bin/sdgen run start --scenario finance --target-id <target-id> --target-db tenant_a --mode create
```

Plan only (no execution):

```bash
./bin/sdgen run start --scenario iot_demo_small --target-id <target-id> --mode create --plan
```

Inline target (not stored):

```bash
./bin/sdgen run start \
  --scenario iot_demo_small \
  --target-kind postgres \
  --target-schema public \
  --target "postgres://user:pass@localhost:5432/db?sslmode=disable" \
  --mode truncate
```

List runs:

```bash
./bin/sdgen run list --limit 20
```

Show run:

```bash
./bin/sdgen run show <run-id>
```

---

## API

### Scenarios (read-only)

- `GET /api/v1/scenarios` — list scenarios
- `GET /api/v1/scenarios/{id}` — get scenario

### Targets (DB-backed)

- `GET /api/v1/targets` — list targets (DSN redacted)
- `POST /api/v1/targets` — create target
- `GET /api/v1/targets/{id}` — get target (DSN redacted)
- `PUT /api/v1/targets/{id}` — update target
- `DELETE /api/v1/targets/{id}` — delete target
- `POST /api/v1/targets/{id}/test` — test connection

**Target test response shape**

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

### Runs

- `POST /api/v1/runs` — create run (executes)
- `POST /api/v1/runs/plan` — plan only (no execution)
- `GET /api/v1/runs` — list runs
- `GET /api/v1/runs/{id}` — get run
- `GET /api/v1/runs/{id}/logs` — get run logs (most recent first; `?limit=N`)

Run detail responses include progress fields:
- `progress_rows_generated`
- `progress_rows_total`
- `progress_entities_done`
- `progress_entities_total`
- `progress_current_entity`

---

## Generators

Built-in generators include:

- `const` — constant value
- `uuid4` — UUID v4
- `uniform_int` — uniform integer distribution
- `uniform_float` — uniform float distribution
- `normal` — normal distribution
- `choice` — random choice (optional weights)
- `faker_name` — random person name
- `faker_city` — random city name
- `faker_device_name` — random device name
- `time_series` — time series with start/step/jitter
- `fk` — foreign key reference

---

## Example scenario (YAML)

```yaml
id: example
name: Example Scenario
version: 1.0.0
seed: 12345

entities:
  - name: users
    target_table: users
    rows: 100
    columns:
      - name: id
        type: uuid
        generator:
          type: uuid4
      - name: name
        type: string
        generator:
          type: faker_name
      - name: age
        type: int
        generator:
          type: uniform_int
          params:
            min: 18
            max: 80
```

---

## Safety / validation notes

- Table, column, and schema identifiers are validated to reject unsafe strings (prevents SQL injection via config).
- DSNs are redacted in API responses and the web UI, but stored raw internally (for now).

---

## License

MIT

```

```
