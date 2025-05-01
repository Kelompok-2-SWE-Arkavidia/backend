package entities

import (
	"github.com/google/uuid"
	"time"
)

type DonationLocation struct {
	ID               uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	Name             string    `json:"name"`
	Address          string    `json:"address"`
	Latitude         float64   `json:"latitude"`
	Longitude        float64   `json:"longitude"`
	OperatingHours   string    `json:"operating_hours"`
	ContactNumber    string    `json:"contact_number"`
	AcceptedFoodType string    `json:"accepted_food_type"`
	Rating           float64   `json:"rating"`
	Reviews          int       `json:"reviews"`
	ImageURL         string    `json:"image_url,omitempty"`

	Donations []*Donation `gorm:"foreignKey:DonationLocationID"`
	Timestamp
}

type Donation struct {
	ID                 uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID             uuid.UUID  `json:"user_id"`
	DonationLocationID uuid.UUID  `json:"donation_location_id"`
	Description        string     `json:"description"`
	DonationMethod     string     `json:"donation_method"` // SelfDelivery or Pickup
	ScheduledDate      time.Time  `json:"scheduled_date"`
	Status             string     `json:"status"` // Pending, Accepted, Completed, Cancelled
	ImageURL           string     `json:"image_url,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	CoinsRewarded      int        `json:"coins_rewarded"`

	User             *User             `gorm:"foreignKey:UserID"`
	DonationLocation *DonationLocation `gorm:"foreignKey:DonationLocationID"`
	DonationItems    []*DonationItem   `gorm:"foreignKey:DonationID"`
	Timestamp
}

type DonationItem struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	DonationID uuid.UUID `json:"donation_id"`
	FoodItemID uuid.UUID `json:"food_item_id"`

	Donation *Donation `gorm:"foreignKey:DonationID"`
	FoodItem *FoodItem `gorm:"foreignKey:FoodItemID"`
	Timestamp
}
