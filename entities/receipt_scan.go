package entities

import (
	"github.com/google/uuid"
)

type ReceiptScan struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	ImageURL   string    `json:"image_url"`
	Status     string    `json:"status"` // "Pending", "Processed", "Failed"
	OcrResults string    `json:"ocr_results,omitempty" gorm:"type:text"`

	User      *User       `gorm:"foreignKey:UserID"`
	FoodItems []*FoodItem `gorm:"foreignKey:ReceiptScanID"`
	Timestamp
}
