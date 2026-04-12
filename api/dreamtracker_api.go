package api

import (
	"boundless-be/controller"
	"boundless-be/middleware"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

func registerDreamTrackerRoutes(router *gin.Engine, dreamTrackerRepo repository.DreamTrackerRepository, userRepo repository.UserRepository) {
	dreamTrackerService := service.NewDreamTrackerService(dreamTrackerRepo)
	dreamTrackerController := controller.NewDreamTrackerController(dreamTrackerService)
	authService := service.NewAuthService(userRepo)
	authMiddleware := middleware.NewAuthMiddleware(authService)

	group := router.Group("/dream-trackers")
	group.Use(authMiddleware.RequireAuth())
	group.GET("/summary", dreamTrackerController.GetSummary)
	group.GET("/grouped", dreamTrackerController.GetGrouped)
	group.GET("/:id", dreamTrackerController.GetByID)
	group.POST("", dreamTrackerController.Create)
	group.POST("/requirements/:id/document", dreamTrackerController.UploadRequirementDocument)
}
