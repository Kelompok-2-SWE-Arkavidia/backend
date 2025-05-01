// File: entities/recipe.go
package entities

import (
	"github.com/google/uuid"
	"time"
)

type Recipe struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	ImageURL        string    `json:"image_url,omitempty"`
	PrepTimeMinutes int       `json:"prep_time_minutes"`
	CookTimeMinutes int       `json:"cook_time_minutes"`
	Servings        int       `json:"servings"`
	DifficultyLevel string    `json:"difficulty_level"`
	CuisineType     string    `json:"cuisine_type"`
	Ingredients     string    `json:"ingredients" gorm:"type:text"`
	Instructions    string    `json:"instructions" gorm:"type:text"`
	NutritionFacts  string    `json:"nutrition_facts" gorm:"type:text"`
	IsGenerated     bool      `json:"is_generated"`

	User *User `gorm:"foreignKey:UserID"`
	Timestamp
}

type RecipeBookmark struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	RecipeID  uuid.UUID `json:"recipe_id"`
	CreatedAt time.Time `gorm:"type:timestamp" json:"created_at"`

	User   *User   `gorm:"foreignKey:UserID"`
	Recipe *Recipe `gorm:"foreignKey:RecipeID"`
}

type RecipeHistory struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID   uuid.UUID `json:"user_id"`
	RecipeID uuid.UUID `json:"recipe_id"`
	CookedAt time.Time `gorm:"type:timestamp" json:"cooked_at"`

	User   *User   `gorm:"foreignKey:UserID"`
	Recipe *Recipe `gorm:"foreignKey:RecipeID"`
}
