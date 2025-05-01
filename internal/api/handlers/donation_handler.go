// File: internal/api/handlers/donation_handler.go
package handlers

import (
	"Go-Starter-Template/domain"
	"Go-Starter-Template/internal/api/presenters"
	"Go-Starter-Template/pkg/donation"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"strconv"
)

type (
	DonationHandler interface {
		GetDonationLocations(c *fiber.Ctx) error
		CreateDonation(c *fiber.Ctx) error
		GetUserDonations(c *fiber.Ctx) error
		GetDonationByID(c *fiber.Ctx) error
		UpdateDonationStatus(c *fiber.Ctx) error
		GetDonationStatistics(c *fiber.Ctx) error
		GetExpiringFoodSuggestions(c *fiber.Ctx) error
	}

	donationHandler struct {
		donationService donation.DonationService
		validator       *validator.Validate
	}
)

func NewDonationHandler(donationService donation.DonationService, validator *validator.Validate) DonationHandler {
	return &donationHandler{
		donationService: donationService,
		validator:       validator,
	}
}

func (h *donationHandler) GetDonationLocations(c *fiber.Ctx) error {
	// Parse request params
	lat, err := strconv.ParseFloat(c.Query("latitude"), 64)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, domain.ErrInvalidCoordinates)
	}

	lng, err := strconv.ParseFloat(c.Query("longitude"), 64)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, domain.ErrInvalidCoordinates)
	}

	radius, err := strconv.ParseFloat(c.Query("radius", "5"), 64)
	if err != nil || radius <= 0 || radius > 10 {
		radius = 5 // Default radius
	}

	req := domain.GetDonationLocationsRequest{
		Latitude:  lat,
		Longitude: lng,
		Radius:    radius,
	}

	if err := h.validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetDonationLocations, err)
	}

	locations, err := h.donationService.GetDonationLocations(c.Context(), req)
	if err != nil {
		if err == domain.ErrDonationLocationUnavailable {
			return presenters.SuccessResponse(c, fiber.Map{
				"locations": []domain.DonationLocation{},
				"message":   "No donation locations found nearby. Try increasing the search radius.",
			}, fiber.StatusOK, "No donation locations found")
		}
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetDonationLocations, err)
	}

	return presenters.SuccessResponse(c, fiber.Map{
		"locations": locations,
	}, fiber.StatusOK, domain.MessageSuccessGetDonationLocations)
}

func (h *donationHandler) CreateDonation(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	// Parse request
	req := new(domain.DonationRequest)
	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	// Get food image if provided
	req.FoodImage, _ = c.FormFile("food_image")

	if err := h.validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedCreateDonation, err)
	}

	donation, err := h.donationService.CreateDonation(c.Context(), *req, userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedCreateDonation, err)
	}

	return presenters.SuccessResponse(c, donation, fiber.StatusCreated, domain.MessageSuccessCreateDonation)
}

func (h *donationHandler) GetUserDonations(c *fiber.Ctx) error {
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

	donations, count, err := h.donationService.GetUserDonations(c.Context(), userID, page, limit)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetDonations, err)
	}

	return presenters.SuccessResponse(c, fiber.Map{
		"donations": donations,
		"pagination": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       count,
			"total_pages": (count + int64(limit) - 1) / int64(limit),
		},
	}, fiber.StatusOK, domain.MessageSuccessGetDonations)
}

func (h *donationHandler) GetDonationByID(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	donationID := c.Params("id")

	if donationID == "" {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetDonations, domain.ErrDonationNotFound)
	}

	donation, err := h.donationService.GetDonationByID(c.Context(), donationID, userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetDonations, err)
	}

	return presenters.SuccessResponse(c, donation, fiber.StatusOK, domain.MessageSuccessGetDonations)
}

func (h *donationHandler) UpdateDonationStatus(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	req := new(domain.UpdateDonationStatusRequest)
	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedUpdateDonation, err)
	}

	if err := h.donationService.UpdateDonationStatus(c.Context(), *req, userID); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedUpdateDonation, err)
	}

	return presenters.SuccessResponse(c, nil, fiber.StatusOK, domain.MessageSuccessUpdateDonation)
}

func (h *donationHandler) GetDonationStatistics(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	stats, err := h.donationService.GetDonationStatistics(c.Context(), userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetDonations, err)
	}

	return presenters.SuccessResponse(c, stats, fiber.StatusOK, domain.MessageSuccessGetDonations)
}

func (h *donationHandler) GetExpiringFoodSuggestions(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	// Parse request params
	lat, err := strconv.ParseFloat(c.Query("latitude"), 64)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, domain.ErrInvalidCoordinates)
	}

	lng, err := strconv.ParseFloat(c.Query("longitude"), 64)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, domain.ErrInvalidCoordinates)
	}

	req := domain.ExpiringFoodSuggestionRequest{
		Latitude:  lat,
		Longitude: lng,
	}

	suggestions, err := h.donationService.GetExpiringFoodSuggestions(c.Context(), req, userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetDonations, err)
	}

	return presenters.SuccessResponse(c, suggestions, fiber.StatusOK, domain.MessageSuccessGetDonations)
}
