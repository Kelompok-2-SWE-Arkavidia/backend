package domain

import (
	"errors"
	"time"
)

var (
	MessageSuccessGetRecipes      = "success get recipes"
	MessageSuccessGetRecipeDetail = "success get recipe detail"
	MessageSuccessSaveRecipe      = "recipe saved successfully"
	MessageSuccessDeleteRecipe    = "recipe deleted successfully"
	MessageSuccessGetHistory      = "success get recipe history"
	MessageSuccessMarkAsCooked    = "recipe marked as cooked successfully"

	MessageFailedGetRecipes      = "failed to get recipes"
	MessageFailedGetRecipeDetail = "failed to get recipe detail"
	MessageFailedSaveRecipe      = "failed to save recipe"
	MessageFailedDeleteRecipe    = "failed to delete recipe"
	MessageFailedGetHistory      = "failed to get recipe history"
	MessageFailedMarkAsCooked    = "failed to mark recipe as cooked"

	ErrRecipeNotFound           = errors.New("recipe not found")
	ErrUnauthorizedRecipeAccess = errors.New("unauthorized access to recipe")
	ErrGeminiAPIFailed          = errors.New("gemini API processing failed")
	ErrNoIngredients            = errors.New("no ingredients available for recipe generation")
)

type (
	RecipeRecommendationRequest struct {
		IncludeExpiringOnly bool   `json:"include_expiring_only"`
		CuisineType         string `json:"cuisine_type,omitempty"`
		DifficultyLevel     string `json:"difficulty_level,omitempty"`
		PreparationTime     int    `json:"preparation_time,omitempty"` // in minutes
	}

	RecipeDetailRequest struct {
		RecipeID string `json:"recipe_id" validate:"required,uuid"`
	}

	Recipe struct {
		ID              string    `json:"id"`
		Title           string    `json:"title"`
		Description     string    `json:"description"`
		ImageURL        string    `json:"image_url,omitempty"`
		PrepTimeMinutes int       `json:"prep_time_minutes"`
		CookTimeMinutes int       `json:"cook_time_minutes"`
		Servings        int       `json:"servings"`
		DifficultyLevel string    `json:"difficulty_level"`
		CuisineType     string    `json:"cuisine_type"`
		CreatedAt       time.Time `json:"created_at"`
		IsBookmarked    bool      `json:"is_bookmarked"`
		IsCooked        bool      `json:"is_cooked,omitempty"`
		CookedAt        time.Time `json:"cooked_at,omitempty"`
	}

	RecipeDetail struct {
		Recipe
		Ingredients       []Ingredient     `json:"ingredients"`
		Instructions      []string         `json:"instructions"`
		NutritionFacts    NutritionFacts   `json:"nutrition_facts"`
		RequiredItems     []AdditionalItem `json:"required_items,omitempty"`
		SubstitutionItems []AdditionalItem `json:"substitution_items,omitempty"`
	}

	Ingredient struct {
		Name            string  `json:"name"`
		Quantity        float64 `json:"quantity"`
		Unit            string  `json:"unit"`
		IsAvailable     bool    `json:"is_available"`
		ExpiryDate      string  `json:"expiry_date,omitempty"`
		DaysUntilExpiry int     `json:"days_until_expiry,omitempty"`
	}

	AdditionalItem struct {
		Name     string  `json:"name"`
		Quantity float64 `json:"quantity"`
		Unit     string  `json:"unit"`
	}

	NutritionFacts struct {
		Calories      int `json:"calories"`
		Protein       int `json:"protein"`
		Carbohydrates int `json:"carbohydrates"`
		Fat           int `json:"fat"`
		Fiber         int `json:"fiber"`
	}

	BookmarkRecipeRequest struct {
		RecipeID string `json:"recipe_id" validate:"required,uuid"`
	}

	MarkAsCookedRequest struct {
		RecipeID string `json:"recipe_id" validate:"required,uuid"`
	}

	RecipeHistoryResponse struct {
		Recipes []Recipe `json:"recipes"`
		Total   int      `json:"total"`
	}

	RecipeRecommendationResponse struct {
		Recipes       []Recipe `json:"recipes"`
		TotalRecipes  int      `json:"total_recipes"`
		ExpiringItems int      `json:"expiring_items"`
	}
)
