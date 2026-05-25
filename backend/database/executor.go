package database

import (
	"encoding/json"

	"xorm.io/xorm"
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

func ExecuteQuery(engine *xorm.Engine, sql string) (*QueryResult, error) {
	rows, err := engine.QueryString(sql)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return &QueryResult{Columns: []QueryResultColumn{}, Rows: []map[string]any{}}, nil
	}

	// Extract column names from the first row
	firstRow := rows[0]
	columns := make([]QueryResultColumn, 0, len(firstRow))
	for key := range firstRow {
		columns = append(columns, QueryResultColumn{Name: key, Type: ""})
	}

	// Convert []map[string]string to []map[string]any for JSON serialization
	resultRows := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		m := make(map[string]any, len(row))
		for k, v := range row {
			m[k] = v
		}
		resultRows = append(resultRows, m)
	}

	return &QueryResult{Columns: columns, Rows: resultRows}, nil
}

func ExecuteStatement(engine *xorm.Engine, sql string) (*ExecResult, error) {
	result, err := engine.Exec(sql)
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
