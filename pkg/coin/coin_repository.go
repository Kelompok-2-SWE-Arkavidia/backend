package coin

import (
	"Go-Starter-Template/entities"
	"context"
	"errors"
	"gorm.io/gorm"
)

type (
	CoinRepository interface {
		// Coin packages
		CreateCoinPackage(ctx context.Context, pkg *entities.CoinPackage) error
		GetCoinPackages(ctx context.Context) ([]*entities.CoinPackage, error)
		GetCoinPackageByID(ctx context.Context, id string) (*entities.CoinPackage, error)

		// User coins
		GetUserBalance(ctx context.Context, userID string) (int, error)
		GetUserCoinStats(ctx context.Context, userID string) (map[string]int, error)

		// Transactions
		CreateCoinTransaction(ctx context.Context, tx *entities.CoinTransaction) error
		GetUserCoinTransactions(ctx context.Context, userID string, page, limit int) ([]*entities.CoinTransaction, int64, error)
	}

	coinRepository struct {
		db *gorm.DB
	}
)

func NewCoinRepository(db *gorm.DB) CoinRepository {
	return &coinRepository{
		db: db,
	}
}

func (r *coinRepository) CreateCoinPackage(ctx context.Context, pkg *entities.CoinPackage) error {
	return r.db.WithContext(ctx).Create(pkg).Error
}

func (r *coinRepository) GetCoinPackages(ctx context.Context) ([]*entities.CoinPackage, error) {
	var packages []*entities.CoinPackage
	if err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("amount ASC").
		Find(&packages).Error; err != nil {
		return nil, err
	}
	return packages, nil
}

func (r *coinRepository) GetCoinPackageByID(ctx context.Context, id string) (*entities.CoinPackage, error) {
	var pkg entities.CoinPackage
	if err := r.db.WithContext(ctx).
		Where("id = ? AND is_active = ?", id, true).
		First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &pkg, nil
}

func (r *coinRepository) GetUserBalance(ctx context.Context, userID string) (int, error) {
	// Query the latest transaction to get the most recent balance
	var latestTx entities.CoinTransaction
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		First(&latestTx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil // No transactions yet, balance is 0
		}
		return 0, err
	}

	return latestTx.Balance, nil
}

func (r *coinRepository) GetUserCoinStats(ctx context.Context, userID string) (map[string]int, error) {
	// Get total purchased
	var totalPurchased int
	purchaseQuery := r.db.WithContext(ctx).
		Model(&entities.CoinTransaction{}).
		Where("user_id = ? AND type = ?", userID, "Purchase").
		Select("COALESCE(SUM(amount), 0) as total")
	if err := purchaseQuery.Row().Scan(&totalPurchased); err != nil {
		return nil, err
	}

	// Get total used
	var totalUsed int
	useQuery := r.db.WithContext(ctx).
		Model(&entities.CoinTransaction{}).
		Where("user_id = ? AND type = ? AND amount < 0", userID, "Use").
		Select("COALESCE(SUM(amount), 0) as total")
	if err := useQuery.Row().Scan(&totalUsed); err != nil {
		return nil, err
	}
	totalUsed = -totalUsed // Convert to positive value

	// Get total rewarded
	var totalRewarded int
	rewardQuery := r.db.WithContext(ctx).
		Model(&entities.CoinTransaction{}).
		Where("user_id = ? AND type = ?", userID, "Reward").
		Select("COALESCE(SUM(amount), 0) as total")
	if err := rewardQuery.Row().Scan(&totalRewarded); err != nil {
		return nil, err
	}

	// Get current balance
	balance, err := r.GetUserBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	return map[string]int{
		"balance":         balance,
		"total_purchased": totalPurchased,
		"total_used":      totalUsed,
		"total_rewarded":  totalRewarded,
	}, nil
}

func (r *coinRepository) CreateCoinTransaction(ctx context.Context, tx *entities.CoinTransaction) error {
	return r.db.WithContext(ctx).Create(tx).Error
}

func (r *coinRepository) GetUserCoinTransactions(ctx context.Context, userID string, page, limit int) ([]*entities.CoinTransaction, int64, error) {
	var transactions []*entities.CoinTransaction
	var count int64
	offset := (page - 1) * limit

	if err := r.db.WithContext(ctx).
		Model(&entities.CoinTransaction{}).
		Where("user_id = ?", userID).
		Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&transactions).Error; err != nil {
		return nil, 0, err
	}

	return transactions, count, nil
}
