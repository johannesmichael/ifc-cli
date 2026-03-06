package db

import (
	"database/sql"
	"sort"
	"strings"

	"ifc-cli/internal/step"
)

// WriteMetadata writes file metadata and extra key-value pairs to the file_metadata table.
func WriteMetadata(db *sql.DB, meta *step.FileMetadata, extra map[string]string) error {
	stmt, err := db.Prepare("INSERT OR REPLACE INTO file_metadata (key, value) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	if meta != nil {
		pairs := []struct{ k, v string }{
			{"description", meta.Description},
			{"implementation_level", meta.ImplementationLevel},
			{"file_name", meta.FileName},
			{"timestamp", meta.Timestamp},
			{"author", strings.Join(meta.Author, "; ")},
			{"organization", strings.Join(meta.Organization, "; ")},
			{"preprocessor", meta.Preprocessor},
			{"originating_system", meta.OriginatingSystem},
			{"authorization", meta.Authorization},
			{"schema_identifiers", strings.Join(meta.SchemaIdentifiers, "; ")},
		}
		for _, p := range pairs {
			if p.v != "" {
				if _, err := stmt.Exec(p.k, p.v); err != nil {
					return err
				}
			}
		}
	}

	if len(extra) > 0 {
		keys := make([]string, 0, len(extra))
		for k := range extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if _, err := stmt.Exec(k, extra[k]); err != nil {
				return err
			}
		}
	}

	return nil
}
