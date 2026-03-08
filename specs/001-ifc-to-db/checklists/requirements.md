# Requirements Quality Checklist: IFC-to-DB

## Specification Completeness

- [ ] All 6 user stories have acceptance scenarios with Given/When/Then
- [ ] Each user story has an independent testability statement
- [ ] Priorities (P1-P3) are assigned and justified
- [ ] Edge cases are documented
- [ ] Non-goals are explicitly stated in the dev plan

## Functional Requirements

- [ ] FR-001 to FR-020 are each independently verifiable
- [ ] No requirement uses ambiguous language ("should", "may", "might")
- [ ] Each requirement specifies a single capability (no compound requirements)
- [ ] All IFC schema versions covered (IFC2x3, IFC4, IFC4x3)
- [ ] All STEP token types enumerated in FR-002
- [ ] All IFC string encoding directives listed in FR-003
- [ ] All 6 property value types specified in FR-008
- [ ] All 6 quantity types specified in FR-009
- [ ] All exit codes defined in FR-018
- [ ] All subcommands listed in FR-017

## Success Criteria

- [ ] SC-001 to SC-008 are measurable with specific numbers
- [ ] Performance targets specified (100 MB/s parse, 30s for 500 MB import)
- [ ] Platform targets specified (Linux, macOS, Windows)
- [ ] Query patterns specified for property, spatial, and geometry access

## Architecture Alignment

- [ ] Package layout matches dev plan (cmd/, internal/step/, internal/db/, internal/extract/, internal/ifc/, internal/cli/)
- [ ] Database schema covers all 7 tables (entities, properties, quantities, relationships, spatial_structure, geometry, file_metadata)
- [ ] Data flow matches: Lexer -> Parser -> Writer -> Post-processors
- [ ] CLI framework specified (cobra)
- [ ] Database driver specified (go-duckdb)

## Dependency Correctness

- [ ] Epic 2 depends on Epic 1 (parser before writer)
- [ ] Epics 3, 4, 5 depend on Epic 2 (need raw entities in DB)
- [ ] Epics 3, 4, 5 are independent of each other (parallelizable)
- [ ] Epic 6 CLI structure (6.1-6.5) can start in parallel with Epic 1
- [ ] Epic 6 polish tasks (6.6+) depend on Epics 1-5

## Risk Coverage

- [ ] CGO/cross-compilation risk addressed
- [ ] Non-standard STEP formatting risk addressed
- [ ] Memory pressure for large files (>2 GB) addressed
- [ ] Property extraction schema version differences addressed
- [ ] Error recovery strategy defined

## Testing Strategy

- [ ] Unit tests specified for each component (lexer, parser, string decoder, JSON serialization)
- [ ] Integration tests specified for end-to-end pipeline
- [ ] Fuzz tests specified for lexer and parser
- [ ] Performance benchmarks specified with targets
- [ ] Test data strategy defined (hand-crafted fixtures + real-world samples)
