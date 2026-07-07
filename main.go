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

// Load the environment variables from the .env file
	vars.Load()

    getApiJsonObject()

	apiIds := extractApiIds()

	apiDetails := make([][]byte, 0)

	for _, apiId := range apiIds {
		apiDetails = append(apiDetails, getApiDetailsJsonObject(apiId))
	}
    


}

// This function iterates through the JSON object and fetches the ID of each API
func extractApiIds() []string {


	jsonObject := getApiJsonObject()
    var data map[string]any

	apiIds := make([]string, 0)


    err := json.Unmarshal(jsonObject, &data)

	if err != nil {
		log.Fatal(err)
	}


	list := data["list"].([]interface{})

	for _, item := range list {
		api := item.(map[string]interface{})

		apiIds = append(apiIds, api["id"].(string))
	}

	return apiIds
}

func getApiJsonObject() []byte {

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


func getApiDetailsJsonObject(apiId string) []byte {


	// Create a new command to execute the curl command with the environment variables
    cmd := exec.Command("curl", "-u", vars.Username + ":" + vars.Password, vars.BaseUrl + "/apis/" + apiId, "-k")
	// Execute the command and capture the output
	jsonObject, err := cmd.Output()
	// Check for errors and log them if any
	if err != nil {
		log.Fatal(err)
	}

	return jsonObject

}

func extractApiPolicies(apiId string) []string {

	jsonObject := getApiDetailsJsonObject(apiId)
	var data map[string]any

	err := json.Unmarshal(jsonObject, &data)
	if err != nil {
		log.Fatal(err)
	}

	list := data["list"].([]interface{})

	apiPolicies := make([]string, 0)

	for _, item := range list {
		api := item.(map[string]interface{})
		apiPolicies = append(apiPolicies, api["policies"].([]string)...)
	}

	return apiPolicies
}