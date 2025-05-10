package dao

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"ningxia_backend/pkg/logger"
)

const (
	dbFile = "./road.db"
)

var (
	db *gorm.DB
)

func InitDB() (err error) {
	db, err = gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		logger.Logger.Errorf("failed to connect database: %v", err)
		return err
	}

	err = db.AutoMigrate(&ProvinceSetting{}, &NationalSetting{}, &Road{})
	if err != nil {
		logger.Logger.Errorf("failed to AutoMigrate: %v", err)
		return err
	}
	return nil
}

func GetDB() *gorm.DB {
	return db
}
