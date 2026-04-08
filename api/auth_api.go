package api

import (
	"boundless-be/controller"
	"boundless-be/middleware"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

func registerAuthRoutes(router *gin.Engine, userRepo repository.UserRepository, paymentRepo repository.PaymentRepository) {
	authService := service.NewAuthService(userRepo)
	authController := controller.NewAuthController(authService, userRepo, paymentRepo)
	authMiddleware := middleware.NewAuthMiddleware(authService)

	router.POST("/auth/register", authController.Register)
	router.POST("/auth/login", authController.Login)
	router.POST("/auth/logout", authMiddleware.RequireAuth(), authController.Logout)
	router.GET("/auth/me", authMiddleware.RequireAuth(), authController.Me)
}
