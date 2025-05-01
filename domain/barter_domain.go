package domain

import (
	"errors"
	"mime/multipart"
	"time"
)

var (
	MessageSuccessGetBarterItems      = "barter items retrieved successfully"
	MessageSuccessCreateBarterItem    = "barter item created successfully"
	MessageSuccessUpdateBarterItem    = "barter item updated successfully"
	MessageSuccessDeleteBarterItem    = "barter item deleted successfully"
	MessageSuccessGetNearbyItems      = "nearby barter items retrieved successfully"
	MessageSuccessBarterExpress       = "barter interest expressed successfully"
	MessageSuccessGetBarterChats      = "barter chats retrieved successfully"
	MessageSuccessSendMessage         = "message sent successfully"
	MessageSuccessCompleteTransaction = "barter transaction completed successfully"
	MessageSuccessGetBarterHistory    = "barter history retrieved successfully"

	MessageFailedGetBarterItems      = "failed to retrieve barter items"
	MessageFailedCreateBarterItem    = "failed to create barter item"
	MessageFailedUpdateBarterItem    = "failed to update barter item"
	MessageFailedDeleteBarterItem    = "failed to delete barter item"
	MessageFailedGetNearbyItems      = "failed to retrieve nearby barter items"
	MessageFailedBarterExpress       = "failed to express barter interest"
	MessageFailedGetBarterChats      = "failed to retrieve barter chats"
	MessageFailedSendMessage         = "failed to send message"
	MessageFailedCompleteTransaction = "failed to complete barter transaction"
	MessageFailedGetBarterHistory    = "failed to retrieve barter history"

	ErrBarterItemNotFound       = errors.New("barter item not found")
	ErrUnauthorizedBarterAccess = errors.New("unauthorized access to barter item")
	ErrInvalidBarterStatus      = errors.New("invalid barter status")
	ErrBarterChatNotFound       = errors.New("barter chat not found")
	ErrBarterAlreadyCompleted   = errors.New("barter already completed")
	ErrBarterSelfTransaction    = errors.New("cannot barter with yourself")
	ErrItemAlreadyBartered      = errors.New("item is already bartered")
	ErrNotEnoughCoins           = errors.New("not enough coins to complete transaction")
)

type (
	BarterItemRequest struct {
		Name          string                `json:"name" validate:"required"`
		Description   string                `json:"description" validate:"required"`
		Quantity      int                   `json:"quantity" validate:"required,min=1"`
		UnitMeasure   string                `json:"unit_measure" validate:"required"`
		ExpiryDate    string                `json:"expiry_date" validate:"required"`
		Condition     string                `json:"condition" validate:"required,oneof=Sealed Opened New Used"`
		Image         *multipart.FileHeader `json:"image" form:"image"`
		FoodItemID    string                `json:"food_item_id,omitempty" validate:"omitempty,uuid"`
		PreferredSwap string                `json:"preferred_swap" validate:"omitempty"`
	}

	BarterItem struct {
		ID            string    `json:"id"`
		UserID        string    `json:"user_id"`
		UserName      string    `json:"user_name,omitempty"`
		UserRating    float64   `json:"user_rating,omitempty"`
		Name          string    `json:"name"`
		Description   string    `json:"description"`
		Quantity      int       `json:"quantity"`
		UnitMeasure   string    `json:"unit_measure"`
		ExpiryDate    time.Time `json:"expiry_date"`
		Condition     string    `json:"condition"`
		ImageURL      string    `json:"image_url,omitempty"`
		Status        string    `json:"status"`             // Available, Reserved, Completed
		Distance      float64   `json:"distance,omitempty"` // in km
		CreatedAt     time.Time `json:"created_at"`
		UpdatedAt     time.Time `json:"updated_at"`
		PreferredSwap string    `json:"preferred_swap,omitempty"`
	}

	GetNearbyBarterItemsRequest struct {
		Latitude      float64 `json:"latitude" validate:"required"`
		Longitude     float64 `json:"longitude" validate:"required"`
		Radius        float64 `json:"radius" validate:"required,min=1,max=10"`
		IncludeOwn    bool    `json:"include_own"`
		Status        string  `json:"status,omitempty" validate:"omitempty,oneof=Available Reserved Completed All"`
		ExpiryMaxDays int     `json:"expiry_max_days,omitempty" validate:"omitempty,min=1"`
	}

	BarterInterestRequest struct {
		BarterItemID string `json:"barter_item_id" validate:"required,uuid"`
		Message      string `json:"message" validate:"required"`
	}

	BarterChat struct {
		ID              string             `json:"id"`
		BarterItemID    string             `json:"barter_item_id"`
		BarterItem      *BarterItem        `json:"barter_item,omitempty"`
		OffererID       string             `json:"offerer_id"`
		OffererName     string             `json:"offerer_name"`
		OwnerID         string             `json:"owner_id"`
		OwnerName       string             `json:"owner_name"`
		LastMessage     string             `json:"last_message"`
		LastMessageTime time.Time          `json:"last_message_time"`
		Status          string             `json:"status"` // Active, Completed, Cancelled
		UnreadCount     int                `json:"unread_count"`
		Messages        []*BarterMessage   `json:"messages,omitempty"`
		OfferedItems    []*BarterItemBrief `json:"offered_items,omitempty"`
		MeetupLocation  *MeetupLocation    `json:"meetup_location,omitempty"`
		MeetupTime      *time.Time         `json:"meetup_time,omitempty"`
		CreatedAt       time.Time          `json:"created_at"`
		CompletedAt     *time.Time         `json:"completed_at,omitempty"`
	}

	BarterMessage struct {
		ID        string    `json:"id"`
		ChatID    string    `json:"chat_id"`
		SenderID  string    `json:"sender_id"`
		Content   string    `json:"content"`
		IsRead    bool      `json:"is_read"`
		CreatedAt time.Time `json:"created_at"`
	}

	BarterItemBrief struct {
		ID         string    `json:"id"`
		Name       string    `json:"name"`
		ImageURL   string    `json:"image_url,omitempty"`
		ExpiryDate time.Time `json:"expiry_date"`
		IsSelected bool      `json:"is_selected"`
	}

	MeetupLocation struct {
		Name      string  `json:"name"`
		Address   string  `json:"address"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}

	SendMessageRequest struct {
		ChatID  string `json:"chat_id" validate:"required,uuid"`
		Content string `json:"content" validate:"required"`
	}

	CompleteBarterRequest struct {
		ChatID          string    `json:"chat_id" validate:"required,uuid"`
		OwnerItemIDs    []string  `json:"owner_item_ids" validate:"required,min=1,dive,uuid"`
		OffererItemIDs  []string  `json:"offerer_item_ids" validate:"required,min=1,dive,uuid"`
		MeetupLocation  string    `json:"meetup_location" validate:"required"`
		MeetupAddress   string    `json:"meetup_address" validate:"required"`
		MeetupLatitude  float64   `json:"meetup_latitude" validate:"required"`
		MeetupLongitude float64   `json:"meetup_longitude" validate:"required"`
		MeetupTime      time.Time `json:"meetup_time" validate:"required"`
	}

	ConfirmBarterCompletionRequest struct {
		ChatID      string `json:"chat_id" validate:"required,uuid"`
		IsCompleted bool   `json:"is_completed" validate:"required"`
		Rating      int    `json:"rating" validate:"min=1,max=5"`
		Comment     string `json:"comment"`
	}

	BarterTransaction struct {
		ID               string         `json:"id"`
		ChatID           string         `json:"chat_id"`
		OwnerID          string         `json:"owner_id"`
		OffererID        string         `json:"offerer_id"`
		OwnerItems       []BarterItem   `json:"owner_items"`
		OffererItems     []BarterItem   `json:"offerer_items"`
		MeetupLocation   MeetupLocation `json:"meetup_location"`
		MeetupTime       time.Time      `json:"meetup_time"`
		OwnerConfirmed   bool           `json:"owner_confirmed"`
		OffererConfirmed bool           `json:"offerer_confirmed"`
		Status           string         `json:"status"` // Pending, Completed, Cancelled
		CoinsCharged     int            `json:"coins_charged"`
		CreatedAt        time.Time      `json:"created_at"`
		CompletedAt      *time.Time     `json:"completed_at,omitempty"`
	}

	BarterStatistics struct {
		TotalBartersInitiated int     `json:"total_barters_initiated"`
		TotalBartersCompleted int     `json:"total_barters_completed"`
		TotalItemsTraded      int     `json:"total_items_traded"`
		TotalCoinsSpent       int     `json:"total_coins_spent"`
		FoodWasteSaved        float64 `json:"food_waste_saved"` // in kg
		EstimatedImpact       string  `json:"estimated_impact"`
	}
)
