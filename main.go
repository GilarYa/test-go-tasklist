package main

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"test-go-tasklist/config"
	"test-go-tasklist/handlers"
	"test-go-tasklist/services"
)

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Initialize database
	db := config.InitDatabase()

	// Initialize services
	depService := services.NewDependencyService(db)
	schService := services.NewScheduleService(db)
	projService := services.NewProjectService(db, depService, schService)
	taskService := services.NewTaskService(db, depService, projService)

	// Wire promote callback: when a dependency project becomes Done,
	// re-evaluate dependent projects to advance their status
	depService.SetPromoteCallback(func(projectID uint) {
		projService.RecalculateProjectStatus(projectID)
	})

	// Initialize handlers
	projectHandler := handlers.NewProjectHandler(projService, depService)
	taskHandler := handlers.NewTaskHandler(taskService, depService)

	// Setup Gin router
	r := gin.Default()

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000", "*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// API Routes
	api := r.Group("/api")
	{
		// Project routes
		projects := api.Group("/projects")
		{
			projects.GET("", projectHandler.GetProjects)
			projects.GET("/:id", projectHandler.GetProject)
			projects.POST("", projectHandler.CreateProject)
			projects.PUT("/:id", projectHandler.UpdateProject)
			projects.DELETE("/:id", projectHandler.DeleteProject)

			// Project dependency routes
			projects.POST("/:id/dependencies", projectHandler.AddProjectDependency)
			projects.DELETE("/:id/dependencies/:depId", projectHandler.RemoveProjectDependency)
		}

		// Task routes
		tasks := api.Group("/tasks")
		{
			tasks.GET("", taskHandler.GetTasks)
			tasks.GET("/:id", taskHandler.GetTask)
			tasks.POST("", taskHandler.CreateTask)
			tasks.PUT("/:id", taskHandler.UpdateTask)
			tasks.DELETE("/:id", taskHandler.DeleteTask)

			// Task dependency routes
			tasks.POST("/:id/dependencies", taskHandler.AddTaskDependency)
			tasks.DELETE("/:id/dependencies/:depId", taskHandler.RemoveTaskDependency)
		}
	}

	// Start server
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s...", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
