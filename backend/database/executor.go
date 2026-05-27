package database

import (
	"database/sql"
	"encoding/json"
)

type QueryResultColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type QueryResult struct {
	Columns []QueryResultColumn `json:"columns"`
	Rows    []map[string]any    `json:"rows"`
}

type ExecResult struct {
	Affected     int64 `json:"affected"`
	LastInsertID int64 `json:"lastInsertId"`
}

func ExecuteQuery(p Provider, db *sql.DB, dbName, sqlStr string) (*QueryResult, error) {
	if err := p.PrepareExec(db, dbName); err != nil {
		return nil, err
	}

	rows, cols, err := queryAny(db, sqlStr)
	if err != nil {
		return nil, err
	}

	columns := make([]QueryResultColumn, 0, len(cols))
	for _, c := range cols {
		columns = append(columns, QueryResultColumn{Name: c, Type: ""})
	}

	if len(rows) == 0 {
		return &QueryResult{Columns: columns, Rows: []map[string]any{}}, nil
	}

	return &QueryResult{Columns: columns, Rows: rows}, nil
}

func ExecuteStatement(p Provider, db *sql.DB, dbName, sqlStr string) (*ExecResult, error) {
	if err := p.PrepareExec(db, dbName); err != nil {
		return nil, err
	}

	result, err := db.Exec(sqlStr)
	if err != nil {
		return nil, err
	}

	affected, _ := result.RowsAffected()
	lastID, _ := result.LastInsertId()

	return &ExecResult{Affected: affected, LastInsertID: lastID}, nil
}

// QueryResultToJSON serializes a QueryResult to JSON bytes.
func QueryResultToJSON(qr *QueryResult) ([]byte, error) {
	return json.Marshal(qr)
}
