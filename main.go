package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"boundless-be/api"
	"boundless-be/database"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Overload(".env"); err != nil && !os.IsNotExist(err) {
		log.Fatalf("failed to load .env: %v", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	db, err := database.NewConnection(databaseURL)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer db.Close()

	log.Println("database connected")
	userRepo := repository.NewUserRepository(db)
	univRepo := repository.NewUniversityRepository(db)
	recRepo := repository.NewRecommendationRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	handler := api.NewHandler(api.Dependencies{
		UserRepo:    userRepo,
		UnivRepo:    univRepo,
		RecRepo:     recRepo,
		PaymentRepo: paymentRepo,
	})

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	adminEmail := strings.TrimSpace(os.Getenv("PAYMENT_ADMIN_EMAIL"))
	if adminEmail != "" {
		sender, err := service.NewSMTPEmailSenderFromEnv()
		if err != nil {
			log.Printf("payment notification email disabled: %v", err)
		} else {
			notifier := service.NewPaymentNotificationService(paymentRepo, sender, adminEmail)
			interval := 5 * time.Minute
			if raw := strings.TrimSpace(os.Getenv("PAYMENT_NOTIFICATION_INTERVAL")); raw != "" {
				if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
					interval = parsed
				}
			}

			go func() {
				ticker := time.NewTicker(interval)
				defer ticker.Stop()

				if err := notifier.RunOnce(appCtx); err != nil {
					log.Printf("payment notification run failed: %v", err)
				}

				for {
					select {
					case <-appCtx.Done():
						return
					case <-ticker.C:
						if err := notifier.RunOnce(appCtx); err != nil {
							log.Printf("payment notification run failed: %v", err)
						}
					}
				}
			}()
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}
	log.Println("server exited")
}
