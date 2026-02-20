package gormutil

import (
	"strings"
	"time"

	"github.com/saltbo/zpan/internal/pkg/logger"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

var defaultDB *gorm.DB

func Init(conf Config, debug bool) {
	db, err := New(conf)
	if err != nil {
		logger.Fatal("Failed to initialize database", "error", err)
	}

	if debug {
		db = db.Debug()
	}

	// Configure connection pool for better concurrency handling
	configureConnectionPool(db, conf.Driver)

	defaultDB = db
}

func AutoMigrate(models []interface{}) {
	defaultDB.AutoMigrate(models...)
}

func DB() *gorm.DB {
	return defaultDB
}

type Config struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

func New(conf Config) (*gorm.DB, error) {
	var director func(dsn string) gorm.Dialector
	switch conf.Driver {
	case "mysql":
		director = mysql.Open
	case "postgres":
		director = postgres.Open
	case "sqlserver":
		director = sqlserver.Open
	case "sqlite3":
		director = sqlite.Open
		conf.DSN = sqliteDSN(conf.DSN)
	case "sqlite":
		director = sqlite.Open
		conf.DSN = sqliteDSN(conf.DSN)
	default:
		logger.Fatal("Unsupported database driver: " + conf.Driver)
	}

	return gorm.Open(director(conf.DSN), &gorm.Config{
		Logger: logger.NewGormSlogger(),
	})
}

func sqliteDSN(dsn string) string {
	// Remove existing parameters if any to ensure clean configuration
	if idx := strings.Index(dsn, "?"); idx != -1 {
		dsn = dsn[:idx]
	}

	// Apply optimized SQLite settings for concurrent access with large batch uploads
	// Significantly aggressive tuning for high-concurrency scenarios:
	// timeout: 120000ms (120s) - handle large folder uploads with many concurrent writes
	// _journal_mode=WAL: Write-Ahead Logging for better concurrency
	// cache=shared: Shared cache for connection pooling
	// mode=rwc: Read-Write-Create mode
	// _busy_timeout: Additional timeout for busy database (must match timeout)
	// synchronous: NORMAL mode balances speed and safety (SQLite default for WAL)
	// temp_store: MEMORY to use memory for temporary tables (reduces I/O)
	dsn += "?_journal_mode=WAL&cache=shared&timeout=120000&mode=rwc&_busy_timeout=120000&synchronous=NORMAL&temp_store=MEMORY&_locking_mode=NORMAL"

	logger.Info("Using SQLite with optimized concurrent DSN: " + dsn)
	return dsn
}

// configureConnectionPool configures the database connection pool for optimal concurrency
func configureConnectionPool(db *gorm.DB, driver string) {
	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("failed to get database connection", "error", err)
		return
	}

	if driver == "sqlite" || driver == "sqlite3" {
		// SQLite handles concurrency differently, set single connection with longer timeout
		// This prevents connection pool overhead for local databases
		sqlDB.SetMaxOpenConns(5)  // Allow multiple connections for concurrent writes
		sqlDB.SetMaxIdleConns(2)  // Keep some idle connections ready
		sqlDB.SetConnMaxLifetime(time.Hour)
	} else {
		// For other databases (MySQL, PostgreSQL, SQL Server)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}
}
