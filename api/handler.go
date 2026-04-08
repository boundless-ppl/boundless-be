package api

import (
	"boundless-be/middleware"
	"boundless-be/repository"
	"os"
	"strings"
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
	router.Use(middleware.SecurityHeaders())

	allowedOrigins := []string{"http://localhost:3000"}
	if raw := strings.TrimSpace(os.Getenv("ALLOWED_CORS_ORIGINS")); raw != "" {
		parts := strings.Split(raw, ",")
		allowedOrigins = allowedOrigins[:0]
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				allowedOrigins = append(allowedOrigins, part)
			}
		}
		if len(allowedOrigins) == 0 {
			allowedOrigins = []string{"http://localhost:3000"}
		}
	}

	router.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.GET("/", func(ctx *gin.Context) {
		ctx.String(200, "hi\n")
	})

	if shouldServeLocalDocuments() {
		router.Static("/documents", os.Getenv("DOCUMENT_STORAGE_DIR"))
	}

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

func shouldServeLocalDocuments() bool {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("DOCUMENT_STORAGE_PROVIDER")))
	if provider != "" && provider != "local" {
		return false
	}

	return strings.TrimSpace(os.Getenv("DOCUMENT_STORAGE_DIR")) != ""
}
