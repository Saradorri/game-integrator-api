package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Wallet   WalletConfig   `mapstructure:"wallet"`
	Log      LogConfig      `mapstructure:"log"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
}

// DatabaseConfig holds database-related configuration
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Name            string        `mapstructure:"name"`
	SSLMode         string        `mapstructure:"ssl"`
	MaxIdleConns    int           `mapstructure:"maxIdleConns"`
	MaxOpenConns    int           `mapstructure:"maxOpenConns"`
	ConnMaxLifetime time.Duration `mapstructure:"connMaxLifetime"`
}

// JWTConfig holds JWT-related configuration
type JWTConfig struct {
	Secret string        `mapstructure:"secret"`
	Expiry time.Duration `mapstructure:"expiry"`
}

// WalletConfig holds wallet service configuration
type WalletConfig struct {
	URL    string `mapstructure:"url"`
	APIKey string `mapstructure:"api_key"`
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level string `mapstructure:"level"`
}

// GetDSN returns the database connection string
func (c *Config) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.Name,
		c.Database.SSLMode,
	)
}

// GetServerAddress returns the server address for binding
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%s", c.Server.Host, c.Server.Port)
}

// GetEnvironment returns the current environment
func GetEnvironment() string {
	if env := os.Getenv("GAME_INTEGRATOR_ENV"); env != "" {
		return env
	}
	if env := os.Getenv("ENV"); env != "" {
		return env
	}
	return "development"
}
