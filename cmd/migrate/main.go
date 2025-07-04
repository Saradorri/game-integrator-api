package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/viper"
)

// Config holds database configuration
type Config struct {
	Database DatabaseConfig `mapstructure:"database"`
}

// DatabaseConfig holds database-related configuration
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"ssl"`
}

func main() {
	var (
		configPath = flag.String("config", "./config", "Path to config directory")
		configFile = flag.String("env", "development", "Environment (development, production)")
		action     = flag.String("action", "up", "Migration action: up, down")
	)
	flag.Parse()

	config, err := loadConfig(*configPath, *configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	migrationsPath := "./migrations"
	if err := validateMigrationsPath(migrationsPath); err != nil {
		log.Fatalf("Failed to validate migrations path: %v", err)
	}

	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			config.Database.User,
			config.Database.Password,
			config.Database.Host,
			config.Database.Port,
			config.Database.Name,
			config.Database.SSLMode,
		),
	)
	if err != nil {
		log.Fatalf("Failed to create migration instance: %v", err)
	}
	defer m.Close()

	switch *action {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("Failed to migrate up: %v", err)
		}
		fmt.Println("Successfully migrated up")
	case "down":
		if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("Failed to migrate down: %v", err)
		}
		fmt.Println("Successfully migrated down")
	default:
		log.Fatalf("Unknown action: %s. Valid actions: up, down", *action)
	}
}

// loadConfig loads configuration from file
func loadConfig(configPath, configFile string) (*Config, error) {
	viper.SetConfigName(fmt.Sprintf("config.%s", configFile))
	viper.SetConfigType("yml")
	viper.AddConfigPath(configPath)

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("could not unmarshal config: %w", err)
	}

	return &config, nil
}

// validateMigrationsPath checks if the migrations directory exists and contains migration files
func validateMigrationsPath(migrationsPath string) error {
	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		return fmt.Errorf("migrations directory does not exist: %s", migrationsPath)
	}

	files, err := filepath.Glob(filepath.Join(migrationsPath, "*.sql"))
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no migration files found in directory: %s", migrationsPath)
	}

	fmt.Printf("Found %d migration files in %s\n", len(files), migrationsPath)
	return nil
}
