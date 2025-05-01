package handlers

import (
	"Go-Starter-Template/domain"
	"Go-Starter-Template/internal/api/presenters"
	"Go-Starter-Template/pkg/coin"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"strconv"
)

type (
	CoinHandler interface {
		GetCoinPackages(c *fiber.Ctx) error
		BuyCoins(c *fiber.Ctx) error
		UseCoins(c *fiber.Ctx) error
		GetUserCoins(c *fiber.Ctx) error
		GetCoinTransactionHistory(c *fiber.Ctx) error
	}

	coinHandler struct {
		coinService coin.CoinService
		validator   *validator.Validate
	}
)

func NewCoinHandler(coinService coin.CoinService, validator *validator.Validate) CoinHandler {
	return &coinHandler{
		coinService: coinService,
		validator:   validator,
	}
}

func (h *coinHandler) GetCoinPackages(c *fiber.Ctx) error {
	packages, err := h.coinService.GetCoinPackages(c.Context())
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetCoinPackages, err)
	}

	return presenters.SuccessResponse(c, packages, fiber.StatusOK, domain.MessageSuccessGetCoinPackages)
}

func (h *coinHandler) BuyCoins(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	req := new(domain.BuyCoinRequest)
	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBuyCoins, err)
	}

	resp, err := h.coinService.BuyCoins(c.Context(), *req, userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBuyCoins, err)
	}

	return presenters.SuccessResponse(c, resp, fiber.StatusOK, domain.MessageSuccessBuyCoins)
}

func (h *coinHandler) UseCoins(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	req := new(domain.UseCoinRequest)
	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedUseCoins, err)
	}

	if err := h.coinService.UseCoins(c.Context(), *req, userID); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedUseCoins, err)
	}

	return presenters.SuccessResponse(c, nil, fiber.StatusOK, domain.MessageSuccessUseCoins)
}

func (h *coinHandler) GetUserCoins(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	coins, err := h.coinService.GetUserCoins(c.Context(), userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetUserCoins, err)
	}

	return presenters.SuccessResponse(c, coins, fiber.StatusOK, domain.MessageSuccessGetUserCoins)
}

func (h *coinHandler) GetCoinTransactionHistory(c *fiber.Ctx) error {
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

	transactions, count, err := h.coinService.GetCoinTransactionHistory(c.Context(), userID, page, limit)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetCoinHistory, err)
	}

	return presenters.SuccessResponse(c, fiber.Map{
		"transactions": transactions,
		"pagination": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       count,
			"total_pages": (count + int64(limit) - 1) / int64(limit),
		},
	}, fiber.StatusOK, domain.MessageSuccessGetCoinHistory)
}
