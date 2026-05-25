package session

import (
	"fmt"

	"xorm.io/xorm"

	"github.com/ys-ll/uniterm/backend/database"
)

type DatabaseSession struct {
	baseSession
	engine *xorm.Engine
	dbType string
	closed bool
}

func NewDatabaseSession(id string) *DatabaseSession {
	return &DatabaseSession{
		baseSession: baseSession{
			id:          id,
			sessionType: "database",
			status:      StatusDisconnected,
		},
	}
}

func (s *DatabaseSession) Connect(config ConnectionConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.setStatus(StatusConnecting)

	// Fall back to config.Type if DBType is not set (e.g., legacy connections)
	if config.DBType == "" {
		config.DBType = config.Type
	}
	s.dbType = config.DBType

	if config.Name != "" {
		s.title = config.Name
	} else {
		s.title = fmt.Sprintf("%s:%s@%s:%d", config.DBType, config.User, config.Host, config.Port)
	}

	dsn, err := database.BuildDSN(config.DBType, config.Host, config.User, config.Password, config.DBName, config.Port)
	if err != nil {
		s.setStatus(StatusError)
		return err
	}

	engine, err := database.NewEngine(config.DBType, dsn)
	if err != nil {
		s.setStatus(StatusError)
		return err
	}

	if err := engine.Ping(); err != nil {
		s.setStatus(StatusError)
		return fmt.Errorf("ping %s: %w", config.DBType, err)
	}

	s.engine = engine
	s.setStatus(StatusConnected)
	return nil
}

func (s *DatabaseSession) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	if s.engine != nil {
		s.engine.Close()
	}
	s.setStatus(StatusDisconnected)
	return nil
}

func (s *DatabaseSession) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status == StatusConnected && s.engine != nil
}

func (s *DatabaseSession) Write(data []byte) error {
	return nil
}

func (s *DatabaseSession) Resize(cols, rows int) error {
	return nil
}

// Engine returns the underlying xorm engine (used by Wails bindings).
func (s *DatabaseSession) Engine() *xorm.Engine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.engine
}

// DBType returns the database type string.
func (s *DatabaseSession) DBType() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dbType
}
