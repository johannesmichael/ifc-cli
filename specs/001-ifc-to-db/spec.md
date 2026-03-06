# Feature Specification: IFC-to-DB Parser & Importer

**Feature Branch**: `001-ifc-to-db`
**Created**: 2026-03-06
**Status**: Draft
**Input**: Development plan: `ifc-parser-development-plan.md`

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Parse IFC Files (Priority: P1)

As a construction data analyst, I want to parse any valid IFC file (IFC2x3, IFC4, IFC4x3) into a stream of structured entities so that I can extract data without depending on heavyweight BIM toolkits.

**Why this priority**: Parsing is the foundation. Nothing else works without a correct, fast STEP lexer and entity parser.

**Independent Test**: Parse 5 real-world IFC files of varying schema versions with zero errors. Verify entity counts match expected values. Measure throughput >= 100 MB/s.

**Acceptance Scenarios**:

1. **Given** a valid IFC2x3 file, **When** I run `ifc-to-db import --dry-run model.ifc`, **Then** I see entity count, type histogram, and parse duration on stdout with exit code 0.
2. **Given** a valid IFC4 file with encoded strings (\X2\, \S\), **When** I parse it, **Then** all string values are correctly decoded to UTF-8.
3. **Given** a malformed IFC file with 3 broken entities among 10,000 valid ones, **When** I parse it, **Then** the parser recovers, processes all valid entities, and reports 3 errors.
4. **Given** a 100 MB IFC file, **When** I parse it, **Then** throughput is >= 100 MB/s on commodity hardware.

---

### User Story 2 - Import to DuckDB (Priority: P1)

As a data analyst, I want to import parsed IFC entities into a DuckDB database so that I can query building data using standard SQL.

**Why this priority**: The database is the primary output of the tool. Raw entity storage enables all downstream queries.

**Independent Test**: Import a 500 MB IFC file in under 30 seconds. Verify all entities are queryable via `SELECT * FROM entities WHERE ifc_type = 'IFCWALL'`.

**Acceptance Scenarios**:

1. **Given** a parsed IFC file, **When** I run `ifc-to-db import model.ifc -o model.duckdb`, **Then** a DuckDB file is created with `entities` and `file_metadata` tables populated.
2. **Given** a large IFC file, **When** I import it, **Then** I see byte-level progress reporting (percentage, entities/sec, MB/s).
3. **Given** an import in progress, **When** I send SIGINT, **Then** the current batch is flushed and the database is closed cleanly.
4. **Given** entity attributes with refs, typed values, and nested lists, **When** I query the `attrs` JSON column, **Then** DuckDB JSON functions correctly extract all value types.

---

### User Story 3 - Query Properties (Priority: P2)

As a construction planner, I want to query element properties (fire rating, load bearing, dimensions) using flat SQL tables so that I don't have to navigate IFC's complex property set indirection.

**Why this priority**: Property queries are the most common use case. Denormalization makes IFC data accessible to non-BIM-experts.

**Independent Test**: After import, `SELECT * FROM properties WHERE prop_name = 'FireRating'` returns all elements with fire rating data with correct values.

**Acceptance Scenarios**:

1. **Given** an imported IFC file with property sets, **When** I query the `properties` table, **Then** I see one row per property per element with pset_name, prop_name, prop_value, value_type, and unit.
2. **Given** elements that inherit properties from type objects (IFCWALLTYPE), **When** I query properties, **Then** I see both instance-level and type-level properties with a source indicator.
3. **Given** elements with quantity sets, **When** I query the `quantities` table, **Then** I see numeric values with units for length, area, volume, weight, count, and time quantities.
4. **Given** elements with material and classification assignments, **When** I query, **Then** material names, layer data, and classification references are accessible.

---

### User Story 4 - Navigate Spatial Hierarchy (Priority: P2)

As a site logistics planner, I want to navigate the spatial hierarchy (Project > Site > Building > Storey > Space) and find which elements are on which floor so that I can plan logistics per zone.

**Why this priority**: Spatial context is essential for construction planning. Knowing "all walls on Floor 2" is a fundamental query.

**Independent Test**: `SELECT * FROM spatial_structure WHERE parent_type = 'IFCBUILDINGSTOREY'` returns all elements with their containing storey and full path.

**Acceptance Scenarios**:

1. **Given** an imported multi-storey building, **When** I query `spatial_structure`, **Then** each element has a parent_id, hierarchy_level, and full path string (e.g., "Project/Site/Building A/Floor 2").
2. **Given** the `relationships` table, **When** I query by rel_type, **Then** I can traverse all IFCREL* connections (aggregation, containment, voiding, filling).
3. **Given** elements assigned to groups and systems, **When** I query relationships, **Then** group memberships are recorded with appropriate context.

---

### User Story 5 - Access Geometry Data (Priority: P3)

As a geometry processing tool developer, I want geometry representations and placement chains stored as JSON so that I can compute volumes and bounding boxes without re-parsing the IFC file.

**Why this priority**: Geometry is stored for later computation, not interpreted during import. This is a data extraction concern, not a rendering concern.

**Independent Test**: `SELECT element_id, representation_type FROM geometry WHERE representation_type = 'SweptSolid'` returns elements with full geometry JSON trees.

**Acceptance Scenarios**:

1. **Given** elements with geometry, **When** I query the `geometry` table, **Then** I see representation_type, representation_json (full entity subtree), and placement_json (placement chain).
2. **Given** elements with nested local placements, **When** I query placement_json, **Then** the full placement chain is serialized correctly.
3. **Given** mapped representations (type-level geometry), **When** I query, **Then** the shared geometry is correctly resolved for each instance.

---

### User Story 6 - CLI Usability & Agent Integration (Priority: P3)

As an autonomous agent (or human user), I want comprehensive machine-readable help, structured output, and clear exit codes so that I can discover and operate all CLI commands without external documentation.

**Why this priority**: CLI polish makes the tool usable in production and by automated pipelines. Machine-readable help is critical for agent integration.

**Independent Test**: `ifc-to-db --help-json` returns valid JSON describing all commands, flags, defaults, and examples. `--output-format json` produces parseable structured output on all commands.

**Acceptance Scenarios**:

1. **Given** any command, **When** I run it with `--help`, **Then** I see summary, description, usage, flags with defaults/types/ranges, examples, exit codes, and related commands.
2. **Given** `--help-json`, **When** I parse the output, **Then** I get the full CLI contract as JSON (commands, arguments, flags with metadata).
3. **Given** `--output-format json` on import, **When** the import completes, **Then** stdout contains a JSON summary with status, counts, timing, warnings, and errors.
4. **Given** various failure modes, **When** errors occur, **Then** distinct exit codes are returned (0=success, 1=parse error, 2=file not found, 3=db error, 4=bad args, 5=partial success).
5. **Given** any platform (Linux, macOS, Windows), **When** I download the binary, **Then** it runs with zero runtime dependencies.

---

### Edge Cases

- What happens when the IFC file uses non-standard STEP formatting or line endings?
- How does the parser handle circular entity references in geometry trees?
- What happens when importing multiple IFC files into the same database?
- How does the tool behave with IFC files > 2 GB (memory-mapped input required)?
- What happens when DuckDB write fails mid-batch (disk full, permissions)?
- How are property sets with null values or empty lists handled?
- What happens with IFC types not recognized by the post-processors?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST parse any valid IFC2x3, IFC4, or IFC4x3 file using a schema-agnostic STEP lexer and entity parser.
- **FR-002**: System MUST tokenize all STEP token types: entity IDs, type names, strings, integers, floats, enums, refs, null ($), derived (*), and structural tokens.
- **FR-003**: System MUST decode IFC string encodings: \S\, \X\, \X2\...\X0\, \X4\...\X0\, and \\\\.
- **FR-004**: System MUST recover from malformed entities by skipping to the next semicolon and continuing parsing.
- **FR-005**: System MUST write parsed entities to DuckDB using the Appender API with configurable batch sizes.
- **FR-006**: System MUST serialize StepValue attribute trees to JSON with type-preserving encoding (refs as `{"ref": N}`, enums as `{"enum": "X"}`, typed values as `{"type": "T", "value": V}`).
- **FR-007**: System MUST extract file metadata from STEP headers (schema version, originating application, timestamps).
- **FR-008**: System MUST denormalize property sets via IFCRELDEFINESBYPROPERTIES resolution, supporting 6 property value types.
- **FR-009**: System MUST extract quantity sets via IFCELEMENTQUANTITY for length, area, volume, weight, count, and time.
- **FR-010**: System MUST resolve property inheritance from type objects via IFCRELDEFINESBYTYPE.
- **FR-011**: System MUST extract material assignments and classification references.
- **FR-012**: System MUST extract all IFCREL* relationships into a generic relationships table.
- **FR-013**: System MUST construct spatial hierarchy paths (Project > Site > Building > Storey > Space > Element).
- **FR-014**: System MUST serialize geometry representations and placement chains as JSON.
- **FR-015**: System MUST provide structured JSON output (`--output-format json`) on all commands.
- **FR-016**: System MUST provide machine-readable CLI discovery via `--help-json`.
- **FR-017**: System MUST support `import`, `info`, `query`, `schema`, and `version` subcommands.
- **FR-018**: System MUST use distinct exit codes for different failure categories (0-5).
- **FR-019**: System MUST report import progress (percentage, entities/sec, MB/s).
- **FR-020**: System MUST handle graceful shutdown on SIGINT (flush batch, close DB).

### Key Entities

- **Token**: Lexical unit from STEP file (kind, value, byte position).
- **StepValue**: Recursive attribute value tree (string, integer, float, enum, ref, list, typed, null, derived).
- **Entity**: Parsed STEP entity instance (ID, type name, attribute list as StepValue tree).
- **Property**: Denormalized property (element_id, pset_name, prop_name, value, type, unit, source).
- **Relationship**: Generic relationship (rel_id, rel_type, source_id, target_id, context).
- **SpatialNode**: Hierarchy node (element_id, type, name, parent_id, level, path).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Parser processes 5 real-world IFC files (IFC2x3 + IFC4) with zero errors at >= 100 MB/s throughput.
- **SC-002**: 500 MB IFC file imports in under 30 seconds including DB writes.
- **SC-003**: All property sets and quantity sets for all elements accessible via flat SQL queries after import.
- **SC-004**: Spatial hierarchy fully reconstructed; any element traceable to its building storey.
- **SC-005**: Geometry JSON available for all elements with representations; placement chains resolved.
- **SC-006**: Single static binary runs on Linux, macOS, and Windows with zero runtime dependencies.
- **SC-007**: `--help-json` exposes full CLI contract; autonomous agents can discover and operate all commands without external docs.
- **SC-008**: Property extraction completes in < 5 seconds for 100k elements.
