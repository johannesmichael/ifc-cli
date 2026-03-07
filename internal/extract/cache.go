package extract

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// CachedEntity holds a pre-loaded entity from the entities table.
type CachedEntity struct {
	ID      uint64
	Type    string
	AttrsJSON string
}

// EntityCache pre-loads all entities into memory for fast lookups.
type EntityCache struct {
	entities map[uint64]*CachedEntity
}

// NewEntityCache loads all entities from the database into memory.
func NewEntityCache(db *sql.DB) (*EntityCache, error) {
	rows, err := db.Query("SELECT id, ifc_type, CAST(attrs AS VARCHAR) FROM entities")
	if err != nil {
		return nil, fmt.Errorf("loading entity cache: %w", err)
	}
	defer rows.Close()

	cache := &EntityCache{
		entities: make(map[uint64]*CachedEntity),
	}

	for rows.Next() {
		var e CachedEntity
		if err := rows.Scan(&e.ID, &e.Type, &e.AttrsJSON); err != nil {
			return nil, fmt.Errorf("scanning entity: %w", err)
		}
		// CAST(attrs AS VARCHAR) wraps JSON in double quotes with escaped internals;
		// unwrap so AttrsJSON holds the raw JSON array.
		if strings.HasPrefix(e.AttrsJSON, "\"") {
			var unquoted string
			if err := json.Unmarshal([]byte(e.AttrsJSON), &unquoted); err == nil {
				e.AttrsJSON = unquoted
			}
		}
		cache.entities[e.ID] = &e
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating entities: %w", err)
	}

	return cache, nil
}

// Get returns a cached entity by ID.
func (c *EntityCache) Get(id uint64) (*CachedEntity, bool) {
	e, ok := c.entities[id]
	return e, ok
}

// GetType returns the IFC type for an entity ID.
func (c *EntityCache) GetType(id uint64) string {
	if e, ok := c.entities[id]; ok {
		return e.Type
	}
	return ""
}

// GetName extracts the Name attribute (typically index 2) from an entity's attrs.
func (c *EntityCache) GetName(id uint64) string {
	e, ok := c.entities[id]
	if !ok {
		return ""
	}
	var attrs []interface{}
	if err := json.Unmarshal([]byte(e.AttrsJSON), &attrs); err != nil || len(attrs) < 3 {
		return ""
	}
	name, ok := attrs[2].(string)
	if !ok {
		return ""
	}
	return name
}

// GetAttrs parses and returns the attrs JSON for an entity.
func (c *EntityCache) GetAttrs(id uint64) ([]json.RawMessage, error) {
	e, ok := c.entities[id]
	if !ok {
		return nil, fmt.Errorf("entity #%d not found", id)
	}
	var attrs []json.RawMessage
	if err := json.Unmarshal([]byte(e.AttrsJSON), &attrs); err != nil {
		return nil, fmt.Errorf("parsing attrs for #%d: %w", id, err)
	}
	return attrs, nil
}

// GetAttrsGeneric parses attrs as []interface{} for generic access.
func (c *EntityCache) GetAttrsGeneric(id uint64) ([]interface{}, error) {
	e, ok := c.entities[id]
	if !ok {
		return nil, fmt.Errorf("entity #%d not found", id)
	}
	var attrs []interface{}
	if err := json.Unmarshal([]byte(e.AttrsJSON), &attrs); err != nil {
		return nil, fmt.Errorf("parsing attrs for #%d: %w", id, err)
	}
	return attrs, nil
}

// TypeLookup returns a map of entity ID to IFC type (replaces buildElementTypeLookup).
func (c *EntityCache) TypeLookup() map[uint64]string {
	m := make(map[uint64]string, len(c.entities))
	for id, e := range c.entities {
		m[id] = e.Type
	}
	return m
}
