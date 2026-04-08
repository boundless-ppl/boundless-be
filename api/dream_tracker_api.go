package api

import (
	"boundless-be/controller"
	"boundless-be/middleware"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

func registerDreamTrackerRoutes(router *gin.Engine, repo repository.DreamTrackerRepository, userRepo repository.UserRepository) {
	svc := service.NewDreamTrackerService(repo)
	ctrl := controller.NewDreamTrackerController(svc)
	authService := service.NewAuthService(userRepo)
	authMiddleware := middleware.NewAuthMiddleware(authService)

	group := router.Group("/dream-trackers")
	group.Use(authMiddleware.RequireAuth())
	group.POST("", ctrl.CreateDreamTracker)
	group.GET("", ctrl.ListDreamTrackers)
	group.GET("/summary", ctrl.GetDreamTrackerDashboardSummary)
	group.GET("/grouped", ctrl.GetGroupedDreamTrackers)
	group.GET("/documents/:id", ctrl.GetDocumentDetail)
	group.POST("/requirements/:id/document", ctrl.UploadDreamRequirementDocument)
	group.POST("/requirements/:id/submit", ctrl.SubmitDreamRequirement)
	group.GET("/:id", ctrl.GetDreamTrackerDetail)
}
