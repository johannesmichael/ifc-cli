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

// ExtractGeometry queries building elements from the cache,
// resolves their geometry representations and placements, and writes
// results to the geometry table.
func ExtractGeometry(db *sql.DB, cache *EntityCache) error {
	elements := filterBuildingElements(cache)

	insert, err := db.Prepare(`INSERT INTO geometry (element_id, element_type, representation_type, representation_json, placement_json, bounding_box_json) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing geometry insert: %w", err)
	}
	defer insert.Close()

	for _, elem := range elements {
		attrs, err := parseAttrs(elem.AttrsJSON)
		if err != nil {
			continue
		}

		// Find representation ref (IFCPRODUCTDEFINITIONSHAPE)
		repID, hasRep := findRefInAttrs(attrs, cache, "IFCPRODUCTDEFINITIONSHAPE")
		if !hasRep {
			continue
		}

		// Resolve representation subtree
		repType, repJSON, err := resolveRepresentation(cache, repID)
		if err != nil {
			continue
		}

		repBytes, err := json.Marshal(repJSON)
		if err != nil {
			continue
		}

		// Find placement ref (IFCLOCALPLACEMENT)
		var placementBytes []byte
		if placementID, hasPlacement := findRefInAttrs(attrs, cache, "IFCLOCALPLACEMENT"); hasPlacement {
			chain, err := resolvePlacement(cache, placementID)
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
			uint32(elem.ID),
			elem.Type,
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

// filterBuildingElements returns all cached entities whose type is a known building element.
func filterBuildingElements(cache *EntityCache) []*CachedEntity {
	typeSet := make(map[string]bool, len(buildingElementTypes))
	for _, t := range buildingElementTypes {
		typeSet[t] = true
	}

	var results []*CachedEntity
	for _, e := range cache.entities {
		if typeSet[e.Type] {
			results = append(results, e)
		}
	}
	return results
}

// parseAttrs parses the JSON attrs array from the entities table.
func parseAttrs(raw string) ([]interface{}, error) {
	var attrs []interface{}
	err := json.Unmarshal([]byte(raw), &attrs)
	return attrs, err
}

// findRefInAttrs searches attrs for a ref whose target entity has the given type.
func findRefInAttrs(attrs []interface{}, cache *EntityCache, targetType string) (uint64, bool) {
	for _, attr := range attrs {
		refID, ok := extractRef(attr)
		if !ok {
			continue
		}
		if cache.GetType(refID) == targetType {
			return refID, true
		}
	}
	return 0, false
}

// resolveRepresentation resolves an IFCPRODUCTDEFINITIONSHAPE to find shape representations.
// Returns the primary representation type and a shallow summary (type, identifier, item types/IDs).
func resolveRepresentation(cache *EntityCache, pdsID uint64) (string, map[string]interface{}, error) {
	pds, ok := cache.Get(pdsID)
	if !ok {
		return "", nil, fmt.Errorf("entity #%d not found in cache", pdsID)
	}

	pdsAttrs, err := parseAttrs(pds.AttrsJSON)
	if err != nil {
		return "", nil, err
	}

	// IFCPRODUCTDEFINITIONSHAPE attrs: [Name, Description, Representations (list of refs)]
	repType := ""
	var repInfo map[string]interface{}

	for _, attr := range pdsAttrs {
		list, ok := attr.([]interface{})
		if !ok {
			continue
		}
		for _, item := range list {
			refID, ok := extractRef(item)
			if !ok {
				continue
			}
			sr, srOk := cache.Get(refID)
			if !srOk || sr.Type != "IFCSHAPEREPRESENTATION" {
				continue
			}
			srAttrs, err := parseAttrs(sr.AttrsJSON)
			if err != nil {
				continue
			}

			// IFCSHAPEREPRESENTATION: [ContextOfItems, RepresentationIdentifier, RepresentationType, Items]
			info := map[string]interface{}{
				"id":   refID,
				"type": sr.Type,
			}
			if len(srAttrs) > 1 {
				if ident, ok := srAttrs[1].(string); ok {
					info["identifier"] = ident
				}
			}
			if len(srAttrs) > 2 {
				if rt, ok := srAttrs[2].(string); ok {
					repType = rt
					info["representation_type"] = rt
				}
			}
			// Collect items as shallow refs with types (one level deep)
			if len(srAttrs) > 3 {
				info["items"] = collectShallowItems(cache, srAttrs[3])
			}

			repInfo = info
			break
		}
		if repInfo != nil {
			break
		}
	}

	if repInfo == nil {
		return "", nil, fmt.Errorf("no shape representation found for IFCPRODUCTDEFINITIONSHAPE #%d", pdsID)
	}

	return repType, repInfo, nil
}

// collectShallowItems extracts item refs from a shape representation's Items attribute,
// resolving one level deep to get the item type and basic info.
func collectShallowItems(cache *EntityCache, attr interface{}) []map[string]interface{} {
	var items []map[string]interface{}

	addItem := func(refID uint64) {
		e, ok := cache.Get(refID)
		if !ok {
			return
		}
		item := map[string]interface{}{
			"id":   refID,
			"type": e.Type,
		}
		items = append(items, item)
	}

	switch v := attr.(type) {
	case []interface{}:
		for _, elem := range v {
			if refID, ok := extractRef(elem); ok {
				addItem(refID)
			}
		}
	case map[string]interface{}:
		if refID, ok := extractRef(v); ok {
			addItem(refID)
		}
	}

	return items
}

// resolvePlacement follows an IFCLOCALPLACEMENT chain and returns the placement objects.
func resolvePlacement(cache *EntityCache, placementID uint64) ([]map[string]interface{}, error) {
	var chain []map[string]interface{}
	currentID := placementID
	visited := make(map[uint64]bool)

	for i := 0; i < 50; i++ {
		if visited[currentID] {
			break
		}
		visited[currentID] = true

		entity, ok := cache.Get(currentID)
		if !ok {
			break
		}

		if entity.Type != "IFCLOCALPLACEMENT" {
			break
		}

		attrs, err := parseAttrs(entity.AttrsJSON)
		if err != nil {
			break
		}

		placement := map[string]interface{}{
			"id":   currentID,
			"type": entity.Type,
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

		// Resolve the RelativePlacement (IFCAXIS2PLACEMENT3D) — shallow
		if len(attrs) > 1 {
			if refID, ok := extractRef(attrs[1]); ok {
				if rp, rpOk := cache.Get(refID); rpOk {
					placement["relative_placement"] = map[string]interface{}{
						"id":   refID,
						"type": rp.Type,
					}
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

