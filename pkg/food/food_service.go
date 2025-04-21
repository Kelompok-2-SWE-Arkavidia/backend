package food

import (
	"Go-Starter-Template/domain"
	"Go-Starter-Template/entities"
	"Go-Starter-Template/internal/utils/storage"
	"context"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type (
	FoodService interface {
		AddFoodItem(ctx context.Context, req domain.AddFoodItemRequest, userID string) (domain.AddFoodItemResponse, error)
		UpdateFoodItem(ctx context.Context, id string, req domain.UpdateFoodItemRequest, userID string) error
		DeleteFoodItem(ctx context.Context, id string, userID string) error
		GetFoodItems(ctx context.Context, userID string, status string, page, limit int) ([]domain.FoodItemResponse, int64, error)
		GetFoodItemByID(ctx context.Context, id string, userID string) (domain.FoodItemResponse, error)
		UploadFoodImage(ctx context.Context, req domain.UploadFoodImageRequest, userID string) error
		UploadReceipt(ctx context.Context, req domain.UploadReceiptRequest, userID string) (domain.UploadReceiptResponse, error)
		SaveScannedItems(ctx context.Context, req domain.SaveScannedItemsRequest, userID string) error
		MarkAsDamaged(ctx context.Context, req domain.MarkAsDamagedRequest, userID string) error
		GetDashboardStats(ctx context.Context, userID string) (domain.DashboardStatsResponse, error)

		// OCR service would be here in a real implementation
		ProcessReceiptOCR(receiptURL string) (map[string]interface{}, error)
	}

	foodService struct {
		foodRepository FoodRepository
		s3             storage.AwsS3
	}
)

func NewFoodService(foodRepository FoodRepository, s3 storage.AwsS3) FoodService {
	return &foodService{
		foodRepository: foodRepository,
		s3:             s3,
	}
}

func (s *foodService) AddFoodItem(ctx context.Context, req domain.AddFoodItemRequest, userID string) (domain.AddFoodItemResponse, error) {
	// Parse expiry date
	expiryDate, err := time.Parse("2006-01-02", req.ExpiryDate)
	if err != nil {
		return domain.AddFoodItemResponse{}, domain.ErrInvalidExpiryDate
	}

	if req.Quantity <= 0 {
		return domain.AddFoodItemResponse{}, domain.ErrInvalidQuantity
	}

	// Determine status based on expiry date
	status := determineStatus(expiryDate)

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return domain.AddFoodItemResponse{}, domain.ErrParseUUID
	}

	foodItem := &entities.FoodItem{
		ID:            uuid.New(),
		UserID:        userUUID,
		Name:          req.Name,
		Quantity:      req.Quantity,
		ExpiryDate:    expiryDate,
		IsPackaged:    req.IsPackaged,
		Status:        status,
		AddedManually: true,
	}

	if err := s.foodRepository.AddFoodItem(ctx, foodItem); err != nil {
		return domain.AddFoodItemResponse{}, err
	}

	return domain.AddFoodItemResponse{
		ID:         foodItem.ID.String(),
		Name:       foodItem.Name,
		Quantity:   foodItem.Quantity,
		ExpiryDate: foodItem.ExpiryDate,
		IsPackaged: foodItem.IsPackaged,
		Status:     foodItem.Status,
	}, nil
}

func (s *foodService) UpdateFoodItem(ctx context.Context, id string, req domain.UpdateFoodItemRequest, userID string) error {
	foodItem, err := s.foodRepository.GetFoodItemByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.ErrFoodItemNotFound
		}
		return err
	}

	// Verify ownership
	if foodItem.UserID.String() != userID {
		return domain.ErrUnauthorizedAccess
	}

	// Update fields if provided
	if req.Name != "" {
		foodItem.Name = req.Name
	}

	if req.Quantity > 0 {
		foodItem.Quantity = req.Quantity
	}

	if req.ExpiryDate != "" {
		expiryDate, err := time.Parse("2006-01-02", req.ExpiryDate)
		if err != nil {
			return domain.ErrInvalidExpiryDate
		}
		foodItem.ExpiryDate = expiryDate

		// Recalculate status based on new expiry date
		foodItem.Status = determineStatus(expiryDate)
	}

	foodItem.IsPackaged = req.IsPackaged

	return s.foodRepository.UpdateFoodItem(ctx, foodItem)
}

func (s *foodService) DeleteFoodItem(ctx context.Context, id string, userID string) error {
	foodItem, err := s.foodRepository.GetFoodItemByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.ErrFoodItemNotFound
		}
		return err
	}

	// Verify ownership
	if foodItem.UserID.String() != userID {
		return domain.ErrUnauthorizedAccess
	}

	// Delete associated image from S3 if it exists
	if foodItem.ImageURL != "" {
		objectKey := s.s3.GetObjectKeyFromLink(foodItem.ImageURL)
		if objectKey != "" {
			_ = s.s3.DeleteFile(objectKey)
		}
	}

	return s.foodRepository.DeleteFoodItem(ctx, id)
}

func (s *foodService) GetFoodItems(ctx context.Context, userID string, status string, page, limit int) ([]domain.FoodItemResponse, int64, error) {
	foodItems, count, err := s.foodRepository.GetFoodItems(ctx, userID, status, page, limit)
	if err != nil {
		return nil, 0, err
	}

	var response []domain.FoodItemResponse
	for _, item := range foodItems {
		response = append(response, domain.FoodItemResponse{
			ID:         item.ID.String(),
			Name:       item.Name,
			Quantity:   item.Quantity,
			ExpiryDate: item.ExpiryDate,
			IsPackaged: item.IsPackaged,
			Status:     item.Status,
			ImageURL:   item.ImageURL,
			CreatedAt:  item.CreatedAt,
		})
	}

	return response, count, nil
}

func (s *foodService) GetFoodItemByID(ctx context.Context, id string, userID string) (domain.FoodItemResponse, error) {
	foodItem, err := s.foodRepository.GetFoodItemByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.FoodItemResponse{}, domain.ErrFoodItemNotFound
		}
		return domain.FoodItemResponse{}, err
	}

	// Verify ownership
	if foodItem.UserID.String() != userID {
		return domain.FoodItemResponse{}, domain.ErrUnauthorizedAccess
	}

	return domain.FoodItemResponse{
		ID:         foodItem.ID.String(),
		Name:       foodItem.Name,
		Quantity:   foodItem.Quantity,
		ExpiryDate: foodItem.ExpiryDate,
		IsPackaged: foodItem.IsPackaged,
		Status:     foodItem.Status,
		ImageURL:   foodItem.ImageURL,
		CreatedAt:  foodItem.CreatedAt,
	}, nil
}

func (s *foodService) UploadFoodImage(ctx context.Context, req domain.UploadFoodImageRequest, userID string) error {
	foodItem, err := s.foodRepository.GetFoodItemByID(ctx, req.FoodItemID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.ErrFoodItemNotFound
		}
		return err
	}

	// Verify ownership
	if foodItem.UserID.String() != userID {
		return domain.ErrUnauthorizedAccess
	}

	// Upload image to S3
	fileName := fmt.Sprintf("food-item-%s", foodItem.ID.String())
	var objectKey string
	var uploadErr error

	if foodItem.ImageURL != "" {
		// Update existing image
		existingKey := s.s3.GetObjectKeyFromLink(foodItem.ImageURL)
		if existingKey != "" {
			objectKey, uploadErr = s.s3.UpdateFile(existingKey, req.Image, storage.AllowImage...)
		} else {
			objectKey, uploadErr = s.s3.UploadFile(fileName, req.Image, "food-items", storage.AllowImage...)
		}
	} else {
		// Upload new image
		objectKey, uploadErr = s.s3.UploadFile(fileName, req.Image, "food-items", storage.AllowImage...)
	}

	if uploadErr != nil {
		return uploadErr
	}

	// Update food item with image URL
	foodItem.ImageURL = s.s3.GetPublicLinkKey(objectKey)

	return s.foodRepository.UpdateFoodItem(ctx, foodItem)
}

func (s *foodService) UploadReceipt(ctx context.Context, req domain.UploadReceiptRequest, userID string) (domain.UploadReceiptResponse, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return domain.UploadReceiptResponse{}, domain.ErrParseUUID
	}

	// Upload receipt image to S3
	scanID := uuid.New()
	fileName := fmt.Sprintf("receipt-%s", scanID.String())
	objectKey, err := s.s3.UploadFile(fileName, req.ReceiptImage, "receipts", storage.AllowImage...)
	if err != nil {
		return domain.UploadReceiptResponse{}, err
	}

	imageURL := s.s3.GetPublicLinkKey(objectKey)

	// Create receipt scan record
	receiptScan := &entities.ReceiptScan{
		ID:       scanID,
		UserID:   userUUID,
		ImageURL: imageURL,
		Status:   "Pending",
	}

	if err := s.foodRepository.CreateReceiptScan(ctx, receiptScan); err != nil {
		// Clean up the uploaded image if there's an error
		_ = s.s3.DeleteFile(objectKey)
		return domain.UploadReceiptResponse{}, err
	}

	// Process the receipt with OCR (in a real implementation, this might be async)
	go func() {
		results, err := s.ProcessReceiptOCR(imageURL)
		if err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error: %s", err.Error())
		} else {
			receiptScan.Status = "Processed"
			receiptScan.OcrResults = fmt.Sprintf("%v", results)
		}
		_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
	}()

	return domain.UploadReceiptResponse{
		ScanID:   scanID.String(),
		ImageURL: imageURL,
		Status:   "Pending",
	}, nil
}

func (s *foodService) SaveScannedItems(ctx context.Context, req domain.SaveScannedItemsRequest, userID string) error {
	scanUUID, err := uuid.Parse(req.ScanID)
	if err != nil {
		return domain.ErrParseUUID
	}

	// Verify scan exists and belongs to user
	scan, err := s.foodRepository.GetReceiptScanByID(ctx, req.ScanID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.ErrInvalidReceiptScan
		}
		return err
	}

	if scan.UserID.String() != userID {
		return domain.ErrUnauthorizedAccess
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return domain.ErrParseUUID
	}

	// Add each food item
	for _, item := range req.Items {
		expiryDate, err := time.Parse("2006-01-02", item.ExpiryDate)
		if err != nil {
			return domain.ErrInvalidExpiryDate
		}

		status := determineStatus(expiryDate)

		scanIDStr := scanUUID.String()
		foodItem := &entities.FoodItem{
			ID:            uuid.New(),
			UserID:        userUUID,
			Name:          item.Name,
			Quantity:      item.Quantity,
			ExpiryDate:    expiryDate,
			IsPackaged:    item.IsPackaged,
			Status:        status,
			AddedManually: false,
			ReceiptScanID: &scanIDStr,
		}

		if err := s.foodRepository.AddFoodItem(ctx, foodItem); err != nil {
			return err
		}
	}

	return nil
}

func (s *foodService) MarkAsDamaged(ctx context.Context, req domain.MarkAsDamagedRequest, userID string) error {
	foodItem, err := s.foodRepository.GetFoodItemByID(ctx, req.FoodItemID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.ErrFoodItemNotFound
		}
		return err
	}

	// Verify ownership
	if foodItem.UserID.String() != userID {
		return domain.ErrUnauthorizedAccess
	}

	return s.foodRepository.MarkFoodItemAsDamaged(ctx, req.FoodItemID)
}

func (s *foodService) GetDashboardStats(ctx context.Context, userID string) (domain.DashboardStatsResponse, error) {
	stats, err := s.foodRepository.GetDashboardStats(ctx, userID)
	if err != nil {
		return domain.DashboardStatsResponse{}, err
	}

	return domain.DashboardStatsResponse{
		TotalItems:       int(stats["total_items"].(int64)),
		SafeItems:        int(stats["safe_items"].(int64)),
		WarningItems:     int(stats["warning_items"].(int64)),
		ExpiredItems:     int(stats["expired_items"].(int64)),
		DamagedItems:     int(stats["damaged_items"].(int64)),
		SavedItems:       int(stats["saved_items"].(int64)),
		WastedItems:      int(stats["wasted_items"].(int64)),
		EstimatedSavings: stats["estimated_savings"].(float64),
	}, nil
}

// Mock implementation for OCR service
func (s *foodService) ProcessReceiptOCR(receiptURL string) (map[string]interface{}, error) {
	// In a real implementation, this would call an OCR service
	// For now, just return a mock result
	return map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"name":     "Milk",
				"quantity": 1,
				"price":    15000,
			},
			{
				"name":     "Bread",
				"quantity": 2,
				"price":    12000,
			},
		},
		"total": 39000,
	}, nil
}

// Helper function to determine food status based on expiry date
func determineStatus(expiryDate time.Time) string {
	now := time.Now()

	if expiryDate.Before(now) {
		return "Expired"
	}

	// Warning if within 3 days of expiry
	warningThreshold := now.AddDate(0, 0, 3)
	if expiryDate.Before(warningThreshold) {
		return "Warning"
	}

	return "Safe"
}

// Helper function to autocomplete expiry date for fresh produce
func estimateExpiryDate(productName string) time.Time {
	// This would use a database or mapping of common fresh produce and their shelf life
	// For simplicity, we're using a basic approach here
	now := time.Now()

	// Default is 7 days
	shelfLife := 7

	// Sample estimations for common items
	switch productName {
	case "Spinach", "Lettuce", "Leafy Greens":
		shelfLife = 3
	case "Tomato", "Bell Pepper":
		shelfLife = 5
	case "Apple", "Orange":
		shelfLife = 14
	case "Banana":
		shelfLife = 5
	case "Broccoli", "Cauliflower":
		shelfLife = 7
	case "Carrot", "Potato":
		shelfLife = 21
	}

	return now.AddDate(0, 0, shelfLife)
}
