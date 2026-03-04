package api

import (
	"boundless-be/repository"

	"github.com/gin-gonic/gin"
)

func NewHandler(userRepo repository.UserRepository) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/", func(ctx *gin.Context) {
		ctx.String(200, "hi\n")
	})
	registerAuthRoutes(router, userRepo)

	return router
}
