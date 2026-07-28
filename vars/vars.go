package vars

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

// Env holds the connection settings for a single named environment.
type Env struct {
	Vhost    string `mapstructure:"vhost"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	BaseURL  string `mapstructure:"base_url"`
}

var (
	BaseURL    string
	Username   string
	Password   string
	Vhost      string
	ConfigPath string
	CurrentEnv string

	Environments map[string]Env

	// EnvOverride is set (e.g. from the -e/--env global flag) before
	// ResolveEnv() runs. When non-empty it takes precedence over the
	// persisted current_env value, for a single invocation, without
	// changing what's saved on disk.
	EnvOverride string

	v *viper.Viper
)

// LoadEnvironments reads config.yaml and populates Environments and the
// persisted current_env selection (if any). It does NOT fail if no
// environment is currently selected - that's ResolveEnv's job - so that
// commands like `set-env` can run against a freshly multi-env config before
// any environment has been chosen.
func LoadEnvironments() {
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

	v = viper.New()
	v.SetConfigFile(ConfigPath)
	v.SetConfigType("yaml")

	if _, err := os.Stat(ConfigPath); os.IsNotExist(err) {
		defaultConfigContent := []byte(`current_env: dev
environments:
  dev:
    vhost: "localhost"
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
		log.Fatalf("Error reading config file %s: %v", ConfigPath, err)
	}

	migrateFlatConfigIfNeeded(v)

	if err := v.UnmarshalKey("environments", &Environments); err != nil {
		log.Fatalf("failed to parse environments in config file %s: %v", ConfigPath, err)
	}

	if len(Environments) == 0 {
		log.Fatalf("no environments defined in config file %s", ConfigPath)
	}

	CurrentEnv = v.GetString("current_env")
}

// migrateFlatConfigIfNeeded detects the old single-environment config
// format (top-level vhost/username/password/base_url, no "environments"
// key) and rewrites the file in place, wrapping those values into a single
// "dev" environment. This runs once; after migration the file uses the
// multi-env format on every subsequent read.
func migrateFlatConfigIfNeeded(v *viper.Viper) {
	if v.IsSet("environments") {
		return
	}
	if !v.IsSet("base_url") {
		return
	}

	log.Printf("Migrating legacy config file %s to multi-environment format (wrapped as \"dev\")", ConfigPath)

	v.Set("environments", map[string]interface{}{
		"dev": map[string]interface{}{
			"vhost":    v.GetString("vhost"),
			"username": v.GetString("username"),
			"password": v.GetString("password"),
			"base_url": v.GetString("base_url"),
		},
	})
	v.Set("current_env", "dev")

	// Legacy top-level keys are left in place (harmless / ignored going
	// forward) since Viper's WriteConfig doesn't support deleting keys.
	if err := v.WriteConfig(); err != nil {
		log.Printf("Warning: failed to persist migrated config to %s: %v", ConfigPath, err)
	}
}

// ResolveEnv determines which environment to use - EnvOverride, then the
// persisted current_env, then (if there's exactly one environment defined)
// that environment automatically - and populates BaseURL/Username/
// Password/Vhost accordingly. It returns an error rather than fataling so
// callers can decide how to present the failure.
func ResolveEnv() error {
	name, err := resolveEnvName()
	if err != nil {
		return err
	}

	env := Environments[name]
	CurrentEnv = name
	Vhost = env.Vhost
	Username = env.Username
	Password = env.Password
	BaseURL = env.BaseURL

	return nil
}

func resolveEnvName() (string, error) {
	if EnvOverride != "" {
		if _, ok := Environments[EnvOverride]; !ok {
			return "", fmt.Errorf("environment %q (from -e/--env) not found; available environments: %s", EnvOverride, strings.Join(ListEnvNames(), ", "))
		}
		return EnvOverride, nil
	}

	if CurrentEnv != "" {
		if _, ok := Environments[CurrentEnv]; !ok {
			return "", fmt.Errorf("current_env %q in %s is not a defined environment; available environments: %s", CurrentEnv, ConfigPath, strings.Join(ListEnvNames(), ", "))
		}
		return CurrentEnv, nil
	}

	if len(Environments) == 1 {
		for name := range Environments {
			return name, nil
		}
	}

	return "", fmt.Errorf(
		"multiple environments defined (%s) but none selected. Run `pctl set-env [env_name]` to persist a default, or pass -e [env_name] to this command",
		strings.Join(ListEnvNames(), ", "),
	)
}

// SetEnv validates that name is a defined environment, persists it as
// current_env in the config file, and updates the in-memory selection so
// it's immediately usable without needing to reload.
func SetEnv(name string) error {
	if _, ok := Environments[name]; !ok {
		return fmt.Errorf("environment %q not found; available environments: %s", name, strings.Join(ListEnvNames(), ", "))
	}

	v.Set("current_env", name)
	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("failed to persist current_env to %s: %w", ConfigPath, err)
	}

	CurrentEnv = name
	env := Environments[name]
	Vhost = env.Vhost
	Username = env.Username
	Password = env.Password
	BaseURL = env.BaseURL

	return nil
}

// ListEnvNames returns the sorted list of defined environment names.
func ListEnvNames() []string {
	names := make([]string, 0, len(Environments))
	for name := range Environments {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
