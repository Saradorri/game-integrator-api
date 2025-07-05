package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/saradorri/gameintegrator/internal/config"
	"github.com/saradorri/gameintegrator/internal/infrastructure/database"
	"github.com/saradorri/gameintegrator/internal/infrastructure/repository"
	"github.com/saradorri/gameintegrator/internal/infrastructure/seeder"
	"github.com/spf13/viper"
)

func main() {
	var (
		configPath = flag.String("config", "./config", "Path to config directory")
		configFile = flag.String("env", "development", "Environment")
	)
	flag.Parse()

	cfg, err := loadConfig(*configPath, *configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.NewDatabase(&database.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		Name:            cfg.Database.Name,
		SSLMode:         cfg.Database.SSLMode,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	userRepo := repository.NewUserRepository(db.DB)
	newSeeder := seeder.NewSeeder(userRepo)

	log.Println("Starting database seeding...")
	if err := newSeeder.SeedUsers(); err != nil {
		log.Fatalf("Failed to seed users: %v", err)
	}
	log.Println("Database seeding completed successfully")
}

// loadConfig loads configuration from file
func loadConfig(configPath, configFile string) (*config.Config, error) {
	viper.SetConfigName(fmt.Sprintf("config.%s", configFile))
	viper.SetConfigType("yml")
	viper.AddConfigPath(configPath)

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	var cfg config.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("could not unmarshal config: %w", err)
	}

	return &cfg, nil
}
