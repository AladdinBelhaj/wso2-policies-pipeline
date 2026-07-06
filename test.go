package main

import (
"bufio"
"fmt"
"log"
"os"
"strings"
)

var baseUrl string
var username string
var password string

func main() {

setENV()

fmt.Println(baseUrl)

}

func setENV() {
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
baseUrl = os.Getenv("BASE_URL")
username = os.Getenv("USERNAME")
password = os.Getenv("PASSWORD")
}