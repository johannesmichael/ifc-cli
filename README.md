# ifc-to-db

A fast CLI tool that parses IFC (Industry Foundation Classes) files and writes their contents into a DuckDB database for SQL analysis — property queries, spatial navigation, relationship traversal, and geometry extraction.

## Why

Existing IFC tools (IfcOpenShell, xBIM) are designed for full read-write BIM workflows. They're slow and complex for pure extraction use cases. `ifc-to-db` does one thing well: extract IFC data into a queryable SQL database.

- Parses any valid IFC2x3, IFC4, or IFC4x3 file
- Streams large models with constant memory usage
- Produces a single DuckDB file queryable with standard SQL
- Ships as a single static binary with zero runtime dependencies

## Quick Start

```bash
# Build
make build

# Import an IFC file
./bin/ifc-to-db import model.ifc

# Query the result
./bin/ifc-to-db query model.duckdb "SELECT ifc_type, COUNT(*) FROM entities GROUP BY ifc_type ORDER BY 2 DESC LIMIT 10"
```

## Setup

### Prerequisites

- **Go 1.21+** (uses `slog` for structured logging)
- **CGO enabled** (required by the [go-duckdb](https://github.com/marcboeker/go-duckdb) driver)
- **GCC/Clang** (C compiler for CGO)

On macOS:
```bash
xcode-select --install
```

On Ubuntu/Debian:
```bash
sudo apt install build-essential
```

### Install from source

```bash
git clone https://github.com/jmrozanec/ifc-cli.git
cd ifc-cli
make build
# Binary at ./bin/ifc-to-db
```

### Verify

```bash
./bin/ifc-to-db version
```

## Usage

### Workflow

```
1. ifc-to-db info model.ifc                    # inspect the file
2. ifc-to-db import model.ifc -o model.duckdb  # parse and import
3. ifc-to-db schema --tables                   # see available tables
4. ifc-to-db query model.duckdb "SELECT ..."   # run queries
```

### Import

Parse an IFC file and write all data to DuckDB:

```bash
# Basic import (creates model.duckdb)
ifc-to-db import model.ifc

# Custom output path
ifc-to-db import model.ifc -o analysis.duckdb

# Skip heavy processing for faster import
ifc-to-db import model.ifc --skip-geometry --skip-relationships

# Only extract specific phases
ifc-to-db import model.ifc --only properties,spatial

# JSON output for scripting
ifc-to-db import model.ifc --output-format json

# In-memory analysis (no file written)
ifc-to-db import model.ifc --memory

# Quiet mode (errors only)
ifc-to-db import model.ifc -q
```

### Query

Run SQL directly against an imported database:

```bash
# All walls
ifc-to-db query model.duckdb "SELECT * FROM entities WHERE ifc_type = 'IFCWALL'"

# CSV export
ifc-to-db query model.duckdb "SELECT * FROM properties" --output-format csv

# JSON lines (one object per row)
ifc-to-db query model.duckdb "SELECT * FROM spatial_structure" --output-format jsonl

# SQL from file
ifc-to-db query model.duckdb --file analysis.sql
```

### Info

Quick file inspection without a full import:

```bash
ifc-to-db info model.ifc
ifc-to-db info model.ifc --output-format json
```

## Database Schema

After import, the DuckDB file contains these tables:

### `entities` — Raw parsed entities

| Column   | Type     | Description |
|----------|----------|-------------|
| id       | UINTEGER | Entity instance ID (#N) |
| ifc_type | VARCHAR  | Entity type (IFCWALL, IFCSLAB, ...) |
| attrs    | JSON     | Full attribute list as JSON array |

### `properties` — Denormalized property sets

| Column       | Type     | Description |
|--------------|----------|-------------|
| element_id   | UINTEGER | Owning element ID |
| element_type | VARCHAR  | Element type (IFCWALL, ...) |
| pset_name    | VARCHAR  | Property set name (Pset_WallCommon, ...) |
| prop_name    | VARCHAR  | Property name (FireRating, LoadBearing, ...) |
| prop_value   | VARCHAR  | Human-readable value |
| value_type   | VARCHAR  | IFC type (IFCLABEL, IFCBOOLEAN, ...) |
| unit         | VARCHAR  | Unit if specified |

### `quantities` — Element quantities

| Column         | Type     | Description |
|----------------|----------|-------------|
| element_id     | UINTEGER | Owning element ID |
| element_type   | VARCHAR  | Element type |
| qset_name      | VARCHAR  | Quantity set name |
| quantity_name   | VARCHAR  | Quantity name (Length, Area, Volume, ...) |
| quantity_value  | DOUBLE   | Numeric value |
| quantity_type   | VARCHAR  | Quantity type (IFCQUANTITYLENGTH, ...) |
| unit           | VARCHAR  | Unit if specified |

### `relationships` — All IFCREL* connections

| Column    | Type     | Description |
|-----------|----------|-------------|
| rel_id    | UINTEGER | Relationship entity ID |
| rel_type  | VARCHAR  | Relationship type (IFCRELAGGREGATES, ...) |
| source_id | UINTEGER | Source entity ID |
| target_id | UINTEGER | Target entity ID |
| context   | VARCHAR  | Relationship context |

### `spatial_structure` — Building hierarchy

| Column          | Type     | Description |
|-----------------|----------|-------------|
| element_id      | UINTEGER | Element ID |
| element_type    | VARCHAR  | Type (IFCPROJECT, IFCSITE, IFCBUILDING, ...) |
| element_name    | VARCHAR  | Element name |
| parent_id       | UINTEGER | Parent element ID |
| parent_type     | VARCHAR  | Parent type |
| hierarchy_level | INTEGER  | Depth (0=project, 1=site, 2=building, ...) |
| path            | VARCHAR  | Full path (Project/Site/Building A/Floor 2) |

### `geometry` — Geometry representations

| Column              | Type     | Description |
|---------------------|----------|-------------|
| element_id          | UINTEGER | Element ID |
| element_type        | VARCHAR  | Element type |
| representation_type | VARCHAR  | Geometry type (SweptSolid, Brep, ...) |
| representation_json | JSON     | Full geometry entity subtree |
| placement_json      | JSON     | Local placement chain |
| bounding_box_json   | JSON     | Axis-aligned bounding box (if computed) |

### `file_metadata` — Import metadata

| Column | Type    | Description |
|--------|---------|-------------|
| key    | VARCHAR | Metadata key |
| value  | VARCHAR | Metadata value |

Keys include: `schema_identifier`, `originating_system`, `file_name`, `timestamp`, `source_file`, `entity_count`, `error_count`, `import_time`.

## Query Cookbook

### Find all elements with a specific property

```sql
SELECT element_id, element_type, prop_value
FROM properties
WHERE prop_name = 'FireRating'
ORDER BY prop_value;
```

### List all property sets

```sql
SELECT DISTINCT pset_name, COUNT(*) as prop_count
FROM properties
GROUP BY pset_name
ORDER BY prop_count DESC;
```

### Elements on a specific floor

```sql
SELECT s.element_id, s.element_type, s.element_name
FROM spatial_structure s
WHERE s.path LIKE '%Floor 2%';
```

### Element count by type

```sql
SELECT ifc_type, COUNT(*) as n
FROM entities
GROUP BY ifc_type
ORDER BY n DESC
LIMIT 20;
```

### Find what contains a specific element

```sql
SELECT s.path, s.parent_type, s.parent_id
FROM spatial_structure s
WHERE s.element_id = 12345;
```

### All elements on a storey

```sql
SELECT s.element_id, s.element_type, s.element_name
FROM spatial_structure s
WHERE s.parent_type = 'IFCBUILDINGSTOREY'
  AND s.parent_id = (
    SELECT element_id FROM spatial_structure WHERE element_name = 'Level 1'
  );
```

### Elements with extruded geometry

```sql
SELECT element_id, element_type
FROM geometry
WHERE representation_type = 'SweptSolid';
```

### Cross-reference properties with spatial location

```sql
SELECT s.path, p.prop_name, p.prop_value
FROM properties p
JOIN spatial_structure s ON p.element_id = s.element_id
WHERE p.pset_name = 'Pset_WallCommon'
ORDER BY s.path;
```

## Build

```bash
make              # lint + test + build
make build        # build only
make test         # run all tests
make bench        # run benchmarks
make lint         # run linter (requires golangci-lint)
make docs         # generate man pages
make release      # cross-platform builds in dist/
make clean        # remove build artifacts
```

### Cross-platform builds

The `make release` target builds for:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

Binaries are placed in `dist/` with SHA-256 checksums.

### CI

GitHub Actions runs tests, linting, and builds on every push and PR. Tagged releases (`v*`) automatically create GitHub releases with platform binaries.

## Agent Integration

`ifc-to-db` is designed to be fully operable by autonomous agents:

```bash
# Discover all commands and flags as JSON
ifc-to-db --help-json

# Machine-readable import results
ifc-to-db import model.ifc --output-format json

# Structured query output
ifc-to-db query model.duckdb "SELECT ..." --output-format jsonl

# Exit codes: 0=success, 1=parse error, 2=file not found,
#             3=db error, 4=bad args, 5=partial success
```

### Shell Completions

```bash
# Bash
source <(ifc-to-db completion bash)

# Zsh
ifc-to-db completion zsh > "${fpath[1]}/_ifc-to-db"

# Fish
ifc-to-db completion fish | source
```

## Performance

Tested on a 24 MB real-world IFC file (375,787 entities):

| Phase | Throughput |
|-------|-----------|
| Lexer | ~423 MB/s |
| Parser | ~110 MB/s |
| Full import (with DB writes) | ~29 MB/s |

Target: >= 100 MB/s parsing throughput on commodity hardware.

## Architecture

```
IFC File (STEP format)
    |
    v
+----------------------+
|   STEP Lexer         |  Byte stream -> Token stream
|   (format-level)     |  No IFC knowledge required
+----------+-----------+
           |
           v
+----------------------+
|   Entity Parser      |  Token stream -> Entity structs
|   (format-level)     |  {ID, Type, Attributes[]}
+----------+-----------+
           |
           v
+----------------------+
|   DuckDB Writer      |  Entity structs -> raw entities table
|   (batch inserts)    |  Batched via Appender API
+----------+-----------+
           |
           v
+----------------------+
|   Post-Processors    |  SQL/Go transforms on raw entities
|   (IFC-aware)        |  -> properties, relationships, geometry
+----------------------+
```

### Package layout

```
ifc-cli/
  cmd/ifc-to-db/       CLI entry point
  internal/
    cli/                Command definitions, flags, output formatting
    step/               STEP lexer, parser, StepValue types
    db/                 DuckDB schema, writer, metadata
    extract/            Post-processors (properties, relationships, geometry)
    ifc/                IFC-specific helpers (string decoding)
  testdata/             Test fixtures
  scripts/              Build and doc generation scripts
```

## License

See [LICENSE](LICENSE) for details.
