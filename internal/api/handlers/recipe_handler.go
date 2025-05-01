package handlers

import (
	"Go-Starter-Template/domain"
	"Go-Starter-Template/internal/api/presenters"
	"Go-Starter-Template/pkg/recipe"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"strconv"
)

type (
	RecipeHandler interface {
		GetRecipeRecommendations(c *fiber.Ctx) error
		GetRecipeDetail(c *fiber.Ctx) error
		BookmarkRecipe(c *fiber.Ctx) error
		RemoveBookmark(c *fiber.Ctx) error
		GetBookmarkedRecipes(c *fiber.Ctx) error
		MarkAsCooked(c *fiber.Ctx) error
		GetRecipeHistory(c *fiber.Ctx) error
	}

	recipeHandler struct {
		recipeService recipe.RecipeService
		validator     *validator.Validate
	}
)

func NewRecipeHandler(recipeService recipe.RecipeService, validator *validator.Validate) RecipeHandler {
	return &recipeHandler{
		recipeService: recipeService,
		validator:     validator,
	}
}

func (h *recipeHandler) GetRecipeRecommendations(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	// Parse request params
	req := domain.RecipeRecommendationRequest{
		IncludeExpiringOnly: c.QueryBool("expiring_only", false),
		CuisineType:         c.Query("cuisine_type", ""),
		DifficultyLevel:     c.Query("difficulty_level", ""),
	}

	prepTime, err := strconv.Atoi(c.Query("prep_time", "0"))
	if err == nil && prepTime > 0 {
		req.PreparationTime = prepTime
	}

	res, err := h.recipeService.GetRecipeRecommendations(c.Context(), req, userID)
	if err != nil {
		if err == domain.ErrNoIngredients {
			return presenters.SuccessResponse(c, fiber.Map{
				"recipes":        []domain.Recipe{},
				"total_recipes":  0,
				"expiring_items": 0,
				"message":        "No ingredients available to recommend recipes. Please add food items to your inventory first.",
			}, fiber.StatusOK, "No ingredients available")
		}
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetRecipes, err)
	}

	return presenters.SuccessResponse(c, res, fiber.StatusOK, domain.MessageSuccessGetRecipes)
}

func (h *recipeHandler) GetRecipeDetail(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	recipeID := c.Params("id")

	if recipeID == "" {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetRecipeDetail, domain.ErrRecipeNotFound)
	}

	res, err := h.recipeService.GetRecipeDetail(c.Context(), recipeID, userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetRecipeDetail, err)
	}

	return presenters.SuccessResponse(c, res, fiber.StatusOK, domain.MessageSuccessGetRecipeDetail)
}

func (h *recipeHandler) BookmarkRecipe(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	req := new(domain.BookmarkRecipeRequest)

	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedSaveRecipe, err)
	}

	if err := h.recipeService.BookmarkRecipe(c.Context(), *req, userID); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedSaveRecipe, err)
	}

	return presenters.SuccessResponse(c, nil, fiber.StatusOK, domain.MessageSuccessSaveRecipe)
}

func (h *recipeHandler) RemoveBookmark(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	req := new(domain.BookmarkRecipeRequest)

	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedDeleteRecipe, err)
	}

	if err := h.recipeService.RemoveBookmark(c.Context(), *req, userID); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedDeleteRecipe, err)
	}

	return presenters.SuccessResponse(c, nil, fiber.StatusOK, domain.MessageSuccessDeleteRecipe)
}

func (h *recipeHandler) GetBookmarkedRecipes(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	// Parse pagination parameters
	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.Query("limit", "20"))
	if err != nil || limit < 1 {
		limit = 20
	}

	recipes, count, err := h.recipeService.GetBookmarkedRecipes(c.Context(), page, limit, userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetRecipes, err)
	}

	return presenters.SuccessResponse(c, fiber.Map{
		"recipes": recipes,
		"pagination": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       count,
			"total_pages": (count + int64(limit) - 1) / int64(limit),
		},
	}, fiber.StatusOK, domain.MessageSuccessGetRecipes)
}

func (h *recipeHandler) MarkAsCooked(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	req := new(domain.MarkAsCookedRequest)

	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedMarkAsCooked, err)
	}

	if err := h.recipeService.MarkAsCooked(c.Context(), *req, userID); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedMarkAsCooked, err)
	}

	return presenters.SuccessResponse(c, nil, fiber.StatusOK, domain.MessageSuccessMarkAsCooked)
}

func (h *recipeHandler) GetRecipeHistory(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	// Parse pagination parameters
	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.Query("limit", "20"))
	if err != nil || limit < 1 {
		limit = 20
	}

	res, err := h.recipeService.GetRecipeHistory(c.Context(), page, limit, userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetHistory, err)
	}

	return presenters.SuccessResponse(c, res, fiber.StatusOK, domain.MessageSuccessGetHistory)
}

// Now let's migrate the database to include the recipe tables by updating the migrate.go file

// In cmd/database/migrate/migrate.go, update the Migrate function to include:
