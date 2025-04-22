package handlers

import (
	"Go-Starter-Template/domain"
	"Go-Starter-Template/internal/api/presenters"
	"Go-Starter-Template/pkg/food"
	"errors"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"strconv"
)

type (
	FoodHandler interface {
		AddFoodItem(c *fiber.Ctx) error
		UpdateFoodItem(c *fiber.Ctx) error
		DeleteFoodItem(c *fiber.Ctx) error
		GetFoodItems(c *fiber.Ctx) error
		GetFoodItemDetails(c *fiber.Ctx) error
		UploadFoodImage(c *fiber.Ctx) error
		UploadReceipt(c *fiber.Ctx) error
		SaveScannedItems(c *fiber.Ctx) error
		MarkAsDamaged(c *fiber.Ctx) error
		GetDashboardStats(c *fiber.Ctx) error
	}

	foodHandler struct {
		foodService food.FoodService
		validator   *validator.Validate
	}
)

func NewFoodHandler(foodService food.FoodService, validator *validator.Validate) FoodHandler {
	return &foodHandler{
		foodService: foodService,
		validator:   validator,
	}
}

func (h *foodHandler) AddFoodItem(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	req := new(domain.AddFoodItemRequest)

	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedAddFoodItem, err)
	}

	res, err := h.foodService.AddFoodItem(c.Context(), *req, userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedAddFoodItem, err)
	}

	return presenters.SuccessResponse(c, res, fiber.StatusCreated, domain.MessageSuccessAddFoodItem)
}

func (h *foodHandler) UpdateFoodItem(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	itemID := c.Params("id")
	req := new(domain.UpdateFoodItemRequest)

	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedUpdateFoodItem, err)
	}

	if err := h.foodService.UpdateFoodItem(c.Context(), itemID, *req, userID); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedUpdateFoodItem, err)
	}

	return presenters.SuccessResponse(c, nil, fiber.StatusOK, domain.MessageSuccessUpdateFoodItem)
}

func (h *foodHandler) DeleteFoodItem(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	itemID := c.Params("id")

	if err := h.foodService.DeleteFoodItem(c.Context(), itemID, userID); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedDeleteFoodItem, err)
	}

	return presenters.SuccessResponse(c, nil, fiber.StatusOK, domain.MessageSuccessDeleteFoodItem)
}

func (h *foodHandler) GetFoodItems(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	status := c.Query("status", "all")

	// Parse pagination parameters
	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.Query("limit", "20"))
	if err != nil || limit < 1 {
		limit = 20
	}

	items, count, err := h.foodService.GetFoodItems(c.Context(), userID, status, page, limit)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetFoodItems, err)
	}

	return presenters.SuccessResponse(c, fiber.Map{
		"items": items,
		"pagination": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       count,
			"total_pages": (count + int64(limit) - 1) / int64(limit),
		},
	}, fiber.StatusOK, domain.MessageSuccessGetFoodItems)
}

func (h *foodHandler) GetFoodItemDetails(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	itemID := c.Params("id")

	item, err := h.foodService.GetFoodItemByID(c.Context(), itemID, userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetFoodItems, err)
	}

	return presenters.SuccessResponse(c, item, fiber.StatusOK, domain.MessageSuccessGetFoodItems)
}

func (h *foodHandler) UploadFoodImage(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	req := new(domain.UploadFoodImageRequest)

	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	file, err := c.FormFile("image")
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	req.Image = file

	if err := h.validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.foodService.UploadFoodImage(c.Context(), *req, userID); err != nil {
		if errors.Is(err, domain.ErrGeminiProcessingFailed) {
			return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedDetectFoodAge, err)
		}
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	return presenters.SuccessResponse(c, nil, fiber.StatusOK, "Image uploaded successfully and food analyzed")
}

func (h *foodHandler) UploadReceipt(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	req := new(domain.UploadReceiptRequest)

	// Get file
	file, err := c.FormFile("receipt_image")
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	req.ReceiptImage = file

	if err := h.validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedUploadReceipt, err)
	}

	res, err := h.foodService.UploadReceipt(c.Context(), *req, userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedUploadReceipt, err)
	}

	return presenters.SuccessResponse(c, res, fiber.StatusOK, domain.MessageSuccessUploadReceipt)
}

func (h *foodHandler) SaveScannedItems(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	req := new(domain.SaveScannedItemsRequest)

	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedSaveScannedItems, err)
	}

	if err := h.foodService.SaveScannedItems(c.Context(), *req, userID); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedSaveScannedItems, err)
	}

	return presenters.SuccessResponse(c, nil, fiber.StatusOK, domain.MessageSuccessSaveScannedItems)
}

func (h *foodHandler) MarkAsDamaged(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	req := new(domain.MarkAsDamagedRequest)

	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedMarkAsDamaged, err)
	}

	if err := h.foodService.MarkAsDamaged(c.Context(), *req, userID); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedMarkAsDamaged, err)
	}

	return presenters.SuccessResponse(c, nil, fiber.StatusOK, domain.MessageSuccessMarkAsDamaged)
}

func (h *foodHandler) GetDashboardStats(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	stats, err := h.foodService.GetDashboardStats(c.Context(), userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetDashboardStats, err)
	}

	return presenters.SuccessResponse(c, stats, fiber.StatusOK, domain.MessageSuccessGetDashboardStats)
}
