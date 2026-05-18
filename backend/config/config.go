package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Server struct {
		Port      int    `yaml:"port" mapstructure:"port"`
		JWTSecret string `yaml:"jwtSecret" mapstructure:"jwtSecret"`
	} `yaml:"server" mapstructure:"server"`
	XUI struct {
		BaseURL  string `yaml:"baseURL" mapstructure:"baseURL"`
		APIToken string `yaml:"apiToken" mapstructure:"apiToken"`
		BasePath string `yaml:"basePath" mapstructure:"basePath"`
	} `yaml:"xui" mapstructure:"xui"`
	Database struct {
		Path string `yaml:"path" mapstructure:"path"`
	} `yaml:"database" mapstructure:"database"`
}

var C Config

// Load reads configuration from file and environment variables.
func Load(cfgFile string) {
	v := viper.New()

	// Defaults
	v.SetDefault("server.port", 9090)
	v.SetDefault("server.jwtSecret", "change-me-in-production")
	v.SetDefault("xui.baseURL", "http://localhost:2053")
	v.SetDefault("xui.apiToken", "")
	v.SetDefault("xui.basePath", "")
	v.SetDefault("database.path", "./data/dashboard.db")

	// Config file
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./config")
		v.AddConfigPath("./deploy")
		v.AddConfigPath(".")
	}

	// Env overrides: XUI_BASEURL, SERVER_PORT, etc.
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Printf("Warning: error reading config file: %v", err)
		}
	}

	if err := v.Unmarshal(&C); err != nil {
		log.Fatalf("Failed to unmarshal config: %v", err)
	}
}
