package domain

import (
	"errors"
)

var (
	MessageSuccessGetUserCoins    = "user coins retrieved successfully"
	MessageSuccessBuyCoins        = "coins purchased successfully"
	MessageSuccessUseCoins        = "coins used successfully"
	MessageSuccessGetCoinPackages = "coin packages retrieved successfully"
	MessageSuccessGetCoinHistory  = "coin transaction history retrieved successfully"
	MessageSuccessRewardCoins     = "coins rewarded successfully"

	MessageFailedGetUserCoins    = "failed to retrieve user coins"
	MessageFailedBuyCoins        = "failed to purchase coins"
	MessageFailedUseCoins        = "failed to use coins"
	MessageFailedGetCoinPackages = "failed to retrieve coin packages"
	MessageFailedGetCoinHistory  = "failed to retrieve coin transaction history"
	MessageFailedRewardCoins     = "failed to reward coins"

	ErrInsufficientCoins  = errors.New("insufficient coins")
	ErrInvalidFeature     = errors.New("invalid premium feature")
	ErrInvalidCoinPackage = errors.New("invalid coin package")
	ErrPaymentFailed      = errors.New("payment processing failed")
)

const (
	// Feature costs
	COST_RECIPE_RECOMMENDATION = 15
	COST_EXPIRY_MANAGEMENT     = 10
	COST_BARTER_TRANSACTION    = 5

	// Reward values
	REWARD_DONATION_PER_ITEM = 5
)

type (
	CoinPackage struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Amount      int     `json:"amount"`
		Price       float64 `json:"price"`
		Currency    string  `json:"currency"`
		Description string  `json:"description,omitempty"`
		ImageURL    string  `json:"image_url,omitempty"`
		IsPopular   bool    `json:"is_popular"`
	}

	BuyCoinRequest struct {
		PackageID string `json:"package_id" validate:"required"`
		Email     string `json:"email" validate:"required,email"`
	}

	BuyCoinResponse struct {
		TransactionID string `json:"transaction_id"`
		InvoiceURL    string `json:"invoice_url"`
	}

	UseCoinRequest struct {
		Feature  string `json:"feature" validate:"required,oneof=Recipe ExpiryManagement Barter"`
		Quantity int    `json:"quantity" validate:"required,min=1"`
	}
)
