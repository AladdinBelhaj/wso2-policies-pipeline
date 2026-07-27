package vars

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

var BaseURL string
var Username string
var Password string
var Vhost string

func Load() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	BaseURL = os.Getenv("BASE_URL")
	Username = os.Getenv("WSO2_USERNAME")
	Password = os.Getenv("WSO2_PASSWORD")
	Vhost = os.Getenv("VHOST")
}
