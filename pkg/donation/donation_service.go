package donation

import (
	"Go-Starter-Template/domain"
	"Go-Starter-Template/entities"
	"Go-Starter-Template/internal/utils/storage"
	"Go-Starter-Template/pkg/food"
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

const (
	COINS_PER_DONATION_ITEM = 5
)

type (
	DonationService interface {
		GetDonationLocations(ctx context.Context, req domain.GetDonationLocationsRequest) ([]*domain.DonationLocation, error)
		CreateDonation(ctx context.Context, req domain.DonationRequest, userID string) (*domain.Donation, error)
		GetUserDonations(ctx context.Context, userID string, page, limit int) ([]*domain.Donation, int64, error)
		GetDonationByID(ctx context.Context, id string, userID string) (*domain.Donation, error)
		UpdateDonationStatus(ctx context.Context, req domain.UpdateDonationStatusRequest, userID string) error
		GetDonationStatistics(ctx context.Context, userID string) (*domain.DonationStatistics, error)
		GetExpiringFoodSuggestions(ctx context.Context, req domain.ExpiringFoodSuggestionRequest, userID string) (*domain.ExpiringFoodSuggestion, error)
	}

	donationService struct {
		donationRepository DonationRepository
		foodRepository     food.FoodRepository
		s3                 storage.AwsS3
	}
)

func NewDonationService(donationRepository DonationRepository, foodRepository food.FoodRepository, s3 storage.AwsS3) DonationService {
	return &donationService{
		donationRepository: donationRepository,
		foodRepository:     foodRepository,
		s3:                 s3,
	}
}

func (s *donationService) GetDonationLocations(ctx context.Context, req domain.GetDonationLocationsRequest) ([]*domain.DonationLocation, error) {
	locations, err := s.donationRepository.GetDonationLocations(ctx, req.Latitude, req.Longitude, req.Radius)
	if err != nil {
		return nil, err
	}

	if len(locations) == 0 {
		return []*domain.DonationLocation{}, domain.ErrDonationLocationUnavailable
	}

	result := make([]*domain.DonationLocation, 0, len(locations))
	for _, loc := range locations {
		result = append(result, &domain.DonationLocation{
			ID:               loc.ID.String(),
			Name:             loc.Name,
			Address:          loc.Address,
			Latitude:         loc.Latitude,
			Longitude:        loc.Longitude,
			OperatingHours:   loc.OperatingHours,
			ContactNumber:    loc.ContactNumber,
			AcceptedFoodType: loc.AcceptedFoodType,
			Rating:           loc.Rating,
			Reviews:          loc.Reviews,
			ImageURL:         loc.ImageURL,
			CreatedAt:        loc.CreatedAt,
		})
	}

	return result, nil
}

func (s *donationService) CreateDonation(ctx context.Context, req domain.DonationRequest, userID string) (*domain.Donation, error) {
	// Validate donation location
	donationLocation, err := s.donationRepository.GetDonationLocationByID(ctx, req.DonationLocationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrInvalidDonationLocation
		}
		return nil, err
	}

	// Parse scheduled date
	scheduledDate, err := time.Parse("2006-01-02", req.ScheduledDate)
	if err != nil {
		return nil, err
	}

	// Validate donation method
	if req.DonationMethod != "SelfDelivery" && req.DonationMethod != "Pickup" {
		return nil, domain.ErrInvalidDonationMethod
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, domain.ErrParseUUID
	}

	locationUUID, err := uuid.Parse(req.DonationLocationID)
	if err != nil {
		return nil, domain.ErrParseUUID
	}

	donationID := uuid.New()

	// Process food image if provided
	var imageURL string
	if req.FoodImage != nil {
		objectKey, err := s.s3.UploadFile(
			fmt.Sprintf("donation-%s", donationID.String()),
			req.FoodImage,
			"donations",
			storage.AllowImage...,
		)
		if err != nil {
			return nil, err
		}
		imageURL = s.s3.GetPublicLinkKey(objectKey)
	}

	// Create donation
	donation := &entities.Donation{
		ID:                 donationID,
		UserID:             userUUID,
		DonationLocationID: locationUUID,
		Description:        req.Description,
		DonationMethod:     req.DonationMethod,
		ScheduledDate:      scheduledDate,
		Status:             "Pending",
		ImageURL:           imageURL,
		CoinsRewarded:      0, // Will be rewarded when completed
	}

	if err := s.donationRepository.CreateDonation(ctx, donation); err != nil {
		return nil, err
	}

	// Add donation items
	foodItemsMap := make(map[string]*entities.FoodItem)
	for _, foodItemID := range req.FoodItems {
		foodItemUUID, err := uuid.Parse(foodItemID)
		if err != nil {
			continue // Skip invalid IDs
		}

		foodItem, err := s.foodRepository.GetFoodItemByID(ctx, foodItemID)
		if err != nil {
			continue // Skip not found items
		}

		if foodItem.UserID.String() != userID {
			continue // Skip unauthorized items
		}

		// Add to donation items
		donationItem := &entities.DonationItem{
			ID:         uuid.New(),
			DonationID: donationID,
			FoodItemID: foodItemUUID,
		}

		if err := s.donationRepository.AddDonationItem(ctx, donationItem); err != nil {
			continue // Skip on error
		}

		foodItemsMap[foodItemID] = foodItem
	}

	// Get donation with items
	createdDonation, err := s.donationRepository.GetDonationByID(ctx, donationID.String())
	if err != nil {
		return nil, err
	}

	// Convert to domain model
	foodItems := make([]*domain.FoodItemSummary, 0, len(createdDonation.DonationItems))
	for _, item := range createdDonation.DonationItems {
		if item.FoodItem != nil {
			foodItems = append(foodItems, &domain.FoodItemSummary{
				ID:         item.FoodItem.ID.String(),
				Name:       item.FoodItem.Name,
				Quantity:   item.FoodItem.Quantity,
				Unit:       item.FoodItem.UnitMeasure,
				ExpiryDate: item.FoodItem.ExpiryDate,
			})
		}
	}

	donationLocation = &entities.DonationLocation{
		ID:             createdDonation.DonationLocation.ID,
		Name:           createdDonation.DonationLocation.Name,
		Address:        createdDonation.DonationLocation.Address,
		OperatingHours: createdDonation.DonationLocation.OperatingHours,
		ContactNumber:  createdDonation.DonationLocation.ContactNumber,
	}

	result := &domain.Donation{
		ID:                 createdDonation.ID.String(),
		UserID:             createdDonation.UserID.String(),
		DonationLocationID: createdDonation.DonationLocationID.String(),
		DonationLocation: &domain.DonationLocation{
			ID:             donationLocation.ID.String(),
			Name:           donationLocation.Name,
			Address:        donationLocation.Address,
			OperatingHours: donationLocation.OperatingHours,
			ContactNumber:  donationLocation.ContactNumber,
		},
		FoodItems:      foodItems,
		Description:    createdDonation.Description,
		DonationMethod: createdDonation.DonationMethod,
		ScheduledDate:  createdDonation.ScheduledDate,
		Status:         createdDonation.Status,
		ImageURL:       createdDonation.ImageURL,
		CreatedAt:      createdDonation.CreatedAt,
		UpdatedAt:      createdDonation.UpdatedAt,
		CoinsRewarded:  createdDonation.CoinsRewarded,
	}

	return result, nil
}

func (s *donationService) GetUserDonations(ctx context.Context, userID string, page, limit int) ([]*domain.Donation, int64, error) {
	donations, count, err := s.donationRepository.GetUserDonations(ctx, userID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*domain.Donation, 0, len(donations))
	for _, donation := range donations {
		// Convert food items
		foodItems := make([]*domain.FoodItemSummary, 0, len(donation.DonationItems))
		for _, item := range donation.DonationItems {
			if item.FoodItem != nil {
				foodItems = append(foodItems, &domain.FoodItemSummary{
					ID:         item.FoodItem.ID.String(),
					Name:       item.FoodItem.Name,
					Quantity:   item.FoodItem.Quantity,
					Unit:       item.FoodItem.UnitMeasure,
					ExpiryDate: item.FoodItem.ExpiryDate,
				})
			}
		}

		// Get donation location
		var donationLocation *domain.DonationLocation
		if donation.DonationLocation != nil {
			donationLocation = &domain.DonationLocation{
				ID:             donation.DonationLocation.ID.String(),
				Name:           donation.DonationLocation.Name,
				Address:        donation.DonationLocation.Address,
				OperatingHours: donation.DonationLocation.OperatingHours,
				ContactNumber:  donation.DonationLocation.ContactNumber,
			}
		}

		result = append(result, &domain.Donation{
			ID:                 donation.ID.String(),
			UserID:             donation.UserID.String(),
			DonationLocationID: donation.DonationLocationID.String(),
			DonationLocation:   donationLocation,
			FoodItems:          foodItems,
			Description:        donation.Description,
			DonationMethod:     donation.DonationMethod,
			ScheduledDate:      donation.ScheduledDate,
			Status:             donation.Status,
			ImageURL:           donation.ImageURL,
			CreatedAt:          donation.CreatedAt,
			UpdatedAt:          donation.UpdatedAt,
			CompletedAt:        donation.CompletedAt,
			CoinsRewarded:      donation.CoinsRewarded,
		})
	}

	return result, count, nil
}

func (s *donationService) GetDonationByID(ctx context.Context, id string, userID string) (*domain.Donation, error) {
	donation, err := s.donationRepository.GetDonationByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrDonationNotFound
		}
		return nil, err
	}

	// Check if user is authorized to view this donation
	if donation.UserID.String() != userID {
		return nil, domain.ErrUnauthorizedDonationAccess
	}

	// Convert food items
	foodItems := make([]*domain.FoodItemSummary, 0, len(donation.DonationItems))
	for _, item := range donation.DonationItems {
		if item.FoodItem != nil {
			foodItems = append(foodItems, &domain.FoodItemSummary{
				ID:         item.FoodItem.ID.String(),
				Name:       item.FoodItem.Name,
				Quantity:   item.FoodItem.Quantity,
				Unit:       item.FoodItem.UnitMeasure,
				ExpiryDate: item.FoodItem.ExpiryDate,
			})
		}
	}

	// Get donation location
	var donationLocation *domain.DonationLocation
	if donation.DonationLocation != nil {
		donationLocation = &domain.DonationLocation{
			ID:               donation.DonationLocation.ID.String(),
			Name:             donation.DonationLocation.Name,
			Address:          donation.DonationLocation.Address,
			Latitude:         donation.DonationLocation.Latitude,
			Longitude:        donation.DonationLocation.Longitude,
			OperatingHours:   donation.DonationLocation.OperatingHours,
			ContactNumber:    donation.DonationLocation.ContactNumber,
			AcceptedFoodType: donation.DonationLocation.AcceptedFoodType,
		}
	}

	result := &domain.Donation{
		ID:                 donation.ID.String(),
		UserID:             donation.UserID.String(),
		DonationLocationID: donation.DonationLocationID.String(),
		DonationLocation:   donationLocation,
		FoodItems:          foodItems,
		Description:        donation.Description,
		DonationMethod:     donation.DonationMethod,
		ScheduledDate:      donation.ScheduledDate,
		Status:             donation.Status,
		ImageURL:           donation.ImageURL,
		CreatedAt:          donation.CreatedAt,
		UpdatedAt:          donation.UpdatedAt,
		CompletedAt:        donation.CompletedAt,
		CoinsRewarded:      donation.CoinsRewarded,
	}

	return result, nil
}

func (s *donationService) UpdateDonationStatus(ctx context.Context, req domain.UpdateDonationStatusRequest, userID string) error {
	// Get the donation
	donation, err := s.donationRepository.GetDonationByID(ctx, req.DonationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrDonationNotFound
		}
		return err
	}

	// Check if user is authorized to update this donation
	if donation.UserID.String() != userID {
		return domain.ErrUnauthorizedDonationAccess
	}

	// Validate status
	if req.Status != "Pending" && req.Status != "Accepted" && req.Status != "Completed" && req.Status != "Cancelled" {
		return domain.ErrInvalidDonationStatus
	}

	var completedAt *time.Time
	var coinsToReward int

	// If status is changing to completed, set completedAt and calculate rewards
	if req.Status == "Completed" && donation.Status != "Completed" {
		now := time.Now()
		completedAt = &now

		// Get donation items
		items, err := s.donationRepository.GetDonationItems(ctx, req.DonationID)
		if err != nil {
			return err
		}

		// Calculate rewards (5 coins per item)
		coinsToReward = len(items) * COINS_PER_DONATION_ITEM

		// Update donation with rewards
		donation.CoinsRewarded = coinsToReward
		// We'll update the user's coins in a separate service call
	}

	// Update donation status
	if err := s.donationRepository.UpdateDonationStatus(ctx, req.DonationID, req.Status, completedAt); err != nil {
		return err
	}

	return nil
}

func (s *donationService) GetDonationStatistics(ctx context.Context, userID string) (*domain.DonationStatistics, error) {
	stats, err := s.donationRepository.GetDonationStatistics(ctx, userID)
	if err != nil {
		return nil, err
	}

	totalDonations := int(stats["total_donations"].(int64))
	completedDonations := int(stats["completed_donations"].(int64))
	pendingDonations := int(stats["pending_donations"].(int64))
	totalItemsDonated := int(stats["total_items_donated"].(int64))
	totalCoinsEarned := int(stats["total_coins_earned"].(int64))
	foodWasteSaved := stats["food_waste_saved"].(float64)
	estimatedCO2Reduced := stats["estimated_co2_reduced"].(float64)
	estimatedMealsServed := stats["estimated_meals_served"].(int)
	estimatedImpact := stats["estimated_impact"].(string)

	return &domain.DonationStatistics{
		TotalDonations:       totalDonations,
		CompletedDonations:   completedDonations,
		PendingDonations:     pendingDonations,
		TotalItemsDonated:    totalItemsDonated,
		TotalCoinsEarned:     totalCoinsEarned,
		EstimatedImpact:      estimatedImpact,
		FoodWasteSaved:       foodWasteSaved,
		EstimatedCO2Reduced:  estimatedCO2Reduced,
		EstimatedMealsServed: estimatedMealsServed,
	}, nil
}

func (s *donationService) GetExpiringFoodSuggestions(ctx context.Context, req domain.ExpiringFoodSuggestionRequest, userID string) (*domain.ExpiringFoodSuggestion, error) {
	// Get food items expiring within 3 days
	expiringItems, err := s.donationRepository.GetExpiringFoodItems(ctx, userID, 3)
	if err != nil {
		return nil, err
	}

	if len(expiringItems) == 0 {
		return &domain.ExpiringFoodSuggestion{
			FoodItems:         []*domain.FoodItemSummary{},
			NearbyLocations:   []*domain.DonationLocation{},
			SuggestionMessage: "No food items are expiring soon. Great job managing your food inventory!",
			PotentialReward:   0,
			EstimatedImpact:   "",
		}, nil
	}

	// Find nearby donation locations
	locations, err := s.donationRepository.GetDonationLocations(ctx, req.Latitude, req.Longitude, 5)
	if err != nil || len(locations) == 0 {
		// Even if we don't have nearby locations, we'll still suggest donating
		locations = []*entities.DonationLocation{}
	}

	// Convert food items to summaries
	foodItems := make([]*domain.FoodItemSummary, 0, len(expiringItems))
	for _, item := range expiringItems {
		foodItems = append(foodItems, &domain.FoodItemSummary{
			ID:         item.ID.String(),
			Name:       item.Name,
			Quantity:   item.Quantity,
			Unit:       item.UnitMeasure,
			ExpiryDate: item.ExpiryDate,
		})
	}

	// Convert locations to domain model
	nearbyLocations := make([]*domain.DonationLocation, 0, len(locations))
	for _, loc := range locations {
		nearbyLocations = append(nearbyLocations, &domain.DonationLocation{
			ID:               loc.ID.String(),
			Name:             loc.Name,
			Address:          loc.Address,
			Latitude:         loc.Latitude,
			Longitude:        loc.Longitude,
			OperatingHours:   loc.OperatingHours,
			ContactNumber:    loc.ContactNumber,
			AcceptedFoodType: loc.AcceptedFoodType,
			Rating:           loc.Rating,
			Reviews:          loc.Reviews,
			ImageURL:         loc.ImageURL,
			CreatedAt:        loc.CreatedAt,
		})
	}

	// Calculate potential reward
	potentialReward := len(foodItems) * COINS_PER_DONATION_ITEM

	// Estimate impact
	estimatedMeals := int(float64(len(foodItems)) * 0.8)
	estimatedImpact := fmt.Sprintf("By donating these items, you could provide approximately %d meals to those in need.", estimatedMeals)

	// Prepare suggestion message
	var suggestionMessage string
	if len(nearbyLocations) > 0 {
		suggestionMessage = fmt.Sprintf("You have %d items expiring soon. We found %d donation centers nearby where you could donate these items and earn %d coins!",
			len(foodItems), len(nearbyLocations), potentialReward)
	} else {
		suggestionMessage = fmt.Sprintf("You have %d items expiring soon. Consider donating them to help reduce food waste and earn %d coins!",
			len(foodItems), potentialReward)
	}

	return &domain.ExpiringFoodSuggestion{
		FoodItems:         foodItems,
		NearbyLocations:   nearbyLocations,
		SuggestionMessage: suggestionMessage,
		PotentialReward:   potentialReward,
		EstimatedImpact:   estimatedImpact,
	}, nil
}
