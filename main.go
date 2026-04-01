package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boundless-be/api"
	"boundless-be/database"
	"boundless-be/repository"

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
	dreamTrackerRepo := repository.NewDreamTrackerRepository(db)
	handler := api.NewHandler(api.Dependencies{
		UserRepo:         userRepo,
		UnivRepo:         univRepo,
		RecRepo:          recRepo,
		DreamTrackerRepo: dreamTrackerRepo,
	})

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

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}
	log.Println("server exited")
}
