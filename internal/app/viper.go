package app

import (
	"fmt"
	"github.com/saradorri/gameintegrator/internal/config"
	"github.com/spf13/viper"
	"strings"
)

func (a *application) setupViper(path string) error {
	// Get environment (default to development)
	env := config.GetEnvironment()

	viper.SetConfigName(fmt.Sprintf("config.%s", env))
	viper.SetConfigType("yml")

	viper.AddConfigPath(path)

	// Enable environment variable override
	viper.AutomaticEnv()
	viper.SetEnvPrefix("GAME_INTEGRATOR")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	err := viper.ReadInConfig()
	if err != nil {
		return fmt.Errorf("could not read config file: %w", err)
	}

	var c config.Config
	err = viper.Unmarshal(&c)
	if err != nil {
		return err
	}
	a.config = &c

	fmt.Println("[x] Config loaded successfully")
	return nil
}
