# sdgen - Synthetic Data Generator

A high-performance synthetic data generator with deterministic seeding, supporting PostgreSQL and SQLite targets.

## Features

- Define scenarios as versioned YAML configs
- Deterministic data generation with seeding
- Built-in generators (UUID, faker, time series, distributions, foreign keys)
- CLI and HTTP API interfaces
- PostgreSQL and SQLite support
- Minimal HTML UI for run control

## Quick Start

### Build

```bash
go mod download
go build -o bin/sdgen ./cmd/sdgen
go build -o bin/sdgen-api ./cmd/sdgen-api
```

### CLI Usage

List scenarios:
```bash
./bin/sdgen scenario list
```

Start a run:
```bash
./bin/sdgen run start --scenario iot_demo_small --target-id dev-sqlite
```

List runs:
```bash
./bin/sdgen run list
```

### API Server

Start the API server:
```bash
./bin/sdgen-api
```

Visit http://localhost:8080 for the web interface.

## Configuration

Environment variables:
- `SDGEN_SCENARIOS_DIR` - Scenarios directory (default: ./scenarios)
- `SDGEN_TARGETS_DIR` - Targets directory (default: ./targets)
- `SDGEN_RUNS_DB` - Runs database path (default: ./sdgen-runs.sqlite)
- `SDGEN_LOG_LEVEL` - Log level (default: info)
- `SDGEN_BIND_ADDR` - API bind address (default: :8080)

## Generators

- `const` - Constant value
- `uuid4` - UUID v4
- `uniform_int` - Uniform integer distribution
- `uniform_float` - Uniform float distribution
- `normal` - Normal distribution
- `choice` - Random choice from list with optional weights
- `faker_name` - Random person name
- `faker_city` - Random city name
- `faker_device_name` - Random device name
- `time_series` - Time series with configurable start/step/jitter
- `fk` - Foreign key reference

## Example Scenario

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

## API Endpoints

- `GET /api/v1/scenarios` - List scenarios
- `GET /api/v1/scenarios/{id}` - Get scenario
- `GET /api/v1/targets` - List targets
- `GET /api/v1/targets/{id}` - Get target
- `POST /api/v1/runs` - Create run
- `GET /api/v1/runs` - List runs
- `GET /api/v1/runs/{id}` - Get run

## License

MIT
