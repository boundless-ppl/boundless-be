package api

import (
	"boundless-be/controller"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

const universityIDPath = "/universities/:id"

func registerUnivRoutes(router *gin.Engine, repo repository.UniversityRepository) {
	svc := service.NewUniversityService(repo)
	ctrl := controller.NewUniversityController(svc)

	router.POST("/universities", ctrl.Create)
	router.GET("/universities", ctrl.GetAll)
	router.GET(universityIDPath, ctrl.GetByID)
	router.PATCH(universityIDPath, ctrl.Update)
	router.DELETE(universityIDPath, ctrl.Delete)
}
