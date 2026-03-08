package extract

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/marcboeker/go-duckdb"

	"ifc-cli/internal/step"
)

// relConfig maps IFC relationship types to their [source, target] attribute positions.
type relAttrPos struct {
	Source int
	Target int
}

var relConfig = map[string]relAttrPos{
	"IFCRELAGGREGATES":                  {4, 5},
	"IFCRELCONTAINEDINSPATIALSTRUCTURE": {4, 5},
	"IFCRELCONNECTSPATHELEMENTS":        {4, 5},
	"IFCRELVOIDSELEMENT":                {4, 5},
	"IFCRELFILLSELEMENT":                {4, 5},
	"IFCRELASSIGNSTOGROUP":              {4, 5},
	"IFCRELDEFINESBYPROPERTIES":         {4, 5},
	"IFCRELDEFINESBYTYPE":               {4, 5},
	"IFCRELASSOCIATESMATERIAL":          {4, 5},
	"IFCRELASSOCIATESCLASSIFICATION":    {4, 5},
}

// extractRef extracts a ref ID from a JSON value. Returns (id, true) if it's a ref object {"ref": N}.
func extractRef(val interface{}) (uint64, bool) {
	m, ok := val.(map[string]interface{})
	if !ok {
		return 0, false
	}
	ref, ok := m["ref"]
	if !ok {
		return 0, false
	}
	switch v := ref.(type) {
	case float64:
		return uint64(v), true
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return uint64(n), true
	}
	return 0, false
}

// extractRefs returns all ref IDs from a JSON value, handling both single refs and lists.
func extractRefs(val interface{}) []uint64 {
	if id, ok := extractRef(val); ok {
		return []uint64{id}
	}
	list, ok := val.([]interface{})
	if !ok {
		return nil
	}
	var refs []uint64
	for _, item := range list {
		if id, ok := extractRef(item); ok {
			refs = append(refs, id)
		}
	}
	return refs
}

// extractRefsFromStep extracts ref IDs from a StepValue (single ref or list of refs).
func extractRefsFromStep(v step.StepValue) []uint64 {
	if v.Kind == step.KindRef {
		return []uint64{v.Ref}
	}
	if v.Kind == step.KindList {
		return StepRefList(v)
	}
	return nil
}

// ExtractRelationships iterates IFCREL* entities from the cache and populates the relationships table.
func ExtractRelationships(sqlDB *sql.DB, cache *EntityCache, onProgress ProgressFunc) error {
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("getting connection: %w", err)
	}
	defer conn.Close()

	var appender *duckdb.Appender
	err = conn.Raw(func(driverConn interface{}) error {
		dc, ok := driverConn.(driver.Conn)
		if !ok {
			return sql.ErrConnDone
		}
		var appErr error
		appender, appErr = duckdb.NewAppenderFromConn(dc, "", "relationships")
		return appErr
	})
	if err != nil {
		return fmt.Errorf("creating appender: %w", err)
	}
	defer appender.Close()

	var count int
	for _, e := range cache.EntitiesByTypePrefix("IFCREL") {
		cfg, ok := relConfig[e.Type]
		if !ok {
			continue
		}

		attrs, ok := cache.GetStepAttrs(e.ID)
		if !ok {
			continue
		}

		if cfg.Source >= len(attrs) || cfg.Target >= len(attrs) {
			continue
		}

		sourceRefs := extractRefsFromStep(attrs[cfg.Source])
		targetRefs := extractRefsFromStep(attrs[cfg.Target])

		ctxNullStr := contextForRelStep(e.Type)
		var ctxVal interface{}
		if ctxNullStr.Valid {
			ctxVal = ctxNullStr.String
		}

		for _, src := range sourceRefs {
			for _, tgt := range targetRefs {
				if err := appender.AppendRow(uint32(e.ID), e.Type, uint32(src), uint32(tgt), ctxVal); err != nil {
					return fmt.Errorf("appending rel: %w", err)
				}
			}
		}
		count++
		if onProgress != nil && count%1000 == 0 {
			onProgress("relationships", count)
		}
	}
	if onProgress != nil {
		onProgress("relationships", count)
	}

	return appender.Flush()
}

// contextForRelStep returns context info for specific relationship types.
func contextForRelStep(ifcType string) sql.NullString {
	switch ifcType {
	case "IFCRELASSIGNSTOGROUP":
		return sql.NullString{String: "group_assignment", Valid: true}
	case "IFCRELDEFINESBYPROPERTIES":
		return sql.NullString{String: "property_definition", Valid: true}
	case "IFCRELDEFINESBYTYPE":
		return sql.NullString{String: "type_definition", Valid: true}
	case "IFCRELASSOCIATESMATERIAL":
		return sql.NullString{String: "material_association", Valid: true}
	case "IFCRELASSOCIATESCLASSIFICATION":
		return sql.NullString{String: "classification", Valid: true}
	case "IFCRELAGGREGATES":
		return sql.NullString{String: "aggregation", Valid: true}
	case "IFCRELCONTAINEDINSPATIALSTRUCTURE":
		return sql.NullString{String: "spatial_containment", Valid: true}
	case "IFCRELVOIDSELEMENT":
		return sql.NullString{String: "void", Valid: true}
	case "IFCRELFILLSELEMENT":
		return sql.NullString{String: "fill", Valid: true}
	default:
		return sql.NullString{}
	}
}

// spatialTypes are the IFC types that form the spatial hierarchy, in order.
var spatialTypes = map[string]int{
	"IFCPROJECT":          0,
	"IFCSITE":             1,
	"IFCBUILDING":         2,
	"IFCBUILDINGSTOREY":   3,
	"IFCSPACE":            4,
}

// isSpatialType checks if an IFC type is part of the spatial hierarchy.
func isSpatialType(ifcType string) bool {
	_, ok := spatialTypes[ifcType]
	return ok
}

