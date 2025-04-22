package food

import (
	"Go-Starter-Template/domain"
	"Go-Starter-Template/entities"
	"Go-Starter-Template/internal/utils"
	"Go-Starter-Template/internal/utils/storage"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
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

		// New methods
		DetectFoodAge(ctx context.Context, imageFile *multipart.FileHeader) (domain.GeminiResponse, error)
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
		UnitMeasure:   req.UnitMeasure, // Added unit measure
		ExpiryDate:    expiryDate,
		IsPackaged:    req.IsPackaged,
		Status:        status,
		AddedManually: true,
	}

	if err := s.foodRepository.AddFoodItem(ctx, foodItem); err != nil {
		return domain.AddFoodItemResponse{}, err
	}

	return domain.AddFoodItemResponse{
		ID:          foodItem.ID.String(),
		Name:        foodItem.Name,
		Quantity:    foodItem.Quantity,
		UnitMeasure: foodItem.UnitMeasure, // Added unit measure
		ExpiryDate:  foodItem.ExpiryDate,
		IsPackaged:  foodItem.IsPackaged,
		Status:      foodItem.Status,
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

	if req.UnitMeasure != "" {
		foodItem.UnitMeasure = req.UnitMeasure
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
			ID:          item.ID.String(),
			Name:        item.Name,
			Quantity:    item.Quantity,
			UnitMeasure: item.UnitMeasure, // Added unit measure
			ExpiryDate:  item.ExpiryDate,
			IsPackaged:  item.IsPackaged,
			Status:      item.Status,
			ImageURL:    item.ImageURL,
			CreatedAt:   item.CreatedAt,
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
		ID:          foodItem.ID.String(),
		Name:        foodItem.Name,
		Quantity:    foodItem.Quantity,
		UnitMeasure: foodItem.UnitMeasure, // Added unit measure
		ExpiryDate:  foodItem.ExpiryDate,
		IsPackaged:  foodItem.IsPackaged,
		Status:      foodItem.Status,
		ImageURL:    foodItem.ImageURL,
		CreatedAt:   foodItem.CreatedAt,
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

	if foodItem.UserID.String() != userID {
		return domain.ErrUnauthorizedAccess
	}

	fileName := fmt.Sprintf("food-item-%s", foodItem.ID.String())
	var objectKey string
	var uploadErr error

	if foodItem.ImageURL != "" {
		existingKey := s.s3.GetObjectKeyFromLink(foodItem.ImageURL)
		if existingKey != "" {
			objectKey, uploadErr = s.s3.UpdateFile(existingKey, req.Image, storage.AllowImage...)
		} else {
			objectKey, uploadErr = s.s3.UploadFile(fileName, req.Image, "food-items", storage.AllowImage...)
		}
	} else {
		objectKey, uploadErr = s.s3.UploadFile(fileName, req.Image, "food-items", storage.AllowImage...)
	}

	if uploadErr != nil {
		return uploadErr
	}

	foodItem.ImageURL = s.s3.GetPublicLinkKey(objectKey)

	geminiResponse, err := s.DetectFoodAge(ctx, req.Image)
	if err != nil {
		fmt.Printf("Error analyzing food image with Gemini: %v\n", err)
	} else {
		foodItem.Name = geminiResponse.FoodType
		foodItem.ExpiryDate = geminiResponse.EstimatedExpiry
		foodItem.Status = determineStatus(geminiResponse.EstimatedExpiry)
	}

	return s.foodRepository.UpdateFoodItem(ctx, foodItem)
}

func (s *foodService) DetectFoodAge(ctx context.Context, imageFile *multipart.FileHeader) (domain.GeminiResponse, error) {
	file, err := imageFile.Open()
	if err != nil {
		return domain.GeminiResponse{}, err
	}
	defer file.Close()

	fileData, err := io.ReadAll(file)
	if err != nil {
		return domain.GeminiResponse{}, err
	}

	base64Image := base64.StdEncoding.EncodeToString(fileData)

	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		return domain.GeminiResponse{}, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	geminiURL := "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro-vision:generateContent?key=" + geminiAPIKey

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"text": "Analyze this food image and provide the following information in JSON format: 1) food type, 2) estimated age in days, 3) estimated expiry date (YYYY-MM-DD), 4) confidence score between 0-1. Only return the JSON response with no additional text.",
					},
					{
						"inline_data": map[string]interface{}{
							"mime_type": imageFile.Header.Get("Content-Type"),
							"data":      base64Image,
						},
					},
				},
			},
		},
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return domain.GeminiResponse{}, err
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", geminiURL, bytes.NewBuffer(requestJSON))
	if err != nil {
		return domain.GeminiResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return domain.GeminiResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return domain.GeminiResponse{}, fmt.Errorf("gemini API error: %s - %s", resp.Status, string(bodyBytes))
	}

	// Parse the response
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return domain.GeminiResponse{}, err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return domain.GeminiResponse{}, domain.ErrGeminiProcessingFailed
	}

	// Extract JSON from text response
	responseText := geminiResp.Candidates[0].Content.Parts[0].Text
	var foodAnalysis domain.GeminiResponse

	// Strip any markdown code blocks if present
	jsonStr := responseText
	if len(jsonStr) >= 7 && jsonStr[:7] == "```json" {
		jsonStr = jsonStr[7:]
		if endIdx := strings.LastIndex(jsonStr, "```"); endIdx >= 0 {
			jsonStr = jsonStr[:endIdx]
		}
	}

	if err := json.Unmarshal([]byte(jsonStr), &foodAnalysis); err != nil {
		return domain.GeminiResponse{}, err
	}

	return foodAnalysis, nil
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

	// Process receipt asynchronously
	go func() {
		// Get the AI model URL from config
		aiModelURL := utils.GetConfig("AI_MODEL_URL")
		if aiModelURL == "" {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = "Error: AI Model URL not configured"
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		// Open the file to send to AI service
		file, err := req.ReceiptImage.Open()
		if err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error opening file: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}
		defer file.Close()

		// Create a new buffer to store file contents
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error reading file: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		// Create a new multipart writer
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Create a form file
		part, err := writer.CreateFormFile("image", req.ReceiptImage.Filename)
		if err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error creating form file: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		// Write file content to form file
		if _, err = part.Write(fileBytes); err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error writing to form file: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		// Close the multipart writer
		if err = writer.Close(); err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error closing writer: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		// Create HTTP request to AI model service
		httpReq, err := http.NewRequest("POST", aiModelURL, body)
		if err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error creating request: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		// Set headers
		httpReq.Header.Set("Content-Type", writer.FormDataContentType())

		// Send request to AI model service
		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error sending request to AI model: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}
		defer resp.Body.Close()

		// Check status code
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("AI model error: %s - %s", resp.Status, string(bodyBytes))
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		// Parse response from AI model
		var aiResponse struct {
			Success bool `json:"success"`
			Items   []struct {
				Name        string `json:"name"`
				Quantity    int    `json:"quantity"`
				UnitMeasure string `json:"unit_measure"`
				ExpiryDate  string `json:"expiry_date"`
				IsPackaged  bool   `json:"is_packaged"`
			} `json:"items"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&aiResponse); err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error parsing AI response: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		if !aiResponse.Success || len(aiResponse.Items) == 0 {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = "AI model couldn't extract any items from receipt"
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		// Save OCR results as JSON string
		resultsJSON, _ := json.Marshal(aiResponse.Items)
		receiptScan.Status = "Processed"
		receiptScan.OcrResults = string(resultsJSON)

		// Update receipt scan status and results
		if err := s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan); err != nil {
			log.Printf("Error updating receipt scan: %v", err)
			return
		}
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
			UnitMeasure:   item.UnitMeasure,
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

	// Update receipt scan status to completed
	scan.Status = "Completed"
	if err := s.foodRepository.UpdateReceiptScan(ctx, scan); err != nil {
		return err
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
