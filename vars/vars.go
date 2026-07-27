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

	v := viper.New()
	v.SetConfigFile(ConfigPath)
	v.SetConfigType("yaml")

	// Set default values
	v.SetDefault("vhost", "localhost")
	v.SetDefault("username", "admin")
	v.SetDefault("password", "admin")
	v.SetDefault("base_url", "https://localhost:9443/api/am/publisher/v4")

	if _, err := os.Stat(ConfigPath); os.IsNotExist(err) {
		defaultConfigContent := []byte(`vhost: "localhost"
username: "admin"
password: "admin"
base_url: "https://localhost:9443/api/am/publisher/v4"
`)
		if err := os.WriteFile(ConfigPath, defaultConfigContent, 0644); err != nil {
			log.Printf("Warning: could not create default config file %s: %v", ConfigPath, err)
		} else {
			fmt.Printf("Created default config file at: %s\n", ConfigPath)
		}
	}

	if err := v.ReadInConfig(); err != nil {
		log.Printf("Error reading config file %s: %v", ConfigPath, err)
	}

	Vhost = v.GetString("vhost")
	Username = v.GetString("username")
	Password = v.GetString("password")
	BaseURL = v.GetString("base_url")
}
