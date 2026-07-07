// This file executes a curl command to fetch APIs from the WSO2 API Manager using environment variables for authentication.
package main

import (
	"fmt"
	"log"
	"os/exec"
	"encoding/json"
	"wso2/scripts/vars"
)
func main() {

	var data map[string]any
// Load the environment variables from the .env file
	vars.Load()
// Create a new command to execute the curl command with the environment variables
    cmd := exec.Command("curl", "-u", vars.Username + ":" + vars.Password, vars.BaseUrl + "/apis", "-k")
// Execute the command and capture the output
	out, err := cmd.Output()
// Check for errors and log them if any
	if err != nil {
		log.Fatal(err)
	}
	
	err := json.Unmarshal(out, &data)

	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(data)

}