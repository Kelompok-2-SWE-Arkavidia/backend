package config

import (
	"Go-Starter-Template/internal/api/handlers"
	"Go-Starter-Template/internal/api/routes"
	"Go-Starter-Template/internal/middleware"
	"Go-Starter-Template/internal/utils"
	"Go-Starter-Template/internal/utils/storage"
	"Go-Starter-Template/pkg/donation"
	"Go-Starter-Template/pkg/food"
	"Go-Starter-Template/pkg/jwt"
	"Go-Starter-Template/pkg/midtrans"
	"Go-Starter-Template/pkg/recipe"
	"Go-Starter-Template/pkg/user"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"gorm.io/gorm"
)

func NewApp(db *gorm.DB) (*fiber.App, error) {
	utils.InitValidator()
	app := fiber.New(fiber.Config{
		EnablePrintRoutes: true,
	})
	middlewares := middleware.NewMiddleware()
	validator := utils.Validate

	// setting up logging and limiter
	err := os.MkdirAll("./logs", os.ModePerm)
	if err != nil {
		log.Fatalf("error creating logs directory: %v", err)
	}
	file, err := os.OpenFile(
		"./logs/app.log",
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0666,
	)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	app.Use(logger.New(logger.Config{
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "Asia/Jakarta",
		Output:     file,
	}))

	app.Use(limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Second,
	}))

	// utils
	s3 := storage.NewAwsS3()

	// Repository
	userRepository := user.NewUserRepository(db)
	midtransRepository := midtrans.NewMidtransRepository(db)
	foodRepository := food.NewFoodRepository(db)
	recipeRepository := recipe.NewRecipeRepository(db)
	donationRepository := donation.NewDonationRepository(db)

	// Service
	jwtService := jwt.NewJWTService()
	userService := user.NewUserService(userRepository, jwtService, s3)
	midtransService := midtrans.NewMidtransService(
		midtransRepository,
		userRepository,
	)
	foodService := food.NewFoodService(foodRepository, s3)
	recipeService := recipe.NewRecipeService(recipeRepository, foodRepository)
	donationService := donation.NewDonationService(donationRepository, foodRepository, s3)

	// Handler
	userHandler := handlers.NewUserHandler(userService, validator, jwtService)
	midtransHandler := handlers.NewMidtransHandler(midtransService, validator)
	foodHandler := handlers.NewFoodHandler(foodService, validator)
	recipeHandler := handlers.NewRecipeHandler(recipeService, validator)
	donationHandler := handlers.NewDonationHandler(donationService, validator)

	// routes
	routesConfig := routes.Config{
		App:             app,
		UserHandler:     userHandler,
		MidtransHandler: midtransHandler,
		FoodHandler:     foodHandler,
		RecipeHandler:   recipeHandler,
		Middleware:      middlewares,
		JWTService:      jwtService,
		DonationHandler: donationHandler,
	}
	routesConfig.Setup()
	return app, nil
}
