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

    getApiJsonObject()

	fetchAPIs()



}

// This function iterates through the JSON object and fetches the ID of each API
func fetchAPIs() {


	jsonObject := getApiJsonObject()
    var data map[string]any

	apis := make([]string, 0)


    err := json.Unmarshal(jsonObject, &data)

	if err != nil {
		log.Fatal(err)
	}


	list := data["list"].([]interface{})

	for _, item := range list {
		api := item.(map[string]interface{})

		apis = append(apis, api["id"].(string))
	}

	fmt.Println(apis)

}

func getApiJsonObject() []byte {

	// Load the environment variables from the .env file
	vars.Load()
	// Create a new command to execute the curl command with the environment variables
    cmd := exec.Command("curl", "-u", vars.Username + ":" + vars.Password, vars.BaseUrl + "/apis", "-k")
	// Execute the command and capture the output
	jsonObject, err := cmd.Output()
	// Check for errors and log them if any
	if err != nil {
		log.Fatal(err)
	}

	return jsonObject
}