package handlers

import (
	"Go-Starter-Template/domain"
	"Go-Starter-Template/internal/api/presenters"
	"Go-Starter-Template/pkg/jwt"
	"Go-Starter-Template/pkg/user"
	"errors"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type (
	UserHandler interface {
		Register(c *fiber.Ctx) error
		Login(c *fiber.Ctx) error
		SendVerificationEmail(c *fiber.Ctx) error
		VerifyEmail(c *fiber.Ctx) error
		Me(c *fiber.Ctx) error
		UpdateUser(c *fiber.Ctx) error
		ForgotPassword(c *fiber.Ctx) error
		ResetPassword(c *fiber.Ctx) error
	}
	userHandler struct {
		UserService user.UserService
		Validator   *validator.Validate
		JWTService  jwt.JWTService
	}
)

func NewUserHandler(userService user.UserService, validator *validator.Validate, jwtService jwt.JWTService) UserHandler {
	return &userHandler{
		UserService: userService,
		Validator:   validator,
		JWTService:  jwtService,
	}
}

func (h *userHandler) Register(c *fiber.Ctx) error {
	req := new(domain.UserRegisterRequest)
	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.Validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedRegister, err)
	}

	res, err := h.UserService.Register(c.Context(), *req)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedRegister, err)
	}
	return presenters.SuccessResponse(c, res, fiber.StatusCreated, domain.MessageSuccessRegister)
}

func (h *userHandler) Login(c *fiber.Ctx) error {
	req := new(domain.UserLoginRequest)
	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.Validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedRegister, err)
	}
	res, err := h.UserService.Login(c.Context(), *req)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedRegister, err)
	}
	return presenters.SuccessResponse(c, res, fiber.StatusOK, domain.MessageSuccessLogin)
}

func (h *userHandler) SendVerificationEmail(c *fiber.Ctx) error {
	req := new(domain.SendVerifyEmailRequest)
	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.Validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedRegister, err)
	}
	if err := h.UserService.SendVerificationEmail(c.Context(), *req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	return presenters.SuccessResponse(c, nil, fiber.StatusOK, domain.MessageSuccessSendVerificationMail)
}

func (h *userHandler) VerifyEmail(c *fiber.Ctx) error {
	token := c.Query("token")

	if token == "" {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, domain.ErrTokenInvalid)
	}

	req := domain.VerifyEmailRequest{
		Token: token,
	}

	if err := h.Validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedRegister, err)
	}

	res, err := h.UserService.VerifyEmail(c.Context(), req)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedRegister, err)
	}
	return presenters.SuccessResponse(c, res, fiber.StatusOK, domain.MessageSuccessVerify)
}

func (h *userHandler) Me(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	//fmt.Println(userID)
	res, err := h.UserService.Me(c.Context(), userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetDetail, err)
	}
	return presenters.SuccessResponse(c, res, fiber.StatusOK, domain.MessageSuccessGetDetail)
}

func (h *userHandler) UpdateUser(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	req := new(domain.UpdateUserRequest)
	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	req.ProfilePicture, _ = c.FormFile("profile_picture")

	if err := h.Validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	res, err := h.UserService.Update(c.Context(), *req, userID)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedUpdateUser, err)
	}
	return presenters.SuccessResponse(c, res, fiber.StatusOK, domain.MessageSuccessUpdateUser)
}

func (h *userHandler) ForgotPassword(c *fiber.Ctx) error {
	req := new(domain.ForgetPasswordRequest)

	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.Validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.UserService.ForgetPassword(c.Context(), *req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedSendEmail, err)
	}
	return presenters.SuccessResponse(c, nil, fiber.StatusOK, domain.MessageSuccessSendEmail)
}

func (h *userHandler) ResetPassword(c *fiber.Ctx) error {
	token := c.Query("token")
	if token == "" {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedGetToken, errors.New("token required"))
	}

	claims, err := h.JWTService.ValidateTokenForgetPassword(token)
	if err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedTokenInvalid, err)
	}

	req := new(domain.ResetPasswordRequest)
	if err := c.BodyParser(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	if err := h.Validator.Struct(req); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedBodyRequest, err)
	}

	email, ok := claims["email"].(string)
	if !ok {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedTokenInvalid, err)
	}

	if err := h.UserService.ResetPassword(c.Context(), email, req.Password); err != nil {
		return presenters.ErrorResponse(c, fiber.StatusBadRequest, domain.MessageFailedUpdatePassword, err)
	}

	return presenters.SuccessResponse(c, nil, fiber.StatusOK, domain.MessageSuccessUpdatePassword)
}
