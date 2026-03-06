package db

import (
	"context"
	"database/sql"
	"database/sql/driver"

	"github.com/marcboeker/go-duckdb"

	"ifc-cli/internal/step"
)

// Writer batches parsed entities and bulk-loads them into DuckDB using the Appender API.
type Writer struct {
	db        *sql.DB
	conn      *sql.Conn
	appender  *duckdb.Appender
	batchSize int
	batch     []*step.Entity
}

// NewWriter creates a Writer that bulk-loads entities into the entities table.
func NewWriter(database *Database, batchSize int) (*Writer, error) {
	if batchSize <= 0 {
		batchSize = 10000
	}

	conn, err := database.DB.Conn(context.Background())
	if err != nil {
		return nil, err
	}

	var appender *duckdb.Appender
	err = conn.Raw(func(driverConn interface{}) error {
		dc, ok := driverConn.(driver.Conn)
		if !ok {
			return sql.ErrConnDone
		}
		var appErr error
		appender, appErr = duckdb.NewAppenderFromConn(dc, "", "entities")
		return appErr
	})
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &Writer{
		db:        database.DB,
		conn:      conn,
		appender:  appender,
		batchSize: batchSize,
		batch:     make([]*step.Entity, 0, batchSize),
	}, nil
}

// Write adds an entity to the batch. Flushes when batch is full.
func (w *Writer) Write(entity *step.Entity) error {
	w.batch = append(w.batch, entity)
	if len(w.batch) >= w.batchSize {
		return w.Flush()
	}
	return nil
}

// Flush writes all buffered entities to DuckDB via the Appender.
func (w *Writer) Flush() error {
	for _, e := range w.batch {
		attrsJSON, err := step.MarshalAttrs(e.Attrs)
		if err != nil {
			return err
		}
		err = w.appender.AppendRow(
			uint32(e.ID),
			e.Type,
			string(attrsJSON),
		)
		if err != nil {
			return err
		}
	}
	w.batch = w.batch[:0]
	return w.appender.Flush()
}

// Close flushes remaining entities and cleans up.
func (w *Writer) Close() error {
	if err := w.Flush(); err != nil {
		w.appender.Close()
		w.conn.Close()
		return err
	}
	if err := w.appender.Close(); err != nil {
		w.conn.Close()
		return err
	}
	return w.conn.Close()
}
