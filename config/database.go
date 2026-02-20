package config

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"test-go-tasklist/models"
)

var (
	db   *gorm.DB
	once sync.Once
)

func InitDatabase() *gorm.DB {
	once.Do(func() {
		err := godotenv.Load()
		if err != nil {
			log.Println("Warning: .env file not found, using system environment variables")
		}

		host := getEnv("DB_HOST", "localhost")
		port := getEnv("DB_PORT", "3306")
		user := getEnv("DB_USERNAME", "root")
		password := getEnv("DB_PASSWORD", "")
		dbName := getEnv("DB_DATABASE", "tasklist-tech")

		// Step 1: Connect WITHOUT database to create it if not exists
		dsnNoDB := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
			user, password, host, port)

		tmpDB, err := gorm.Open(mysql.Open(dsnNoDB), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			log.Fatalf("Failed to connect to MySQL server: %v", err)
		}

		// Create database if not exists
		createSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbName)
		if err := tmpDB.Exec(createSQL).Error; err != nil {
			log.Fatalf("Failed to create database: %v", err)
		}
		log.Printf("Database '%s' ensured to exist", dbName)

		// Close temp connection
		sqlDB, _ := tmpDB.DB()
		sqlDB.Close()

		// Step 2: Connect WITH database
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			user, password, host, port, dbName)

		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}

		log.Println("Database connected successfully")

		err = db.AutoMigrate(
			&models.Project{},
			&models.Task{},
			&models.TaskDependency{},
			&models.ProjectDependency{},
		)
		if err != nil {
			log.Fatalf("Failed to auto-migrate: %v", err)
		}

		log.Println("Database migration completed")
	})

	return db
}

func GetDB() *gorm.DB {
	if db == nil {
		return InitDatabase()
	}
	return db
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
