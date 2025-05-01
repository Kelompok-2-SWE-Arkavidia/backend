package donation

import (
	"Go-Starter-Template/entities"
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"time"
)

type (
	DonationRepository interface {
		CreateDonationLocation(ctx context.Context, location *entities.DonationLocation) error
		GetDonationLocations(ctx context.Context, lat, lng float64, radius float64) ([]*entities.DonationLocation, error)
		GetDonationLocationByID(ctx context.Context, id string) (*entities.DonationLocation, error)

		CreateDonation(ctx context.Context, donation *entities.Donation) error
		AddDonationItem(ctx context.Context, donationItem *entities.DonationItem) error
		GetDonationByID(ctx context.Context, id string) (*entities.Donation, error)
		GetUserDonations(ctx context.Context, userID string, page, limit int) ([]*entities.Donation, int64, error)
		UpdateDonationStatus(ctx context.Context, id string, status string, completedAt *time.Time) error
		GetDonationItems(ctx context.Context, donationID string) ([]*entities.DonationItem, error)
		GetDonationStatistics(ctx context.Context, userID string) (map[string]interface{}, error)

		GetExpiringFoodItems(ctx context.Context, userID string, days int) ([]*entities.FoodItem, error)
	}

	donationRepository struct {
		db *gorm.DB
	}
)

func NewDonationRepository(db *gorm.DB) DonationRepository {
	return &donationRepository{db: db}
}

func (r *donationRepository) CreateDonationLocation(ctx context.Context, location *entities.DonationLocation) error {
	return r.db.WithContext(ctx).Create(location).Error
}

func (r *donationRepository) GetDonationLocations(ctx context.Context, lat, lng float64, radius float64) ([]*entities.DonationLocation, error) {
	var locations []*entities.DonationLocation

	// Using PostgreSQL's earthdistance extension for location-based queries
	// Make sure you've installed the extension with:
	// CREATE EXTENSION IF NOT EXISTS "earthdistance" CASCADE;
	// CREATE EXTENSION IF NOT EXISTS "cube";
	query := `
		SELECT *,
		       earth_distance(ll_to_earth(?, ?), ll_to_earth(latitude, longitude)) as distance
		FROM donation_locations
		WHERE earth_box(ll_to_earth(?, ?), ?) @> ll_to_earth(latitude, longitude)
		ORDER BY distance ASC
	`

	// radius in km, convert to meters for the query
	radiusMeters := radius * 1000

	if err := r.db.WithContext(ctx).Raw(query, lat, lng, lat, lng, radiusMeters).Scan(&locations).Error; err != nil {
		return nil, err
	}

	return locations, nil
}

func (r *donationRepository) GetDonationLocationByID(ctx context.Context, id string) (*entities.DonationLocation, error) {
	var location entities.DonationLocation
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&location).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &location, nil
}

func (r *donationRepository) CreateDonation(ctx context.Context, donation *entities.Donation) error {
	return r.db.WithContext(ctx).Create(donation).Error
}

func (r *donationRepository) AddDonationItem(ctx context.Context, donationItem *entities.DonationItem) error {
	return r.db.WithContext(ctx).Create(donationItem).Error
}

func (r *donationRepository) GetDonationByID(ctx context.Context, id string) (*entities.Donation, error) {
	var donation entities.Donation
	if err := r.db.WithContext(ctx).
		Preload("User").
		Preload("DonationLocation").
		Preload("DonationItems").
		Preload("DonationItems.FoodItem").
		Where("id = ?", id).
		First(&donation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &donation, nil
}

func (r *donationRepository) GetUserDonations(ctx context.Context, userID string, page, limit int) ([]*entities.Donation, int64, error) {
	var donations []*entities.Donation
	var count int64
	offset := (page - 1) * limit

	if err := r.db.WithContext(ctx).
		Model(&entities.Donation{}).
		Where("user_id = ?", userID).
		Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.WithContext(ctx).
		Preload("DonationLocation").
		Preload("DonationItems").
		Preload("DonationItems.FoodItem").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&donations).Error; err != nil {
		return nil, 0, err
	}

	return donations, count, nil
}

func (r *donationRepository) UpdateDonationStatus(ctx context.Context, id string, status string, completedAt *time.Time) error {
	updates := map[string]interface{}{
		"status": status,
	}

	if completedAt != nil {
		updates["completed_at"] = completedAt
	}

	if status == "Completed" && completedAt == nil {
		now := time.Now()
		updates["completed_at"] = now
	}

	return r.db.WithContext(ctx).
		Model(&entities.Donation{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *donationRepository) GetDonationItems(ctx context.Context, donationID string) ([]*entities.DonationItem, error) {
	var items []*entities.DonationItem
	if err := r.db.WithContext(ctx).
		Preload("FoodItem").
		Where("donation_id = ?", donationID).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *donationRepository) GetDonationStatistics(ctx context.Context, userID string) (map[string]interface{}, error) {
	var totalDonations, completedDonations, pendingDonations int64
	var totalItemsDonated int64
	var totalCoinsEarned int64

	// Count total donations
	if err := r.db.WithContext(ctx).
		Model(&entities.Donation{}).
		Where("user_id = ?", userID).
		Count(&totalDonations).Error; err != nil {
		return nil, err
	}

	// Count completed donations
	if err := r.db.WithContext(ctx).
		Model(&entities.Donation{}).
		Where("user_id = ? AND status = ?", userID, "Completed").
		Count(&completedDonations).Error; err != nil {
		return nil, err
	}

	// Count pending donations
	if err := r.db.WithContext(ctx).
		Model(&entities.Donation{}).
		Where("user_id = ? AND status = ?", userID, "Pending").
		Count(&pendingDonations).Error; err != nil {
		return nil, err
	}

	// Count total items donated
	if err := r.db.WithContext(ctx).
		Model(&entities.DonationItem{}).
		Joins("JOIN donations ON donation_items.donation_id = donations.id").
		Where("donations.user_id = ?", userID).
		Count(&totalItemsDonated).Error; err != nil {
		return nil, err
	}

	// Calculate total coins earned
	var result struct {
		TotalCoins int64
	}
	if err := r.db.WithContext(ctx).
		Model(&entities.Donation{}).
		Select("COALESCE(SUM(coins_rewarded), 0) as total_coins").
		Where("user_id = ? AND status = ?", userID, "Completed").
		Scan(&result).Error; err != nil {
		return nil, err
	}
	totalCoinsEarned = result.TotalCoins

	// Estimate impact metrics
	foodWasteSaved := float64(totalItemsDonated) * 0.25           // Assuming average 0.25kg per item
	estimatedCO2Reduced := foodWasteSaved * 2.5                   // 2.5kg CO2 per kg of food waste
	estimatedMealsServed := int(float64(totalItemsDonated) * 0.8) // Assume 0.8 meals per item on average

	stats := map[string]interface{}{
		"total_donations":        totalDonations,
		"completed_donations":    completedDonations,
		"pending_donations":      pendingDonations,
		"total_items_donated":    totalItemsDonated,
		"total_coins_earned":     totalCoinsEarned,
		"food_waste_saved":       foodWasteSaved,
		"estimated_co2_reduced":  estimatedCO2Reduced,
		"estimated_meals_served": estimatedMealsServed,
		"estimated_impact":       fmt.Sprintf("You've helped provide approximately %d meals to those in need.", estimatedMealsServed),
	}

	return stats, nil
}

func (r *donationRepository) GetExpiringFoodItems(ctx context.Context, userID string, days int) ([]*entities.FoodItem, error) {
	var foodItems []*entities.FoodItem

	expiryThreshold := time.Now().AddDate(0, 0, days)

	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND expiry_date <= ? AND status = ?", userID, expiryThreshold, "Safe").
		Order("expiry_date ASC").
		Find(&foodItems).Error; err != nil {
		return nil, err
	}

	return foodItems, nil
}
