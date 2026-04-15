package controller

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"boundless-be/dto"
	"boundless-be/errs"
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
	RefreshAccess(refreshToken string) (string, error)
}

type PremiumRepository interface {
	FindCurrentPremiumSubscription(ctx context.Context, userID string, reference time.Time) (model.UserSubscription, error)
}

type AuthController struct {
	authService AuthService
	userRepo    repository.UserRepository
	premiumRepo PremiumRepository
}

func NewAuthController(authService AuthService, userRepo repository.UserRepository, premiumRepo PremiumRepository) *AuthController {
	return &AuthController{authService: authService, userRepo: userRepo, premiumRepo: premiumRepo}
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

		log.Printf("auth login failed for email %s: %v", req.Email, err)
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

func (c *AuthController) Refresh(ctx *gin.Context) {
	var req dto.RefreshRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		return
	}

	newAccessToken, err := c.authService.RefreshAccess(req.RefreshToken)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}

	ctx.JSON(http.StatusOK, dto.RefreshResponse{AccessToken: newAccessToken})
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

	if c.premiumRepo != nil {
		premiumSub, err := c.premiumRepo.FindCurrentPremiumSubscription(ctx.Request.Context(), user.UserID, time.Now().UTC())
		if err != nil {
			if !errors.Is(err, errs.ErrPremiumSubscriptionNotFound) {
				log.Printf("auth me premium lookup failed for user %s: %v", user.UserID, err)
			}
		} else {
			response.IsPremium = true
			startAt := premiumSub.StartDate.UTC().Format(time.RFC3339)
			endAt := premiumSub.EndDate.UTC().Format(time.RFC3339)
			response.PremiumStartAt = &startAt
			response.PremiumEndAt = &endAt
		}
	}

	ctx.JSON(http.StatusOK, response)
}
