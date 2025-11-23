package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"pr-reviewer-service/internal/handler"
	"pr-reviewer-service/internal/service"
	"pr-reviewer-service/internal/storage"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

func main() {
	host := getEnv("POSTGRES_HOST", "localhost")
	port := getEnv("POSTGRES_PORT", "5432")
	user := getEnv("POSTGRES_USER", "reviewer")
	password := getEnv("POSTGRES_PASSWORD", "reviewer123")
	dbname := getEnv("POSTGRES_DB", "pr_reviewer_db")
	serverPort := getEnv("SERVER_PORT", "8080")

	log.Println("Waiting for database...")
	if err := waitForDB(host, port, user, password, dbname); err != nil {
		log.Fatalf("Database not available: %v", err)
	}

	log.Println("Running migrations...")
	if err := runMigrations(host, port, user, password, dbname); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	store, err := storage.New(host, port, user, password, dbname)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer store.Close()

	svc := service.New(store)
	h := handler.New(svc)

	r := mux.NewRouter()

	r.HandleFunc("/team/add", h.CreateTeam).Methods("POST")
	r.HandleFunc("/team/get", h.GetTeam).Methods("GET")
	r.HandleFunc("/team/deactivate", h.DeactivateTeam).Methods("POST")
	r.HandleFunc("/users/setIsActive", h.SetUserActive).Methods("POST")
	r.HandleFunc("/pullRequest/create", h.CreatePR).Methods("POST")
	r.HandleFunc("/pullRequest/merge", h.MergePR).Methods("POST")
	r.HandleFunc("/pullRequest/reassign", h.ReassignReviewer).Methods("POST")
	r.HandleFunc("/users/getReview", h.GetUserReviews).Methods("GET")
	r.HandleFunc("/statistics", h.GetStatistics).Methods("GET")

	addr := fmt.Sprintf(":%s", serverPort)
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func waitForDB(host, port, user, password, dbname string) error {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	for i := 0; i < 30; i++ {
		db, err := sql.Open("postgres", connStr)
		if err == nil {
			if err := db.Ping(); err == nil {
				db.Close()
				return nil
			}
			db.Close()
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("database not ready after 30 seconds")
}

func runMigrations(host, port, user, password, dbname string) error {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	migrationFiles, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		return err
	}

	for _, file := range migrationFiles {
		log.Printf("Applying migration: %s", file)
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %v", file, err)
		}

		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to apply migration %s: %v", file, err)
		}
	}

	log.Println("Migrations completed successfully")
	return nil
}
