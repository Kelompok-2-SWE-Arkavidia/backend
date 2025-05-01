package migration

import (
	"Go-Starter-Template/entities"
	"fmt"
	"log"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";")
	db.Exec("CREATE EXTENSION IF NOT EXISTS \"earthdistance\" CASCADE;")
	db.Exec("CREATE EXTENSION IF NOT EXISTS \"cube\";")

	if err := db.AutoMigrate(&entities.User{}); err != nil {
		log.Fatalf("Error migrating user database: %v", err)
		return err
	}
	if err := db.AutoMigrate(&entities.Transaction{}); err != nil {
		log.Fatalf("Error migrating transaction database: %v", err)
		return err
	}

	if err := db.AutoMigrate(&entities.FoodItem{}); err != nil {
		log.Fatalf("Error migrating food item database: %v", err)
		return err
	}
	if err := db.AutoMigrate(&entities.ReceiptScan{}); err != nil {
		log.Fatalf("Error migrating receipt scan database: %v", err)
		return err
	}

	if err := db.AutoMigrate(&entities.Recipe{}); err != nil {
		log.Fatalf("Error migrating recipe database: %v", err)
		return err
	}
	if err := db.AutoMigrate(&entities.RecipeBookmark{}); err != nil {
		log.Fatalf("Error migrating recipe bookmark database: %v", err)
		return err
	}
	if err := db.AutoMigrate(&entities.RecipeHistory{}); err != nil {
		log.Fatalf("Error migrating recipe history database: %v", err)
	}

	fmt.Println("Database migration complete")
	return nil
}
