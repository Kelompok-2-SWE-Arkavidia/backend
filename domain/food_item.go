package domain

import (
	"errors"
	"mime/multipart"
	"time"
)

var (
	MessageSuccessAddFoodItem       = "food item added successfully"
	MessageSuccessUpdateFoodItem    = "food item updated successfully"
	MessageSuccessDeleteFoodItem    = "food item deleted successfully"
	MessageSuccessGetFoodItems      = "food items retrieved successfully"
	MessageSuccessUploadReceipt     = "receipt uploaded successfully"
	MessageSuccessSaveScannedItems  = "scanned items saved successfully"
	MessageSuccessMarkAsDamaged     = "food item marked as damaged"
	MessageSuccessGetDashboardStats = "dashboard statistics retrieved successfully"

	MessageFailedAddFoodItem       = "failed to add food item"
	MessageFailedUpdateFoodItem    = "failed to update food item"
	MessageFailedDeleteFoodItem    = "failed to delete food item"
	MessageFailedGetFoodItems      = "failed to retrieve food items"
	MessageFailedUploadReceipt     = "failed to upload receipt"
	MessageFailedProcessReceipt    = "failed to process receipt"
	MessageFailedSaveScannedItems  = "failed to save scanned items"
	MessageFailedMarkAsDamaged     = "failed to mark food item as damaged"
	MessageFailedGetDashboardStats = "failed to retrieve dashboard statistics"
	MessageFailedDetectFoodAge     = "failed to detect food age from image"

	ErrFoodItemNotFound        = errors.New("food item not found")
	ErrReceiptProcessingFailed = errors.New("receipt processing failed")
	ErrInvalidExpiryDate       = errors.New("invalid expiry date")
	ErrInvalidQuantity         = errors.New("quantity must be positive")
	ErrInvalidImageFormat      = errors.New("invalid image format")
	ErrInvalidReceiptScan      = errors.New("invalid receipt scan ID")
	ErrUnauthorizedAccess      = errors.New("unauthorized access to food item")
	ErrGeminiProcessingFailed  = errors.New("gemini processing failed")
)

type (
	AddFoodItemRequest struct {
		Name        string `json:"name" validate:"required"`
		Quantity    int    `json:"quantity" validate:"required,min=1"`
		UnitMeasure string `json:"unit_measure" validate:"required"`
		ExpiryDate  string `json:"expiry_date" validate:"required"`
		IsPackaged  bool   `json:"is_packaged"`
	}

	AddFoodItemResponse struct {
		ID          string    `json:"id"`
		Name        string    `json:"name"`
		Quantity    int       `json:"quantity"`
		UnitMeasure string    `json:"unit_measure"`
		ExpiryDate  time.Time `json:"expiry_date"`
		IsPackaged  bool      `json:"is_packaged"`
		Status      string    `json:"status"`
	}

	UpdateFoodItemRequest struct {
		Name        string `json:"name" validate:"omitempty"`
		Quantity    int    `json:"quantity" validate:"omitempty,min=1"`
		UnitMeasure string `json:"unit_measure" validate:"omitempty"`
		ExpiryDate  string `json:"expiry_date" validate:"omitempty"`
		IsPackaged  bool   `json:"is_packaged"`
	}

	UploadFoodImageRequest struct {
		FoodItemID string                `json:"food_id" form:"food_id" validate:"required,uuid"`
		Image      *multipart.FileHeader `json:"image" form:"image" validate:"required"`
	}

	UploadReceiptRequest struct {
		ReceiptImage *multipart.FileHeader `json:"receipt_image" form:"receipt_image" validate:"required"`
	}

	UploadReceiptResponse struct {
		ScanID   string `json:"scan_id"`
		ImageURL string `json:"image_url"`
		Status   string `json:"status"`
	}

	ScannedItemRequest struct {
		Name        string `json:"name" validate:"required"`
		Quantity    int    `json:"quantity" validate:"required,min=1"`
		UnitMeasure string `json:"unit_measure" validate:"required"`
		ExpiryDate  string `json:"expiry_date" validate:"required"`
		IsPackaged  bool   `json:"is_packaged"`
	}

	SaveScannedItemsRequest struct {
		ScanID string               `json:"scan_id" validate:"required,uuid"`
		Items  []ScannedItemRequest `json:"items" validate:"required,dive"`
	}

	FoodItemResponse struct {
		ID          string    `json:"id"`
		Name        string    `json:"name"`
		Quantity    int       `json:"quantity"`
		UnitMeasure string    `json:"unit_measure"`
		ExpiryDate  time.Time `json:"expiry_date"`
		IsPackaged  bool      `json:"is_packaged"`
		Status      string    `json:"status"`
		ImageURL    string    `json:"image_url,omitempty"`
		CreatedAt   time.Time `json:"created_at"`
	}

	MarkAsDamagedRequest struct {
		FoodItemID string `json:"food_item_id" validate:"required,uuid"`
	}

	DashboardStatsResponse struct {
		TotalItems       int     `json:"total_items"`
		SafeItems        int     `json:"safe_items"`
		WarningItems     int     `json:"warning_items"`
		ExpiredItems     int     `json:"expired_items"`
		DamagedItems     int     `json:"damaged_items"`
		SavedItems       int     `json:"saved_items"`
		WastedItems      int     `json:"wasted_items"`
		EstimatedSavings float64 `json:"estimated_savings"`
	}

	GeminiResponse struct {
		FoodType        string    `json:"foodType"`
		EstimatedAge    int       `json:"estimatedAgeDays"`
		EstimatedExpiry time.Time `json:"-"` // Diisi secara manual dari expiryDate
		Confidence      float64   `json:"confidenceScore"`
	}
)
