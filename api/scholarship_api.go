package api

import (
	"boundless-be/controller"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

func registerScholarshipRoutes(router *gin.Engine, repo repository.ScholarshipRepository) {
	svc := service.NewScholarshipService(repo)
	ctrl := controller.NewScholarshipController(svc)

	router.GET("/scholarships", ctrl.List)
	router.GET("/scholarships/:id", ctrl.GetByID)
}
