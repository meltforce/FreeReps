# FreeReps Fix Plan

## Phase 1 Progress

### Completed
- [x] Step 0: Specs and fix_plan
- [x] Step 1: Project scaffolding + Docker Compose
- [x] Step 2: Config loading
- [x] Step 3: Database schema + migrations
- [x] Step 4: Storage layer
- [x] Step 5: Domain models
- [x] Step 6: HAE ingest provider
- [x] Step 7: Alpha Progression provider
- [x] Step 8: HTTP server + middleware
- [x] Step 9: API endpoints
- [x] Step 10: Frontend
- [x] Step 11: Wire main.go

## Phase 1.5 Progress — CLI File Import

### Completed
- [x] Step 1: Updated hae-export-format.md spec with real JSON schemas
- [x] Step 2: .hae file format models (internal/models/hae_file.go)
- [x] Step 3: LZFSE decompression (internal/importer/decompress.go)
- [x] Step 4: File importer core logic (internal/importer/importer.go)
- [x] Step 5: Workout HR correlation (internal/importer/correlate.go)
- [x] Step 6: CLI entry point (cmd/freereps-import/main.go)
- [x] Step 7: Tests (hae_file_test.go + importer_test.go)

### In Progress
_(none)_

### Pending
- [ ] Integration test with real TimescaleDB + sample .hae files

### Known Bugs
_(none yet — needs integration testing with real TimescaleDB)_

### Notes
- Integration tests require running TimescaleDB (docker compose up db)
- The `web/dist` directory must exist for `go build` (frontend must be built first)
- Go upgraded to 1.24.0 due to golang-migrate v4.19.1 requiring it
- CLI usage: `freereps-import -config config.yaml -path /path/to/AutoSync [-dry-run]`
- Active energy .hae files contain dual units (kJ + kcal) — importer filters to kcal only
- Sleep stages in .hae files use field names (awake/core/deep/rem) not a "value" string
- HR field names in .hae files are lowercase (min/avg/max) unlike REST API (Min/Avg/Max)
- Workout HR correlation runs after all imports, only for workouts with no existing HR data
