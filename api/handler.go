package api

import (
	"boundless-be/repository"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	UserRepo         repository.UserRepository
	UnivRepo         repository.UniversityRepository
	RecRepo          repository.RecommendationRepository
	DreamTrackerRepo repository.DreamTrackerRepository
}

func NewHandler(dep Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.GET("/", func(ctx *gin.Context) {
		ctx.String(200, "hi\n")
	})

	registerAuthRoutes(router, dep.UserRepo)

	if dep.UnivRepo != nil {
		registerUnivRoutes(router, dep.UnivRepo)
	}

	if dep.RecRepo != nil && dep.UserRepo != nil {
		registerRecommendationRoutes(router, dep.RecRepo, dep.UserRepo)
	}

	if dep.DreamTrackerRepo != nil && dep.UserRepo != nil {
		registerDreamTrackerRoutes(router, dep.DreamTrackerRepo, dep.UserRepo)
	}

	return router
}
