package entities

import (
	"github.com/google/uuid"
	"time"
)

type FoodItem struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	Name          string    `json:"name"`
	Quantity      int       `json:"quantity"`
	UnitMeasure   string    `json:"unit_measure"`
	ExpiryDate    time.Time `json:"expiry_date"`
	IsPackaged    bool      `json:"is_packaged"`
	Status        string    `json:"status"` // "Safe", "Warning", "Expired", "Damaged"
	ImageURL      string    `json:"image_url,omitempty"`
	AddedManually bool      `json:"added_manually"`
	ReceiptScanID *string   `json:"receipt_scan_id,omitempty"`

	User *User `gorm:"foreignKey:UserID"`
	Timestamp
}
