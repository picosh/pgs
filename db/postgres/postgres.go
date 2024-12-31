package postgres

import (
	"database/sql"
	"log/slog"
)

type PsqlDB struct {
	Logger *slog.Logger
	Db     *sql.DB
}

func NewDB(databaseUrl string, logger *slog.Logger) *PsqlDB {
	var err error
	d := &PsqlDB{
		Logger: logger,
	}
	d.Logger.Info("Connecting to postgres", "databaseUrl", databaseUrl)

	db, err := sql.Open("postgres", databaseUrl)
	if err != nil {
		d.Logger.Error(err.Error())
	}
	d.Db = db
	return d
}
