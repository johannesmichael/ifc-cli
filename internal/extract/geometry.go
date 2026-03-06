package extract

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// buildingElementTypes lists IFC types that can have geometry representations.
var buildingElementTypes = []string{
	"IFCWALL", "IFCWALLSTANDARDCASE",
	"IFCSLAB", "IFCSLABSTANDARDCASE",
	"IFCBEAM", "IFCBEAMSTANDARDCASE",
	"IFCCOLUMN", "IFCCOLUMNSTANDARDCASE",
	"IFCDOOR", "IFCDOORSTANDARDCASE",
	"IFCWINDOW", "IFCWINDOWSTANDARDCASE",
	"IFCPLATE", "IFCPLATESTANDARDCASE",
	"IFCMEMBER", "IFCMEMBERSTANDARDCASE",
	"IFCCOVERING", "IFCCURTAINWALL",
	"IFCSTAIRFLIGHT", "IFCSTAIR",
	"IFCRAMPFLIGHT", "IFCRAMP",
	"IFCRAILING", "IFCROOF",
	"IFCFOOTING", "IFCPILE",
	"IFCFURNISHINGELEMENT",
	"IFCFLOWSEGMENT", "IFCFLOWTERMINAL", "IFCFLOWFITTING",
	"IFCDISTRIBUTIONELEMENT", "IFCBUILDINGELEMENTPROXY",
	"IFCOPENINGELEMENT", "IFCSPACE",
	"IFCSITE", "IFCBUILDING", "IFCBUILDINGSTOREY",
}

// entityRow holds data fetched from the entities table.
type entityRow struct {
	id      uint64
	ifcType string
	attrs   string // raw JSON
}

// ExtractGeometry queries building elements from the entities table,
// resolves their geometry representations and placements, and writes
// results to the geometry table.
func ExtractGeometry(db *sql.DB) error {
	elements, err := queryBuildingElements(db)
	if err != nil {
		return fmt.Errorf("querying building elements: %w", err)
	}

	insert, err := db.Prepare(`INSERT INTO geometry (element_id, element_type, representation_type, representation_json, placement_json, bounding_box_json) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing geometry insert: %w", err)
	}
	defer insert.Close()

	for _, elem := range elements {
		attrs, err := parseAttrs(elem.attrs)
		if err != nil {
			continue
		}

		// Find representation ref (IFCPRODUCTDEFINITIONSHAPE)
		repID, hasRep := findRefInAttrs(attrs, db, "IFCPRODUCTDEFINITIONSHAPE")
		if !hasRep {
			continue
		}

		// Resolve representation subtree
		repType, repJSON, err := resolveRepresentation(db, repID)
		if err != nil {
			continue
		}

		repBytes, err := json.Marshal(repJSON)
		if err != nil {
			continue
		}

		// Find placement ref (IFCLOCALPLACEMENT)
		var placementBytes []byte
		if placementID, hasPlacement := findRefInAttrs(attrs, db, "IFCLOCALPLACEMENT"); hasPlacement {
			chain, err := resolvePlacement(db, placementID)
			if err == nil && len(chain) > 0 {
				placementBytes, _ = json.Marshal(chain)
			}
		}

		bbox := computeBoundingBox(repJSON)
		var bboxBytes []byte
		if bbox != nil {
			bboxBytes, _ = json.Marshal(bbox)
		}

		var placementStr, bboxStr *string
		if placementBytes != nil {
			s := string(placementBytes)
			placementStr = &s
		}
		if bboxBytes != nil {
			s := string(bboxBytes)
			bboxStr = &s
		}

		_, err = insert.Exec(
			uint32(elem.id),
			elem.ifcType,
			repType,
			string(repBytes),
			placementStr,
			bboxStr,
		)
		if err != nil {
			continue
		}
	}

	return nil
}

// queryBuildingElements returns all entities whose ifc_type is a known building element.
func queryBuildingElements(db *sql.DB) ([]entityRow, error) {
	// Build IN clause
	placeholders := ""
	args := make([]interface{}, len(buildingElementTypes))
	for i, t := range buildingElementTypes {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
		args[i] = t
	}

	query := fmt.Sprintf("SELECT id, ifc_type, CAST(attrs AS VARCHAR) FROM entities WHERE ifc_type IN (%s)", placeholders)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []entityRow
	for rows.Next() {
		var e entityRow
		if err := rows.Scan(&e.id, &e.ifcType, &e.attrs); err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

// parseAttrs parses the JSON attrs array from the entities table.
func parseAttrs(raw string) ([]interface{}, error) {
	var attrs []interface{}
	err := json.Unmarshal([]byte(raw), &attrs)
	return attrs, err
}

// findRefInAttrs searches attrs for a ref whose target entity has the given type.
func findRefInAttrs(attrs []interface{}, db *sql.DB, targetType string) (uint64, bool) {
	for _, attr := range attrs {
		refID, ok := extractRef(attr)
		if !ok {
			continue
		}
		// Check if this ref points to the target type
		var ifcType string
		err := db.QueryRow("SELECT ifc_type FROM entities WHERE id = ?", uint32(refID)).Scan(&ifcType)
		if err != nil {
			continue
		}
		if ifcType == targetType {
			return refID, true
		}
	}
	return 0, false
}

// resolveRepresentation resolves an IFCPRODUCTDEFINITIONSHAPE to find shape representations.
// Returns the primary representation type and the full subtree as a JSON-serializable map.
func resolveRepresentation(db *sql.DB, pdsID uint64) (string, map[string]interface{}, error) {
	// Fetch the IFCPRODUCTDEFINITIONSHAPE entity
	pds, err := fetchEntity(db, pdsID)
	if err != nil {
		return "", nil, err
	}

	pdsAttrs, err := parseAttrs(pds.attrs)
	if err != nil {
		return "", nil, err
	}

	// IFCPRODUCTDEFINITIONSHAPE attrs: [Name, Description, Representations (list of refs)]
	// The Representations attribute is typically the 3rd attr (index 2)
	repType := ""
	var repTree map[string]interface{}

	// Find the list of shape representation refs
	for _, attr := range pdsAttrs {
		list, ok := attr.([]interface{})
		if !ok {
			continue
		}
		// Each item in the list should be a ref to an IFCSHAPEREPRESENTATION
		for _, item := range list {
			refID, ok := extractRef(item)
			if !ok {
				continue
			}
			sr, err := fetchEntity(db, refID)
			if err != nil || sr.ifcType != "IFCSHAPEREPRESENTATION" {
				continue
			}
			srAttrs, err := parseAttrs(sr.attrs)
			if err != nil {
				continue
			}

			// IFCSHAPEREPRESENTATION attrs: [ContextOfItems, RepresentationIdentifier, RepresentationType, Items]
			// RepresentationType is index 2
			if len(srAttrs) > 2 {
				if rt, ok := srAttrs[2].(string); ok {
					repType = rt
				}
			}

			// Collect the full subtree
			visited := make(map[uint64]bool)
			tree, err := collectSubtree(db, refID, visited, 0)
			if err == nil {
				repTree = tree
			}
			break // use the first shape representation
		}
		if repTree != nil {
			break
		}
	}

	if repTree == nil {
		return "", nil, fmt.Errorf("no shape representation found for IFCPRODUCTDEFINITIONSHAPE #%d", pdsID)
	}

	return repType, repTree, nil
}

// collectSubtree recursively collects all referenced entities starting from rootID.
// Returns a JSON-serializable tree with "id", "type", and "attrs" fields.
func collectSubtree(db *sql.DB, rootID uint64, visited map[uint64]bool, depth int) (map[string]interface{}, error) {
	if depth > 50 {
		return map[string]interface{}{"id": rootID, "error": "depth limit exceeded"}, nil
	}
	if visited[rootID] {
		return map[string]interface{}{"id": rootID, "circular_ref": true}, nil
	}
	visited[rootID] = true

	entity, err := fetchEntity(db, rootID)
	if err != nil {
		return nil, err
	}

	attrs, err := parseAttrs(entity.attrs)
	if err != nil {
		return nil, err
	}

	// Resolve refs in attrs to nested subtrees
	resolvedAttrs := make([]interface{}, len(attrs))
	for i, attr := range attrs {
		resolvedAttrs[i] = resolveAttrRefs(db, attr, visited, depth+1)
	}

	return map[string]interface{}{
		"id":    rootID,
		"type":  entity.ifcType,
		"attrs": resolvedAttrs,
	}, nil
}

// resolveAttrRefs recursively resolves ref values in an attribute to their entity subtrees.
func resolveAttrRefs(db *sql.DB, attr interface{}, visited map[uint64]bool, depth int) interface{} {
	switch v := attr.(type) {
	case map[string]interface{}:
		if refID, ok := extractRef(v); ok {
			tree, err := collectSubtree(db, refID, visited, depth)
			if err != nil {
				return v // keep the raw ref if we can't resolve
			}
			return tree
		}
		return v
	case []interface{}:
		resolved := make([]interface{}, len(v))
		for i, item := range v {
			resolved[i] = resolveAttrRefs(db, item, visited, depth)
		}
		return resolved
	default:
		return attr
	}
}

// resolvePlacement follows an IFCLOCALPLACEMENT chain and returns the placement objects.
func resolvePlacement(db *sql.DB, placementID uint64) ([]map[string]interface{}, error) {
	var chain []map[string]interface{}
	currentID := placementID
	visited := make(map[uint64]bool)

	for i := 0; i < 50; i++ {
		if visited[currentID] {
			break
		}
		visited[currentID] = true

		entity, err := fetchEntity(db, currentID)
		if err != nil {
			break
		}

		if entity.ifcType != "IFCLOCALPLACEMENT" {
			break
		}

		attrs, err := parseAttrs(entity.attrs)
		if err != nil {
			break
		}

		placement := map[string]interface{}{
			"id":   currentID,
			"type": entity.ifcType,
		}

		// IFCLOCALPLACEMENT attrs: [PlacementRelTo (ref or null), RelativePlacement (ref to IFCAXIS2PLACEMENT3D)]
		var nextID uint64
		hasNext := false

		if len(attrs) > 0 {
			if refID, ok := extractRef(attrs[0]); ok {
				nextID = refID
				hasNext = true
			}
		}

		// Resolve the RelativePlacement (IFCAXIS2PLACEMENT3D)
		if len(attrs) > 1 {
			if refID, ok := extractRef(attrs[1]); ok {
				relVisited := make(map[uint64]bool)
				relPlacement, err := collectSubtree(db, refID, relVisited, 0)
				if err == nil {
					placement["relative_placement"] = relPlacement
				}
			}
		}

		chain = append(chain, placement)

		if !hasNext {
			break
		}
		currentID = nextID
	}

	return chain, nil
}

// computeBoundingBox is a placeholder for future bounding box computation.
func computeBoundingBox(repJSON map[string]interface{}) map[string]interface{} {
	// TODO: implement for simple geometry types
	return nil
}

// fetchEntity retrieves a single entity from the database by ID.
func fetchEntity(db *sql.DB, id uint64) (*entityRow, error) {
	var e entityRow
	err := db.QueryRow("SELECT id, ifc_type, CAST(attrs AS VARCHAR) FROM entities WHERE id = ?", uint32(id)).Scan(&e.id, &e.ifcType, &e.attrs)
	if err != nil {
		return nil, fmt.Errorf("fetching entity #%d: %w", id, err)
	}
	return &e, nil
}
