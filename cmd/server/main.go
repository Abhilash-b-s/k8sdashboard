package main

import (
	"log"
	"os"

	"k8s-dashboard/pkg/api"
	"k8s-dashboard/pkg/k8s"

	_ "k8s-dashboard/docs" // swagger docs
)

// @title Kubernetes Dashboard API
// @version 1.0
// @description API for Kubernetes Dashboard - provides endpoints for managing and monitoring Kubernetes resources
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@example.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api
// @schemes http https

func main() {
	// Initialize Kubernetes client
	if err := k8s.InitClient(); err != nil {
		log.Fatalf("Failed to initialize Kubernetes client: %v", err)
	}

	// Start informers
	k8s.StartInformers()

	// Setup Router
	router := api.SetupRouter()

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting Kubernetes Dashboard on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
