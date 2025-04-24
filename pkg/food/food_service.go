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
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
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
	expiryDate, err := time.Parse("2006-01-02", req.ExpiryDate)
	if err != nil {
		return domain.AddFoodItemResponse{}, domain.ErrInvalidExpiryDate
	}

	if req.Quantity <= 0 {
		return domain.AddFoodItemResponse{}, domain.ErrInvalidQuantity
	}

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
		UnitMeasure:   req.UnitMeasure,
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
		UnitMeasure: foodItem.UnitMeasure,
		ExpiryDate:  foodItem.ExpiryDate,
		IsPackaged:  foodItem.IsPackaged,
		Status:      foodItem.Status,
	}, nil
}

func (s *foodService) UpdateFoodItem(ctx context.Context, id string, req domain.UpdateFoodItemRequest, userID string) error {
	foodItem, err := s.foodRepository.GetFoodItemByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrFoodItemNotFound
		}
		return err
	}

	if foodItem.UserID.String() != userID {
		return domain.ErrUnauthorizedAccess
	}

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

		foodItem.Status = determineStatus(expiryDate)
	}

	foodItem.IsPackaged = req.IsPackaged

	return s.foodRepository.UpdateFoodItem(ctx, foodItem)
}

func (s *foodService) DeleteFoodItem(ctx context.Context, id string, userID string) error {
	foodItem, err := s.foodRepository.GetFoodItemByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrFoodItemNotFound
		}
		return err
	}

	if foodItem.UserID.String() != userID {
		return domain.ErrUnauthorizedAccess
	}

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
			UnitMeasure: item.UnitMeasure,
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.FoodItemResponse{}, domain.ErrFoodItemNotFound
		}
		return domain.FoodItemResponse{}, err
	}

	if foodItem.UserID.String() != userID {
		return domain.FoodItemResponse{}, domain.ErrUnauthorizedAccess
	}

	return domain.FoodItemResponse{
		ID:          foodItem.ID.String(),
		Name:        foodItem.Name,
		Quantity:    foodItem.Quantity,
		UnitMeasure: foodItem.UnitMeasure,
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
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

	geminiAPIKey := utils.GetConfig("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		return domain.GeminiResponse{}, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	geminiModel := utils.GetConfig("GEMINI_MODEL")
	if geminiModel == "" {
		return domain.GeminiResponse{}, fmt.Errorf("GEMINI_MODEL environment variable not set")
	}

	// Tentukan MIME type yang benar
	mimeType := imageFile.Header.Get("Content-Type")
	if mimeType == "" {
		// Default ke image/jpeg jika Content-Type tidak ada
		mimeType = "image/jpeg"

		// Atau coba tentukan berdasarkan ekstensi file
		filename := imageFile.Filename
		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".png":
			mimeType = "image/png"
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".gif":
			mimeType = "image/gif"
		case ".webp":
			mimeType = "image/webp"
		}
	}

	geminiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", geminiModel, geminiAPIKey)

	// Prompt yang lebih spesifik dan meminta format JSON yang ketat
	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"text": "Analyze this food image and respond ONLY with a valid JSON object containing exactly these fields: 'foodType' (string), 'estimatedAgeDays' (number), 'expiryDate' (string in YYYY-MM-DD format), and 'confidenceScore' (number between 0 and 1). Do not include any explanations, markdown formatting, or extra text.",
					},
					{
						"inline_data": map[string]interface{}{
							"mime_type": mimeType,
							"data":      base64Image,
						},
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.1,
			"topP":        0.8,
			"topK":        40,
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

	responseText := geminiResp.Candidates[0].Content.Parts[0].Text

	// Log response untuk debugging
	fmt.Println("Gemini raw response:", responseText)

	// Ekstrak JSON dari teks respons (jika dalam markdown atau komentar)
	jsonPattern := regexp.MustCompile(`(?s)\{.*\}`)
	matches := jsonPattern.FindString(responseText)
	if matches != "" {
		responseText = matches
	}

	// Pembersihan tambahan untuk format JSON yang tidak standar
	responseText = strings.TrimSpace(responseText)
	if strings.HasPrefix(responseText, "```json") {
		responseText = strings.TrimPrefix(responseText, "```json")
		responseText = strings.TrimSuffix(responseText, "```")
	} else if strings.HasPrefix(responseText, "```") {
		responseText = strings.TrimPrefix(responseText, "```")
		responseText = strings.TrimSuffix(responseText, "```")
	}
	responseText = strings.TrimSpace(responseText)

	// Coba parsing ke struct yang diharapkan
	var foodAnalysis domain.GeminiResponse
	if err := json.Unmarshal([]byte(responseText), &foodAnalysis); err != nil {
		// Jika gagal, coba struktur alternatif
		type AlternativeResponse struct {
			FoodType         string  `json:"foodType"`
			EstimatedAgeDays int     `json:"estimatedAgeDays"`
			ExpiryDate       string  `json:"expiryDate"`
			ConfidenceScore  float64 `json:"confidenceScore"`
		}

		var altResponse AlternativeResponse
		if altErr := json.Unmarshal([]byte(responseText), &altResponse); altErr != nil {
			return domain.GeminiResponse{}, fmt.Errorf("failed to parse Gemini response: %v - Raw response: %s", err, responseText)
		}

		// Parse tanggal kedaluwarsa
		expiryDate, dateErr := time.Parse("2006-01-02", altResponse.ExpiryDate)
		if dateErr != nil {
			// Jika format tanggal tidak sesuai, gunakan estimasi hari
			expiryDate = time.Now().AddDate(0, 0, altResponse.EstimatedAgeDays)
		}

		// Konversi ke domain.GeminiResponse
		foodAnalysis = domain.GeminiResponse{
			FoodType:        altResponse.FoodType,
			EstimatedAge:    altResponse.EstimatedAgeDays,
			EstimatedExpiry: expiryDate,
			Confidence:      altResponse.ConfidenceScore,
		}
	}

	// Pastikan nilai-nilai masuk akal
	if foodAnalysis.FoodType == "" {
		foodAnalysis.FoodType = "Unknown Food"
	}

	if foodAnalysis.EstimatedAge < 0 {
		foodAnalysis.EstimatedAge = 0
	}

	if foodAnalysis.Confidence < 0 || foodAnalysis.Confidence > 1 {
		foodAnalysis.Confidence = 0.5 // default middle value
	}

	// Jika tanggal kedaluwarsa nol, buat perkiraan berdasarkan umur
	zeroTime := time.Time{}
	if foodAnalysis.EstimatedExpiry == zeroTime {
		foodAnalysis.EstimatedExpiry = time.Now().AddDate(0, 0, foodAnalysis.EstimatedAge)
	}

	return foodAnalysis, nil
}

func (s *foodService) UploadReceipt(ctx context.Context, req domain.UploadReceiptRequest, userID string) (domain.UploadReceiptResponse, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return domain.UploadReceiptResponse{}, domain.ErrParseUUID
	}

	scanID := uuid.New()
	fileName := fmt.Sprintf("receipt-%s", scanID.String())
	objectKey, err := s.s3.UploadFile(fileName, req.ReceiptImage, "receipts", storage.AllowImage...)
	if err != nil {
		return domain.UploadReceiptResponse{}, err
	}

	imageURL := s.s3.GetPublicLinkKey(objectKey)

	receiptScan := &entities.ReceiptScan{
		ID:       scanID,
		UserID:   userUUID,
		ImageURL: imageURL,
		Status:   "Pending",
	}

	if err := s.foodRepository.CreateReceiptScan(ctx, receiptScan); err != nil {
		_ = s.s3.DeleteFile(objectKey)
		return domain.UploadReceiptResponse{}, err
	}

	go func() {
		aiModelURL := utils.GetConfig("AI_MODEL_URL")
		if aiModelURL == "" {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = "Error: AI Model URL not configured"
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		file, err := req.ReceiptImage.Open()
		if err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error opening file: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error reading file: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreateFormFile("image", req.ReceiptImage.Filename)
		if err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error creating form file: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		if _, err = part.Write(fileBytes); err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error writing to form file: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		if err = writer.Close(); err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error closing writer: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		httpReq, err := http.NewRequest("POST", aiModelURL, body)
		if err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error creating request: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

		httpReq.Header.Set("Content-Type", writer.FormDataContentType())

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("Error sending request to AI model: %s", err.Error())
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			receiptScan.Status = "Failed"
			receiptScan.OcrResults = fmt.Sprintf("AI model error: %s - %s", resp.Status, string(bodyBytes))
			_ = s.foodRepository.UpdateReceiptScan(context.Background(), receiptScan)
			return
		}

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

		resultsJSON, _ := json.Marshal(aiResponse.Items)
		receiptScan.Status = "Processed"
		receiptScan.OcrResults = string(resultsJSON)

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

	scan, err := s.foodRepository.GetReceiptScanByID(ctx, req.ScanID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
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

	scan.Status = "Completed"
	if err := s.foodRepository.UpdateReceiptScan(ctx, scan); err != nil {
		return err
	}

	return nil
}

func (s *foodService) MarkAsDamaged(ctx context.Context, req domain.MarkAsDamagedRequest, userID string) error {
	foodItem, err := s.foodRepository.GetFoodItemByID(ctx, req.FoodItemID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrFoodItemNotFound
		}
		return err
	}

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

	warningThreshold := now.AddDate(0, 0, 3)
	if expiryDate.Before(warningThreshold) {
		return "Warning"
	}

	return "Safe"
}
