package database

import (
	"fmt"
	"strings"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type TableInfo struct {
	Name string `json:"name"`
}

type ColumnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	DefaultVal string `json:"defaultVal"`
	IsPrimary  bool   `json:"isPrimary"`
}

type IndexInfo struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
}

type SchemaResult struct {
	Columns []ColumnInfo `json:"columns"`
	Indexes []IndexInfo  `json:"indexes"`
}

// GetDatabases returns database names.
func GetDatabases(engine *xorm.Engine, dbType string) ([]string, error) {
	switch dbType {
	case "mysql":
		results, err := engine.QueryString("SHOW DATABASES")
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(results))
		for _, row := range results {
			names = append(names, row["Database"])
		}
		return names, nil
	case "postgres":
		results, err := engine.QueryString("SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname")
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(results))
		for _, row := range results {
			names = append(names, row["datname"])
		}
		return names, nil
	case "rqlite":
		return []string{"main"}, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// GetTables returns table names for a database.
func GetTables(engine *xorm.Engine, dbType, dbName string) ([]TableInfo, error) {
	switch dbType {
	case "mysql":
		engine.Exec(fmt.Sprintf("USE `%s`", dbName))
	}

	tables, err := engine.DBMetas()
	if err != nil {
		return nil, fmt.Errorf("get table metas: %w", err)
	}

	infos := make([]TableInfo, 0, len(tables))
	for _, t := range tables {
		infos = append(infos, TableInfo{Name: t.Name})
	}
	return infos, nil
}

// GetTableSchema returns columns and indexes for a table.
func GetTableSchema(engine *xorm.Engine, dbName, tableName string) (*SchemaResult, error) {
	tables, err := engine.DBMetas()
	if err != nil {
		return nil, fmt.Errorf("get table metas: %w", err)
	}

	var target *schemas.Table
	for i := range tables {
		if tables[i].Name == tableName {
			target = tables[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	columns := make([]ColumnInfo, 0, len(target.Columns()))
	seq := target.ColumnsSeq()
	for _, colName := range seq {
		col := target.GetColumn(colName)
		columns = append(columns, ColumnInfo{
			Name:       col.Name,
			Type:       strings.ToUpper(col.SQLType.Name),
			Nullable:   col.Nullable,
			DefaultVal: col.Default,
			IsPrimary:  col.IsPrimaryKey,
		})
	}

	indexes := make([]IndexInfo, 0)
	for _, idx := range target.Indexes {
		indexes = append(indexes, IndexInfo{
			Name:    idx.Name,
			Columns: idx.Cols,
			Unique:  idx.Type == schemas.UniqueType,
		})
	}

	return &SchemaResult{Columns: columns, Indexes: indexes}, nil
}
