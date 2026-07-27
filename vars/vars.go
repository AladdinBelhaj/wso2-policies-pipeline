// This file fetches environment variables from the .env file and assigns them to global variables for use in the application.
package vars

import (
	"bufio"
	"os"
	"strings"
)

var BaseURL string
var Username string
var Password string
var Vhost string

func Load() {
	// Open the .env file if it exists
	if envFile, err := os.Open("./.env"); err == nil {
		defer envFile.Close()
		scanner := bufio.NewScanner(envFile)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			envVar := strings.SplitN(line, "=", 2)
			if len(envVar) == 2 {
				os.Setenv(envVar[0], envVar[1])
			}
		}
	}

	// assign environment variables
	BaseURL = os.Getenv("BASE_URL")
	Username = os.Getenv("USERNAME")
	Password = os.Getenv("PASSWORD")
	Vhost = os.Getenv("VHOST")
}
