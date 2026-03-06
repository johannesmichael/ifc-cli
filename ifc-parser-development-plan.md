# IFC-to-DB: Development Plan

## Project Overview

### Vision

A fast, focused command-line tool that parses IFC (Industry Foundation Classes) files and writes their contents into a DuckDB database for downstream analysis — property queries, schedule integration, spatial analysis, and eventual graph reconstruction. The tool deliberately avoids the complexity of full BIM toolkits (IfcOpenShell, xBIM) by doing one thing well: extraction.

### Problem Statement

Construction logistics and site planning workflows depend heavily on data locked inside IFC files. Existing tools (IfcOpenShell, xBIM) are designed for full read-write BIM workflows, making them slow and complex for pure extraction use cases. Teams that need to connect BIM data to timetables, schedules, and analytical pipelines spend more time fighting tooling than analyzing data.

### Goals

- Parse any valid IFC2x3 / IFC4 / IFC4x3 file without schema-specific hardcoding in the parser layer.
- Write all entity data, properties, relationships, and geometry into a DuckDB database.
- Achieve parsing throughput of at least 100 MB/s on commodity hardware.
- Provide a single static binary with zero runtime dependencies beyond the DuckDB file output.
- Enable property set queries, relationship traversal, and geometry extraction entirely through SQL after import.
- Support iterative enrichment — re-derive tables from raw entities without re-parsing the source file.

### Non-Goals (Explicit Exclusions)

- Editing or writing IFC files. This is a read-only tool.
- Geometry evaluation or rendering. Geometry is stored for later computation, not interpreted during import.
- Full EXPRESS schema validation. The parser accepts well-formed STEP files; it does not enforce IFC schema constraints.
- Real-time streaming or server mode. This is a batch CLI tool.

---

## Tech Stack

### Language: Go

**Rationale:** Faster iteration cycles than Rust during the exploratory phase. Strong standard library for file I/O, string processing, and CLI tooling. Static binary compilation for easy distribution across teams. Adequate performance for the parsing workload (the bottleneck will be I/O and database writes, not CPU-bound parsing). Good ecosystem for database drivers and future integrations (RDF stores, graph databases).

**Trade-off acknowledged:** Go's garbage collector may cause minor pauses during large file processing. Mitigated by minimizing allocations in the hot path (lexer/parser loop) and using buffer pools.

### Database: DuckDB

**Rationale:** Columnar analytical database optimized for the query patterns we care about — scanning large property tables, aggregating by element type, filtering by classification. Embeddable with no server process. Excellent JSON support for semi-structured IFC attribute data. SQL-native, making it accessible to non-programmers on the team.

**Driver:** `github.com/marcboeker/go-duckdb` (CGO-based, wraps the DuckDB C library).

**Build consideration:** CGO requirement means cross-compilation needs Docker or CI matrix builds. Plan for Linux (primary), macOS, and Windows targets.

### Future Integrations (Planned, Not Initial Scope)

- **Graph reconstruction:** Post-import SQL/Go process that builds a relationship graph from the `relationships` table. Could target SurrealDB, Neo4j, or an RDF triplestore depending on team needs.
- **Geometry computation:** Separate tool or library that reads geometry JSON from DuckDB and computes volumes, areas, bounding boxes. Candidates: Go bindings to OpenCascade, or a purpose-built evaluator for the subset of IFC geometry types that matter (extrusions, B-reps, tessellated geometry).
- **Schedule linking:** SQL views or materialized tables that join IFC element data with external schedule/timetable data (CSV, JSON, or direct database links).

---

## Architecture

### High-Level Data Flow

```
IFC File (STEP format)
    │
    ▼
┌──────────────────────┐
│   STEP Lexer         │  Byte stream → Token stream
│   (format-level)     │  No IFC knowledge required
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│   Entity Parser      │  Token stream → Entity structs
│   (format-level)     │  {ID, Type, Attributes[]}
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│   DuckDB Writer      │  Entity structs → raw `entities` table
│   (batch inserts)    │  Batched via Appender API
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│   Post-Processors    │  SQL/Go transforms on raw entities
│   (IFC-aware)        │  → properties, relationships, geometry
└──────────────────────┘
```

### Package Layout

```
ifc-to-db/
├── cmd/
│   └── ifc-to-db/
│       └── main.go              # CLI entry point (cobra root command, subcommand registration)
│
├── internal/
│   ├── cli/
│   │   ├── help.go              # --help-json generator, structured help output
│   │   ├── help_test.go
│   │   ├── output.go            # Output formatting (text, json, csv, jsonl)
│   │   └── exitcodes.go         # Exit code constants and documentation
│   │
│   ├── step/
│   │   ├── lexer.go             # STEP tokenizer
│   │   ├── lexer_test.go
│   │   ├── parser.go            # Entity-level parser
│   │   ├── parser_test.go
│   │   ├── tokens.go            # Token type definitions
│   │   └── values.go            # StepValue sum type and JSON serialization
│   │
│   ├── db/
│   │   ├── schema.go            # DDL statements, table creation
│   │   ├── writer.go            # Batch entity writer using Appender API
│   │   ├── writer_test.go
│   │   └── queries.go           # Post-processing SQL (property extraction, etc.)
│   │
│   ├── extract/
│   │   ├── properties.go        # Property set denormalization logic
│   │   ├── properties_test.go
│   │   ├── relationships.go     # IFCREL* → relationships table
│   │   ├── relationships_test.go
│   │   ├── geometry.go          # Geometry subtree serialization
│   │   ├── geometry_test.go
│   │   ├── spatial.go           # Spatial hierarchy (site/building/storey/space)
│   │   └── spatial_test.go
│   │
│   └── ifc/
│       ├── strings.go           # IFC string decoding (\X\, \X2\, \X4\, \S\)
│       ├── strings_test.go
│       ├── types.go             # IFC type name constants (for readability, not enforcement)
│       └── schema_hints.go      # Optional: attribute name lookup tables per schema version
│
├── testdata/
│   ├── minimal.ifc              # Hand-crafted minimal valid IFC file
│   ├── wall_with_properties.ifc # Single wall with property sets
│   ├── spatial_hierarchy.ifc    # Building → Storey → Space hierarchy
│   └── complex_geometry.ifc     # Various geometry representation types
│
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### Core Data Structures

```go
// internal/step/tokens.go

type TokenKind int

const (
    TokenEntityID    TokenKind = iota  // #123 at statement start
    TokenTypeName                       // IFCWALL, IFCPROPERTYSINGLEVALUE
    TokenString                         // 'Hello world'
    TokenInteger                        // 42, -7
    TokenFloat                          // 3.14, 1.5E-3
    TokenEnum                           // .ELEMENT., .T., .F.
    TokenRef                            // #456 inside attribute list
    TokenNull                           // $
    TokenDerived                        // *
    TokenLParen                         // (
    TokenRParen                         // )
    TokenComma                          // ,
    TokenSemicolon                      // ;
    TokenEquals                         // =
    TokenEOF
)

type Token struct {
    Kind  TokenKind
    Value string   // raw text (string content, number text, type name)
    Pos   int      // byte offset in source for error reporting
}
```

```go
// internal/step/values.go

type ValueKind int

const (
    KindString   ValueKind = iota
    KindInteger
    KindFloat
    KindEnum
    KindRef
    KindList
    KindTyped    // e.g. IFCLENGTHMEASURE(2.5)
    KindNull
    KindDerived
)

type StepValue struct {
    Kind     ValueKind
    Str      string       // for KindString, KindEnum, KindTyped (type name)
    Int      int64        // for KindInteger
    Float    float64      // for KindFloat
    Ref      uint64       // for KindRef
    List     []StepValue  // for KindList
    Inner    *StepValue   // for KindTyped (the wrapped value)
}

// Entity represents a single parsed STEP entity instance
type Entity struct {
    ID    uint64
    Type  string
    Attrs []StepValue
}
```

---

## Database Schema

### Raw Entity Store (Populated During Parse)

```sql
CREATE TABLE entities (
    id          UINTEGER PRIMARY KEY,
    ifc_type    VARCHAR NOT NULL,
    attrs       JSON NOT NULL
);

CREATE INDEX idx_entity_type ON entities(ifc_type);
```

### Denormalized Property Store (Post-Processing)

```sql
CREATE TABLE properties (
    element_id      UINTEGER NOT NULL,
    element_type    VARCHAR NOT NULL,
    pset_name       VARCHAR NOT NULL,
    prop_name       VARCHAR NOT NULL,
    prop_value      VARCHAR,
    value_type      VARCHAR,
    unit            VARCHAR
);

CREATE INDEX idx_prop_element ON properties(element_id);
CREATE INDEX idx_prop_pset ON properties(pset_name);
CREATE INDEX idx_prop_name ON properties(prop_name);
CREATE INDEX idx_prop_element_type ON properties(element_type);
```

### Quantity Store (Post-Processing)

```sql
CREATE TABLE quantities (
    element_id      UINTEGER NOT NULL,
    element_type    VARCHAR NOT NULL,
    qset_name       VARCHAR NOT NULL,
    quantity_name   VARCHAR NOT NULL,
    quantity_value  DOUBLE,
    quantity_type   VARCHAR,
    unit            VARCHAR
);
```

### Relationship Store (Post-Processing)

```sql
CREATE TABLE relationships (
    rel_id          UINTEGER NOT NULL,
    rel_type        VARCHAR NOT NULL,
    source_id       UINTEGER NOT NULL,
    target_id       UINTEGER NOT NULL,
    context         VARCHAR
);

CREATE INDEX idx_rel_source ON relationships(source_id);
CREATE INDEX idx_rel_target ON relationships(target_id);
CREATE INDEX idx_rel_type ON relationships(rel_type);
```

### Spatial Hierarchy (Post-Processing)

```sql
CREATE TABLE spatial_structure (
    element_id      UINTEGER NOT NULL,
    element_type    VARCHAR NOT NULL,
    element_name    VARCHAR,
    parent_id       UINTEGER,
    parent_type     VARCHAR,
    hierarchy_level INTEGER,
    path            VARCHAR
);
```

### Geometry Store (Post-Processing)

```sql
CREATE TABLE geometry (
    element_id           UINTEGER PRIMARY KEY,
    element_type         VARCHAR NOT NULL,
    representation_type  VARCHAR,
    representation_json  JSON,
    placement_json       JSON,
    bounding_box_json    JSON
);
```

### File Metadata

```sql
CREATE TABLE file_metadata (
    key    VARCHAR PRIMARY KEY,
    value  VARCHAR
);
-- Contains: schema version, originating application, timestamp,
-- file name, file size, entity count, parse duration
```

---

## Development Phases

### Phase 1: STEP Lexer and Entity Parser

**Objective:** Read any STEP/IFC file and produce a stream of `Entity` structs. No IFC knowledge, no database, just parsing.

**Tasks:**

1.1. **Project scaffolding**
   - Initialize Go module (`ifc-to-db`)
   - Set up directory structure as defined in architecture
   - Configure `Makefile` with `build`, `test`, `lint`, `bench` targets
   - Add `.golangci-lint.yml` with strict settings

1.2. **Token definitions and lexer skeleton**
   - Define `TokenKind` enum and `Token` struct
   - Implement `Lexer` struct that wraps a `[]byte` source with a cursor position
   - Implement `NextToken() (Token, error)` as the core lexer method
   - Handle comment lines (lines starting with `/*` or within `/* ... */` blocks)

1.3. **Lexer: numeric literals**
   - Integer parsing: optional sign, digit sequence
   - Float parsing: optional sign, digit sequence with decimal point and/or exponent (`E`, `e`)
   - Edge case: distinguish negative numbers from other uses of `-`

1.4. **Lexer: string literals**
   - Single-quote delimited strings
   - Escaped single quotes via `''`
   - Multi-line strings (string content can span lines)
   - Raw byte preservation (defer IFC encoding to a later step)

1.5. **Lexer: IFC string decoding**
   - `\S\` directive (ISO 8859 encoding shift)
   - `\X\` directive (single hex-encoded byte)
   - `\X2\` ... `\X0\` directive (UCS-2 hex-encoded characters)
   - `\X4\` ... `\X0\` directive (UCS-4 hex-encoded characters)
   - `\\` escape for literal backslash
   - Unit tests with known encoded strings from real IFC files

1.6. **Lexer: remaining token types**
   - Entity IDs and references (`#` followed by digits)
   - Type names (uppercase letters, digits, underscores)
   - Enum values (`.` delimited, e.g., `.ELEMENT.`, `.T.`, `.F.`, `.NOTDEFINED.`)
   - Structural tokens: `(`, `)`, `,`, `;`, `=`
   - Special values: `$` (null), `*` (derived)

1.7. **Lexer: whitespace and header handling**
   - Skip whitespace between tokens (space, tab, newline, carriage return)
   - Handle the STEP file header section (`ISO-10303-21;`, `HEADER;`, `ENDSEC;`, `DATA;`, `END-ISO-10303-21;`)
   - Extract header metadata (file description, file name, file schema) as separate entities or metadata struct

1.8. **Entity parser**
   - Parse statement pattern: `#ID = TYPENAME ( attr, attr, ... ) ;`
   - Build `StepValue` tree for attribute lists (recursive for nested lists and typed values)
   - Handle typed attribute values like `IFCLENGTHMEASURE(2.5)`
   - Handle empty lists `()`
   - Handle entity references within attribute lists

1.9. **Entity parser: error handling and recovery**
   - Report errors with byte offset and line number
   - On malformed entity: log warning, skip to next semicolon, continue parsing
   - Track statistics: total entities parsed, errors encountered, entities by type

1.10. **Stream interface**
   - Implement `Parser` struct with `Next() (*Entity, error)` method
   - Return `io.EOF` at end of data section
   - Zero-copy where possible: use byte slices referencing the memory-mapped source

1.11. **Testing and validation**
   - Unit tests for each token type
   - Unit tests for nested attribute lists, typed values, edge cases
   - Integration test: parse a minimal IFC file, verify entity count and types
   - Integration test: parse a real-world IFC file (find or create a small public sample)
   - Fuzz testing on the lexer with `go test -fuzz`

1.12. **Performance baseline**
   - Benchmark: parse a 100 MB IFC file, measure time and memory
   - Profile with `pprof`, identify allocation hotspots
   - Implement string interning for type names (there are only ~800 distinct IFC types)
   - Target: 100 MB/s parsing throughput

**Deliverable:** `ifc-to-db parse --dry-run <file.ifc>` prints entity count, type histogram, and parse duration.

---

### Phase 2: DuckDB Writer and Raw Entity Import

**Objective:** Write parsed entities into a DuckDB database, populating the `entities` table and `file_metadata`.

**Tasks:**

2.1. **DuckDB integration setup**
   - Add `go-duckdb` dependency
   - Implement `db.Open(path string) (*Database, error)` that creates or opens a DuckDB file
   - Run schema DDL on first open (create all tables if not exist)
   - Handle CGO build configuration in Makefile

2.2. **Batch writer using Appender API**
   - Implement `Writer` struct wrapping DuckDB appender for the `entities` table
   - Batch size configuration (default: 10,000 entities per flush)
   - `StepValue` to JSON serialization for the `attrs` column
   - Automatic flush on batch size threshold and on close

2.3. **JSON serialization of StepValue**
   - Define JSON representation:
     - `KindString` → `"value"`
     - `KindInteger` → `42`
     - `KindFloat` → `3.14`
     - `KindEnum` → `{"enum": "ELEMENT"}`
     - `KindRef` → `{"ref": 123}`
     - `KindList` → `[...]`
     - `KindTyped` → `{"type": "IFCLENGTHMEASURE", "value": 2.5}`
     - `KindNull` → `null`
     - `KindDerived` → `{"derived": true}`
   - Use `json.Marshal` with custom marshaler, benchmark against manual byte-buffer writing
   - If manual is 2x+ faster, use manual approach for the hot path

2.4. **File metadata extraction**
   - Parse STEP header section for: FILE_DESCRIPTION, FILE_NAME, FILE_SCHEMA
   - Record: source filename, file size, schema identifier (e.g., `IFC4`), originating application
   - Record: parse start time, parse duration, entity count
   - Write to `file_metadata` table as key-value pairs

2.5. **Progress reporting**
   - Byte-offset-based progress (percentage of file parsed)
   - Entity count and current parse rate (entities/sec, MB/s)
   - Optional: `--quiet` flag to suppress progress output
   - Optional: `--json-progress` flag for machine-readable progress (for integration with other tools)

2.6. **End-to-end pipeline**
   - Wire lexer → parser → writer in the CLI command
   - Memory-map the input file using `golang.org/x/exp/mmap` or `syscall.Mmap`
   - Handle graceful shutdown (SIGINT) — flush pending batch, close database cleanly

2.7. **Testing**
   - Test: parse and import a small IFC file, verify entity count in DB matches parse count
   - Test: JSON roundtrip — serialize StepValue to JSON, verify it can be queried with DuckDB JSON functions
   - Test: multiple imports into same database (should this append or replace? — default to replace with `--append` flag)
   - Benchmark: import a 100 MB file, measure total time including DB writes

**Deliverable:** `ifc-to-db import <file.ifc> -o <output.duckdb>` creates a database with populated `entities` and `file_metadata` tables.

---

### Phase 3: Property Set Denormalization

**Objective:** Extract all property sets and quantity sets from the raw entity table into flat, queryable tables.

**Tasks:**

3.1. **IFCRELDEFINESBYPROPERTIES resolver**
   - Query `entities` for all rows where `ifc_type = 'IFCRELDEFINESBYPROPERTIES'`
   - For each rel entity: extract the list of related element refs (attr position 4) and the property set ref (attr position 5)
   - Build an in-memory map: `propertySetID → []elementID`

3.2. **IFCPROPERTYSET unpacker**
   - Query `entities` for all property set IDs found in the previous step
   - Extract: property set name (attr position 2), list of property refs (attr position 4)
   - For each property ref, resolve to the concrete property entity

3.3. **Property value extraction**
   - Handle `IFCPROPERTYSINGLEVALUE`: name, nominal value (typed), unit
   - Handle `IFCPROPERTYENUMERATEDVALUE`: name, list of enum values
   - Handle `IFCPROPERTYLISTVALUE`: name, list of values
   - Handle `IFCPROPERTYBOUNDEDVALUE`: name, upper/lower bounds
   - Handle `IFCPROPERTYTABLEVALUE`: name, defining/defined value lists
   - Handle `IFCPROPERTYREFERENCEVALUE`: name, reference to another entity
   - For each: extract the human-readable value as a string, preserve the IFC type name

3.4. **IFCELEMENTQUANTITY unpacker**
   - Same pattern as property sets but for quantity sets
   - Handle `IFCQUANTITYLENGTH`, `IFCQUANTITYAREA`, `IFCQUANTITYVOLUME`, `IFCQUANTITYWEIGHT`, `IFCQUANTITYCOUNT`, `IFCQUANTITYTIME`
   - Extract numeric value and unit

3.5. **IFCRELDEFINESBYTYPE property inheritance**
   - Elements can inherit properties from their type object (`IFCWALLTYPE`, `IFCSLABTYPE`, etc.)
   - Query `IFCRELDEFINESBYTYPE` to find type → element mappings
   - Merge type-level properties with element-level properties (element-level overrides type-level)
   - Flag source of each property: `'instance'` or `'type'`

3.6. **Material extraction**
   - Query `IFCRELASSOCIATESMATERIAL` for material assignments
   - Handle `IFCMATERIAL`, `IFCMATERIALLAYERSET`, `IFCMATERIALLAYERSETUSAGE`
   - Extract material names, layer thicknesses, layer ordering
   - Store in a dedicated `materials` table or as additional properties

3.7. **Classification extraction**
   - Query `IFCRELASSOCIATESCLASSIFICATION` for classification references
   - Extract classification system name (e.g., Uniclass, OmniClass) and item reference
   - Store in a dedicated `classifications` table or as additional properties

3.8. **Write to properties and quantities tables**
   - Batch insert resolved properties into `properties` table
   - Batch insert resolved quantities into `quantities` table
   - Include element type in each row for easier filtering

3.9. **Testing**
   - Test with IFC files containing known property sets, verify exact values
   - Test property inheritance from type objects
   - Test edge cases: empty property sets, properties with null values, properties referencing other entities
   - Performance test: property extraction on a large file (target: < 5 seconds for 100k elements)

**Deliverable:** After import, `SELECT * FROM properties WHERE prop_name = 'FireRating'` returns all elements with fire rating data. `ifc-to-db import` now populates `properties` and `quantities` automatically after raw import.

---

### Phase 4: Relationship and Spatial Hierarchy Extraction

**Objective:** Populate the `relationships` table with all IFCREL* entity connections and build the spatial hierarchy tree.

**Tasks:**

4.1. **Generic IFCREL* extraction**
   - Identify all entity types starting with `IFCREL` in the raw entities table
   - For each rel type, determine the source and target attribute positions:
     - `IFCRELAGGREGATES`: relating object (pos 4) → related objects (pos 5)
     - `IFCRELCONTAINEDINSPATIALSTRUCTURE`: related elements (pos 4) → relating structure (pos 5)
     - `IFCRELCONNECTSPATHELEMENTS`: relating element (pos 4) → related element (pos 5)
     - `IFCRELVOIDSELEMENT`: relating building element (pos 4) → related opening element (pos 5)
     - `IFCRELFILLSELEMENT`: relating opening element (pos 4) → related building element (pos 5)
     - `IFCRELASSIGNSTASKS`, `IFCRELASSIGNSTOGROUP`: for schedule/group connections
   - Create a configuration map of rel types to their source/target attribute positions
   - Extract and insert into `relationships` table

4.2. **Spatial hierarchy construction**
   - Query `IFCRELAGGREGATES` to build the tree: `IFCPROJECT` → `IFCSITE` → `IFCBUILDING` → `IFCBUILDINGSTOREY` → `IFCSPACE`
   - Query `IFCRELCONTAINEDINSPATIALSTRUCTURE` to assign elements to their containing storey/space
   - Build a path string for each element (e.g., `Project/Site/Building A/Floor 2/Room 201`)
   - Compute hierarchy level (0 = project, 1 = site, etc.)
   - Write to `spatial_structure` table

4.3. **Group and system extraction**
   - Query `IFCRELASSIGNSTOGROUP` for group memberships (e.g., distribution systems, zones)
   - Store group relationships in the relationships table with appropriate context

4.4. **Testing**
   - Test spatial hierarchy with a multi-storey building
   - Test relationship extraction with known connection patterns
   - Verify bidirectional traversal: given an element, find its container; given a storey, find its elements

**Deliverable:** `SELECT * FROM spatial_structure WHERE parent_type = 'IFCBUILDINGSTOREY' AND element_name LIKE '%Floor 2%'` returns all elements on Floor 2.

---

### Phase 5: Geometry Serialization

**Objective:** Serialize each element's geometry representation and placement as JSON blobs for later computation.

**Tasks:**

5.1. **Representation resolver**
   - For each building element, follow the `Representation` attribute to `IFCPRODUCTDEFINITIONSHAPE`
   - From there, follow to `IFCSHAPEREPRESENTATION` entries
   - For each shape representation, identify the type: `SweptSolid`, `Brep`, `Tessellation`, `MappedRepresentation`, `Clipping`, `CSG`

5.2. **Entity subtree collector**
   - Given a root entity ID (the shape representation), recursively collect all referenced entities
   - Build a JSON tree preserving the structure: each node has `id`, `type`, `attrs` (with refs resolved to nested objects)
   - Handle cycles (shouldn't occur in valid IFC but defend against it)
   - Set a depth limit (e.g., 50 levels) to prevent runaway recursion

5.3. **Placement chain resolver**
   - Follow `ObjectPlacement` → `IFCLOCALPLACEMENT` → parent placement chain
   - Each placement has an `IFCAXIS2PLACEMENT3D` with location, axis, and ref direction
   - Serialize the full placement chain as JSON
   - Optionally: compute the composed 4x4 transformation matrix (world coordinates)

5.4. **Bounding box computation (optional, deferred)**
   - For simple geometry types (extrusions, boxes), compute axis-aligned bounding box
   - Store as `{"min": [x, y, z], "max": [x, y, z]}`
   - Skip for complex types (B-rep, CSG) — these require a geometry kernel

5.5. **Write to geometry table**
   - Insert one row per element with geometry
   - Include representation type as a queryable column for filtering

5.6. **Testing**
   - Test with extruded solid geometry (most common in IFC)
   - Test placement chain resolution with nested local placements
   - Test MappedRepresentation (type-level geometry shared across instances)
   - Verify JSON output is valid and re-parseable

**Deliverable:** `SELECT element_id, representation_type FROM geometry WHERE representation_type = 'SweptSolid'` returns all elements with extruded geometry, with full geometry trees available in JSON.

---

### Phase 6: CLI Polish, Help System, and Distribution

**Objective:** Production-ready CLI with comprehensive, machine-readable help documentation on every command, flag, and parameter — designed to be fully operable by autonomous agents without prior knowledge. Proper error handling, structured output, and cross-platform builds.

**Tasks:**

6.1. **CLI framework and command structure**
   - Use `cobra` (github.com/spf13/cobra) as the CLI framework for built-in help generation, subcommand support, and flag parsing
   - Root command: `ifc-to-db` — displays full help overview, lists all subcommands
   - Subcommands:
     - `ifc-to-db import <file.ifc> [flags]` — full import pipeline
     - `ifc-to-db info <file.ifc> [flags]` — quick file inspection without full import
     - `ifc-to-db query <database.duckdb> <sql> [flags]` — run SQL against an output database
     - `ifc-to-db schema [flags]` — print the DuckDB schema DDL (so agents know what tables and columns exist)
     - `ifc-to-db version` — version, build info, Go version, DuckDB version
   - Every command must be invocable with `--help` to produce its full documentation

6.2. **Import command flags (detailed)**
   - `-o, --output <path>` — output DuckDB file path (default: `<input_name>.duckdb`)
   - `--memory` — use in-memory DuckDB instead of writing a file (for throwaway analysis; output lost on exit)
   - `--skip-properties` — skip property set denormalization (only populate raw `entities` table)
   - `--skip-geometry` — skip geometry subtree serialization
   - `--skip-relationships` — skip relationship and spatial hierarchy extraction
   - `--skip-quantities` — skip quantity set extraction
   - `--only <phase,...>` — run only specified post-processing phases (values: `properties`, `quantities`, `relationships`, `spatial`, `geometry`). Mutually exclusive with `--skip-*` flags.
   - `--append` — append to existing database instead of replacing. Requires existing database at output path.
   - `--batch-size <n>` — entities per write batch (default: 10000, valid range: 100–1000000)
   - `-q, --quiet` — suppress all progress output, only emit errors
   - `-v, --verbose` — detailed logging including per-phase timing and entity counts
   - `--log-file <path>` — write structured log to file in addition to stderr
   - `--output-format <format>` — format for summary output: `text` (default, human-readable), `json` (machine-readable summary on stdout after completion)
   - Each flag must include in its help string: the purpose, the default value, valid values or range, and an example if non-obvious

6.3. **Info command flags (detailed)**
   - `--output-format <format>` — `text` (default) or `json`
   - JSON output schema includes: `schema_version`, `originating_application`, `entity_count`, `type_histogram` (map of type name → count), `file_size_bytes`, `has_properties` (bool), `has_geometry` (bool)
   - Useful for agents to pre-inspect a file before deciding whether to run a full import

6.4. **Query command flags (detailed)**
   - `--output-format <format>` — `table` (default, aligned columns), `csv`, `json`, `jsonl` (one JSON object per row)
   - `--header / --no-header` — include/exclude column headers (default: include)
   - `--null-value <string>` — string to represent NULL values in output (default: empty string)
   - Accepts SQL as positional argument or via `--file <path>` for longer queries
   - Exit code 0 on success, 1 on SQL error, 2 on connection/file error

6.5. **Schema command**
   - `ifc-to-db schema` — print full DDL for all tables the tool creates
   - `ifc-to-db schema --table <name>` — print DDL for a specific table
   - `ifc-to-db schema --tables` — list all table names, one per line
   - `ifc-to-db schema --columns <table>` — list columns for a table with name, type, and description
   - `ifc-to-db schema --output-format json` — JSON output of the full schema definition including column descriptions
   - This is critical for agents: they need to know what tables exist, what columns are available, and what types they are, without having to introspect the database directly

6.6. **Comprehensive help text requirements**
   - Every command's `--help` output must include:
     - A one-line summary (what this command does)
     - A description paragraph (when to use it, what it produces)
     - Usage syntax with argument placeholders
     - Full flag listing with defaults, types, valid ranges, and constraints
     - At least two usage examples showing common invocations
     - Exit code documentation
     - Related commands (e.g., `import --help` mentions `schema` and `query`)
   - The root `--help` must include a workflow overview showing the typical command sequence:
     ```
     Typical workflow:
       1. ifc-to-db info model.ifc                    # inspect the file
       2. ifc-to-db import model.ifc -o model.duckdb  # parse and import
       3. ifc-to-db schema --tables                   # see available tables
       4. ifc-to-db query model.duckdb "SELECT ..."   # run queries
     ```
   - Flag grouping: flags should be grouped by category in help output (Input/Output, Processing, Logging, Format)

6.7. **Machine-readable help and discoverability**
   - `ifc-to-db --help-json` — output the entire CLI structure as JSON: all commands, all flags with metadata (name, short name, type, default, description, valid values, required/optional)
   - This is the primary interface for autonomous agents to discover capabilities
   - JSON help schema:
     ```json
     {
       "name": "ifc-to-db",
       "version": "0.1.0",
       "description": "...",
       "commands": [
         {
           "name": "import",
           "summary": "Parse an IFC file and write contents to DuckDB",
           "description": "...",
           "usage": "ifc-to-db import <file.ifc> [flags]",
           "arguments": [
             {"name": "file", "type": "path", "required": true, "description": "Path to IFC file"}
           ],
           "flags": [
             {
               "name": "output",
               "short": "o",
               "type": "path",
               "default": "<input_name>.duckdb",
               "required": false,
               "description": "Output DuckDB file path"
             }
           ],
           "examples": ["..."],
           "exit_codes": {"0": "success", "1": "parse error", "2": "file not found"}
         }
       ]
     }
     ```

6.8. **Structured output for agent consumption**
   - Import command with `--output-format json` emits a summary object on stdout upon completion:
     ```json
     {
       "status": "success",
       "input_file": "model.ifc",
       "output_file": "model.duckdb",
       "schema_version": "IFC4",
       "duration_ms": 12340,
       "entities_parsed": 158432,
       "entities_errored": 3,
       "tables_populated": ["entities", "properties", "quantities", "relationships", "spatial_structure", "geometry"],
       "row_counts": {
         "entities": 158432,
         "properties": 423891,
         "quantities": 12044,
         "relationships": 34221,
         "spatial_structure": 1203,
         "geometry": 8842
       },
       "phases": [
         {"name": "parse", "duration_ms": 4200, "status": "success"},
         {"name": "properties", "duration_ms": 3100, "status": "success"},
         {"name": "geometry", "duration_ms": 2800, "status": "success"}
       ],
       "warnings": ["3 entities skipped due to malformed attributes"],
       "errors": []
     }
     ```
   - All error output goes to stderr, structured output goes to stdout — agents can capture them separately
   - Non-zero exit codes for every failure category:
     - 0: success
     - 1: parse error (file is malformed or unreadable)
     - 2: file not found or permission denied
     - 3: database error (DuckDB write failure)
     - 4: invalid arguments (bad flags, missing required args)
     - 5: partial success (import completed with warnings/skipped entities)

6.9. **Error handling hardening**
   - Structured error types with context (file position, entity ID, phase)
   - Graceful degradation: if property extraction fails for one element, log and continue
   - Summary report at end: entities parsed, properties extracted, errors encountered, time per phase
   - Error output in JSON format when `--output-format json` is set

6.10. **Logging**
   - Structured logging with `slog` (Go 1.21+)
   - Levels: error (always), warn (default), info (verbose), debug (development)
   - Log file option: `--log-file <path>`
   - Log entries as JSON when `--output-format json` is active

6.11. **Man page and completion generation**
   - Generate man pages from cobra command definitions (`cobra-doc`)
   - Generate shell completions:
     - `ifc-to-db completion bash` — Bash completion script
     - `ifc-to-db completion zsh` — Zsh completion script
     - `ifc-to-db completion fish` — Fish completion script
     - `ifc-to-db completion powershell` — PowerShell completion script
   - Completions should include flag values where applicable (e.g., `--output-format` completes to `text`, `json`, `csv`)

6.12. **Cross-platform builds**
   - CI pipeline (GitHub Actions) building for:
     - Linux amd64, arm64
     - macOS amd64 (Intel), arm64 (Apple Silicon)
     - Windows amd64
   - DuckDB static linking for each platform
   - Release artifacts as compressed archives with checksums

6.13. **Documentation**
   - README with quick start, usage examples, and query cookbook
   - Query cookbook: common SQL patterns for property queries, spatial filtering, element counting
   - Architecture document (this plan, updated post-implementation)
   - CLI reference document auto-generated from the `--help-json` output
   - Agent integration guide: how to use `--help-json`, `--output-format json`, and exit codes for automation

6.14. **Testing**
   - End-to-end test: import a real IFC file, run a set of expected queries, verify results
   - Help text tests: assert every command has examples, every flag has a default documented, `--help-json` is valid JSON
   - Output format tests: verify JSON output is parseable and matches documented schema
   - Exit code tests: verify correct exit codes for success, parse error, missing file, bad arguments
   - Performance regression test: import benchmark file, assert time < threshold
   - Cross-platform smoke tests in CI

**Deliverable:** Downloadable binaries on GitHub Releases. `ifc-to-db import model.ifc && duckdb model.duckdb "SELECT * FROM properties LIMIT 10"` works out of the box. `ifc-to-db --help-json` gives an autonomous agent everything it needs to operate the tool without documentation lookup.

---

## Milestones and Success Criteria

| Milestone | Phase | Success Criteria |
|-----------|-------|------------------|
| Parser works | 1 | Parses 5 real-world IFC files (IFC2x3 + IFC4) with zero errors. Throughput ≥ 100 MB/s. |
| Raw import works | 2 | Imports a 500 MB IFC file in under 30 seconds. All entities queryable via SQL. |
| Properties queryable | 3 | All property sets and quantity sets for all elements accessible via flat SQL queries. |
| Relationships navigable | 4 | Spatial hierarchy reconstructed. Any element traceable to its building storey. |
| Geometry stored | 5 | Geometry JSON available for all elements with representation. Placement chains resolved. |
| CLI shippable | 6 | Single binary, cross-platform, documented, tested. `--help-json` exposes full CLI contract. `--output-format json` on all commands. Every flag documented with type, default, and range. Autonomous agents can discover and operate all commands without external docs. Usable by team members without development setup. |

---

## Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| go-duckdb CGO issues on Windows | Medium | High | Pre-build DuckDB lib per platform in CI. Fallback: SQLite driver as alternative backend. |
| IFC files with non-standard STEP formatting | Medium | Medium | Lenient parser with error recovery. Collect malformed files as test corpus. |
| Memory pressure on very large files (> 2 GB) | Low | High | Memory-map input. Stream parsing (never hold full file in memory). Batch DB writes. |
| DuckDB JSON query performance for geometry trees | Medium | Low | JSON is stored for external consumption, not queried internally. Keep geometry table as blob store. |
| Property extraction logic varies across IFC schema versions | Medium | Medium | Start with IFC4, add IFC2x3 attribute position differences as configuration. Test against both. |
| go-duckdb driver instability or breaking changes | Low | Medium | Pin dependency version. Maintain integration test suite. Evaluate `github.com/marcboeker/go-duckdb` vs alternatives. |

---

## Testing Strategy

### Unit Tests
- Lexer: one test per token kind, edge cases (empty strings, max-length numbers, deeply nested lists)
- Parser: one test per entity pattern (simple, nested lists, typed values, multiple refs)
- String decoder: one test per encoding directive
- StepValue JSON serialization: roundtrip tests

### Integration Tests
- Parse → import → query pipeline with small IFC fixtures
- Property extraction correctness against known property values
- Spatial hierarchy correctness against known building structure

### Fuzz Tests
- Lexer fuzzing with `go test -fuzz` to find crash-inducing inputs
- Parser fuzzing with semi-valid STEP fragments

### Performance Tests
- Parsing throughput benchmark (MB/s)
- Import throughput benchmark (entities/sec)
- Property extraction benchmark (elements/sec)
- Memory usage profiling for large files

### Test Data
- Hand-crafted minimal fixtures in `testdata/`
- Real-world IFC sample files (public domain or from team's own projects)
- Generated stress-test files with high entity counts

---

## Open Questions for Future Decisions

1. **Schema-aware attribute naming:** Should the tool ship with embedded attribute name tables for IFC2x3/IFC4/IFC4x3, or should users provide them? Embedded is more convenient; external is more maintainable.

2. **Graph database target:** When the team is ready for graph reconstruction, which store? SurrealDB (all-in-one but young), Neo4j (mature but operational overhead), RDF triplestore (standards-based, good for linked data)? Decision deferred to post-Phase 4.

3. **Geometry evaluation:** Is there a need to compute actual volumes/areas during import, or is storing the geometry tree sufficient? If computation is needed, evaluate Go bindings for OpenCascade vs. implementing a minimal evaluator for common geometry types (extrusions, tessellated geometry).

4. **Schedule integration format:** How are timetables and schedules currently stored? CSV, MS Project XML, Primavera? This determines how to build the join layer between IFC elements and schedule activities.

5. **Multi-file support:** Should the tool support importing multiple IFC files into the same database (e.g., architectural + structural + MEP models)? This requires handling entity ID collisions across files.
