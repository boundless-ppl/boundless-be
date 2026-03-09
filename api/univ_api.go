package api

import (
	"boundless-be/controller"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

func registerUnivRoutes(router *gin.Engine, repo repository.UniversityRepository) {
	svc := service.NewUniversityService(repo)
	ctrl := controller.NewUniversityController(svc)

	router.POST("/universities", ctrl.Create)
	router.GET("/universities", ctrl.GetAll)
	router.GET("/universities/:id", ctrl.GetByID)
	router.PATCH("/universities/:id", ctrl.Update)
	router.DELETE("/universities/:id", ctrl.Delete)
}
