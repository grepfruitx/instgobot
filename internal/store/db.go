package store

import (
	"context"
	"embed"
	"io/fs"
	"time"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

type Store struct {
	db *gorm.DB
}

func Open(path string) (*Store, error) {
	dsn := path + "?_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	migrations, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, migrations)
	if err != nil {
		return nil, err
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
