package database

import (
    "fmt"

    _ "github.com/go-sql-driver/mysql"
    _ "github.com/lib/pq"
    _ "github.com/rqlite/gorqlite/stdlib"

    "xorm.io/xorm"
)

// driverName maps DBType to the database/sql driver name for xorm.
var driverName = map[string]string{
    "mysql":    "mysql",
    "postgres": "postgres",
    "rqlite":   "rqlite",
}

// BuildDSN builds a DSN string from connection config fields.
func BuildDSN(dbType, host, user, password, dbName string, port int) (string, error) {
    switch dbType {
    case "mysql":
        dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true",
            user, password, host, port, dbName)
        return dsn, nil
    case "postgres":
        dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
            host, port, user, password, dbName)
        return dsn, nil
    case "rqlite":
        if user != "" && password != "" {
            return fmt.Sprintf("http://%s:%s@%s:%d/", user, password, host, port), nil
        }
        return fmt.Sprintf("http://%s:%d/", host, port), nil
    default:
        return "", fmt.Errorf("unsupported database type: %s", dbType)
    }
}

// NewEngine creates an xorm.Engine for the given database type and DSN.
func NewEngine(dbType, dsn string) (*xorm.Engine, error) {
    drv, ok := driverName[dbType]
    if !ok {
        return nil, fmt.Errorf("unsupported database type: %s", dbType)
    }
    engine, err := xorm.NewEngine(drv, dsn)
    if err != nil {
        return nil, fmt.Errorf("open %s: %w", dbType, err)
    }
    return engine, nil
}
