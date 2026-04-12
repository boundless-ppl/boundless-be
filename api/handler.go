package api

import (
	"boundless-be/repository"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	UserRepo    repository.UserRepository
	UnivRepo    repository.UniversityRepository
	RecRepo     repository.RecommendationRepository
	PaymentRepo repository.PaymentRepository
}

func NewHandler(dep Dependencies) *gin.Engine {
	router := gin.New()
	origins := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")
	router.Use(gin.Recovery())

	router.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.GET("/", func(ctx *gin.Context) {
		ctx.String(200, "hi\n")
	})

	registerAuthRoutes(router, dep.UserRepo, dep.PaymentRepo)

	if dep.UnivRepo != nil {
		registerUnivRoutes(router, dep.UnivRepo)
	}

	if dep.RecRepo != nil && dep.UserRepo != nil {
		registerRecommendationRoutes(router, dep.RecRepo, dep.UserRepo)
	}

	if dep.PaymentRepo != nil && dep.UserRepo != nil {
		registerPaymentRoutes(router, dep.PaymentRepo, dep.UserRepo)
	}

	return router
}
