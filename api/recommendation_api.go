package api

import (
	"boundless-be/controller"
	"boundless-be/middleware"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

func registerRecommendationRoutes(router *gin.Engine, recRepo repository.RecommendationRepository, userRepo repository.UserRepository) {
	svc := service.NewRecommendationService(recRepo)
	ctrl := controller.NewRecommendationController(svc)
	authService := service.NewAuthService(userRepo)
	authMiddleware := middleware.NewAuthMiddleware(authService)

	recommendationGroup := router.Group("/recommendations")
	recommendationGroup.Use(authMiddleware.RequireAuth())
	recommendationGroup.POST("/documents", ctrl.UploadDocument)
	recommendationGroup.POST("/submissions", ctrl.CreateSubmission)
	recommendationGroup.POST("/profile", ctrl.CreateProfileRecommendation)
	recommendationGroup.POST("/transcript", ctrl.CreateTranscriptRecommendation)
	recommendationGroup.POST("/cv", ctrl.CreateCVRecommendation)
	recommendationGroup.GET("/submissions/:id", ctrl.GetSubmissionDetail)
}
