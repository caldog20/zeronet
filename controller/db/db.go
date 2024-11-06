package db

import (
	"fmt"
	"runtime"

	// "github.com/glebarez/sqlite"
	log "github.com/sirupsen/logrus"
	gormv2logrus "github.com/thomas-tacquet/gormv2-logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/caldog20/zeronet/controller/types"
)

type Store struct {
	db *gorm.DB
}

func New(path string, e *log.Entry) (*Store, error) {
	gormLogger := gormv2logrus.NewGormlog(gormv2logrus.WithLogrusEntry(e))
	gormLogger.LogMode(logger.Info)

	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:%s?cache=shared&_journal_mode=WAL&_synchronous=1", path)),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      gormLogger,
		},
	)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(runtime.NumCPU())

	err = sqlDB.Ping()
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&types.Peer{})

	return &Store{db: db}, nil
}
