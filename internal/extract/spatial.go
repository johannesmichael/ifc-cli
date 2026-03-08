package extract

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"

	"github.com/marcboeker/go-duckdb"
)

// spatialNode represents an element in the spatial hierarchy tree.
type spatialNode struct {
	ID       uint64
	Type     string
	Name     string
	ParentID uint64
	Level    int
	Path     string
	Children []*spatialNode
}

// ExtractSpatialHierarchy builds the spatial hierarchy from the cache and writes to spatial_structure.
func ExtractSpatialHierarchy(db *sql.DB, cache *EntityCache, onProgress ProgressFunc) error {
	nodes := make(map[uint64]*spatialNode)
	childToParent := make(map[uint64]uint64)

	// Step 1: Build spatial tree from IFCRELAGGREGATES in cache.
	// attrs[4] = RelatingObject (parent, single ref), attrs[5] = RelatedObjects (children, list of refs).
	for _, e := range cache.EntitiesByType("IFCRELAGGREGATES") {
		attrs, ok := cache.GetStepAttrs(e.ID)
		if !ok || len(attrs) < 6 {
			continue
		}
		parentRef, ok := StepRef(attrs[4])
		if !ok {
			continue
		}
		childRefs := StepRefList(attrs[5])

		// Ensure parent node exists
		if _, ok := nodes[parentRef]; !ok {
			nodes[parentRef] = &spatialNode{
				ID:   parentRef,
				Type: cache.GetType(parentRef),
				Name: cache.GetName(parentRef),
			}
		}

		for _, childRef := range childRefs {
			if _, ok := nodes[childRef]; !ok {
				nodes[childRef] = &spatialNode{
					ID:   childRef,
					Type: cache.GetType(childRef),
					Name: cache.GetName(childRef),
				}
			}
			parent := nodes[parentRef]
			child := nodes[childRef]
			if isSpatialType(parent.Type) && isSpatialType(child.Type) {
				child.ParentID = parentRef
				parent.Children = append(parent.Children, child)
				childToParent[childRef] = parentRef
			}
		}
	}

	// Step 2: Get containment from IFCRELCONTAINEDINSPATIALSTRUCTURE in cache.
	// attrs[4] = RelatedElements (list of element refs), attrs[5] = RelatingStructure (single spatial ref).
	containment := make(map[uint64]uint64)
	for _, e := range cache.EntitiesByType("IFCRELCONTAINEDINSPATIALSTRUCTURE") {
		attrs, ok := cache.GetStepAttrs(e.ID)
		if !ok || len(attrs) < 6 {
			continue
		}
		elementRefs := StepRefList(attrs[4])
		spatialRef, ok := StepRef(attrs[5])
		if !ok {
			continue
		}
		for _, elemRef := range elementRefs {
			containment[elemRef] = spatialRef
		}
		// Ensure spatial node exists
		if _, ok := nodes[spatialRef]; !ok {
			nodes[spatialRef] = &spatialNode{
				ID:   spatialRef,
				Type: cache.GetType(spatialRef),
				Name: cache.GetName(spatialRef),
			}
		}
	}

	// Step 3: Find roots (spatial nodes with no parent) and compute levels/paths.
	var roots []*spatialNode
	for id, node := range nodes {
		if _, hasParent := childToParent[id]; !hasParent && isSpatialType(node.Type) {
			roots = append(roots, node)
		}
	}

	// Assign levels and paths via BFS
	for _, root := range roots {
		assignLevelAndPath(root, 0, "")
	}
	if onProgress != nil {
		onProgress("spatial tree", len(nodes))
	}

	// Step 4: Write to spatial_structure table using DuckDB Appender.
	conn, err := db.Conn(context.Background())
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
		appender, appErr = duckdb.NewAppenderFromConn(dc, "", "spatial_structure")
		return appErr
	})
	if err != nil {
		return fmt.Errorf("creating appender: %w", err)
	}
	defer appender.Close()

	appendSpatialRow := func(elemID uint64, elemType, elemName string, parentID uint64, parentType string, level int, path string) error {
		var nameVal interface{}
		if elemName != "" {
			nameVal = elemName
		}
		var pidVal, ptypeVal interface{}
		if parentID != 0 {
			pidVal = uint32(parentID)
			if parentType != "" {
				ptypeVal = parentType
			}
		}
		return appender.AppendRow(uint32(elemID), elemType, nameVal, pidVal, ptypeVal, int32(level), path)
	}

	// Insert all spatial hierarchy nodes
	var insertNode func(n *spatialNode) error
	insertNode = func(n *spatialNode) error {
		parentType := ""
		if n.ParentID != 0 {
			if p, ok := nodes[n.ParentID]; ok {
				parentType = p.Type
			}
		}
		if err := appendSpatialRow(n.ID, n.Type, n.Name, n.ParentID, parentType, n.Level, n.Path); err != nil {
			return fmt.Errorf("appending spatial node %d: %w", n.ID, err)
		}
		for _, child := range n.Children {
			if err := insertNode(child); err != nil {
				return err
			}
		}
		return nil
	}

	for _, root := range roots {
		if err := insertNode(root); err != nil {
			return err
		}
	}

	// Insert contained (non-spatial) elements
	for elemID, containerID := range containment {
		container, ok := nodes[containerID]
		if !ok {
			continue
		}
		elemType := cache.GetType(elemID)
		elemName := cache.GetName(elemID)

		level := container.Level + 1

		path := container.Path
		if elemName != "" {
			path = path + "/" + elemName
		} else {
			path = path + "/" + elemType
		}

		if err := appendSpatialRow(elemID, elemType, elemName, containerID, container.Type, level, path); err != nil {
			return fmt.Errorf("appending contained element %d: %w", elemID, err)
		}
	}

	if onProgress != nil {
		onProgress("spatial insertions", len(nodes)+len(containment))
	}

	return appender.Flush()
}

// assignLevelAndPath recursively sets level and path on the spatial tree.
func assignLevelAndPath(node *spatialNode, level int, parentPath string) {
	node.Level = level
	name := node.Name
	if name == "" {
		name = node.Type
	}
	if parentPath == "" {
		node.Path = name
	} else {
		node.Path = parentPath + "/" + name
	}
	for _, child := range node.Children {
		assignLevelAndPath(child, level+1, node.Path)
	}
}

// nullStr returns a sql.NullString, marking empty strings as null.
func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
