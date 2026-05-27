package database


// ── Shared types ──

type TableInfo struct {
	Name string `json:"name"`
	Type string `json:"type"` // "table" or "view"
}

type ColumnInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Nullable    bool   `json:"nullable"`
	DefaultVal  string `json:"defaultVal"`
	DefaultType string `json:"defaultType"` // "none" | "null" | "value" | "auto"
	IsPrimary   bool   `json:"isPrimary"`
	Comment     string `json:"comment"`
	Collation   string `json:"collation"`
	OnUpdate    bool   `json:"onUpdate"`
}

type IndexInfo struct {
	Name      string   `json:"name"`
	Columns   []string `json:"columns"`
	Unique    bool     `json:"unique"`
	IsPrimary bool     `json:"isPrimary"`
}

type SchemaResult struct {
	Columns []ColumnInfo `json:"columns"`
	Indexes []IndexInfo  `json:"indexes"`
}

// ColumnDef is the structured input for adding or modifying a column.
type ColumnDef struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Nullable    bool   `json:"nullable"`
	DefaultVal  string `json:"defaultVal"`
	DefaultType string `json:"defaultType"` // "none" | "null" | "value" | "auto"
	Comment     string `json:"comment"`
	Collation   string `json:"collation"`
	OnUpdate    bool   `json:"onUpdate"`
}

// IndexDef is the structured input for creating an index.
type IndexDef struct {
	Name      string   `json:"name"`
	Columns   []string `json:"columns"`
	Unique    bool     `json:"unique"`
	IsPrimary bool     `json:"isPrimary"`
}

// DBCapabilities describes the features supported by a database type.
type DBCapabilities struct {
	SupportsAutoIncrement      bool     `json:"supportsAutoIncrement"`
	SupportsOnUpdate           bool     `json:"supportsOnUpdate"`
	SupportsCollation          bool     `json:"supportsCollation"`
	SupportsComment            bool     `json:"supportsComment"`
	SupportsModifyColumn       bool     `json:"supportsModifyColumn"`
	SupportsPrimaryKey         bool     `json:"supportsPrimaryKey"`
	AutoIncrementForcesNotNull bool     `json:"autoIncrementForcesNotNull"`
	ColumnTypes                []string `json:"columnTypes"`
	IntTypes                   []string `json:"intTypes"`
}
