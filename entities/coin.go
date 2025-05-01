package entities

import (
	"github.com/google/uuid"
)

type CoinPackage struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	Name        string    `json:"name"`
	Amount      int       `json:"amount"`
	Price       float64   `json:"price"`
	Currency    string    `json:"currency"`
	Description string    `json:"description,omitempty"`
	ImageURL    string    `json:"image_url,omitempty"`
	IsPopular   bool      `json:"is_popular"`
	IsActive    bool      `json:"is_active"`

	Timestamp
}

type CoinTransaction struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Amount      int       `json:"amount"`
	Type        string    `json:"type"` // Purchase, Use, Reward
	Feature     string    `json:"feature,omitempty"`
	Description string    `json:"description"`
	Balance     int       `json:"balance"`

	User *User `gorm:"foreignKey:UserID"`
	Timestamp
}
