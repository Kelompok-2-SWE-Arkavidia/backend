package coin

import (
	"Go-Starter-Template/domain"
	"Go-Starter-Template/entities"
	"Go-Starter-Template/pkg/midtrans"
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type (
	CoinService interface {
		GetCoinPackages(ctx context.Context) ([]*domain.CoinPackage, error)
		BuyCoins(ctx context.Context, req domain.BuyCoinRequest, userID string) (*domain.BuyCoinResponse, error)
		UseCoins(ctx context.Context, req domain.UseCoinRequest, userID string) error
		GetUserCoins(ctx context.Context, userID string) (*domain.UserCoins, error)
		GetCoinTransactionHistory(ctx context.Context, userID string, page, limit int) ([]*domain.CoinTransaction, int64, error)
		RewardCoins(ctx context.Context, req domain.RewardCoinRequest) error
	}

	coinService struct {
		coinRepository  CoinRepository
		midtransService midtrans.MidtransService
	}
)

func NewCoinService(coinRepository CoinRepository, midtransService midtrans.MidtransService) CoinService {
	return &coinService{
		coinRepository:  coinRepository,
		midtransService: midtransService,
	}
}

func (s *coinService) GetCoinPackages(ctx context.Context) ([]*domain.CoinPackage, error) {
	packages, err := s.coinRepository.GetCoinPackages(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*domain.CoinPackage, 0, len(packages))
	for _, pkg := range packages {
		result = append(result, &domain.CoinPackage{
			ID:          pkg.ID.String(),
			Name:        pkg.Name,
			Amount:      pkg.Amount,
			Price:       pkg.Price,
			Currency:    pkg.Currency,
			Description: pkg.Description,
			ImageURL:    pkg.ImageURL,
			IsPopular:   pkg.IsPopular,
		})
	}

	return result, nil
}

func (s *coinService) BuyCoins(ctx context.Context, req domain.BuyCoinRequest, userID string) (*domain.BuyCoinResponse, error) {
	// Get coin package
	pkg, err := s.coinRepository.GetCoinPackageByID(ctx, req.PackageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrInvalidCoinPackage
		}
		return nil, err
	}

	// Process payment with Midtrans
	midtransReq := domain.MidtransPaymentRequest{
		Amount: int64(pkg.Price),
		Email:  req.Email,
	}

	midtransResp, err := s.midtransService.CreateTransaction(ctx, midtransReq, userID)
	if err != nil {
		return nil, domain.ErrPaymentFailed
	}

	// Return invoice URL
	return &domain.BuyCoinResponse{
		TransactionID: "",
		InvoiceURL:    midtransResp.Invoice,
	}, nil
}

func (s *coinService) UseCoins(ctx context.Context, req domain.UseCoinRequest, userID string) error {
	// Determine coin cost based on feature
	coinCost := 0
	feature := ""

	switch req.Feature {
	case "Recipe":
		coinCost = domain.COST_RECIPE_RECOMMENDATION
		feature = "Recipe recommendation"
	case "ExpiryManagement":
		coinCost = domain.COST_EXPIRY_MANAGEMENT
		feature = "Expiry management"
	case "Barter":
		coinCost = domain.COST_BARTER_TRANSACTION
		feature = "Barter transaction"
	default:
		return domain.ErrInvalidFeature
	}

	// Override amount if provided
	if req.Amount > 0 {
		coinCost = req.Amount
	}

	// Check if user has enough coins
	currentBalance, err := s.coinRepository.GetUserBalance(ctx, userID)
	if err != nil {
		return err
	}

	if currentBalance < coinCost {
		return domain.ErrInsufficientCoins
	}

	// Create transaction
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return err
	}

	newBalance := currentBalance - coinCost

	description := fmt.Sprintf("Used %d coins for %s", coinCost, feature)
	if req.Metadata != "" {
		description += ": " + req.Metadata
	}

	transaction := &entities.CoinTransaction{
		ID:          uuid.New(),
		UserID:      userUUID,
		Amount:      -coinCost, // Negative for spending
		Type:        "Use",
		Feature:     req.Feature,
		Description: description,
		Balance:     newBalance,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return s.coinRepository.CreateCoinTransaction(ctx, transaction)
}

func (s *coinService) GetUserCoins(ctx context.Context, userID string) (*domain.UserCoins, error) {
	stats, err := s.coinRepository.GetUserCoinStats(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &domain.UserCoins{
		Balance:        stats["balance"],
		TotalPurchased: stats["total_purchased"],
		TotalUsed:      stats["total_used"],
		TotalRewarded:  stats["total_rewarded"],
	}, nil
}

func (s *coinService) GetCoinTransactionHistory(ctx context.Context, userID string, page, limit int) ([]*domain.CoinTransaction, int64, error) {
	transactions, count, err := s.coinRepository.GetUserCoinTransactions(ctx, userID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*domain.CoinTransaction, 0, len(transactions))
	for _, tx := range transactions {
		result = append(result, &domain.CoinTransaction{
			ID:          tx.ID.String(),
			UserID:      tx.UserID.String(),
			Amount:      tx.Amount,
			Type:        tx.Type,
			Feature:     tx.Feature,
			Description: tx.Description,
			Balance:     tx.Balance,
			CreatedAt:   tx.CreatedAt,
		})
	}

	return result, count, nil
}

func (s *coinService) RewardCoins(ctx context.Context, req domain.RewardCoinRequest) error {
	// Parse user ID
	userUUID, err := uuid.Parse(req.UserID)
	if err != nil {
		return err
	}

	// Get current balance
	currentBalance, err := s.coinRepository.GetUserBalance(ctx, req.UserID)
	if err != nil {
		return err
	}

	newBalance := currentBalance + req.Amount

	description := fmt.Sprintf("Rewarded %d coins for %s", req.Amount, req.Reason)
	if req.Description != "" {
		description = req.Description
	}

	// Create transaction
	transaction := &entities.CoinTransaction{
		ID:          uuid.New(),
		UserID:      userUUID,
		Amount:      req.Amount,
		Type:        "Reward",
		Feature:     "Reward",
		Description: description,
		Balance:     newBalance,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return s.coinRepository.CreateCoinTransaction(ctx, transaction)
}
