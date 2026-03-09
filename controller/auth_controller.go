package controller

import (
	"context"
	"errors"
	"net/http"

	"boundless-be/dto"
	"boundless-be/middleware"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

type AuthService interface {
	Register(ctx context.Context, fullName, role, email, password string) error
	Login(ctx context.Context, email, password string) (service.AuthTokens, error)
	Logout(token string) error
}

type AuthController struct {
	authService AuthService
}

func NewAuthController(authService AuthService) *AuthController {
	return &AuthController{authService: authService}
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
