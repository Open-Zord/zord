package database

import (
	"fmt"

	"github.com/Open-Zord/zord/pkg/logger"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// RegistryKey is the key under which the *sqlx.DB is registered in the registry.
const RegistryKey = "db"

type MySQL struct {
	logger   *logger.Logger
	DB       *sqlx.DB
	dbUser   string
	dbPass   string
	dbURL    string
	dbPort   string
	database string
}

func NewMySQL(
	l *logger.Logger,
	dbUser string,
	dbPass string,
	dbURL string,
	dbPort string,
	database string,
) *MySQL {
	return &MySQL{
		logger:   l,
		dbUser:   dbUser,
		dbPass:   dbPass,
		dbURL:    dbURL,
		dbPort:   dbPort,
		database: database,
	}
}

// Connect abre e valida (Ping) a conexão, armazenando-a em DB. Devolve error em
// vez de derrubar o processo — a decisão de abortar o boot é do composition root.
func (m *MySQL) Connect() error {
	config := mysql.Config{
		User:      m.dbUser,
		Passwd:    m.dbPass,
		Addr:      m.dbURL + ":" + m.dbPort,
		Net:       "tcp",
		ParseTime: true,
		DBName:    m.database,
	}

	conn, err := sqlx.Open("mysql", config.FormatDSN())
	if err != nil {
		return fmt.Errorf("abrir conexão mysql: %w", err)
	}
	conn.SetMaxOpenConns(5)
	conn.SetMaxIdleConns(3)
	if err := conn.Ping(); err != nil {
		return fmt.Errorf("ping mysql: %w", err)
	}
	m.DB = conn
	return nil
}
