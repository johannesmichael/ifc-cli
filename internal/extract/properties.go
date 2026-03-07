package extract

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Property represents a denormalized property row.
type Property struct {
	ElementID   uint64
	ElementType string
	PSetName    string
	PropName    string
	PropValue   string
	ValueType   string
	Unit        string
	Source      string // "instance" or "type"
}

// Quantity represents a denormalized quantity row.
type Quantity struct {
	ElementID     uint64
	ElementType   string
	QSetName      string
	QuantityName  string
	QuantityValue float64
	QuantityType  string
	Unit          string
}

// ExtractProperties extracts all properties and quantities from the entities table
// and inserts them into the properties and quantities tables.
func ExtractProperties(db *sql.DB, cache *EntityCache) error {
	// Build element type lookup from cache
	elementTypes := cache.TypeLookup()

	// Task 3.1-3.3: Extract instance-level properties
	instanceProps, err := extractInstanceProperties(db, cache, elementTypes)
	if err != nil {
		return fmt.Errorf("extracting instance properties: %w", err)
	}

	// Task 3.5: Extract type-level properties
	typeProps, err := extractTypeProperties(db, cache, elementTypes)
	if err != nil {
		return fmt.Errorf("extracting type properties: %w", err)
	}

	// Merge: instance overrides type
	allProps := mergeProperties(instanceProps, typeProps)

	// Task 3.4: Extract quantities
	quantities, err := extractQuantities(db, cache, elementTypes)
	if err != nil {
		return fmt.Errorf("extracting quantities: %w", err)
	}

	// Task 3.6: Extract materials as properties
	materialProps, err := extractMaterials(db, cache, elementTypes)
	if err != nil {
		return fmt.Errorf("extracting materials: %w", err)
	}
	allProps = append(allProps, materialProps...)

	// Task 3.7: Extract classifications as properties
	classProps, err := extractClassifications(db, cache, elementTypes)
	if err != nil {
		return fmt.Errorf("extracting classifications: %w", err)
	}
	allProps = append(allProps, classProps...)

	// Task 3.8: Batch insert
	if err := insertProperties(db, allProps); err != nil {
		return fmt.Errorf("inserting properties: %w", err)
	}
	if err := insertQuantities(db, quantities); err != nil {
		return fmt.Errorf("inserting quantities: %w", err)
	}

	return nil
}

// JSON attr helpers

func extractRefFromRaw(raw json.RawMessage) (uint64, bool) {
	var obj struct {
		Ref uint64 `json:"ref"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return 0, false
	}
	if obj.Ref == 0 {
		// Distinguish real zero from missing field
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return 0, false
		}
		if _, ok := m["ref"]; !ok {
			return 0, false
		}
	}
	return obj.Ref, true
}

func extractRefListFromRaw(raw json.RawMessage) []uint64 {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	var refs []uint64
	for _, item := range items {
		if ref, ok := extractRefFromRaw(item); ok {
			refs = append(refs, ref)
		}
	}
	return refs
}

func extractString(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

func extractEnum(raw json.RawMessage) (string, bool) {
	var obj struct {
		Enum string `json:"enum"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", false
	}
	if obj.Enum == "" {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return "", false
		}
		if _, ok := m["enum"]; !ok {
			return "", false
		}
	}
	return obj.Enum, true
}

type typedValue struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

func extractTypedValue(raw json.RawMessage) (string, string, bool) {
	var tv typedValue
	if err := json.Unmarshal(raw, &tv); err != nil {
		return "", "", false
	}
	if tv.Type == "" {
		return "", "", false
	}
	valStr := formatRawValue(tv.Value)
	return tv.Type, valStr, true
}

func formatRawValue(raw json.RawMessage) string {
	if raw == nil || string(raw) == "null" {
		return ""
	}
	// Try string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try number
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	// Try bool (from enum .T./.F.)
	var obj struct {
		Enum string `json:"enum"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Enum != "" {
		if obj.Enum == "T" {
			return "true"
		}
		if obj.Enum == "F" {
			return "false"
		}
		return obj.Enum
	}
	// Fallback: raw JSON
	return string(raw)
}

func formatAttrValue(raw json.RawMessage) string {
	if raw == nil || string(raw) == "null" {
		return ""
	}
	// Try typed value first
	if typeName, val, ok := extractTypedValue(raw); ok {
		_ = typeName
		return val
	}
	// Try string
	if s, ok := extractString(raw); ok {
		return s
	}
	// Try enum
	if e, ok := extractEnum(raw); ok {
		return e
	}
	return formatRawValue(raw)
}

func formatAttrValueType(raw json.RawMessage) string {
	if raw == nil || string(raw) == "null" {
		return ""
	}
	if typeName, _, ok := extractTypedValue(raw); ok {
		return typeName
	}
	return ""
}

// Task 3.1-3.3: Extract properties from IFCRELDEFINESBYPROPERTIES → IFCPROPERTYSET → individual properties
func extractInstanceProperties(db *sql.DB, cache *EntityCache, elementTypes map[uint64]string) ([]Property, error) {
	rows, err := db.Query("SELECT id, attrs FROM entities WHERE ifc_type = 'IFCRELDEFINESBYPROPERTIES'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// psetID → []elementID
	psetToElements := make(map[uint64][]uint64)
	for rows.Next() {
		var id uint64
		var attrsJSON string
		if err := rows.Scan(&id, &attrsJSON); err != nil {
			return nil, err
		}
		var attrs []json.RawMessage
		if err := json.Unmarshal([]byte(attrsJSON), &attrs); err != nil {
			continue
		}
		if len(attrs) < 6 {
			continue
		}
		elementRefs := extractRefListFromRaw(attrs[4])
		psetRef, ok := extractRefFromRaw(attrs[5])
		if !ok || len(elementRefs) == 0 {
			continue
		}
		psetToElements[psetRef] = append(psetToElements[psetRef], elementRefs...)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var properties []Property
	for psetID, elementIDs := range psetToElements {
		psetType := cache.GetType(psetID)
		psetAttrs, err := cache.GetAttrs(psetID)
		if err != nil {
			continue
		}

		if psetType == "IFCPROPERTYSET" {
			props, err := extractPropertySet(cache, psetAttrs, elementIDs, elementTypes, "instance")
			if err != nil {
				continue
			}
			properties = append(properties, props...)
		}
	}

	return properties, nil
}

func extractPropertySet(cache *EntityCache, psetAttrs []json.RawMessage, elementIDs []uint64, elementTypes map[uint64]string, source string) ([]Property, error) {
	if len(psetAttrs) < 5 {
		return nil, nil
	}

	psetName, _ := extractString(psetAttrs[2])
	propRefs := extractRefListFromRaw(psetAttrs[4])

	var properties []Property
	for _, propRef := range propRefs {
		propType := cache.GetType(propRef)
		propAttrs, err := cache.GetAttrs(propRef)
		if err != nil {
			continue
		}

		propName, propValue, valueType, unit := extractPropertyValue(propType, propAttrs)
		if propName == "" {
			continue
		}

		for _, elemID := range elementIDs {
			properties = append(properties, Property{
				ElementID:   elemID,
				ElementType: elementTypes[elemID],
				PSetName:    psetName,
				PropName:    propName,
				PropValue:   propValue,
				ValueType:   valueType,
				Unit:        unit,
				Source:      source,
			})
		}
	}

	return properties, nil
}

// Task 3.3: Handle different property types
func extractPropertyValue(propType string, attrs []json.RawMessage) (name, value, valueType, unit string) {
	if len(attrs) == 0 {
		return
	}

	switch propType {
	case "IFCPROPERTYSINGLEVALUE":
		// attrs: [0]=Name, [1]=Description, [2]=NominalValue, [3]=Unit
		if len(attrs) < 3 {
			return
		}
		name, _ = extractString(attrs[0])
		value = formatAttrValue(attrs[2])
		valueType = formatAttrValueType(attrs[2])
		if len(attrs) > 3 {
			unit = formatAttrValue(attrs[3])
		}

	case "IFCPROPERTYENUMERATEDVALUE":
		// attrs: [0]=Name, [1]=Description, [2]=EnumerationValues, [3]=EnumerationReference
		if len(attrs) < 3 {
			return
		}
		name, _ = extractString(attrs[0])
		value = formatListValues(attrs[2])
		valueType = propType

	case "IFCPROPERTYLISTVALUE":
		// attrs: [0]=Name, [1]=Description, [2]=ListValues, [3]=Unit
		if len(attrs) < 3 {
			return
		}
		name, _ = extractString(attrs[0])
		value = formatListValues(attrs[2])
		valueType = propType
		if len(attrs) > 3 {
			unit = formatAttrValue(attrs[3])
		}

	case "IFCPROPERTYBOUNDEDVALUE":
		// attrs: [0]=Name, [1]=Description, [2]=UpperBoundValue, [3]=LowerBoundValue, [4]=Unit, [5]=SetPointValue
		if len(attrs) < 4 {
			return
		}
		name, _ = extractString(attrs[0])
		upper := formatAttrValue(attrs[2])
		lower := formatAttrValue(attrs[3])
		value = lower + " - " + upper
		valueType = propType
		if len(attrs) > 4 {
			unit = formatAttrValue(attrs[4])
		}

	case "IFCPROPERTYTABLEVALUE":
		// attrs: [0]=Name, [1]=Description, [2]=DefiningValues, [3]=DefinedValues, ...
		if len(attrs) < 4 {
			return
		}
		name, _ = extractString(attrs[0])
		defining := formatListValues(attrs[2])
		defined := formatListValues(attrs[3])
		value = defining + " → " + defined
		valueType = propType

	case "IFCPROPERTYREFERENCEVALUE":
		// attrs: [0]=Name, [1]=Description, [2]=UsageName, [3]=PropertyReference
		if len(attrs) < 2 {
			return
		}
		name, _ = extractString(attrs[0])
		if len(attrs) > 3 {
			if ref, ok := extractRefFromRaw(attrs[3]); ok {
				value = fmt.Sprintf("#%d", ref)
			}
		}
		valueType = propType
	}

	return
}

func formatListValues(raw json.RawMessage) string {
	if raw == nil || string(raw) == "null" {
		return ""
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return formatAttrValue(raw)
	}
	vals := make([]string, 0, len(items))
	for _, item := range items {
		v := formatAttrValue(item)
		if v != "" {
			vals = append(vals, v)
		}
	}
	return strings.Join(vals, ", ")
}

// Task 3.4: Extract quantities from IFCELEMENTQUANTITY
func extractQuantities(db *sql.DB, cache *EntityCache, elementTypes map[uint64]string) ([]Quantity, error) {
	// First find IFCRELDEFINESBYPROPERTIES that reference IFCELEMENTQUANTITY
	rows, err := db.Query("SELECT id, attrs FROM entities WHERE ifc_type = 'IFCRELDEFINESBYPROPERTIES'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	qsetToElements := make(map[uint64][]uint64)
	for rows.Next() {
		var id uint64
		var attrsJSON string
		if err := rows.Scan(&id, &attrsJSON); err != nil {
			return nil, err
		}
		var attrs []json.RawMessage
		if err := json.Unmarshal([]byte(attrsJSON), &attrs); err != nil {
			continue
		}
		if len(attrs) < 6 {
			continue
		}
		elementRefs := extractRefListFromRaw(attrs[4])
		psetRef, ok := extractRefFromRaw(attrs[5])
		if !ok || len(elementRefs) == 0 {
			continue
		}
		// Check if it's an IFCELEMENTQUANTITY
		if elementTypes[psetRef] == "IFCELEMENTQUANTITY" {
			qsetToElements[psetRef] = append(qsetToElements[psetRef], elementRefs...)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var quantities []Quantity
	for qsetID, elementIDs := range qsetToElements {
		qsetAttrs, err := cache.GetAttrs(qsetID)
		if err != nil {
			continue
		}
		if len(qsetAttrs) < 5 {
			continue
		}

		qsetName, _ := extractString(qsetAttrs[2])
		quantityRefs := extractRefListFromRaw(qsetAttrs[4])

		for _, qRef := range quantityRefs {
			qType := cache.GetType(qRef)
			qAttrs, err := cache.GetAttrs(qRef)
			if err != nil {
				continue
			}
			qName, qValue, qTypeName, qUnit := extractQuantityValue(qType, qAttrs)
			if qName == "" {
				continue
			}
			for _, elemID := range elementIDs {
				quantities = append(quantities, Quantity{
					ElementID:     elemID,
					ElementType:   elementTypes[elemID],
					QSetName:      qsetName,
					QuantityName:  qName,
					QuantityValue: qValue,
					QuantityType:  qTypeName,
					Unit:          qUnit,
				})
			}
		}
	}

	return quantities, nil
}

func extractQuantityValue(qType string, attrs []json.RawMessage) (name string, value float64, quantityType, unit string) {
	if len(attrs) < 4 {
		return
	}
	name, _ = extractString(attrs[0])

	quantityTypeMap := map[string]string{
		"IFCQUANTITYLENGTH": "Length",
		"IFCQUANTITYAREA":   "Area",
		"IFCQUANTITYVOLUME": "Volume",
		"IFCQUANTITYWEIGHT": "Weight",
		"IFCQUANTITYCOUNT":  "Count",
		"IFCQUANTITYTIME":   "Time",
	}

	quantityType, ok := quantityTypeMap[qType]
	if !ok {
		return "", 0, "", ""
	}

	// Value is typically at attr index 3 for quantities
	// attrs: [0]=Name, [1]=Description, [2]=Unit, [3]=Value
	if len(attrs) > 3 {
		value = extractFloat(attrs[3])
	}

	if len(attrs) > 2 {
		unit = formatAttrValue(attrs[2])
	}

	return
}

func extractFloat(raw json.RawMessage) float64 {
	if raw == nil || string(raw) == "null" {
		return 0
	}
	// Try direct number
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f
	}
	// Try typed value
	var tv typedValue
	if err := json.Unmarshal(raw, &tv); err == nil && tv.Type != "" {
		var v float64
		if err := json.Unmarshal(tv.Value, &v); err == nil {
			return v
		}
	}
	return 0
}

// Task 3.5: Extract type-level properties via IFCRELDEFINESBYTYPE
func extractTypeProperties(db *sql.DB, cache *EntityCache, elementTypes map[uint64]string) ([]Property, error) {
	rows, err := db.Query("SELECT id, attrs FROM entities WHERE ifc_type = 'IFCRELDEFINESBYTYPE'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type typeMapping struct {
		elementIDs []uint64
		typeID     uint64
	}

	var mappings []typeMapping
	for rows.Next() {
		var id uint64
		var attrsJSON string
		if err := rows.Scan(&id, &attrsJSON); err != nil {
			return nil, err
		}
		var attrs []json.RawMessage
		if err := json.Unmarshal([]byte(attrsJSON), &attrs); err != nil {
			continue
		}
		if len(attrs) < 6 {
			continue
		}
		elementRefs := extractRefListFromRaw(attrs[4])
		typeRef, ok := extractRefFromRaw(attrs[5])
		if !ok || len(elementRefs) == 0 {
			continue
		}
		mappings = append(mappings, typeMapping{elementIDs: elementRefs, typeID: typeRef})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var properties []Property
	for _, m := range mappings {
		// Get the type entity's HasPropertySets (typically attr index 4 for type entities)
		typeAttrs, err := cache.GetAttrs(m.typeID)
		if err != nil {
			continue
		}

		// Type objects have property sets referenced via HasPropertySets attribute
		// Position varies by type, but commonly index 5 for most IFC type objects
		// Try to find list-of-refs in attrs
		for _, attr := range typeAttrs {
			psetRefs := extractRefListFromRaw(attr)
			if len(psetRefs) == 0 {
				continue
			}
			for _, psetRef := range psetRefs {
				psetType, ok := elementTypes[psetRef]
				if !ok || psetType != "IFCPROPERTYSET" {
					continue
				}
				psetAttrs, err := cache.GetAttrs(psetRef)
				if err != nil {
					continue
				}
				props, err := extractPropertySet(cache, psetAttrs, m.elementIDs, elementTypes, "type")
				if err != nil {
					continue
				}
				properties = append(properties, props...)
			}
		}
	}

	return properties, nil
}

// mergeProperties merges instance and type properties, with instance overriding type.
func mergeProperties(instance, typeLevel []Property) []Property {
	// Track which (elementID, psetName, propName) combos exist at instance level
	type key struct {
		elementID uint64
		psetName  string
		propName  string
	}
	instanceKeys := make(map[key]struct{}, len(instance))
	for _, p := range instance {
		instanceKeys[key{p.ElementID, p.PSetName, p.PropName}] = struct{}{}
	}

	merged := make([]Property, 0, len(instance)+len(typeLevel))
	merged = append(merged, instance...)
	for _, p := range typeLevel {
		if _, exists := instanceKeys[key{p.ElementID, p.PSetName, p.PropName}]; !exists {
			merged = append(merged, p)
		}
	}
	return merged
}

// Task 3.6: Extract material associations
func extractMaterials(db *sql.DB, cache *EntityCache, elementTypes map[uint64]string) ([]Property, error) {
	rows, err := db.Query("SELECT id, attrs FROM entities WHERE ifc_type = 'IFCRELASSOCIATESMATERIAL'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var properties []Property
	for rows.Next() {
		var id uint64
		var attrsJSON string
		if err := rows.Scan(&id, &attrsJSON); err != nil {
			return nil, err
		}
		var attrs []json.RawMessage
		if err := json.Unmarshal([]byte(attrsJSON), &attrs); err != nil {
			continue
		}
		if len(attrs) < 6 {
			continue
		}

		elementRefs := extractRefListFromRaw(attrs[4])
		materialRef, ok := extractRefFromRaw(attrs[5])
		if !ok || len(elementRefs) == 0 {
			continue
		}

		materialType := cache.GetType(materialRef)
		materialAttrs, err := cache.GetAttrs(materialRef)
		if err != nil {
			continue
		}

		var materialProps []Property
		switch materialType {
		case "IFCMATERIAL":
			if len(materialAttrs) > 0 {
				name, _ := extractString(materialAttrs[0])
				for _, elemID := range elementRefs {
					materialProps = append(materialProps, Property{
						ElementID:   elemID,
						ElementType: elementTypes[elemID],
						PSetName:    "Material",
						PropName:    "Material",
						PropValue:   name,
						ValueType:   "IFCMATERIAL",
						Source:      "instance",
					})
				}
			}

		case "IFCMATERIALLAYERSET":
			if len(materialAttrs) > 1 {
				setName := ""
				if len(materialAttrs) > 1 {
					setName, _ = extractString(materialAttrs[1])
				}
				layerRefs := extractRefListFromRaw(materialAttrs[0])
				for i, layerRef := range layerRefs {
					layerAttrs, err := cache.GetAttrs(layerRef)
					if err != nil {
						continue
					}
					layerName := ""
					thickness := ""
					if len(layerAttrs) > 0 {
						// IFCMATERIALLAYER: [0]=Material ref, [1]=LayerThickness, [2]=IsVentilated, ...
						if matRef, ok := extractRefFromRaw(layerAttrs[0]); ok {
							matAttrs, err := cache.GetAttrs(matRef)
							if err == nil && len(matAttrs) > 0 {
								layerName, _ = extractString(matAttrs[0])
							}
						}
						if len(layerAttrs) > 1 {
							thickness = formatAttrValue(layerAttrs[1])
						}
					}
					psetName := "Material"
					if setName != "" {
						psetName = "Material: " + setName
					}
					for _, elemID := range elementRefs {
						if layerName != "" {
							materialProps = append(materialProps, Property{
								ElementID:   elemID,
								ElementType: elementTypes[elemID],
								PSetName:    psetName,
								PropName:    fmt.Sprintf("Layer %d Material", i+1),
								PropValue:   layerName,
								ValueType:   "IFCMATERIAL",
								Source:      "instance",
							})
						}
						if thickness != "" {
							materialProps = append(materialProps, Property{
								ElementID:   elemID,
								ElementType: elementTypes[elemID],
								PSetName:    psetName,
								PropName:    fmt.Sprintf("Layer %d Thickness", i+1),
								PropValue:   thickness,
								ValueType:   "IFCLENGTHMEASURE",
								Source:      "instance",
							})
						}
					}
				}
			}

		case "IFCMATERIALLAYERSETUSAGE":
			// attrs: [0]=ForLayerSet (ref), [1]=LayerSetDirection, [2]=DirectionSense, [3]=OffsetFromReferenceLine
			if len(materialAttrs) > 0 {
				if layerSetRef, ok := extractRefFromRaw(materialAttrs[0]); ok {
					// Recursively get the layer set
					lsType := cache.GetType(layerSetRef)
					lsAttrs, err := cache.GetAttrs(layerSetRef)
					if err == nil && lsType == "IFCMATERIALLAYERSET" {
						// Re-process as layer set
						tempAttrs := make([]json.RawMessage, len(lsAttrs))
						copy(tempAttrs, lsAttrs)
						subProps := extractMaterialLayerSet(cache, tempAttrs, elementRefs, elementTypes)
						materialProps = append(materialProps, subProps...)
					}
				}
			}
		}

		properties = append(properties, materialProps...)
	}

	return properties, rows.Err()
}

func extractMaterialLayerSet(cache *EntityCache, attrs []json.RawMessage, elementRefs []uint64, elementTypes map[uint64]string) []Property {
	var props []Property
	if len(attrs) < 1 {
		return props
	}
	setName := ""
	if len(attrs) > 1 {
		setName, _ = extractString(attrs[1])
	}
	layerRefs := extractRefListFromRaw(attrs[0])
	for i, layerRef := range layerRefs {
		layerAttrs, err := cache.GetAttrs(layerRef)
		if err != nil {
			continue
		}
		layerName := ""
		thickness := ""
		if len(layerAttrs) > 0 {
			if matRef, ok := extractRefFromRaw(layerAttrs[0]); ok {
				matAttrs, err := cache.GetAttrs(matRef)
				if err == nil && len(matAttrs) > 0 {
					layerName, _ = extractString(matAttrs[0])
				}
			}
			if len(layerAttrs) > 1 {
				thickness = formatAttrValue(layerAttrs[1])
			}
		}
		psetName := "Material"
		if setName != "" {
			psetName = "Material: " + setName
		}
		for _, elemID := range elementRefs {
			if layerName != "" {
				props = append(props, Property{
					ElementID:   elemID,
					ElementType: elementTypes[elemID],
					PSetName:    psetName,
					PropName:    fmt.Sprintf("Layer %d Material", i+1),
					PropValue:   layerName,
					ValueType:   "IFCMATERIAL",
					Source:      "instance",
				})
			}
			if thickness != "" {
				props = append(props, Property{
					ElementID:   elemID,
					ElementType: elementTypes[elemID],
					PSetName:    psetName,
					PropName:    fmt.Sprintf("Layer %d Thickness", i+1),
					PropValue:   thickness,
					ValueType:   "IFCLENGTHMEASURE",
					Source:      "instance",
				})
			}
		}
	}
	return props
}

// Task 3.7: Extract classification associations
func extractClassifications(db *sql.DB, cache *EntityCache, elementTypes map[uint64]string) ([]Property, error) {
	rows, err := db.Query("SELECT id, attrs FROM entities WHERE ifc_type = 'IFCRELASSOCIATESCLASSIFICATION'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var properties []Property
	for rows.Next() {
		var id uint64
		var attrsJSON string
		if err := rows.Scan(&id, &attrsJSON); err != nil {
			return nil, err
		}
		var attrs []json.RawMessage
		if err := json.Unmarshal([]byte(attrsJSON), &attrs); err != nil {
			continue
		}
		if len(attrs) < 6 {
			continue
		}

		elementRefs := extractRefListFromRaw(attrs[4])
		classRef, ok := extractRefFromRaw(attrs[5])
		if !ok || len(elementRefs) == 0 {
			continue
		}

		// Get classification reference (IFCCLASSIFICATIONREFERENCE)
		classType := cache.GetType(classRef)
		classAttrs, err := cache.GetAttrs(classRef)
		if err != nil {
			continue
		}

		if classType != "IFCCLASSIFICATIONREFERENCE" {
			continue
		}
		// IFCCLASSIFICATIONREFERENCE: [0]=Location, [1]=Identification/ItemReference, [2]=Name, [3]=ReferencedSource
		itemRef := ""
		className := ""
		systemName := ""
		if len(classAttrs) > 1 {
			itemRef, _ = extractString(classAttrs[1])
		}
		if len(classAttrs) > 2 {
			className, _ = extractString(classAttrs[2])
		}
		if len(classAttrs) > 3 {
			// ReferencedSource is a ref to IFCCLASSIFICATION
			if sysRef, ok := extractRefFromRaw(classAttrs[3]); ok {
				sysAttrs, err := cache.GetAttrs(sysRef)
				if err == nil && len(sysAttrs) > 0 {
					systemName, _ = extractString(sysAttrs[0])
				}
			}
		}

		psetName := "Classification"
		if systemName != "" {
			psetName = "Classification: " + systemName
		}

		for _, elemID := range elementRefs {
			if itemRef != "" {
				properties = append(properties, Property{
					ElementID:   elemID,
					ElementType: elementTypes[elemID],
					PSetName:    psetName,
					PropName:    "ItemReference",
					PropValue:   itemRef,
					ValueType:   "IFCCLASSIFICATIONREFERENCE",
					Source:      "instance",
				})
			}
			if className != "" {
				properties = append(properties, Property{
					ElementID:   elemID,
					ElementType: elementTypes[elemID],
					PSetName:    psetName,
					PropName:    "Name",
					PropValue:   className,
					ValueType:   "IFCCLASSIFICATIONREFERENCE",
					Source:      "instance",
				})
			}
		}
	}

	return properties, rows.Err()
}

// Task 3.8: Batch insert properties and quantities
func insertProperties(db *sql.DB, properties []Property) error {
	if len(properties) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO properties (element_id, element_type, pset_name, prop_name, prop_value, value_type, unit) VALUES (?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range properties {
		_, err := stmt.Exec(p.ElementID, p.ElementType, p.PSetName, p.PropName, p.PropValue, p.ValueType, p.Unit)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func insertQuantities(db *sql.DB, quantities []Quantity) error {
	if len(quantities) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO quantities (element_id, element_type, qset_name, quantity_name, quantity_value, quantity_type, unit) VALUES (?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, q := range quantities {
		_, err := stmt.Exec(q.ElementID, q.ElementType, q.QSetName, q.QuantityName, q.QuantityValue, q.QuantityType, q.Unit)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
