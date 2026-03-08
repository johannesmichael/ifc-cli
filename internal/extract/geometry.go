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
func ExtractGeometry(sqlDB *sql.DB, cache *EntityCache, onProgress ProgressFunc) error {
	elements := filterBuildingElements(cache)

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
		appender, appErr = duckdb.NewAppenderFromConn(dc, "", "geometry")
		return appErr
	})
	if err != nil {
		return fmt.Errorf("creating appender: %w", err)
	}
	defer appender.Close()

	var count int
	for _, elem := range elements {
		attrs, ok := cache.GetStepAttrs(elem.ID)
		if !ok {
			continue
		}

		// Find representation ref (IFCPRODUCTDEFINITIONSHAPE)
		repID, hasRep := findRefInStepAttrs(attrs, cache, "IFCPRODUCTDEFINITIONSHAPE")
		if !hasRep {
			continue
		}

		repType, repJSON, err := resolveRepresentationStep(cache, repID)
		if err != nil {
			continue
		}

		repBytes, err := json.Marshal(repJSON)
		if err != nil {
			continue
		}

		var placementBytes []byte
		if placementID, hasPlacement := findRefInStepAttrs(attrs, cache, "IFCLOCALPLACEMENT"); hasPlacement {
			chain, err := resolvePlacementStep(cache, placementID)
			if err == nil && len(chain) > 0 {
				placementBytes, _ = json.Marshal(chain)
			}
		}

		bbox := computeBoundingBox(repJSON)
		var bboxBytes []byte
		if bbox != nil {
			bboxBytes, _ = json.Marshal(bbox)
		}

		var placementStr, bboxStr interface{}
		if placementBytes != nil {
			placementStr = string(placementBytes)
		}
		if bboxBytes != nil {
			bboxStr = string(bboxBytes)
		}

		if err := appender.AppendRow(
			uint32(elem.ID),
			elem.Type,
			repType,
			string(repBytes),
			placementStr,
			bboxStr,
		); err != nil {
			continue
		}
		count++
		if onProgress != nil && count%100 == 0 {
			onProgress("geometry", count)
		}
	}
	if onProgress != nil {
		onProgress("geometry", count)
	}

	return appender.Flush()
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

// findRefInStepAttrs searches StepValue attrs for a ref whose target entity has the given type.
func findRefInStepAttrs(attrs []step.StepValue, cache *EntityCache, targetType string) (uint64, bool) {
	for _, attr := range attrs {
		if ref, ok := StepRef(attr); ok {
			if cache.GetType(ref) == targetType {
				return ref, true
			}
		}
	}
	return 0, false
}

// resolveRepresentationStep resolves an IFCPRODUCTDEFINITIONSHAPE to find shape representations.
// Returns the primary representation type and a shallow summary (type, identifier, item types/IDs).
func resolveRepresentationStep(cache *EntityCache, pdsID uint64) (string, map[string]interface{}, error) {
	pds, ok := cache.Get(pdsID)
	if !ok {
		return "", nil, fmt.Errorf("entity #%d not found in cache", pdsID)
	}

	pdsAttrs, ok := cache.GetStepAttrs(pds.ID)
	if !ok {
		return "", nil, fmt.Errorf("no step attrs for #%d", pdsID)
	}

	// IFCPRODUCTDEFINITIONSHAPE attrs: [Name, Description, Representations (list of refs)]
	repType := ""
	var repInfo map[string]interface{}

	for _, attr := range pdsAttrs {
		if attr.Kind != step.KindList {
			continue
		}
		for _, item := range attr.List {
			refID, ok := StepRef(item)
			if !ok {
				continue
			}
			sr, srOk := cache.Get(refID)
			if !srOk || sr.Type != "IFCSHAPEREPRESENTATION" {
				continue
			}
			srAttrs, srOk := cache.GetStepAttrs(refID)
			if !srOk {
				continue
			}

			// IFCSHAPEREPRESENTATION: [ContextOfItems, RepresentationIdentifier, RepresentationType, Items]
			info := map[string]interface{}{
				"id":   refID,
				"type": sr.Type,
			}
			if len(srAttrs) > 1 {
				if ident, ok := StepString(srAttrs[1]); ok {
					info["identifier"] = ident
				}
			}
			if len(srAttrs) > 2 {
				if rt, ok := StepString(srAttrs[2]); ok {
					repType = rt
					info["representation_type"] = rt
				}
			}
			// Collect items as shallow refs with types (one level deep)
			if len(srAttrs) > 3 {
				info["items"] = collectShallowItemsStep(cache, srAttrs[3])
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

// collectShallowItemsStep extracts item refs from a shape representation's Items attribute,
// resolving one level deep to get the item type and basic info.
func collectShallowItemsStep(cache *EntityCache, attr step.StepValue) []map[string]interface{} {
	var items []map[string]interface{}

	addItem := func(refID uint64) {
		e, ok := cache.Get(refID)
		if !ok {
			return
		}
		items = append(items, map[string]interface{}{
			"id":   refID,
			"type": e.Type,
		})
	}

	switch attr.Kind {
	case step.KindList:
		for _, elem := range attr.List {
			if refID, ok := StepRef(elem); ok {
				addItem(refID)
			}
		}
	case step.KindRef:
		addItem(attr.Ref)
	}

	return items
}

// resolvePlacementStep follows an IFCLOCALPLACEMENT chain and returns the placement objects.
func resolvePlacementStep(cache *EntityCache, placementID uint64) ([]map[string]interface{}, error) {
	var chain []map[string]interface{}
	currentID := placementID
	visited := make(map[uint64]bool)

	for i := 0; i < 50; i++ {
		if visited[currentID] {
			break
		}
		visited[currentID] = true

		entity, ok := cache.Get(currentID)
		if !ok || entity.Type != "IFCLOCALPLACEMENT" {
			break
		}

		attrs, ok := cache.GetStepAttrs(currentID)
		if !ok {
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
			if refID, ok := StepRef(attrs[0]); ok {
				nextID = refID
				hasNext = true
			}
		}

		// Resolve the RelativePlacement (IFCAXIS2PLACEMENT3D) — shallow
		if len(attrs) > 1 {
			if refID, ok := StepRef(attrs[1]); ok {
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
