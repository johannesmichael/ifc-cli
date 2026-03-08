package extract

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"ifc-cli/internal/step"
)

// CachedEntity holds a pre-loaded entity from the entities table.
type CachedEntity struct {
	ID        uint64
	Type      string
	GlobalID  string
	AttrsJSON string
	Attrs     []step.StepValue // in-memory parsed attrs (populated by Put, not by DB load)
}

// EntityCache pre-loads all entities into memory for fast lookups.
type EntityCache struct {
	entities map[uint64]*CachedEntity
}

// NewEntityCacheEmpty creates an empty EntityCache for in-memory population during parsing.
func NewEntityCacheEmpty() *EntityCache {
	return &EntityCache{
		entities: make(map[uint64]*CachedEntity),
	}
}

// NewEntityCache loads all entities from the database into memory.
func NewEntityCache(db *sql.DB) (*EntityCache, error) {
	rows, err := db.Query("SELECT id, ifc_type, COALESCE(global_id, ''), CAST(attrs AS VARCHAR) FROM entities")
	if err != nil {
		return nil, fmt.Errorf("loading entity cache: %w", err)
	}
	defer rows.Close()

	cache := &EntityCache{
		entities: make(map[uint64]*CachedEntity),
	}

	for rows.Next() {
		var e CachedEntity
		if err := rows.Scan(&e.ID, &e.Type, &e.GlobalID, &e.AttrsJSON); err != nil {
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
		// Parse JSON attrs into StepValues so GetStepAttrs works for DB-loaded entities.
		if e.AttrsJSON != "" {
			if parsed, perr := step.UnmarshalAttrs([]byte(e.AttrsJSON)); perr == nil {
				e.Attrs = parsed
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
	// Prefer in-memory StepValue attrs
	if e.Attrs != nil && len(e.Attrs) >= 3 {
		if e.Attrs[2].Kind == step.KindString {
			return e.Attrs[2].Str
		}
		return ""
	}
	// Fallback to JSON
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

// EntitiesByType returns all cached entities with the given IFC type.
func (c *EntityCache) EntitiesByType(ifcType string) []*CachedEntity {
	var result []*CachedEntity
	for _, e := range c.entities {
		if e.Type == ifcType {
			result = append(result, e)
		}
	}
	return result
}

// EntitiesByTypePrefix returns all cached entities whose type starts with the given prefix.
func (c *EntityCache) EntitiesByTypePrefix(prefix string) []*CachedEntity {
	var result []*CachedEntity
	for _, e := range c.entities {
		if strings.HasPrefix(e.Type, prefix) {
			result = append(result, e)
		}
	}
	return result
}

// Put adds or replaces an entity in the cache with in-memory parsed attrs.
func (c *EntityCache) Put(id uint64, ifcType, globalID string, attrs []step.StepValue) {
	c.entities[id] = &CachedEntity{
		ID:       id,
		Type:     ifcType,
		GlobalID: globalID,
		Attrs:    attrs,
	}
}

// GetStepAttrs returns the in-memory parsed attrs for an entity, if available.
func (c *EntityCache) GetStepAttrs(id uint64) ([]step.StepValue, bool) {
	e, ok := c.entities[id]
	if !ok || e.Attrs == nil {
		return nil, false
	}
	return e.Attrs, true
}

// Len returns the number of entities in the cache.
func (c *EntityCache) Len() int {
	return len(c.entities)
}

// GetGlobalID returns the GlobalId for an entity ID, or empty string if not found.
func (c *EntityCache) GetGlobalID(id uint64) string {
	if e, ok := c.entities[id]; ok {
		return e.GlobalID
	}
	return ""
}

// TypeLookup returns a map of entity ID to IFC type (replaces buildElementTypeLookup).
func (c *EntityCache) TypeLookup() map[uint64]string {
	m := make(map[uint64]string, len(c.entities))
	for id, e := range c.entities {
		m[id] = e.Type
	}
	return m
}
