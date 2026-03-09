package api

import (
	"boundless-be/repository"

	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	UserRepo repository.UserRepository
	UnivRepo repository.UniversityRepository
}

func NewHandler(dep Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/", func(ctx *gin.Context) {
		ctx.String(200, "hi\n")
	})

	registerAuthRoutes(router, dep.UserRepo)

	if dep.UnivRepo != nil {
		registerUnivRoutes(router, dep.UnivRepo)
	}

	return router
}
