package api

import (
	"boundless-be/controller"
	"boundless-be/middleware"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

func registerAuthRoutes(router *gin.Engine, userRepo repository.UserRepository) {
	authService := service.NewAuthService(userRepo)
	authController := controller.NewAuthController(authService)
	authMiddleware := middleware.NewAuthMiddleware(authService)
	loginLimiter := middleware.NewLoginAttemptLimiter(0, 0, 0)

	router.POST("/auth/register", authController.Register)
	router.POST("/auth/login", middleware.NewLoginRateLimitMiddleware(loginLimiter), authController.Login)
	router.POST("/auth/logout", authMiddleware.RequireAuth(), authController.Logout)
}
