package migration

import (
	entities2 "Go-Starter-Template/entities"
	"fmt"
	"log"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	// Setup PostgreSQL extensions for geographical calculations
	db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";")
	db.Exec("CREATE EXTENSION IF NOT EXISTS \"earthdistance\" CASCADE;")
	db.Exec("CREATE EXTENSION IF NOT EXISTS \"cube\";")

	if err := db.AutoMigrate(&entities2.User{}); err != nil {
		log.Fatalf("Error migrating user database: %v", err)
		return err
	}
	if err := db.AutoMigrate(&entities2.Transaction{}); err != nil {
		log.Fatalf("Error migrating transaction database: %v", err)
		return err
	}

	if err := db.AutoMigrate(&entities2.FoodItem{}); err != nil {
		log.Fatalf("Error migrating food item database: %v", err)
		return err
	}
	if err := db.AutoMigrate(&entities2.ReceiptScan{}); err != nil {
		log.Fatalf("Error migrating receipt scan database: %v", err)
		return err
	}

	fmt.Println("Database migration complete")
	return nil
}
