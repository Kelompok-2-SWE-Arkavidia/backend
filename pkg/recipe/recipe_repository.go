package recipe

import (
	"Go-Starter-Template/entities"
	"context"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type (
	RecipeRepository interface {
		CreateRecipe(ctx context.Context, recipe *entities.Recipe) error
		GetRecipeByID(ctx context.Context, id string) (*entities.Recipe, error)
		GetRecipes(ctx context.Context, userID string, page, limit int) ([]*entities.Recipe, int64, error)
		GetRecipeBookmarks(ctx context.Context, userID string, page, limit int) ([]*entities.Recipe, int64, error)
		BookmarkRecipe(ctx context.Context, userID, recipeID string) error
		RemoveBookmark(ctx context.Context, userID, recipeID string) error
		IsRecipeBookmarked(ctx context.Context, userID, recipeID string) (bool, error)
		AddRecipeHistory(ctx context.Context, userID, recipeID string) error
		GetRecipeHistory(ctx context.Context, userID string, page, limit int) ([]*entities.Recipe, int64, error)
		IsRecipeInHistory(ctx context.Context, userID, recipeID string) (bool, error)
	}

	recipeRepository struct {
		db *gorm.DB
	}
)

func NewRecipeRepository(db *gorm.DB) RecipeRepository {
	return &recipeRepository{db: db}
}

func (r *recipeRepository) CreateRecipe(ctx context.Context, recipe *entities.Recipe) error {
	return r.db.WithContext(ctx).Create(recipe).Error
}

func (r *recipeRepository) GetRecipeByID(ctx context.Context, id string) (*entities.Recipe, error) {
	var recipe entities.Recipe
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&recipe).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &recipe, nil
}

func (r *recipeRepository) GetRecipes(ctx context.Context, userID string, page, limit int) ([]*entities.Recipe, int64, error) {
	var recipes []*entities.Recipe
	var count int64
	offset := (page - 1) * limit

	if err := r.db.WithContext(ctx).Model(&entities.Recipe{}).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.WithContext(ctx).
		Offset(offset).
		Limit(limit).
		Order("created_at desc").
		Find(&recipes).Error; err != nil {
		return nil, 0, err
	}

	return recipes, count, nil
}

func (r *recipeRepository) GetRecipeBookmarks(ctx context.Context, userID string, page, limit int) ([]*entities.Recipe, int64, error) {
	var recipes []*entities.Recipe
	var count int64
	offset := (page - 1) * limit

	if err := r.db.WithContext(ctx).
		Model(&entities.Recipe{}).
		Joins("JOIN recipe_bookmarks ON recipes.id = recipe_bookmarks.recipe_id").
		Where("recipe_bookmarks.user_id = ?", userID).
		Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.WithContext(ctx).
		Joins("JOIN recipe_bookmarks ON recipes.id = recipe_bookmarks.recipe_id").
		Where("recipe_bookmarks.user_id = ?", userID).
		Offset(offset).
		Limit(limit).
		Order("recipe_bookmarks.created_at desc").
		Find(&recipes).Error; err != nil {
		return nil, 0, err
	}

	return recipes, count, nil
}

func (r *recipeRepository) BookmarkRecipe(ctx context.Context, userID, recipeID string) error {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return err
	}

	recipeUUID, err := uuid.Parse(recipeID)
	if err != nil {
		return err
	}

	// Check if already bookmarked
	var existingBookmark entities.RecipeBookmark
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND recipe_id = ?", userUUID, recipeUUID).
		First(&existingBookmark).Error; err == nil {
		// Already bookmarked
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	bookmark := entities.RecipeBookmark{
		ID:        uuid.New(),
		UserID:    userUUID,
		RecipeID:  recipeUUID,
		CreatedAt: time.Now(),
	}

	return r.db.WithContext(ctx).Create(&bookmark).Error
}

func (r *recipeRepository) RemoveBookmark(ctx context.Context, userID, recipeID string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND recipe_id = ?", userID, recipeID).
		Delete(&entities.RecipeBookmark{}).Error
}

func (r *recipeRepository) IsRecipeBookmarked(ctx context.Context, userID, recipeID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&entities.RecipeBookmark{}).
		Where("user_id = ? AND recipe_id = ?", userID, recipeID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *recipeRepository) AddRecipeHistory(ctx context.Context, userID, recipeID string) error {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return err
	}

	recipeUUID, err := uuid.Parse(recipeID)
	if err != nil {
		return err
	}

	// Check if already in history
	var existingHistory entities.RecipeHistory
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND recipe_id = ?", userUUID, recipeUUID).
		First(&existingHistory).Error; err == nil {
		// Update cooked timestamp
		existingHistory.CookedAt = time.Now()
		return r.db.WithContext(ctx).Save(&existingHistory).Error
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	history := entities.RecipeHistory{
		ID:       uuid.New(),
		UserID:   userUUID,
		RecipeID: recipeUUID,
		CookedAt: time.Now(),
	}

	return r.db.WithContext(ctx).Create(&history).Error
}

func (r *recipeRepository) GetRecipeHistory(ctx context.Context, userID string, page, limit int) ([]*entities.Recipe, int64, error) {
	var recipes []*entities.Recipe
	var count int64
	offset := (page - 1) * limit

	if err := r.db.WithContext(ctx).
		Model(&entities.Recipe{}).
		Joins("JOIN recipe_histories ON recipes.id = recipe_histories.recipe_id").
		Where("recipe_histories.user_id = ?", userID).
		Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.WithContext(ctx).
		Joins("JOIN recipe_histories ON recipes.id = recipe_histories.recipe_id").
		Where("recipe_histories.user_id = ?", userID).
		Offset(offset).
		Limit(limit).
		Order("recipe_histories.cooked_at desc").
		Find(&recipes).Error; err != nil {
		return nil, 0, err
	}

	return recipes, count, nil
}

func (r *recipeRepository) IsRecipeInHistory(ctx context.Context, userID, recipeID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&entities.RecipeHistory{}).
		Where("user_id = ? AND recipe_id = ?", userID, recipeID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
