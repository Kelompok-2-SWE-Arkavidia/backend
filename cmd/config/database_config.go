package config

import (
	"Go-Starter-Template/internal/utils"
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ConnectDB() (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Jakarta",
		utils.GetConfig("DB_HOST"),
		utils.GetConfig("DB_USER"),
		utils.GetConfig("DB_PASSWORD"),
		utils.GetConfig("DB_NAME"),
		utils.GetConfig("DB_PORT"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
		return nil, err
	}
	return db, nil
}
