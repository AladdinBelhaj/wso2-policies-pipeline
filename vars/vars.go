// This file fetches environment variables from the .env file and assigns them to global variables for use in the application.
package vars

import (
"bufio"
"log"
"os"
"strings"
)

var BaseUrl string
var Username string
var Password string
var Endpoint string

func Load() {
//	Open the .env file
envFile, err := os.Open("./.env")
//	check for errors
if err !=  nil {
log.Fatalln(err)
}

 //	defer closing the file until the function exits
defer envFile.Close()

// create a new scanner to read each row
scanner := bufio.NewScanner(envFile)

//	 loop through each row of the .env file
for scanner.Scan() {
//	split the text on the equal sign to get the key and value
envVar := strings.Split(scanner.Text(), "=")
//	os.Setenv takes in a key and a value which are both strings
os.Setenv(envVar[0], envVar[1])
}
//	check for errors with scanner.Scan
if err := scanner.Err(); err !=  nil {
log.Fatal(err)
}
// assign the environment variable using the os.Getenv method
BaseUrl = os.Getenv("BASE_URL")
Username = os.Getenv("USERNAME")
Password = os.Getenv("PASSWORD")
Endpoint = os.Getenv("Endpoint")
}