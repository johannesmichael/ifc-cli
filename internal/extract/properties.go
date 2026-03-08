package extract

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"

	"github.com/marcboeker/go-duckdb"

	"ifc-cli/internal/step"
)

// ProgressFunc is called to report extraction progress.
type ProgressFunc func(detail string, count int)

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
func ExtractProperties(db *sql.DB, cache *EntityCache, onProgress ProgressFunc) error {
	elementTypes := cache.TypeLookup()

	// Single pass over IFCRELDEFINESBYPROPERTIES
	psetToElements, qsetToElements := loadRelDefinesByProperties(cache, elementTypes)

	instanceProps, err := extractInstanceProperties(cache, psetToElements, elementTypes)
	if err != nil {
		return fmt.Errorf("extracting instance properties: %w", err)
	}
	if onProgress != nil {
		onProgress("instance properties", len(instanceProps))
	}

	typeProps, err := extractTypeProperties(cache, elementTypes)
	if err != nil {
		return fmt.Errorf("extracting type properties: %w", err)
	}
	if onProgress != nil {
		onProgress("type properties", len(typeProps))
	}

	allProps := mergeProperties(instanceProps, typeProps)
	if onProgress != nil {
		onProgress("properties", len(allProps))
	}

	quantities, err := extractQuantities(cache, qsetToElements, elementTypes)
	if err != nil {
		return fmt.Errorf("extracting quantities: %w", err)
	}
	if onProgress != nil {
		onProgress("quantities", len(quantities))
	}

	materialProps, err := extractMaterials(cache, elementTypes)
	if err != nil {
		return fmt.Errorf("extracting materials: %w", err)
	}
	allProps = append(allProps, materialProps...)
	if onProgress != nil {
		onProgress("materials", len(materialProps))
	}

	classProps, err := extractClassifications(cache, elementTypes)
	if err != nil {
		return fmt.Errorf("extracting classifications: %w", err)
	}
	allProps = append(allProps, classProps...)
	if onProgress != nil {
		onProgress("classifications", len(classProps))
	}

	if err := insertProperties(db, allProps); err != nil {
		return fmt.Errorf("inserting properties: %w", err)
	}
	if err := insertQuantities(db, quantities); err != nil {
		return fmt.Errorf("inserting quantities: %w", err)
	}
	return nil
}

// loadRelDefinesByProperties iterates cache entries of type IFCRELDEFINESBYPROPERTIES once,
// splitting into pset vs qset maps by checking elementTypes[psetRef].
func loadRelDefinesByProperties(cache *EntityCache, elementTypes map[uint64]string) (psetToElements, qsetToElements map[uint64][]uint64) {
	psetToElements = make(map[uint64][]uint64)
	qsetToElements = make(map[uint64][]uint64)

	for _, e := range cache.EntitiesByType("IFCRELDEFINESBYPROPERTIES") {
		attrs, ok := cache.GetStepAttrs(e.ID)
		if !ok || len(attrs) < 6 {
			continue
		}
		elementRefs := StepRefList(attrs[4])
		psetRef, ok := StepRef(attrs[5])
		if !ok || len(elementRefs) == 0 {
			continue
		}
		if elementTypes[psetRef] == "IFCELEMENTQUANTITY" {
			qsetToElements[psetRef] = append(qsetToElements[psetRef], elementRefs...)
		} else {
			psetToElements[psetRef] = append(psetToElements[psetRef], elementRefs...)
		}
	}
	return
}

// extractInstanceProperties extracts instance-level properties using pre-built psetToElements map.
func extractInstanceProperties(cache *EntityCache, psetToElements map[uint64][]uint64, elementTypes map[uint64]string) ([]Property, error) {
	var properties []Property
	for psetID, elementIDs := range psetToElements {
		psetType := cache.GetType(psetID)
		attrs, ok := cache.GetStepAttrs(psetID)
		if !ok {
			continue
		}
		if psetType == "IFCPROPERTYSET" {
			props := extractPropertySet(cache, attrs, elementIDs, elementTypes, "instance")
			properties = append(properties, props...)
		}
	}
	return properties, nil
}

func extractPropertySet(cache *EntityCache, psetAttrs []step.StepValue, elementIDs []uint64, elementTypes map[uint64]string, source string) []Property {
	if len(psetAttrs) < 5 {
		return nil
	}
	psetName, _ := StepString(psetAttrs[2])
	propRefs := StepRefList(psetAttrs[4])

	var properties []Property
	for _, propRef := range propRefs {
		propType := cache.GetType(propRef)
		propAttrs, ok := cache.GetStepAttrs(propRef)
		if !ok {
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
	return properties
}

// extractPropertyValue extracts property name, value, type, and unit from a property entity's StepValue attrs.
func extractPropertyValue(propType string, attrs []step.StepValue) (name, value, valueType, unit string) {
	if len(attrs) == 0 {
		return
	}
	switch propType {
	case "IFCPROPERTYSINGLEVALUE":
		if len(attrs) < 3 {
			return
		}
		name, _ = StepString(attrs[0])
		value = StepFormatValue(attrs[2])
		valueType = StepFormatValueType(attrs[2])
		if len(attrs) > 3 {
			unit = StepFormatValue(attrs[3])
		}
	case "IFCPROPERTYENUMERATEDVALUE":
		if len(attrs) < 3 {
			return
		}
		name, _ = StepString(attrs[0])
		value = StepFormatValue(attrs[2])
		valueType = propType
	case "IFCPROPERTYLISTVALUE":
		if len(attrs) < 3 {
			return
		}
		name, _ = StepString(attrs[0])
		value = StepFormatValue(attrs[2])
		valueType = propType
		if len(attrs) > 3 {
			unit = StepFormatValue(attrs[3])
		}
	case "IFCPROPERTYBOUNDEDVALUE":
		if len(attrs) < 4 {
			return
		}
		name, _ = StepString(attrs[0])
		upper := StepFormatValue(attrs[2])
		lower := StepFormatValue(attrs[3])
		value = lower + " - " + upper
		valueType = propType
		if len(attrs) > 4 {
			unit = StepFormatValue(attrs[4])
		}
	case "IFCPROPERTYTABLEVALUE":
		if len(attrs) < 4 {
			return
		}
		name, _ = StepString(attrs[0])
		defining := StepFormatValue(attrs[2])
		defined := StepFormatValue(attrs[3])
		value = defining + " → " + defined
		valueType = propType
	case "IFCPROPERTYREFERENCEVALUE":
		if len(attrs) < 2 {
			return
		}
		name, _ = StepString(attrs[0])
		if len(attrs) > 3 {
			if ref, ok := StepRef(attrs[3]); ok {
				value = fmt.Sprintf("#%d", ref)
			}
		}
		valueType = propType
	}
	return
}

// extractQuantities extracts quantities using pre-built qsetToElements map.
func extractQuantities(cache *EntityCache, qsetToElements map[uint64][]uint64, elementTypes map[uint64]string) ([]Quantity, error) {
	var quantities []Quantity
	for qsetID, elementIDs := range qsetToElements {
		qsetAttrs, ok := cache.GetStepAttrs(qsetID)
		if !ok || len(qsetAttrs) < 5 {
			continue
		}
		qsetName, _ := StepString(qsetAttrs[2])
		quantityRefs := StepRefList(qsetAttrs[4])
		for _, qRef := range quantityRefs {
			qType := cache.GetType(qRef)
			qAttrs, ok := cache.GetStepAttrs(qRef)
			if !ok {
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

func extractQuantityValue(qType string, attrs []step.StepValue) (name string, value float64, quantityType, unit string) {
	if len(attrs) < 4 {
		return
	}
	name, _ = StepString(attrs[0])
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
	if len(attrs) > 3 {
		value = StepFloat(attrs[3])
	}
	if len(attrs) > 2 {
		unit = StepFormatValue(attrs[2])
	}
	return
}

// extractTypeProperties extracts type-level properties via IFCRELDEFINESBYTYPE.
func extractTypeProperties(cache *EntityCache, elementTypes map[uint64]string) ([]Property, error) {
	var properties []Property
	for _, e := range cache.EntitiesByType("IFCRELDEFINESBYTYPE") {
		attrs, ok := cache.GetStepAttrs(e.ID)
		if !ok || len(attrs) < 6 {
			continue
		}
		elementRefs := StepRefList(attrs[4])
		typeRef, ok := StepRef(attrs[5])
		if !ok || len(elementRefs) == 0 {
			continue
		}
		typeAttrs, typeOk := cache.GetStepAttrs(typeRef)
		if !typeOk {
			continue
		}
		for _, attr := range typeAttrs {
			psetRefs := StepRefList(attr)
			if len(psetRefs) == 0 {
				continue
			}
			for _, psetRef := range psetRefs {
				psetType, exists := elementTypes[psetRef]
				if !exists || psetType != "IFCPROPERTYSET" {
					continue
				}
				psetAttrs, psetOk := cache.GetStepAttrs(psetRef)
				if !psetOk {
					continue
				}
				props := extractPropertySet(cache, psetAttrs, elementRefs, elementTypes, "type")
				properties = append(properties, props...)
			}
		}
	}
	return properties, nil
}

// mergeProperties merges instance and type properties, with instance overriding type.
func mergeProperties(instance, typeLevel []Property) []Property {
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

// extractMaterials extracts material associations.
func extractMaterials(cache *EntityCache, elementTypes map[uint64]string) ([]Property, error) {
	var properties []Property
	for _, e := range cache.EntitiesByType("IFCRELASSOCIATESMATERIAL") {
		attrs, ok := cache.GetStepAttrs(e.ID)
		if !ok || len(attrs) < 6 {
			continue
		}
		elementRefs := StepRefList(attrs[4])
		materialRef, ok := StepRef(attrs[5])
		if !ok || len(elementRefs) == 0 {
			continue
		}
		materialType := cache.GetType(materialRef)
		materialAttrs, matOk := cache.GetStepAttrs(materialRef)
		if !matOk {
			continue
		}
		var materialProps []Property
		switch materialType {
		case "IFCMATERIAL":
			if len(materialAttrs) > 0 {
				name, _ := StepString(materialAttrs[0])
				for _, elemID := range elementRefs {
					materialProps = append(materialProps, Property{
						ElementID: elemID, ElementType: elementTypes[elemID],
						PSetName: "Material", PropName: "Material",
						PropValue: name, ValueType: "IFCMATERIAL", Source: "instance",
					})
				}
			}
		case "IFCMATERIALLAYERSET":
			if len(materialAttrs) > 1 {
				subProps := extractMaterialLayerSetStep(cache, materialAttrs, elementRefs, elementTypes)
				materialProps = append(materialProps, subProps...)
			}
		case "IFCMATERIALLAYERSETUSAGE":
			if len(materialAttrs) > 0 {
				if layerSetRef, ok := StepRef(materialAttrs[0]); ok {
					lsType := cache.GetType(layerSetRef)
					lsAttrs, lsOk := cache.GetStepAttrs(layerSetRef)
					if lsOk && lsType == "IFCMATERIALLAYERSET" {
						subProps := extractMaterialLayerSetStep(cache, lsAttrs, elementRefs, elementTypes)
						materialProps = append(materialProps, subProps...)
					}
				}
			}
		}
		properties = append(properties, materialProps...)
	}
	return properties, nil
}

func extractMaterialLayerSetStep(cache *EntityCache, attrs []step.StepValue, elementRefs []uint64, elementTypes map[uint64]string) []Property {
	var props []Property
	if len(attrs) < 1 {
		return props
	}
	setName := ""
	if len(attrs) > 1 {
		setName, _ = StepString(attrs[1])
	}
	layerRefs := StepRefList(attrs[0])
	for i, layerRef := range layerRefs {
		layerAttrs, ok := cache.GetStepAttrs(layerRef)
		if !ok {
			continue
		}
		layerName := ""
		thickness := ""
		if len(layerAttrs) > 0 {
			if matRef, ok := StepRef(layerAttrs[0]); ok {
				matAttrs, matOk := cache.GetStepAttrs(matRef)
				if matOk && len(matAttrs) > 0 {
					layerName, _ = StepString(matAttrs[0])
				}
			}
			if len(layerAttrs) > 1 {
				thickness = StepFormatValue(layerAttrs[1])
			}
		}
		psetName := "Material"
		if setName != "" {
			psetName = "Material: " + setName
		}
		for _, elemID := range elementRefs {
			if layerName != "" {
				props = append(props, Property{
					ElementID: elemID, ElementType: elementTypes[elemID],
					PSetName: psetName, PropName: fmt.Sprintf("Layer %d Material", i+1),
					PropValue: layerName, ValueType: "IFCMATERIAL", Source: "instance",
				})
			}
			if thickness != "" {
				props = append(props, Property{
					ElementID: elemID, ElementType: elementTypes[elemID],
					PSetName: psetName, PropName: fmt.Sprintf("Layer %d Thickness", i+1),
					PropValue: thickness, ValueType: "IFCLENGTHMEASURE", Source: "instance",
				})
			}
		}
	}
	return props
}

// extractClassifications extracts classification associations.
func extractClassifications(cache *EntityCache, elementTypes map[uint64]string) ([]Property, error) {
	var properties []Property
	for _, e := range cache.EntitiesByType("IFCRELASSOCIATESCLASSIFICATION") {
		attrs, ok := cache.GetStepAttrs(e.ID)
		if !ok || len(attrs) < 6 {
			continue
		}
		elementRefs := StepRefList(attrs[4])
		classRef, ok := StepRef(attrs[5])
		if !ok || len(elementRefs) == 0 {
			continue
		}
		classType := cache.GetType(classRef)
		classAttrs, classOk := cache.GetStepAttrs(classRef)
		if !classOk || classType != "IFCCLASSIFICATIONREFERENCE" {
			continue
		}
		itemRef := ""
		className := ""
		systemName := ""
		if len(classAttrs) > 1 {
			itemRef, _ = StepString(classAttrs[1])
		}
		if len(classAttrs) > 2 {
			className, _ = StepString(classAttrs[2])
		}
		if len(classAttrs) > 3 {
			if sysRef, ok := StepRef(classAttrs[3]); ok {
				sysAttrs, sysOk := cache.GetStepAttrs(sysRef)
				if sysOk && len(sysAttrs) > 0 {
					systemName, _ = StepString(sysAttrs[0])
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
					ElementID: elemID, ElementType: elementTypes[elemID],
					PSetName: psetName, PropName: "ItemReference",
					PropValue: itemRef, ValueType: "IFCCLASSIFICATIONREFERENCE", Source: "instance",
				})
			}
			if className != "" {
				properties = append(properties, Property{
					ElementID: elemID, ElementType: elementTypes[elemID],
					PSetName: psetName, PropName: "Name",
					PropValue: className, ValueType: "IFCCLASSIFICATIONREFERENCE", Source: "instance",
				})
			}
		}
	}
	return properties, nil
}

// insertProperties batch-inserts properties into the properties table using the DuckDB Appender API.
func insertProperties(sqlDB *sql.DB, properties []Property) error {
	if len(properties) == 0 {
		return nil
	}

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
		appender, appErr = duckdb.NewAppenderFromConn(dc, "", "properties")
		return appErr
	})
	if err != nil {
		return fmt.Errorf("creating appender: %w", err)
	}
	defer appender.Close()

	for _, p := range properties {
		if err := appender.AppendRow(
			uint32(p.ElementID),
			p.ElementType,
			p.PSetName,
			p.PropName,
			p.PropValue,
			p.ValueType,
			p.Unit,
		); err != nil {
			return fmt.Errorf("appending property: %w", err)
		}
	}

	return appender.Flush()
}

// insertQuantities batch-inserts quantities into the quantities table using the DuckDB Appender API.
func insertQuantities(sqlDB *sql.DB, quantities []Quantity) error {
	if len(quantities) == 0 {
		return nil
	}

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
		appender, appErr = duckdb.NewAppenderFromConn(dc, "", "quantities")
		return appErr
	})
	if err != nil {
		return fmt.Errorf("creating appender: %w", err)
	}
	defer appender.Close()

	for _, q := range quantities {
		if err := appender.AppendRow(
			uint32(q.ElementID),
			q.ElementType,
			q.QSetName,
			q.QuantityName,
			q.QuantityValue,
			q.QuantityType,
			q.Unit,
		); err != nil {
			return fmt.Errorf("appending quantity: %w", err)
		}
	}

	return appender.Flush()
}
