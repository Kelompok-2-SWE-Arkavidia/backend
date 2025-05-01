package entities

import (
	"github.com/google/uuid"
	"time"
)

type BarterItem struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID        uuid.UUID  `json:"user_id"`
	FoodItemID    *uuid.UUID `json:"food_item_id,omitempty"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	Quantity      int        `json:"quantity"`
	UnitMeasure   string     `json:"unit_measure"`
	ExpiryDate    time.Time  `json:"expiry_date"`
	Condition     string     `json:"condition"` // Sealed, Opened, New, Used
	ImageURL      string     `json:"image_url,omitempty"`
	Status        string     `json:"status"` // Available, Reserved, Completed
	Latitude      float64    `json:"latitude,omitempty"`
	Longitude     float64    `json:"longitude,omitempty"`
	PreferredSwap string     `json:"preferred_swap,omitempty"`

	User        *User         `gorm:"foreignKey:UserID"`
	FoodItem    *FoodItem     `gorm:"foreignKey:FoodItemID"`
	BarterChats []*BarterChat `gorm:"foreignKey:BarterItemID"`
	Timestamp
}

type BarterChat struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	BarterItemID    uuid.UUID `json:"barter_item_id"`
	OffererID       uuid.UUID `json:"offerer_id"`
	OwnerID         uuid.UUID `json:"owner_id"`
	Status          string    `json:"status"` // Active, Completed, Cancelled
	LastMessageTime time.Time `json:"last_message_time"`

	BarterItem  *BarterItem        `gorm:"foreignKey:BarterItemID"`
	Offerer     *User              `gorm:"foreignKey:OffererID"`
	Owner       *User              `gorm:"foreignKey:OwnerID"`
	Messages    []*BarterMessage   `gorm:"foreignKey:ChatID"`
	Transaction *BarterTransaction `gorm:"foreignKey:ChatID"`
	Timestamp
}

type BarterMessage struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	ChatID   uuid.UUID `json:"chat_id"`
	SenderID uuid.UUID `json:"sender_id"`
	Content  string    `json:"content"`
	IsRead   bool      `json:"is_read"`

	Chat   *BarterChat `gorm:"foreignKey:ChatID"`
	Sender *User       `gorm:"foreignKey:SenderID"`
	Timestamp
}

type BarterTransaction struct {
	ID               uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	ChatID           uuid.UUID  `json:"chat_id"`
	MeetupName       string     `json:"meetup_name"`
	MeetupAddress    string     `json:"meetup_address"`
	MeetupLatitude   float64    `json:"meetup_latitude"`
	MeetupLongitude  float64    `json:"meetup_longitude"`
	MeetupTime       time.Time  `json:"meetup_time"`
	OwnerConfirmed   bool       `json:"owner_confirmed"`
	OffererConfirmed bool       `json:"offerer_confirmed"`
	Status           string     `json:"status"` // Pending, Completed, Cancelled
	CoinsCharged     int        `json:"coins_charged"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`

	Chat             *BarterChat              `gorm:"foreignKey:ChatID"`
	TransactionItems []*BarterTransactionItem `gorm:"foreignKey:TransactionID"`
	Timestamp
}

type BarterTransactionItem struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	TransactionID uuid.UUID `json:"transaction_id"`
	BarterItemID  uuid.UUID `json:"barter_item_id"`
	OwnerItem     bool      `json:"owner_item"` // true if item belongs to owner, false if to offerer

	Transaction *BarterTransaction `gorm:"foreignKey:TransactionID"`
	BarterItem  *BarterItem        `gorm:"foreignKey:BarterItemID"`
	Timestamp
}
