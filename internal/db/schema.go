package db

import "database/sql"

const createEntities = `CREATE TABLE IF NOT EXISTS entities (
    id          UINTEGER PRIMARY KEY,
    ifc_type    VARCHAR NOT NULL,
    global_id   VARCHAR,
    attrs       JSON NOT NULL
)`

const createEntitiesIdx = `CREATE INDEX IF NOT EXISTS idx_entity_type ON entities(ifc_type)`

const createEntitiesGlobalIDIdx = `CREATE INDEX IF NOT EXISTS idx_entity_global_id ON entities(global_id)`

const createProperties = `CREATE TABLE IF NOT EXISTS properties (
    element_id      UINTEGER NOT NULL,
    element_type    VARCHAR NOT NULL,
    pset_name       VARCHAR NOT NULL,
    prop_name       VARCHAR NOT NULL,
    prop_value      VARCHAR,
    value_type      VARCHAR,
    unit            VARCHAR
)`

const createPropertiesIdxElement = `CREATE INDEX IF NOT EXISTS idx_prop_element ON properties(element_id)`
const createPropertiesIdxPset = `CREATE INDEX IF NOT EXISTS idx_prop_pset ON properties(pset_name)`
const createPropertiesIdxName = `CREATE INDEX IF NOT EXISTS idx_prop_name ON properties(prop_name)`
const createPropertiesIdxElementType = `CREATE INDEX IF NOT EXISTS idx_prop_element_type ON properties(element_type)`

const createQuantities = `CREATE TABLE IF NOT EXISTS quantities (
    element_id      UINTEGER NOT NULL,
    element_type    VARCHAR NOT NULL,
    qset_name       VARCHAR NOT NULL,
    quantity_name   VARCHAR NOT NULL,
    quantity_value  DOUBLE,
    quantity_type   VARCHAR,
    unit            VARCHAR
)`

const createRelationships = `CREATE TABLE IF NOT EXISTS relationships (
    rel_id          UINTEGER NOT NULL,
    rel_type        VARCHAR NOT NULL,
    source_id       UINTEGER NOT NULL,
    target_id       UINTEGER NOT NULL,
    context         VARCHAR
)`

const createRelationshipsIdxSource = `CREATE INDEX IF NOT EXISTS idx_rel_source ON relationships(source_id)`
const createRelationshipsIdxTarget = `CREATE INDEX IF NOT EXISTS idx_rel_target ON relationships(target_id)`
const createRelationshipsIdxType = `CREATE INDEX IF NOT EXISTS idx_rel_type ON relationships(rel_type)`

const createSpatialStructure = `CREATE TABLE IF NOT EXISTS spatial_structure (
    element_id      UINTEGER NOT NULL,
    element_type    VARCHAR NOT NULL,
    element_name    VARCHAR,
    parent_id       UINTEGER,
    parent_type     VARCHAR,
    hierarchy_level INTEGER,
    path            VARCHAR
)`

const createGeometry = `CREATE TABLE IF NOT EXISTS geometry (
    element_id           UINTEGER PRIMARY KEY,
    element_type         VARCHAR NOT NULL,
    representation_type  VARCHAR,
    representation_json  VARCHAR,
    placement_json       VARCHAR,
    bounding_box_json    VARCHAR
)`

const createFileMetadata = `CREATE TABLE IF NOT EXISTS file_metadata (
    key    VARCHAR PRIMARY KEY,
    value  VARCHAR
)`

const createPropertiesView = `CREATE OR REPLACE VIEW properties_v AS
SELECT p.*, e.global_id
FROM properties p
LEFT JOIN entities e ON p.element_id = e.id`

const createQuantitiesView = `CREATE OR REPLACE VIEW quantities_v AS
SELECT q.*, e.global_id
FROM quantities q
LEFT JOIN entities e ON q.element_id = e.id`

const createRelationshipsView = `CREATE OR REPLACE VIEW relationships_v AS
SELECT r.*, src.global_id AS source_global_id, tgt.global_id AS target_global_id
FROM relationships r
LEFT JOIN entities src ON r.source_id = src.id
LEFT JOIN entities tgt ON r.target_id = tgt.id`

const createSpatialStructureView = `CREATE OR REPLACE VIEW spatial_structure_v AS
SELECT s.*, e.global_id, pe.global_id AS parent_global_id
FROM spatial_structure s
LEFT JOIN entities e ON s.element_id = e.id
LEFT JOIN entities pe ON s.parent_id = pe.id`

const createGeometryView = `CREATE OR REPLACE VIEW geometry_v AS
SELECT g.*, e.global_id
FROM geometry g
LEFT JOIN entities e ON g.element_id = e.id`

var ddlStatements = []string{
	createEntities,
	createEntitiesIdx,
	createEntitiesGlobalIDIdx,
	createProperties,
	createPropertiesIdxElement,
	createPropertiesIdxPset,
	createPropertiesIdxName,
	createPropertiesIdxElementType,
	createQuantities,
	createRelationships,
	createRelationshipsIdxSource,
	createRelationshipsIdxTarget,
	createRelationshipsIdxType,
	createSpatialStructure,
	createGeometry,
	createFileMetadata,
	createPropertiesView,
	createQuantitiesView,
	createRelationshipsView,
	createSpatialStructureView,
	createGeometryView,
}

// DDLStatements returns all DDL statements (CREATE TABLE and CREATE INDEX)
// in the order they should be executed.
func DDLStatements() []string {
	out := make([]string, len(ddlStatements))
	copy(out, ddlStatements)
	return out
}

// CreateSchema executes all DDL statements to create the database schema.
func CreateSchema(db *sql.DB) error {
	for _, stmt := range ddlStatements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
