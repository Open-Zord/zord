package database

import (
	"log"

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

func (m *MySQL) Connect() {
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
		log.Fatalln(err)
	}
	conn.SetMaxOpenConns(5)
	conn.SetMaxIdleConns(3)
	if err := conn.Ping(); err != nil {
		log.Fatalln(err)
	}
	m.DB = conn
}
