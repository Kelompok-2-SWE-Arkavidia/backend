package domain

import (
	"errors"
	"mime/multipart"
	"time"
)

var (
	MessageSuccessGetDonationLocations = "donation locations retrieved successfully"
	MessageSuccessCreateDonation       = "donation created successfully"
	MessageSuccessGetDonations         = "donations retrieved successfully"
	MessageSuccessUpdateDonation       = "donation updated successfully"
	MessageSuccessDeleteDonation       = "donation deleted successfully"
	MessageSuccessGetNearbyDonations   = "nearby donations retrieved successfully"

	MessageFailedGetDonationLocations = "failed to retrieve donation locations"
	MessageFailedCreateDonation       = "failed to create donation"
	MessageFailedGetDonations         = "failed to retrieve donations"
	MessageFailedUpdateDonation       = "failed to update donation"
	MessageFailedDeleteDonation       = "failed to delete donation"
	MessageFailedGetNearbyDonations   = "failed to retrieve nearby donations"

	ErrDonationNotFound            = errors.New("donation not found")
	ErrUnauthorizedDonationAccess  = errors.New("unauthorized access to donation")
	ErrInvalidDonationLocation     = errors.New("invalid donation location")
	ErrInvalidDonationStatus       = errors.New("invalid donation status")
	ErrInvalidDonationMethod       = errors.New("invalid donation method")
	ErrInvalidCoordinates          = errors.New("invalid coordinates")
	ErrDonationLocationUnavailable = errors.New("donation locations unavailable")
)

type (
	// Donation location types
	DonationLocation struct {
		ID               string    `json:"id"`
		Name             string    `json:"name"`
		Address          string    `json:"address"`
		Latitude         float64   `json:"latitude"`
		Longitude        float64   `json:"longitude"`
		Distance         float64   `json:"distance,omitempty"`
		OperatingHours   string    `json:"operating_hours"`
		ContactNumber    string    `json:"contact_number"`
		AcceptedFoodType string    `json:"accepted_food_type"`
		Rating           float64   `json:"rating"`
		Reviews          int       `json:"reviews"`
		ImageURL         string    `json:"image_url,omitempty"`
		CreatedAt        time.Time `json:"created_at"`
	}

	GetDonationLocationsRequest struct {
		Latitude  float64 `json:"latitude" validate:"required"`
		Longitude float64 `json:"longitude" validate:"required"`
		Radius    float64 `json:"radius" validate:"required,min=1,max=10"`
	}

	DonationRequest struct {
		DonationLocationID string                `json:"donation_location_id" validate:"required,uuid"`
		FoodItems          []string              `json:"food_items" validate:"required,min=1,dive,uuid"`
		Description        string                `json:"description" validate:"omitempty"`
		DonationMethod     string                `json:"donation_method" validate:"required,oneof=SelfDelivery Pickup"`
		ScheduledDate      string                `json:"scheduled_date" validate:"required"`
		FoodImage          *multipart.FileHeader `json:"food_image" form:"food_image"`
	}

	Donation struct {
		ID                 string             `json:"id"`
		UserID             string             `json:"user_id"`
		DonationLocationID string             `json:"donation_location_id"`
		DonationLocation   *DonationLocation  `json:"donation_location,omitempty"`
		FoodItems          []*FoodItemSummary `json:"food_items"`
		Description        string             `json:"description"`
		DonationMethod     string             `json:"donation_method"`
		ScheduledDate      time.Time          `json:"scheduled_date"`
		Status             string             `json:"status"`
		ImageURL           string             `json:"image_url,omitempty"`
		CreatedAt          time.Time          `json:"created_at"`
		UpdatedAt          time.Time          `json:"updated_at"`
		CompletedAt        *time.Time         `json:"completed_at,omitempty"`
		CoinsRewarded      int                `json:"coins_rewarded"`
	}

	FoodItemSummary struct {
		ID         string    `json:"id"`
		Name       string    `json:"name"`
		Quantity   int       `json:"quantity"`
		Unit       string    `json:"unit"`
		ExpiryDate time.Time `json:"expiry_date"`
	}

	UpdateDonationStatusRequest struct {
		DonationID string `json:"donation_id" validate:"required,uuid"`
		Status     string `json:"status" validate:"required,oneof=Pending Accepted Completed Cancelled"`
	}

	DonationStatistics struct {
		TotalDonations       int     `json:"total_donations"`
		CompletedDonations   int     `json:"completed_donations"`
		PendingDonations     int     `json:"pending_donations"`
		TotalItemsDonated    int     `json:"total_items_donated"`
		TotalCoinsEarned     int     `json:"total_coins_earned"`
		EstimatedImpact      string  `json:"estimated_impact"`
		FoodWasteSaved       float64 `json:"food_waste_saved"`      // in kg
		EstimatedCO2Reduced  float64 `json:"estimated_co2_reduced"` // in kg
		EstimatedMealsServed int     `json:"estimated_meals_served"`
	}

	ExpiringFoodSuggestionRequest struct {
		Latitude  float64 `json:"latitude" validate:"required"`
		Longitude float64 `json:"longitude" validate:"required"`
	}

	ExpiringFoodSuggestion struct {
		FoodItems         []*FoodItemSummary  `json:"food_items"`
		NearbyLocations   []*DonationLocation `json:"nearby_locations"`
		SuggestionMessage string              `json:"suggestion_message"`
		PotentialReward   int                 `json:"potential_reward"`
		EstimatedImpact   string              `json:"estimated_impact"`
	}
)
