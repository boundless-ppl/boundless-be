package api

import (
	"boundless-be/controller"
	"boundless-be/middleware"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

func registerAuthRoutes(router *gin.Engine, userRepo repository.UserRepository, paymentRepo repository.PaymentRepository, authLimiter *middleware.RateLimiter) {
	authService := service.NewAuthService(userRepo)
	authController := controller.NewAuthController(authService, userRepo, paymentRepo)
	authMiddleware := middleware.NewAuthMiddleware(authService)

	strictLimit := authLimiter.Limit()
	router.POST("/auth/register", strictLimit, authController.Register)
	router.POST("/auth/login", strictLimit, authController.Login)
	router.POST("/auth/refresh", strictLimit, authController.Refresh)
	router.POST("/auth/logout", authMiddleware.RequireAuth(), authController.Logout)
	router.GET("/auth/me", authMiddleware.RequireAuth(), authController.Me)
	router.PUT("/auth/me", authMiddleware.RequireAuth(), authController.UpdateProfile)
	router.PUT("/auth/me/password", authMiddleware.RequireAuth(), authController.ChangePassword)
}
