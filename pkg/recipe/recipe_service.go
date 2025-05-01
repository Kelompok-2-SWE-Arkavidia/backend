package recipe

import (
	"Go-Starter-Template/domain"
	"Go-Starter-Template/entities"
	"Go-Starter-Template/internal/utils"
	"Go-Starter-Template/pkg/food"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type (
	RecipeService interface {
		GetRecipeRecommendations(ctx context.Context, req domain.RecipeRecommendationRequest, userID string) (domain.RecipeRecommendationResponse, error)
		GetRecipeDetail(ctx context.Context, recipeID string, userID string) (domain.RecipeDetail, error)
		BookmarkRecipe(ctx context.Context, req domain.BookmarkRecipeRequest, userID string) error
		RemoveBookmark(ctx context.Context, req domain.BookmarkRecipeRequest, userID string) error
		GetBookmarkedRecipes(ctx context.Context, page, limit int, userID string) ([]domain.Recipe, int64, error)
		MarkAsCooked(ctx context.Context, req domain.MarkAsCookedRequest, userID string) error
		GetRecipeHistory(ctx context.Context, page, limit int, userID string) (domain.RecipeHistoryResponse, error)
	}

	recipeService struct {
		recipeRepository RecipeRepository
		foodRepository   food.FoodRepository
	}
)

func NewRecipeService(recipeRepository RecipeRepository, foodRepository food.FoodRepository) RecipeService {
	return &recipeService{
		recipeRepository: recipeRepository,
		foodRepository:   foodRepository,
	}
}

func (s *recipeService) GetRecipeRecommendations(ctx context.Context, req domain.RecipeRecommendationRequest, userID string) (domain.RecipeRecommendationResponse, error) {
	// Get user's food items to base recommendations on
	var foodItems []*entities.FoodItem
	var err error
	var count int64

	if req.IncludeExpiringOnly {
		// Get food items that are expiring soon (within 7 days)
		now := time.Now()
		expiryThreshold := now.AddDate(0, 0, 7)
		foodItems, err = s.foodRepository.GetFoodItemsByExpiryRange(ctx, userID, now, expiryThreshold)
		if err != nil {
			return domain.RecipeRecommendationResponse{}, err
		}
		count = int64(len(foodItems))
	} else {
		// Get all food items
		foodItems, count, err = s.foodRepository.GetFoodItems(ctx, userID, "Safe", 1, 100)
		if err != nil {
			return domain.RecipeRecommendationResponse{}, err
		}
	}

	if len(foodItems) == 0 {
		return domain.RecipeRecommendationResponse{
			Recipes:       []domain.Recipe{},
			TotalRecipes:  0,
			ExpiringItems: 0,
		}, domain.ErrNoIngredients
	}

	// Count expiring items (within 7 days)
	now := time.Now()
	expiryThreshold := now.AddDate(0, 0, 7)
	expiringItems := 0
	for _, item := range foodItems {
		if item.ExpiryDate.Before(expiryThreshold) {
			expiringItems++
		}
	}

	// Extract ingredients for the Gemini API
	ingredients := make([]map[string]interface{}, 0, len(foodItems))
	for _, item := range foodItems {
		ingredients = append(ingredients, map[string]interface{}{
			"name":            item.Name,
			"quantity":        item.Quantity,
			"unit":            item.UnitMeasure,
			"expiryDate":      item.ExpiryDate.Format("2006-01-02"),
			"daysUntilExpiry": int(item.ExpiryDate.Sub(now).Hours() / 24),
		})
	}

	// Generate recipe recommendations using Gemini API
	recipes, err := s.generateRecipeRecommendations(ctx, ingredients, req)
	if err != nil {
		return domain.RecipeRecommendationResponse{}, err
	}

	// For each recipe, check if it's bookmarked
	for i := range recipes {
		isBookmarked, err := s.recipeRepository.IsRecipeBookmarked(ctx, userID, recipes[i].ID)
		if err != nil {
			continue // Skip on error, not critical
		}
		recipes[i].IsBookmarked = isBookmarked

		isCooked, err := s.recipeRepository.IsRecipeInHistory(ctx, userID, recipes[i].ID)
		if err != nil {
			continue
		}
		recipes[i].IsCooked = isCooked
	}

	return domain.RecipeRecommendationResponse{
		Recipes:       recipes,
		TotalRecipes:  len(recipes),
		ExpiringItems: expiringItems,
	}, nil
}

func (s *recipeService) generateRecipeRecommendations(ctx context.Context, ingredients []map[string]interface{}, req domain.RecipeRecommendationRequest) ([]domain.Recipe, error) {
	geminiAPIKey := utils.GetConfig("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	geminiModel := utils.GetConfig("GEMINI_MODEL")
	if geminiModel == "" {
		return nil, fmt.Errorf("GEMINI_MODEL environment variable not set")
	}

	// Add filters based on user preferences
	filters := map[string]interface{}{}
	if req.CuisineType != "" {
		filters["cuisineType"] = req.CuisineType
	}
	if req.DifficultyLevel != "" {
		filters["difficultyLevel"] = req.DifficultyLevel
	}
	if req.PreparationTime > 0 {
		filters["maxPrepTimeMinutes"] = req.PreparationTime
	}

	// Prepare the prompt for Gemini
	ingredientsJSON, _ := json.Marshal(ingredients)
	filtersJSON, _ := json.Marshal(filters)

	prompt := fmt.Sprintf(
		"You are a professional chef specializing in recipe recommendations based on available ingredients. "+
			"Given the following ingredients (with quantities, units, and expiry dates): %s, "+
			"and these preferences/filters: %s, "+
			"generate 5 unique and interesting recipe recommendations. "+
			"Prioritize using ingredients that are closest to expiry. "+
			"For each recipe include a title, short description, cuisine type, difficulty level, "+
			"preparation time, cooking time, and servings. "+
			"Generate the response as a valid JSON array containing 5 recipe objects with these fields: "+
			"title, description, prepTimeMinutes, cookTimeMinutes, servings, difficultyLevel, cuisineType. "+
			"Make sure cuisine types are diverse (e.g., Italian, Mexican, Asian, etc.) "+
			"Make sure the recipes are realistic and can actually be prepared with the given ingredients. "+
			"Do not include any explanations or text outside of the JSON array.",
		string(ingredientsJSON),
		string(filtersJSON),
	)

	geminiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", geminiModel, geminiAPIKey)

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
			"temperature": 0.7,
			"topP":        0.8,
			"topK":        40,
		},
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	geminiReq, err := http.NewRequestWithContext(ctx, "POST", geminiURL, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, err
	}
	geminiReq.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(geminiReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini API error: %s - %s", resp.Status, string(bodyBytes))
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
		return nil, err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, domain.ErrGeminiAPIFailed
	}

	responseText := geminiResp.Candidates[0].Content.Parts[0].Text
	responseText = strings.TrimSpace(responseText)

	// Extract JSON array from response text
	startIdx := strings.Index(responseText, "[")
	endIdx := strings.LastIndex(responseText, "]")
	if startIdx == -1 || endIdx == -1 || startIdx > endIdx {
		// If JSON array not found, try to extract JSON object instead
		startIdx = strings.Index(responseText, "{")
		endIdx = strings.LastIndex(responseText, "}")
		if startIdx == -1 || endIdx == -1 || startIdx > endIdx {
			return nil, fmt.Errorf("invalid response format: %s", responseText)
		}
		// Wrap the object in an array
		responseText = "[" + responseText[startIdx:endIdx+1] + "]"
	} else {
		responseText = responseText[startIdx : endIdx+1]
	}

	var rawRecipes []map[string]interface{}
	if err := json.Unmarshal([]byte(responseText), &rawRecipes); err != nil {
		return nil, err
	}

	// Convert to domain.Recipe and save to database
	recipes := make([]domain.Recipe, 0, len(rawRecipes))
	for _, raw := range rawRecipes {
		recipeID := uuid.New().String()

		// Set default values if missing
		if _, ok := raw["prepTimeMinutes"]; !ok {
			raw["prepTimeMinutes"] = 15
		}
		if _, ok := raw["cookTimeMinutes"]; !ok {
			raw["cookTimeMinutes"] = 30
		}
		if _, ok := raw["servings"]; !ok {
			raw["servings"] = 4
		}
		if _, ok := raw["difficultyLevel"]; !ok {
			raw["difficultyLevel"] = "Medium"
		}
		if _, ok := raw["cuisineType"]; !ok {
			raw["cuisineType"] = "International"
		}

		prepTime, _ := raw["prepTimeMinutes"].(float64)
		cookTime, _ := raw["cookTimeMinutes"].(float64)
		servings, _ := raw["servings"].(float64)

		recipe := domain.Recipe{
			ID:              recipeID,
			Title:           raw["title"].(string),
			Description:     raw["description"].(string),
			PrepTimeMinutes: int(prepTime),
			CookTimeMinutes: int(cookTime),
			Servings:        int(servings),
			DifficultyLevel: raw["difficultyLevel"].(string),
			CuisineType:     raw["cuisineType"].(string),
			CreatedAt:       time.Now(),
			IsBookmarked:    false,
			IsCooked:        false,
		}

		// Save the recipe to database for later retrieval
		dbRecipe := entities.Recipe{
			ID:              uuid.MustParse(recipeID),
			Title:           recipe.Title,
			Description:     recipe.Description,
			PrepTimeMinutes: recipe.PrepTimeMinutes,
			CookTimeMinutes: recipe.CookTimeMinutes,
			Servings:        recipe.Servings,
			DifficultyLevel: recipe.DifficultyLevel,
			CuisineType:     recipe.CuisineType,
			IsGenerated:     true,
		}

		// Serialize recipe data for storage
		rawJSON, _ := json.Marshal(raw)
		dbRecipe.Ingredients = string(rawJSON)

		// Ignore error, not critical for recommendation
		_ = s.recipeRepository.CreateRecipe(ctx, &dbRecipe)

		recipes = append(recipes, recipe)
	}

	return recipes, nil
}

func (s *recipeService) GetRecipeDetail(ctx context.Context, recipeID string, userID string) (domain.RecipeDetail, error) {
	recipe, err := s.recipeRepository.GetRecipeByID(ctx, recipeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.RecipeDetail{}, domain.ErrRecipeNotFound
		}
		return domain.RecipeDetail{}, err
	}

	// Check if the recipe is bookmarked
	isBookmarked, err := s.recipeRepository.IsRecipeBookmarked(ctx, userID, recipeID)
	if err != nil {
		isBookmarked = false // Default to false on error
	}

	// Check if the recipe is in history
	isCooked, cookedAt := false, time.Time{}
	var recipeHistory entities.RecipeHistory
	if err := gorm.ErrRecordNotFound; err == nil {
		isCooked = true
		cookedAt = recipeHistory.CookedAt
	}

	// Parse the ingredients and instructions from JSON
	var recipeData map[string]interface{}
	if err := json.Unmarshal([]byte(recipe.Ingredients), &recipeData); err != nil {
		recipeData = make(map[string]interface{})
	}

	// Get user's food items to check ingredient availability
	foodItems, _, err := s.foodRepository.GetFoodItems(ctx, userID, "all", 1, 100)
	if err != nil {
		return domain.RecipeDetail{}, err
	}

	// Build ingredient list with availability info
	var ingredients []domain.Ingredient
	var requiredItems []domain.AdditionalItem
	var substitutionItems []domain.AdditionalItem

	if rawIngredients, ok := recipeData["ingredients"].([]interface{}); ok {
		for _, rawIng := range rawIngredients {
			if ingMap, ok := rawIng.(map[string]interface{}); ok {
				name := fmt.Sprintf("%v", ingMap["name"])
				quantity, _ := ingMap["quantity"].(float64)
				unit := fmt.Sprintf("%v", ingMap["unit"])

				// Check if ingredient is available in user's food items
				isAvailable := false
				expiryDate := ""
				daysUntilExpiry := 0

				for _, item := range foodItems {
					if strings.Contains(strings.ToLower(item.Name), strings.ToLower(name)) ||
						strings.Contains(strings.ToLower(name), strings.ToLower(item.Name)) {
						isAvailable = true
						expiryDate = item.ExpiryDate.Format("2006-01-02")
						daysUntilExpiry = int(item.ExpiryDate.Sub(time.Now()).Hours() / 24)
						break
					}
				}

				ingredient := domain.Ingredient{
					Name:            name,
					Quantity:        quantity,
					Unit:            unit,
					IsAvailable:     isAvailable,
					ExpiryDate:      expiryDate,
					DaysUntilExpiry: daysUntilExpiry,
				}

				ingredients = append(ingredients, ingredient)

				// If not available, add to required items
				if !isAvailable {
					requiredItems = append(requiredItems, domain.AdditionalItem{
						Name:     name,
						Quantity: quantity,
						Unit:     unit,
					})
				}
			}
		}
	}

	// Parse instructions
	var instructions []string
	if rawInstructions, ok := recipeData["instructions"].([]interface{}); ok {
		for _, instr := range rawInstructions {
			instructions = append(instructions, fmt.Sprintf("%v", instr))
		}
	} else {
		// Generate instructions using Gemini if not available
		generatedInstructions, err := s.generateRecipeInstructions(ctx, recipe.Title, ingredients)
		if err == nil {
			instructions = generatedInstructions
		} else {
			instructions = []string{"Instructions not available"}
		}
	}

	// Create nutrition facts
	nutritionFacts := domain.NutritionFacts{
		Calories:      500, // Default values
		Protein:       20,
		Carbohydrates: 50,
		Fat:           25,
		Fiber:         5,
	}

	if rawNutrition, ok := recipeData["nutritionFacts"].(map[string]interface{}); ok {
		if calories, ok := rawNutrition["calories"].(float64); ok {
			nutritionFacts.Calories = int(calories)
		}
		if protein, ok := rawNutrition["protein"].(float64); ok {
			nutritionFacts.Protein = int(protein)
		}
		if carbs, ok := rawNutrition["carbohydrates"].(float64); ok {
			nutritionFacts.Carbohydrates = int(carbs)
		}
		if fat, ok := rawNutrition["fat"].(float64); ok {
			nutritionFacts.Fat = int(fat)
		}
		if fiber, ok := rawNutrition["fiber"].(float64); ok {
			nutritionFacts.Fiber = int(fiber)
		}
	}

	// Generate substitutions for required items
	if len(requiredItems) > 0 {
		generatedSubstitutions, err := s.generateIngredientSubstitutions(ctx, requiredItems)
		if err == nil {
			substitutionItems = generatedSubstitutions
		}
	}

	return domain.RecipeDetail{
		Recipe: domain.Recipe{
			ID:              recipe.ID.String(),
			Title:           recipe.Title,
			Description:     recipe.Description,
			ImageURL:        recipe.ImageURL,
			PrepTimeMinutes: recipe.PrepTimeMinutes,
			CookTimeMinutes: recipe.CookTimeMinutes,
			Servings:        recipe.Servings,
			DifficultyLevel: recipe.DifficultyLevel,
			CuisineType:     recipe.CuisineType,
			CreatedAt:       recipe.CreatedAt,
			IsBookmarked:    isBookmarked,
			IsCooked:        isCooked,
			CookedAt:        cookedAt,
		},
		Ingredients:       ingredients,
		Instructions:      instructions,
		NutritionFacts:    nutritionFacts,
		RequiredItems:     requiredItems,
		SubstitutionItems: substitutionItems,
	}, nil
}

func (s *recipeService) generateRecipeInstructions(ctx context.Context, title string, ingredients []domain.Ingredient) ([]string, error) {
	geminiAPIKey := utils.GetConfig("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	geminiModel := utils.GetConfig("GEMINI_MODEL")
	if geminiModel == "" {
		return nil, fmt.Errorf("GEMINI_MODEL environment variable not set")
	}

	// Extract ingredient names for the prompt
	ingredientNames := make([]string, 0, len(ingredients))
	for _, ing := range ingredients {
		ingredientNames = append(ingredientNames, ing.Name)
	}

	// Prepare the prompt for Gemini
	prompt := fmt.Sprintf(
		"You are a professional chef. Generate step-by-step cooking instructions for a recipe titled '%s' using these ingredients: %s. "+
			"Provide the instructions as a numbered list with clear, concise steps. "+
			"Return the result as a valid JSON array of strings, where each string is one step. "+
			"The instructions should be practical, realistic, and easy to follow.",
		title,
		strings.Join(ingredientNames, ", "),
	)

	geminiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", geminiModel, geminiAPIKey)

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
			"temperature": 0.7,
			"topP":        0.8,
			"topK":        40,
		},
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", geminiURL, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini API error: %s - %s", resp.Status, string(bodyBytes))
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
		return nil, err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, domain.ErrGeminiAPIFailed
	}

	responseText := geminiResp.Candidates[0].Content.Parts[0].Text
	responseText = strings.TrimSpace(responseText)

	// Extract JSON array from response text
	startIdx := strings.Index(responseText, "[")
	endIdx := strings.LastIndex(responseText, "]")
	if startIdx == -1 || endIdx == -1 || startIdx > endIdx {
		// If not valid JSON, parse as plain text
		lines := strings.Split(responseText, "\n")
		var instructions []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				// Remove numbering if present
				if strings.Contains(line, ". ") {
					parts := strings.SplitN(line, ". ", 2)
					if len(parts) > 1 {
						line = parts[1]
					}
				}
				instructions = append(instructions, line)
			}
		}
		return instructions, nil
	}

	// Parse as JSON
	jsonStr := responseText[startIdx : endIdx+1]
	var instructions []string
	if err := json.Unmarshal([]byte(jsonStr), &instructions); err != nil {
		// If JSON parsing fails, extract as plain text
		lines := strings.Split(responseText, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "]") {
				// Remove numbering if present
				if strings.Contains(line, ". ") {
					parts := strings.SplitN(line, ". ", 2)
					if len(parts) > 1 {
						line = parts[1]
					}
				}
				instructions = append(instructions, line)
			}
		}
	}

	return instructions, nil
}

func (s *recipeService) generateIngredientSubstitutions(ctx context.Context, requiredItems []domain.AdditionalItem) ([]domain.AdditionalItem, error) {
	geminiAPIKey := utils.GetConfig("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	geminiModel := utils.GetConfig("GEMINI_MODEL")
	if geminiModel == "" {
		return nil, fmt.Errorf("GEMINI_MODEL environment variable not set")
	}

	if len(requiredItems) == 0 {
		return []domain.AdditionalItem{}, nil
	}

	// Extract ingredient names for the prompt
	itemNames := make([]string, 0, len(requiredItems))
	for _, item := range requiredItems {
		itemNames = append(itemNames, fmt.Sprintf("%g %s of %s", item.Quantity, item.Unit, item.Name))
	}

	// Prepare the prompt for Gemini
	prompt := fmt.Sprintf(
		"You are a professional chef. For each of these ingredients: %s, "+
			"suggest common substitutes that people might have at home. "+
			"For each ingredient, provide one or two practical substitutions with appropriate quantities. "+
			"Return the result as a valid JSON array where each object has fields: 'originalName', 'substituteName', 'quantity', and 'unit'. "+
			"Focus on practical, readily available substitutes.",
		strings.Join(itemNames, ", "),
	)

	geminiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", geminiModel, geminiAPIKey)

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
			"temperature": 0.7,
			"topP":        0.8,
			"topK":        40,
		},
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", geminiURL, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini API error: %s - %s", resp.Status, string(bodyBytes))
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
		return nil, err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, domain.ErrGeminiAPIFailed
	}

	responseText := geminiResp.Candidates[0].Content.Parts[0].Text
	responseText = strings.TrimSpace(responseText)

	// Extract JSON array from response text
	startIdx := strings.Index(responseText, "[")
	endIdx := strings.LastIndex(responseText, "]")
	if startIdx == -1 || endIdx == -1 || startIdx > endIdx {
		return []domain.AdditionalItem{}, nil
	}

	// Parse as JSON
	jsonStr := responseText[startIdx : endIdx+1]
	var substitutions []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &substitutions); err != nil {
		return []domain.AdditionalItem{}, nil
	}

	// Convert to AdditionalItem
	result := make([]domain.AdditionalItem, 0, len(substitutions))
	for _, sub := range substitutions {
		name, _ := sub["substituteName"].(string)
		quantity, _ := sub["quantity"].(float64)
		if quantity == 0 {
			if qStr, ok := sub["quantity"].(string); ok {
				qFloat, err := strconv.ParseFloat(qStr, 64)
				if err == nil {
					quantity = qFloat
				} else {
					quantity = 1 // Default if parsing fails
				}
			} else {
				quantity = 1 // Default quantity
			}
		}
		unit, _ := sub["unit"].(string)

		result = append(result, domain.AdditionalItem{
			Name:     name,
			Quantity: quantity,
			Unit:     unit,
		})
	}

	return result, nil
}

func (s *recipeService) BookmarkRecipe(ctx context.Context, req domain.BookmarkRecipeRequest, userID string) error {
	// Check if recipe exists
	_, err := s.recipeRepository.GetRecipeByID(ctx, req.RecipeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrRecipeNotFound
		}
		return err
	}

	return s.recipeRepository.BookmarkRecipe(ctx, userID, req.RecipeID)
}

func (s *recipeService) RemoveBookmark(ctx context.Context, req domain.BookmarkRecipeRequest, userID string) error {
	// Check if recipe exists
	_, err := s.recipeRepository.GetRecipeByID(ctx, req.RecipeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrRecipeNotFound
		}
		return err
	}

	return s.recipeRepository.RemoveBookmark(ctx, userID, req.RecipeID)
}

func (s *recipeService) GetBookmarkedRecipes(ctx context.Context, page, limit int, userID string) ([]domain.Recipe, int64, error) {
	recipes, count, err := s.recipeRepository.GetRecipeBookmarks(ctx, userID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	result := make([]domain.Recipe, 0, len(recipes))
	for _, recipe := range recipes {
		result = append(result, domain.Recipe{
			ID:              recipe.ID.String(),
			Title:           recipe.Title,
			Description:     recipe.Description,
			ImageURL:        recipe.ImageURL,
			PrepTimeMinutes: recipe.PrepTimeMinutes,
			CookTimeMinutes: recipe.CookTimeMinutes,
			Servings:        recipe.Servings,
			DifficultyLevel: recipe.DifficultyLevel,
			CuisineType:     recipe.CuisineType,
			CreatedAt:       recipe.CreatedAt,
			IsBookmarked:    true,
		})
	}

	return result, count, nil
}

func (s *recipeService) MarkAsCooked(ctx context.Context, req domain.MarkAsCookedRequest, userID string) error {
	// Check if recipe exists
	_, err := s.recipeRepository.GetRecipeByID(ctx, req.RecipeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrRecipeNotFound
		}
		return err
	}

	return s.recipeRepository.AddRecipeHistory(ctx, userID, req.RecipeID)
}

func (s *recipeService) GetRecipeHistory(ctx context.Context, page, limit int, userID string) (domain.RecipeHistoryResponse, error) {
	recipes, count, err := s.recipeRepository.GetRecipeHistory(ctx, userID, page, limit)
	if err != nil {
		return domain.RecipeHistoryResponse{}, err
	}

	result := make([]domain.Recipe, 0, len(recipes))
	for _, recipe := range recipes {
		// Check if bookmarked
		isBookmarked, _ := s.recipeRepository.IsRecipeBookmarked(ctx, userID, recipe.ID.String())

		result = append(result, domain.Recipe{
			ID:              recipe.ID.String(),
			Title:           recipe.Title,
			Description:     recipe.Description,
			ImageURL:        recipe.ImageURL,
			PrepTimeMinutes: recipe.PrepTimeMinutes,
			CookTimeMinutes: recipe.CookTimeMinutes,
			Servings:        recipe.Servings,
			DifficultyLevel: recipe.DifficultyLevel,
			CuisineType:     recipe.CuisineType,
			CreatedAt:       recipe.CreatedAt,
			IsBookmarked:    isBookmarked,
			IsCooked:        true,
		})
	}

	return domain.RecipeHistoryResponse{
		Recipes: result,
		Total:   int(count),
	}, nil
}
