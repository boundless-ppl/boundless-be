package controller

import (
	"context"
	"errors"
	"net/http"
	"time"

	"boundless-be/dto"
	"boundless-be/middleware"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

type AuthService interface {
	Register(ctx context.Context, fullName, role, email, password string) error
	Login(ctx context.Context, email, password string) (service.AuthTokens, error)
	Logout(token string) error
}

type PaymentStatusRepository interface {
	FindCurrentPremiumSubscription(ctx context.Context, userID string, reference time.Time) (model.UserSubscription, error)
	FindLatestPendingPaymentByUser(ctx context.Context, userID string, reference time.Time) (model.Payment, error)
}

type AuthController struct {
	authService AuthService
	userRepo    repository.UserRepository
	statusRepo  PaymentStatusRepository
}

func NewAuthController(authService AuthService, userRepo repository.UserRepository, statusRepo PaymentStatusRepository) *AuthController {
	return &AuthController{authService: authService, userRepo: userRepo, statusRepo: statusRepo}
}

func (c *AuthController) Register(ctx *gin.Context) {
	var req dto.RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		return
	}

	const defaultRole = "user"
	err := c.authService.Register(ctx.Request.Context(), req.NamaLengkap, defaultRole, req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		case errors.Is(err, repository.ErrEmailExists):
			ctx.JSON(http.StatusConflict, dto.ErrorResponse{Error: "request failed"})
		default:
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "request failed"})
		}
		return
	}

	ctx.JSON(http.StatusCreated, dto.MessageResponse{
		Message: "user registered successfully",
	})
}

func (c *AuthController) Login(ctx *gin.Context) {
	var req dto.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		return
	}

	tokens, err := c.authService.Login(ctx.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrAccountLocked) {
			ctx.JSON(http.StatusTooManyRequests, dto.ErrorResponse{
				Error: err.Error(),
			})
			return
		}

		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}

	ctx.JSON(http.StatusOK, dto.AuthResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	})
}

func (c *AuthController) Logout(ctx *gin.Context) {
	token, exists := ctx.Get(middleware.TokenContextKey)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}

	if err := c.authService.Logout(token.(string)); err != nil {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}

	ctx.Status(http.StatusNoContent)
}

func (c *AuthController) Me(ctx *gin.Context) {
	userID, exists := ctx.Get(middleware.UserIDContextKey)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}

	user, err := c.userRepo.FindByID(ctx.Request.Context(), userID.(string))
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}

	response := dto.MeResponse{
		UserID:      user.UserID,
		NamaLengkap: user.NamaLengkap,
		Email:       user.Email,
		Role:        string(user.Role),
		IsPremium:   false,
	}

	if c.statusRepo != nil {
		status, err := service.BuildMeStatus(ctx.Request.Context(), c.statusRepo, user.UserID, time.Now().UTC())
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
			return
		}

		response.IsPremium = status.IsPremium
		response.PremiumStartAt = status.PremiumStartAt
		response.PremiumEndAt = status.PremiumEndAt
		response.HasPendingPayment = status.HasPendingPayment
		response.TransactionID = status.TransactionID
	}

	ctx.JSON(http.StatusOK, response)
}
