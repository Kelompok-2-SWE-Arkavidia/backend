package barter

import (
	"Go-Starter-Template/entities"
	"context"
	"errors"
	"gorm.io/gorm"
	"strconv"
	"time"
)

const (
	BARTER_TRANSACTION_COIN_COST = 5
)

type (
	BarterRepository interface {
		// Barter Items
		CreateBarterItem(ctx context.Context, item *entities.BarterItem) error
		GetBarterItemByID(ctx context.Context, id string) (*entities.BarterItem, error)
		UpdateBarterItem(ctx context.Context, item *entities.BarterItem) error
		DeleteBarterItem(ctx context.Context, id string) error
		GetUserBarterItems(ctx context.Context, userID string, status string, page, limit int) ([]*entities.BarterItem, int64, error)
		GetNearbyBarterItems(ctx context.Context, lat, lng float64, radius float64, userID string, includeOwn bool, status string, expiryMaxDays int) ([]*entities.BarterItem, error)
		UpdateBarterItemStatus(ctx context.Context, id string, status string) error

		// Barter Chats
		CreateBarterChat(ctx context.Context, chat *entities.BarterChat) error
		GetBarterChatByID(ctx context.Context, id string) (*entities.BarterChat, error)
		GetUserBarterChats(ctx context.Context, userID string, status string, page, limit int) ([]*entities.BarterChat, int64, error)
		GetBarterChatByItemAndUsers(ctx context.Context, itemID, offererID, ownerID string) (*entities.BarterChat, error)
		UpdateBarterChatStatus(ctx context.Context, id string, status string) error

		// Barter Messages
		AddBarterMessage(ctx context.Context, message *entities.BarterMessage) error
		GetBarterMessages(ctx context.Context, chatID string, page, limit int) ([]*entities.BarterMessage, int64, error)
		MarkMessagesAsRead(ctx context.Context, chatID, userID string) error
		GetUnreadMessageCount(ctx context.Context, chatID, userID string) (int, error)
		UpdateLastMessageTime(ctx context.Context, chatID string, time time.Time) error

		// Barter Transactions
		CreateBarterTransaction(ctx context.Context, transaction *entities.BarterTransaction) error
		AddBarterTransactionItem(ctx context.Context, item *entities.BarterTransactionItem) error
		GetBarterTransactionByChat(ctx context.Context, chatID string) (*entities.BarterTransaction, error)
		GetBarterTransactionItems(ctx context.Context, transactionID string) ([]*entities.BarterTransactionItem, error)
		ConfirmBarterTransaction(ctx context.Context, transactionID string, userID string, isOwner bool) error
		GetBarterStatistics(ctx context.Context, userID string) (map[string]interface{}, error)
	}

	barterRepository struct {
		db *gorm.DB
	}
)

func NewBarterRepository(db *gorm.DB) BarterRepository {
	return &barterRepository{
		db: db,
	}
}

func (r *barterRepository) CreateBarterItem(ctx context.Context, item *entities.BarterItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *barterRepository) GetBarterItemByID(ctx context.Context, id string) (*entities.BarterItem, error) {
	var item entities.BarterItem
	if err := r.db.WithContext(ctx).
		Preload("User").
		Where("id = ?", id).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &item, nil
}

func (r *barterRepository) UpdateBarterItem(ctx context.Context, item *entities.BarterItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *barterRepository) DeleteBarterItem(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&entities.BarterItem{}, "id = ?", id).Error
}

func (r *barterRepository) GetUserBarterItems(ctx context.Context, userID string, status string, page, limit int) ([]*entities.BarterItem, int64, error) {
	var items []*entities.BarterItem
	var count int64
	offset := (page - 1) * limit

	query := r.db.WithContext(ctx).Where("user_id = ?", userID)

	if status != "All" && status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Model(&entities.BarterItem{}).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if err := query.
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, count, nil
}

func (r *barterRepository) GetNearbyBarterItems(ctx context.Context, lat, lng float64, radius float64, userID string, includeOwn bool, status string, expiryMaxDays int) ([]*entities.BarterItem, error) {
	var items []*entities.BarterItem

	// Using PostgreSQL's earthdistance extension for location-based queries
	// Make sure you've installed the extension with:
	// CREATE EXTENSION IF NOT EXISTS "earthdistance" CASCADE;
	// CREATE EXTENSION IF NOT EXISTS "cube";
	query := `
		SELECT *, 
		       earth_distance(ll_to_earth(?, ?), ll_to_earth(latitude, longitude)) as distance 
		FROM barter_items 
		WHERE earth_box(ll_to_earth(?, ?), ?) @> ll_to_earth(latitude, longitude) 
	`

	// Add status filter if provided
	if status != "All" && status != "" {
		query += " AND status = '" + status + "'"
	} else {
		// Default to Available items
		query += " AND status = 'Available'"
	}

	// Exclude user's own items if requested
	if !includeOwn {
		query += " AND user_id != '" + userID + "'"
	}

	// Add expiry date filter if provided
	if expiryMaxDays > 0 {
		expiryDate := time.Now().AddDate(0, 0, expiryMaxDays)
		query += " AND expiry_date <= '" + expiryDate.Format("2006-01-02") + "'"
	}

	// Order by distance
	query += " ORDER BY distance ASC"

	// radius in km, convert to meters for the query
	radiusMeters := radius * 1000

	if err := r.db.WithContext(ctx).Raw(query, lat, lng, lat, lng, radiusMeters).Scan(&items).Error; err != nil {
		return nil, err
	}

	// Eager-load users for each item
	for i, item := range items {
		if err := r.db.WithContext(ctx).Model(&item).Association("User").Find(&item.User); err != nil {
			continue
		}
		items[i] = item
	}

	return items, nil
}

func (r *barterRepository) UpdateBarterItemStatus(ctx context.Context, id string, status string) error {
	return r.db.WithContext(ctx).
		Model(&entities.BarterItem{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *barterRepository) CreateBarterChat(ctx context.Context, chat *entities.BarterChat) error {
	return r.db.WithContext(ctx).Create(chat).Error
}

func (r *barterRepository) GetBarterChatByID(ctx context.Context, id string) (*entities.BarterChat, error) {
	var chat entities.BarterChat
	if err := r.db.WithContext(ctx).
		Preload("BarterItem").
		Preload("Offerer").
		Preload("Owner").
		Where("id = ?", id).
		First(&chat).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &chat, nil
}

func (r *barterRepository) GetUserBarterChats(ctx context.Context, userID string, status string, page, limit int) ([]*entities.BarterChat, int64, error) {
	var chats []*entities.BarterChat
	var count int64
	offset := (page - 1) * limit

	query := r.db.WithContext(ctx).
		Where("offerer_id = ? OR owner_id = ?", userID, userID)

	if status != "All" && status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Model(&entities.BarterChat{}).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if err := query.
		Preload("BarterItem").
		Preload("Offerer").
		Preload("Owner").
		Order("last_message_time DESC").
		Offset(offset).
		Limit(limit).
		Find(&chats).Error; err != nil {
		return nil, 0, err
	}

	return chats, count, nil
}

func (r *barterRepository) GetBarterChatByItemAndUsers(ctx context.Context, itemID, offererID, ownerID string) (*entities.BarterChat, error) {
	var chat entities.BarterChat
	if err := r.db.WithContext(ctx).
		Where("barter_item_id = ? AND offerer_id = ? AND owner_id = ?", itemID, offererID, ownerID).
		First(&chat).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Return nil, nil if not found
		}
		return nil, err
	}
	return &chat, nil
}

func (r *barterRepository) UpdateBarterChatStatus(ctx context.Context, id string, status string) error {
	return r.db.WithContext(ctx).
		Model(&entities.BarterChat{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *barterRepository) AddBarterMessage(ctx context.Context, message *entities.BarterMessage) error {
	return r.db.WithContext(ctx).Create(message).Error
}

func (r *barterRepository) GetBarterMessages(ctx context.Context, chatID string, page, limit int) ([]*entities.BarterMessage, int64, error) {
	var messages []*entities.BarterMessage
	var count int64
	offset := (page - 1) * limit

	if err := r.db.WithContext(ctx).
		Model(&entities.BarterMessage{}).
		Where("chat_id = ?", chatID).
		Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.WithContext(ctx).
		Preload("Sender").
		Where("chat_id = ?", chatID).
		Order("created_at ASC").
		Offset(offset).
		Limit(limit).
		Find(&messages).Error; err != nil {
		return nil, 0, err
	}

	return messages, count, nil
}

func (r *barterRepository) MarkMessagesAsRead(ctx context.Context, chatID, userID string) error {
	return r.db.WithContext(ctx).
		Model(&entities.BarterMessage{}).
		Where("chat_id = ? AND sender_id != ? AND is_read = ?", chatID, userID, false).
		Update("is_read", true).Error
}

func (r *barterRepository) GetUnreadMessageCount(ctx context.Context, chatID, userID string) (int, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&entities.BarterMessage{}).
		Where("chat_id = ? AND sender_id != ? AND is_read = ?", chatID, userID, false).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *barterRepository) UpdateLastMessageTime(ctx context.Context, chatID string, time time.Time) error {
	return r.db.WithContext(ctx).
		Model(&entities.BarterChat{}).
		Where("id = ?", chatID).
		Update("last_message_time", time).Error
}

func (r *barterRepository) CreateBarterTransaction(ctx context.Context, transaction *entities.BarterTransaction) error {
	return r.db.WithContext(ctx).Create(transaction).Error
}

func (r *barterRepository) AddBarterTransactionItem(ctx context.Context, item *entities.BarterTransactionItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *barterRepository) GetBarterTransactionByChat(ctx context.Context, chatID string) (*entities.BarterTransaction, error) {
	var transaction entities.BarterTransaction
	if err := r.db.WithContext(ctx).
		Where("chat_id = ?", chatID).
		First(&transaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Return nil, nil if not found
		}
		return nil, err
	}
	return &transaction, nil
}

func (r *barterRepository) GetBarterTransactionItems(ctx context.Context, transactionID string) ([]*entities.BarterTransactionItem, error) {
	var items []*entities.BarterTransactionItem
	if err := r.db.WithContext(ctx).
		Preload("BarterItem").
		Where("transaction_id = ?", transactionID).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *barterRepository) ConfirmBarterTransaction(ctx context.Context, transactionID string, userID string, isOwner bool) error {
	var fields map[string]interface{}

	if isOwner {
		fields = map[string]interface{}{
			"owner_confirmed": true,
		}
	} else {
		fields = map[string]interface{}{
			"offerer_confirmed": true,
		}
	}

	// Update the transaction
	if err := r.db.WithContext(ctx).
		Model(&entities.BarterTransaction{}).
		Where("id = ?", transactionID).
		Updates(fields).Error; err != nil {
		return err
	}

	// Check if both parties have confirmed
	var transaction entities.BarterTransaction
	if err := r.db.WithContext(ctx).
		Where("id = ?", transactionID).
		First(&transaction).Error; err != nil {
		return err
	}

	if transaction.OwnerConfirmed && transaction.OffererConfirmed {
		// Update status to Completed and set CompletedAt
		now := time.Now()
		if err := r.db.WithContext(ctx).
			Model(&entities.BarterTransaction{}).
			Where("id = ?", transactionID).
			Updates(map[string]interface{}{
				"status":       "Completed",
				"completed_at": now,
			}).Error; err != nil {
			return err
		}

		// Update the chat status to Completed
		if err := r.UpdateBarterChatStatus(ctx, transaction.ChatID.String(), "Completed"); err != nil {
			return err
		}

		// Get transaction items and update their status
		items, err := r.GetBarterTransactionItems(ctx, transactionID)
		if err != nil {
			return err
		}

		for _, item := range items {
			if err := r.UpdateBarterItemStatus(ctx, item.BarterItem.ID.String(), "Completed"); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *barterRepository) GetBarterStatistics(ctx context.Context, userID string) (map[string]interface{}, error) {
	// Count total barters initiated as offerer
	var bartersInitiated int64
	if err := r.db.WithContext(ctx).
		Model(&entities.BarterChat{}).
		Where("offerer_id = ?", userID).
		Count(&bartersInitiated).Error; err != nil {
		return nil, err
	}

	// Count total barters completed (either as offerer or owner)
	var bartersCompleted int64
	if err := r.db.WithContext(ctx).
		Model(&entities.BarterChat{}).
		Where("(offerer_id = ? OR owner_id = ?) AND status = ?", userID, userID, "Completed").
		Count(&bartersCompleted).Error; err != nil {
		return nil, err
	}

	// Count total items traded
	var itemsTraded int64
	if err := r.db.WithContext(ctx).
		Model(&entities.BarterTransactionItem{}).
		Joins("JOIN barter_transactions ON barter_transaction_items.transaction_id = barter_transactions.id").
		Joins("JOIN barter_chats ON barter_transactions.chat_id = barter_chats.id").
		Where("(barter_chats.offerer_id = ? OR barter_chats.owner_id = ?) AND barter_transactions.status = ?", userID, userID, "Completed").
		Count(&itemsTraded).Error; err != nil {
		return nil, err
	}

	// Calculate total coins spent
	totalCoinsSpent := bartersCompleted * int64(BARTER_TRANSACTION_COIN_COST)

	// Estimate food waste saved (assuming 0.3kg per item)
	foodWasteSaved := float64(itemsTraded) * 0.3

	// Create impact message
	estimatedImpact := "You've helped save approximately " +
		strconv.FormatFloat(foodWasteSaved, 'f', 1, 64) +
		"kg of food from being wasted through bartering!"

	stats := map[string]interface{}{
		"total_barters_initiated": bartersInitiated,
		"total_barters_completed": bartersCompleted,
		"total_items_traded":      itemsTraded,
		"total_coins_spent":       totalCoinsSpent,
		"food_waste_saved":        foodWasteSaved,
		"estimated_impact":        estimatedImpact,
	}

	return stats, nil
}
