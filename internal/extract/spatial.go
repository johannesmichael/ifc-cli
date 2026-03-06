package extract

import (
	"database/sql"
	"fmt"
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

// ExtractSpatialHierarchy builds the spatial hierarchy from relationships and writes to spatial_structure.
func ExtractSpatialHierarchy(db *sql.DB) error {
	nodes := make(map[uint64]*spatialNode)
	childToParent := make(map[uint64]uint64)

	// Step 1: Get all IFCRELAGGREGATES relationships to build the spatial tree.
	// source_id is the relating object (parent), target_id is a related object (child).
	rows, err := db.Query(`
		SELECT source_id, target_id FROM relationships
		WHERE rel_type = 'IFCRELAGGREGATES'
	`)
	if err != nil {
		return fmt.Errorf("query aggregation rels: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var sourceID, targetID uint64
		if err := rows.Scan(&sourceID, &targetID); err != nil {
			return fmt.Errorf("scan aggregation: %w", err)
		}

		// Ensure nodes exist
		if _, ok := nodes[sourceID]; !ok {
			nodes[sourceID] = &spatialNode{
				ID:   sourceID,
				Type: entityType(db, sourceID),
				Name: entityName(db, sourceID),
			}
		}
		if _, ok := nodes[targetID]; !ok {
			nodes[targetID] = &spatialNode{
				ID:   targetID,
				Type: entityType(db, targetID),
				Name: entityName(db, targetID),
			}
		}

		// Only process spatial types for the hierarchy
		parent := nodes[sourceID]
		child := nodes[targetID]
		if isSpatialType(parent.Type) && isSpatialType(child.Type) {
			child.ParentID = sourceID
			parent.Children = append(parent.Children, child)
			childToParent[targetID] = sourceID
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("aggregation rows: %w", err)
	}

	// Step 2: Get containment relationships (elements in spatial containers).
	// For IFCRELCONTAINEDINSPATIALSTRUCTURE: source=attr[4] (elements list), target=attr[5] (spatial element).
	containRows, err := db.Query(`
		SELECT source_id, target_id FROM relationships
		WHERE rel_type = 'IFCRELCONTAINEDINSPATIALSTRUCTURE'
	`)
	if err != nil {
		return fmt.Errorf("query containment rels: %w", err)
	}
	defer containRows.Close()

	// Map of element_id -> container_id for non-spatial elements
	containment := make(map[uint64]uint64)
	for containRows.Next() {
		var elementID, spatialID uint64
		if err := containRows.Scan(&elementID, &spatialID); err != nil {
			return fmt.Errorf("scan containment: %w", err)
		}
		containment[elementID] = spatialID

		// Ensure spatial node exists
		if _, ok := nodes[spatialID]; !ok {
			nodes[spatialID] = &spatialNode{
				ID:   spatialID,
				Type: entityType(db, spatialID),
				Name: entityName(db, spatialID),
			}
		}
	}
	if err := containRows.Err(); err != nil {
		return fmt.Errorf("containment rows: %w", err)
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

	// Step 4: Write to spatial_structure table.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO spatial_structure
		(element_id, element_type, element_name, parent_id, parent_type, hierarchy_level, path)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	// Insert all spatial hierarchy nodes
	var insertNode func(n *spatialNode) error
	insertNode = func(n *spatialNode) error {
		var parentID sql.NullInt64
		var parentType sql.NullString
		if n.ParentID != 0 {
			parentID = sql.NullInt64{Int64: int64(n.ParentID), Valid: true}
			if p, ok := nodes[n.ParentID]; ok {
				parentType = sql.NullString{String: p.Type, Valid: true}
			}
		}
		if _, err := stmt.Exec(n.ID, n.Type, nullStr(n.Name), parentID, parentType, n.Level, n.Path); err != nil {
			return fmt.Errorf("insert spatial node %d: %w", n.ID, err)
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
		elemType := entityType(db, elemID)
		elemName := entityName(db, elemID)

		// Level is one deeper than the container
		level := container.Level + 1

		// Build path from container
		path := container.Path
		if elemName != "" {
			path = path + "/" + elemName
		} else {
			path = path + "/" + elemType
		}

		parentID := sql.NullInt64{Int64: int64(containerID), Valid: true}
		parentType := sql.NullString{String: container.Type, Valid: true}

		if _, err := stmt.Exec(elemID, elemType, nullStr(elemName), parentID, parentType, level, path); err != nil {
			return fmt.Errorf("insert contained element %d: %w", elemID, err)
		}
	}

	return tx.Commit()
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
