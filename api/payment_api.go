package api

import (
	"boundless-be/controller"
	"boundless-be/middleware"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

func registerPaymentRoutes(router *gin.Engine, paymentRepo repository.PaymentRepository, userRepo repository.UserRepository) {
	paymentService := service.NewPaymentService(paymentRepo)
	paymentController := controller.NewPaymentController(paymentService)
	authService := service.NewAuthService(userRepo)
	authMiddleware := middleware.NewAuthMiddleware(authService)

	router.GET("/subscriptions/packages", paymentController.ListPackages)

	paymentGroup := router.Group("/payments")
	paymentGroup.Use(authMiddleware.RequireAuth())
	paymentGroup.POST("", paymentController.CreatePayment)
	paymentGroup.GET("/:id", paymentController.GetMyPayment)
	paymentGroup.POST("/:id/proof", paymentController.UploadPaymentProof)

	adminPayments := router.Group("/admin/payments")
	adminPayments.Use(authMiddleware.RequireAuth())
	adminPayments.Use(authMiddleware.RequireRole("admin"))
	adminPayments.GET("", paymentController.ListAdminPayments)
	adminPayments.PATCH("/:id/status", paymentController.UpdatePaymentStatus)
}
