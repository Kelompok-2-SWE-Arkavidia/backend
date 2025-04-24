package food

import (
	"Go-Starter-Template/entities"
	"context"
	"errors"
	"gorm.io/gorm"
	"time"
)

type (
	FoodRepository interface {
		AddFoodItem(ctx context.Context, foodItem *entities.FoodItem) error
		GetFoodItemByID(ctx context.Context, id string) (*entities.FoodItem, error)
		UpdateFoodItem(ctx context.Context, foodItem *entities.FoodItem) error
		DeleteFoodItem(ctx context.Context, id string) error
		GetFoodItems(ctx context.Context, userID string, status string, page, limit int) ([]*entities.FoodItem, int64, error)
		GetFoodItemsByExpiryRange(ctx context.Context, userID string, startDate, endDate time.Time) ([]*entities.FoodItem, error)
		MarkFoodItemAsDamaged(ctx context.Context, id string) error
		GetDashboardStats(ctx context.Context, userID string) (map[string]interface{}, error)

		// Receipt scanning related
		CreateReceiptScan(ctx context.Context, receiptScan *entities.ReceiptScan) error
		GetReceiptScanByID(ctx context.Context, id string) (*entities.ReceiptScan, error)
		UpdateReceiptScan(ctx context.Context, receiptScan *entities.ReceiptScan) error
	}

	foodRepository struct {
		db *gorm.DB
	}
)

func NewFoodRepository(db *gorm.DB) FoodRepository {
	return &foodRepository{db: db}
}

func (r *foodRepository) AddFoodItem(ctx context.Context, foodItem *entities.FoodItem) error {
	return r.db.WithContext(ctx).Create(foodItem).Error
}

func (r *foodRepository) GetFoodItemByID(ctx context.Context, id string) (*entities.FoodItem, error) {
	var foodItem entities.FoodItem
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&foodItem).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &foodItem, nil
}

func (r *foodRepository) UpdateFoodItem(ctx context.Context, foodItem *entities.FoodItem) error {
	return r.db.WithContext(ctx).Save(foodItem).Error
}

func (r *foodRepository) DeleteFoodItem(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&entities.FoodItem{}).Error
}

func (r *foodRepository) GetFoodItems(ctx context.Context, userID string, status string, page, limit int) ([]*entities.FoodItem, int64, error) {
	var foodItems []*entities.FoodItem
	var count int64

	offset := (page - 1) * limit

	query := r.db.WithContext(ctx).Where("user_id = ?", userID)

	if status != "all" && status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Model(&entities.FoodItem{}).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Offset(offset).Limit(limit).Order("expiry_date asc").Find(&foodItems).Error; err != nil {
		return nil, 0, err
	}

	return foodItems, count, nil
}

func (r *foodRepository) GetFoodItemsByExpiryRange(ctx context.Context, userID string, startDate, endDate time.Time) ([]*entities.FoodItem, error) {
	var foodItems []*entities.FoodItem

	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND expiry_date BETWEEN ? AND ? AND status = ?",
			userID, startDate, endDate, "Safe").
		Order("expiry_date asc").
		Find(&foodItems).Error; err != nil {
		return nil, err
	}

	return foodItems, nil
}

func (r *foodRepository) MarkFoodItemAsDamaged(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&entities.FoodItem{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"status": "Damaged"}).Error
}

func (r *foodRepository) GetDashboardStats(ctx context.Context, userID string) (map[string]interface{}, error) {
	var totalItems, safeItems, warningItems, expiredItems, damagedItems int64

	// Count total items
	if err := r.db.WithContext(ctx).Model(&entities.FoodItem{}).
		Where("user_id = ?", userID).
		Count(&totalItems).Error; err != nil {
		return nil, err
	}

	// Count by status
	if err := r.db.WithContext(ctx).Model(&entities.FoodItem{}).
		Where("user_id = ? AND status = ?", userID, "Safe").
		Count(&safeItems).Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Model(&entities.FoodItem{}).
		Where("user_id = ? AND status = ?", userID, "Warning").
		Count(&warningItems).Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Model(&entities.FoodItem{}).
		Where("user_id = ? AND status = ?", userID, "Expired").
		Count(&expiredItems).Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Model(&entities.FoodItem{}).
		Where("user_id = ? AND status = ?", userID, "Damaged").
		Count(&damagedItems).Error; err != nil {
		return nil, err
	}

	// Additional metrics can be calculated here
	savedItems := safeItems + warningItems
	wastedItems := expiredItems + damagedItems

	// This is just a placeholder for demonstration
	estimatedSavings := float64(savedItems) * 10000 // Assume Rp 10,000 per saved item

	stats := map[string]interface{}{
		"total_items":       totalItems,
		"safe_items":        safeItems,
		"warning_items":     warningItems,
		"expired_items":     expiredItems,
		"damaged_items":     damagedItems,
		"saved_items":       savedItems,
		"wasted_items":      wastedItems,
		"estimated_savings": estimatedSavings,
	}

	return stats, nil
}

func (r *foodRepository) CreateReceiptScan(ctx context.Context, receiptScan *entities.ReceiptScan) error {
	return r.db.WithContext(ctx).Create(receiptScan).Error
}

func (r *foodRepository) GetReceiptScanByID(ctx context.Context, id string) (*entities.ReceiptScan, error) {
	var receiptScan entities.ReceiptScan
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&receiptScan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &receiptScan, nil
}

func (r *foodRepository) UpdateReceiptScan(ctx context.Context, receiptScan *entities.ReceiptScan) error {
	return r.db.WithContext(ctx).Save(receiptScan).Error
}
