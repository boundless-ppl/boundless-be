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
	UserRepo         repository.UserRepository
	UnivRepo         repository.UniversityRepository
	RecRepo          repository.RecommendationRepository
	PaymentRepo      repository.PaymentRepository
	DreamTrackerRepo repository.DreamTrackerRepository
}

func NewHandler(dep Dependencies) *gin.Engine {
	router := gin.New()
	origins := normalizeAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
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

	if dep.DreamTrackerRepo != nil && dep.UserRepo != nil {
		registerDreamTrackerRoutes(router, dep.DreamTrackerRepo, dep.UserRepo)
	}

	return router
}

func normalizeAllowedOrigins(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{"*"}
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		candidate := strings.TrimSpace(part)
		if candidate == "" {
			continue
		}
		if candidate == "*" {
			return []string{"*"}
		}
		if strings.Contains(candidate, "://") {
			if _, ok := seen[candidate]; !ok {
				origins = append(origins, candidate)
				seen[candidate] = struct{}{}
			}
			continue
		}

		for _, scheme := range []string{"http://", "https://"} {
			origin := scheme + candidate
			if _, ok := seen[origin]; ok {
				continue
			}
			origins = append(origins, origin)
			seen[origin] = struct{}{}
		}
	}

	if len(origins) == 0 {
		return []string{"*"}
	}

	return origins
}
