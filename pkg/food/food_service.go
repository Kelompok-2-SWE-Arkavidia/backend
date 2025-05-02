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
		GetReceiptScanResult(ctx context.Context, scanID string, userID string) (map[string]interface{}, error)
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

	mimeType := imageFile.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/jpeg"

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

	jsonPattern := regexp.MustCompile(`(?s)\{.*\}`)
	matches := jsonPattern.FindString(responseText)
	if matches != "" {
		responseText = matches
	}

	responseText = strings.TrimSpace(responseText)
	if strings.HasPrefix(responseText, "```json") {
		responseText = strings.TrimPrefix(responseText, "```json")
		responseText = strings.TrimSuffix(responseText, "```")
	} else if strings.HasPrefix(responseText, "```") {
		responseText = strings.TrimPrefix(responseText, "```")
		responseText = strings.TrimSuffix(responseText, "```")
	}
	responseText = strings.TrimSpace(responseText)

	var foodAnalysis domain.GeminiResponse
	if err := json.Unmarshal([]byte(responseText), &foodAnalysis); err != nil {
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

		expiryDate, dateErr := time.Parse("2006-01-02", altResponse.ExpiryDate)
		if dateErr != nil {
			expiryDate = time.Now().AddDate(0, 0, altResponse.EstimatedAgeDays)
		}

		foodAnalysis = domain.GeminiResponse{
			FoodType:        altResponse.FoodType,
			EstimatedAge:    altResponse.EstimatedAgeDays,
			EstimatedExpiry: expiryDate,
			Confidence:      altResponse.ConfidenceScore,
		}
	}

	if foodAnalysis.FoodType == "" {
		foodAnalysis.FoodType = "Unknown Food"
	}

	if foodAnalysis.EstimatedAge < 0 {
		foodAnalysis.EstimatedAge = 0
	}

	if foodAnalysis.Confidence < 0 || foodAnalysis.Confidence > 1 {
		foodAnalysis.Confidence = 0.5
	}

	zeroTime := time.Time{}
	if foodAnalysis.EstimatedExpiry == zeroTime {
		foodAnalysis.EstimatedExpiry = time.Now().AddDate(0, 0, foodAnalysis.EstimatedAge)
	}

	return foodAnalysis, nil
}

func (s *foodService) UploadReceipt(ctx context.Context, req domain.UploadReceiptRequest, userID string) (domain.UploadReceiptResponse, error) {
	items, err := s.processReceiptWithGemini(ctx, req.ReceiptImage)
	if err != nil {
		if strings.Contains(err.Error(), "failed to parse") && len(items) > 0 {
			log.Printf("Warning: %v", err)
		} else {
			return domain.UploadReceiptResponse{}, fmt.Errorf("error processing receipt with Gemini: %w", err)
		}
	}

	scanID := uuid.New()

	return domain.UploadReceiptResponse{
		ScanID: scanID.String(),
		Status: "Processed",
		Items:  items,
	}, nil
}

func (s *foodService) processReceiptWithGemini(ctx context.Context, receiptImage *multipart.FileHeader) ([]map[string]interface{}, error) {
	file, err := receiptImage.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	fileData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	base64Image := base64.StdEncoding.EncodeToString(fileData)

	geminiAPIKey := utils.GetConfig("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		return nil, errors.New("GEMINI_API_KEY not configured")
	}

	geminiModel := utils.GetConfig("GEMINI_MODEL")
	if geminiModel == "" {
		geminiModel = "gemini-pro-vision"
	}

	mimeType := receiptImage.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/jpeg"

		filename := receiptImage.Filename
		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".png":
			mimeType = "image/png"
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".webp":
			mimeType = "image/webp"
		}
	}

	geminiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", geminiModel, geminiAPIKey)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						// Modify the prompt in the Gemini API request:
						"text": "You are an expert in analyzing receipt images. This is a grocery or food receipt. Extract the food items and their details.\n\n" +
							"Return your analysis as a valid, well-formed JSON array, where each object has these fields:\n" +
							"- name: the food item name (string)\n" +
							"- price: the price shown on receipt (string)\n" +
							"- estimated_age: typical shelf life in days (number)\n" +
							"- expiry_date: calculated expiry date based on today (YYYY-MM-DD format)\n" +
							"- unit_measure: the most likely unit (string - e.g., 'kg', 'pcs')\n" +
							"- is_packaged: whether it's packaged (boolean)\n" +
							"- category: food category (string)\n" +
							"- confidence: your confidence (number between 0-1)\n\n" +
							"IMPORTANT: Your response must be ONLY the valid JSON array - do not include any explanations, notes, or markdown formatting. Make sure your JSON is properly closed with brackets and is syntactically valid.",
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
			"temperature":     0.1,
			"topP":            0.8,
			"topK":            40,
			"maxOutputTokens": 1024,
		},
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	httpClient := &http.Client{Timeout: 60 * time.Second} // Longer timeout for OCR
	req, err := http.NewRequestWithContext(ctx, "POST", geminiURL, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling Gemini API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini API error: %s - %s", resp.Status, string(bodyBytes))
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
		return nil, fmt.Errorf("error decoding Gemini response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, errors.New("no content in Gemini response")
	}

	responseText := geminiResp.Candidates[0].Content.Parts[0].Text

	jsonPattern := regexp.MustCompile(`(?s)\[\s*\{.*\}\s*\]`)
	matches := jsonPattern.FindString(responseText)
	if matches != "" {
		responseText = matches
	}

	responseText = strings.TrimSpace(responseText)
	if strings.HasPrefix(responseText, "```json") {
		responseText = strings.TrimPrefix(responseText, "```json")
		responseText = strings.TrimSuffix(responseText, "```")
	} else if strings.HasPrefix(responseText, "```") {
		responseText = strings.TrimPrefix(responseText, "```")
		responseText = strings.TrimSuffix(responseText, "```")
	}
	responseText = strings.TrimSpace(responseText)

	if strings.HasPrefix(responseText, "[") && !strings.HasSuffix(responseText, "]") {
		lastBraceIndex := strings.LastIndex(responseText, "}")
		if lastBraceIndex > 0 {
			responseText = responseText[:lastBraceIndex+1] + "\n]"
		}
	}

	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(responseText), &items); err != nil {
		items = []map[string]interface{}{}

		objPattern := regexp.MustCompile(`\{[^{}]*\}`)
		objMatches := objPattern.FindAllString(responseText, -1)

		for _, objStr := range objMatches {
			var item map[string]interface{}
			if jsonErr := json.Unmarshal([]byte(objStr), &item); jsonErr == nil {
				items = append(items, item)
			}
		}

		if len(items) == 0 {
			return nil, fmt.Errorf("failed to parse Gemini JSON response: %w, raw response: %s", err, responseText)
		}
	}

	now := time.Now()
	for i, item := range items {
		if _, ok := item["name"]; !ok {
			item["name"] = "Unknown Item"
		}

		if _, ok := item["estimated_age"]; !ok {
			item["estimated_age"] = 7
		}

		if _, ok := item["expiry_date"]; !ok {
			estimatedAge, ok := item["estimated_age"].(float64)
			if !ok {
				estimatedAge = 7
			}

			expiryDate := now.AddDate(0, 0, int(estimatedAge))
			item["expiry_date"] = expiryDate.Format("2006-01-02")
		}

		if _, ok := item["unit_measure"]; !ok {
			item["unit_measure"] = "pcs"
		}

		if _, ok := item["is_packaged"]; !ok {
			item["is_packaged"] = true
		}

		if _, ok := item["confidence"]; !ok {
			item["confidence"] = 0.7
		}

		if _, ok := item["quantity"]; !ok {
			item["quantity"] = 1
		}

		items[i] = item
	}

	return items, nil
}

func (s *foodService) getFoodAgeEstimationFromGemini(ctx context.Context, foodName string) (domain.GeminiResponse, error) {
	geminiAPIKey := utils.GetConfig("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		return domain.GeminiResponse{}, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	geminiModel := utils.GetConfig("GEMINI_MODEL")
	if geminiModel == "" {
		return domain.GeminiResponse{}, fmt.Errorf("GEMINI_MODEL environment variable not set")
	}

	geminiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", geminiModel, geminiAPIKey)

	prompt := fmt.Sprintf(
		"Analyze the food item '%s' and respond ONLY with a valid JSON object containing exactly these fields: "+
			"'foodType' (corrected/more descriptive name of the food), "+
			"'estimatedAgeDays' (likely shelf life of this food in days), "+
			"'expiryDate' (estimated expiry date as string in YYYY-MM-DD format based on average shelf life), "+
			"'confidenceScore' (number between 0 and 1 indicating your confidence). "+
			"Do not include any explanations, just the JSON.",
		foodName)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"text": prompt,
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

	jsonPattern := regexp.MustCompile(`(?s)\{.*\}`)
	matches := jsonPattern.FindString(responseText)
	if matches != "" {
		responseText = matches
	}

	responseText = strings.TrimSpace(responseText)
	if strings.HasPrefix(responseText, "```json") {
		responseText = strings.TrimPrefix(responseText, "```json")
		responseText = strings.TrimSuffix(responseText, "```")
	} else if strings.HasPrefix(responseText, "```") {
		responseText = strings.TrimPrefix(responseText, "```")
		responseText = strings.TrimSuffix(responseText, "```")
	}
	responseText = strings.TrimSpace(responseText)

	type FoodAnalysisResponse struct {
		FoodType         string  `json:"foodType"`
		EstimatedAgeDays int     `json:"estimatedAgeDays"`
		ExpiryDate       string  `json:"expiryDate"`
		ConfidenceScore  float64 `json:"confidenceScore"`
	}

	var foodAnalysisResp FoodAnalysisResponse
	if err := json.Unmarshal([]byte(responseText), &foodAnalysisResp); err != nil {
		return domain.GeminiResponse{}, fmt.Errorf("failed to parse Gemini response: %v - Raw response: %s", err, responseText)
	}

	expiryDate, dateErr := time.Parse("2006-01-02", foodAnalysisResp.ExpiryDate)
	if dateErr != nil {
		expiryDate = time.Now().AddDate(0, 0, foodAnalysisResp.EstimatedAgeDays)
	}

	return domain.GeminiResponse{
		FoodType:        foodAnalysisResp.FoodType,
		EstimatedAge:    foodAnalysisResp.EstimatedAgeDays,
		EstimatedExpiry: expiryDate,
		Confidence:      foodAnalysisResp.ConfidenceScore,
	}, nil
}

func (s *foodService) GetReceiptScanResult(ctx context.Context, scanID string, userID string) (map[string]interface{}, error) {
	scan, err := s.foodRepository.GetReceiptScanByID(ctx, scanID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrInvalidReceiptScan
		}
		return nil, err
	}

	if scan.UserID.String() != userID {
		return nil, domain.ErrUnauthorizedAccess
	}

	result := map[string]interface{}{
		"id":         scan.ID.String(),
		"image_url":  scan.ImageURL,
		"status":     scan.Status,
		"created_at": scan.CreatedAt,
	}

	if scan.Status == "Processed" && scan.OcrResults != "" {
		var items []map[string]interface{}
		if err := json.Unmarshal([]byte(scan.OcrResults), &items); err != nil {
			var singleItem map[string]interface{}
			if err := json.Unmarshal([]byte(scan.OcrResults), &singleItem); err != nil {
				result["items"] = []interface{}{}
				result["error"] = "Failed to parse OCR results"
			} else {
				result["items"] = []map[string]interface{}{singleItem}
			}
		} else {
			result["items"] = items
		}
	} else if scan.Status == "Failed" {
		result["error"] = scan.OcrResults
		result["items"] = []interface{}{}
	} else {
		result["items"] = []interface{}{}
	}

	return result, nil
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
		// Parse expiry date from string
		expiryDate, err := time.Parse("2006-01-02", item.ExpiryDate)
		if err != nil {
			return domain.ErrInvalidExpiryDate
		}

		// Determine food status based on expiry date
		status := determineStatus(expiryDate)

		// Create food item record
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

		// Save food item to database
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
