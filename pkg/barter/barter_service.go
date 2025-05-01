package barter

import (
	"Go-Starter-Template/domain"
	"Go-Starter-Template/entities"
	"Go-Starter-Template/internal/utils/storage"
	"Go-Starter-Template/pkg/user"
	"context"
	"fmt"
	"github.com/google/uuid"
	"time"
)

type (
	BarterService interface {
		CreateBarterItem(ctx context.Context, req domain.BarterItemRequest, userID string, lat, lng float64) (*domain.BarterItem, error)
		GetBarterItemByID(ctx context.Context, id string) (*domain.BarterItem, error)
		UpdateBarterItem(ctx context.Context, id string, req domain.BarterItemRequest, userID string) (*domain.BarterItem, error)
		DeleteBarterItem(ctx context.Context, id string, userID string) error
		GetUserBarterItems(ctx context.Context, userID string, status string, page, limit int) ([]*domain.BarterItem, int64, error)
		GetNearbyBarterItems(ctx context.Context, req domain.GetNearbyBarterItemsRequest, userID string) ([]*domain.BarterItem, error)

		ExpressBarterInterest(ctx context.Context, req domain.BarterInterestRequest, userID string) (*domain.BarterChat, error)
		GetUserBarterChats(ctx context.Context, userID string, status string, page, limit int) ([]*domain.BarterChat, error)
		GetBarterChatByID(ctx context.Context, id string, userID string) (*domain.BarterChat, error)
		SendBarterMessage(ctx context.Context, req domain.SendMessageRequest, userID string) (*domain.BarterMessage, error)

		CompleteBarterTransaction(ctx context.Context, req domain.CompleteBarterRequest, userID string) (*domain.BarterTransaction, error)
		ConfirmBarterCompletion(ctx context.Context, req domain.ConfirmBarterCompletionRequest, userID string) error
		GetBarterStatistics(ctx context.Context, userID string) (*domain.BarterStatistics, error)
	}

	barterService struct {
		barterRepository BarterRepository
		userService      user.UserService
		s3               storage.AwsS3
	}
)

func NewBarterService(barterRepository BarterRepository, userService user.UserService, s3 storage.AwsS3) BarterService {
	return &barterService{
		barterRepository: barterRepository,
		userService:      userService,
		s3:               s3,
	}
}

func (s *barterService) CreateBarterItem(ctx context.Context, req domain.BarterItemRequest, userID string, lat, lng float64) (*domain.BarterItem, error) {
	// Parse expiry date
	expiryDate, err := time.Parse("2006-01-02", req.ExpiryDate)
	if err != nil {
		return nil, err
	}

	// Validate condition
	if req.Condition != "Sealed" && req.Condition != "Opened" && req.Condition != "New" && req.Condition != "Used" {
		req.Condition = "Opened" // Default condition
	}

	// Parse user ID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}

	// Create barter item
	barterItemID := uuid.New()

	// Process image if provided
	var imageURL string
	if req.Image != nil {
		objectKey, err := s.s3.UploadFile(
			fmt.Sprintf("barter-%s", barterItemID.String()),
			req.Image,
			"barters",
			storage.AllowImage...,
		)
		if err != nil {
			return nil, err
		}
		imageURL = s.s3.GetPublicLinkKey(objectKey)
	}

	// Process food item reference if provided
	var foodItemUUID *uuid.UUID
	if req.FoodItemID != "" {
		parsed, err := uuid.Parse(req.FoodItemID)
		if err == nil {
			foodItemUUID = &parsed
		}
	}

	// Create barter item entity
	barterItem := &entities.BarterItem{
		ID:            barterItemID,
		UserID:        userUUID,
		FoodItemID:    foodItemUUID,
		Name:          req.Name,
		Description:   req.Description,
		Quantity:      req.Quantity,
		UnitMeasure:   req.UnitMeasure,
		ExpiryDate:    expiryDate,
		Condition:     req.Condition,
		ImageURL:      imageURL,
		Status:        "Available",
		Latitude:      lat,
		Longitude:     lng,
		PreferredSwap: req.PreferredSwap,
	}

	if err := s.barterRepository.CreateBarterItem(ctx, barterItem); err != nil {
		return nil, err
	}

	// Get user details
	user, err := s.userService.Me(ctx, userID)
	if err == nil {
		return &domain.BarterItem{
			ID:            barterItemID.String(),
			UserID:        userID,
			UserName:      user.Name,
			Name:          req.Name,
			Description:   req.Description,
			Quantity:      req.Quantity,
			UnitMeasure:   req.UnitMeasure,
			ExpiryDate:    expiryDate,
			Condition:     req.Condition,
			ImageURL:      imageURL,
			Status:        "Available",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			PreferredSwap: req.PreferredSwap,
		}, nil
	}

	// Return without user details if something went wrong
	return &domain.BarterItem{
		ID:            barterItemID.String(),
		UserID:        userID,
		Name:          req.Name,
		Description:   req.Description,
		Quantity:      req.Quantity,
		UnitMeasure:   req.UnitMeasure,
		ExpiryDate:    expiryDate,
		Condition:     req.Condition,
		ImageURL:      imageURL,
		Status:        "Available",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		PreferredSwap: req.PreferredSwap,
	}, nil
}
