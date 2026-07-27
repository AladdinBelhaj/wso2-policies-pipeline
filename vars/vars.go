package vars

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

var BaseURL string
var Username string
var Password string
var Vhost string
var ConfigPath string

func Load() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Warning: unable to get home directory: %v", err)
		home = "."
	}

	var configDir string
	if runtime.GOOS == "windows" {
		configDir = filepath.Join(home, ".pctl")
	} else {
		configDir = filepath.Join(home, ".config", "pctl")
	}

	ConfigPath = filepath.Join(configDir, "config.yaml")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		log.Printf("Warning: could not create config directory %s: %v", configDir, err)
	}

	viper.SetConfigFile(ConfigPath)
	viper.SetConfigType("yaml")

	// Set default values
	viper.SetDefault("vhost", "localhost:9443")
	viper.SetDefault("username", "admin")
	viper.SetDefault("password", "admin")
	viper.SetDefault("base_url", "https://localhost:9443")

	// Enable environment variable override
	viper.AutomaticEnv()

	if _, err := os.Stat(ConfigPath); os.IsNotExist(err) {
		defaultConfigContent := []byte(`vhost: "localhost:9443"
username: "admin"
password: "admin"
base_url: "https://localhost:9443"
`)
		if err := os.WriteFile(ConfigPath, defaultConfigContent, 0644); err != nil {
			log.Printf("Warning: could not create default config file %s: %v", ConfigPath, err)
		} else {
			fmt.Printf("Created default config file at: %s\n", ConfigPath)
		}
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Error reading config file %s: %v", ConfigPath, err)
	}

	Vhost = viper.GetString("vhost")
	Username = viper.GetString("username")
	Password = viper.GetString("password")
	BaseURL = viper.GetString("base_url")
}
