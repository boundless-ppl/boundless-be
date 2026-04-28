package api

import (
	"boundless-be/middleware"
	"boundless-be/repository"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type Dependencies struct {
	UserRepo         repository.UserRepository
	UnivRepo         repository.UniversityRepository
	RecRepo          repository.RecommendationRepository
	PaymentRepo      repository.PaymentRepository
	DreamTrackerRepo repository.DreamTrackerRepository
	ScholarshipRepo  repository.ScholarshipRepository
}

func NewHandler(dep Dependencies) *gin.Engine {
	router := gin.New()
	router.MaxMultipartMemory = 1 << 20
	origins := normalizeAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	router.Use(gin.Recovery())

	// Global: 60 req/min per IP, burst 20.
	globalLimiter := middleware.NewRateLimiter(rate.Limit(1), 20)
	router.Use(globalLimiter.Limit())

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
	router.GET("/favicon.ico", func(ctx *gin.Context) {
		ctx.Status(204)
	})

	// Stricter: 10 req/min per IP on unauthenticated auth endpoints, burst 5.
	authLimiter := middleware.NewRateLimiter(rate.Limit(10.0/60), 5)
	registerAuthRoutes(router, dep.UserRepo, dep.PaymentRepo, authLimiter)

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

	if dep.ScholarshipRepo != nil {
		registerScholarshipRoutes(router, dep.ScholarshipRepo)
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
