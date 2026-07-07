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
    
	apiIds := extractApiIds()

	extractApiPolicies(apiIds)

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

func extractApiPolicies(apiIds []string) {

	 apiPolicies := make([]map[string]any, 0)

	for _, apiId := range apiIds {
		var api map[string]any
		data := getApiDetailsJsonObject(apiId)
		err := json.Unmarshal(data, &api)
		if err != nil {
			log.Fatal(err)
		}

		operations := make([]map[string]any, 0)

		for _, operation := range api["operations"].([]interface{}) {
			op := operation.(map[string]interface{})

			operationObject := map[string]any{
				"target":            op["target"],
				"verb":              op["verb"],
				"operationPolicies": op["operationPolicies"],
			}

			operations = append(operations, operationObject)
		}

		apiObject := map[string]any{
			"apiId":      api["id"],
			"operations": operations,
		}

		apiPolicies = append(apiPolicies, apiObject)
	}

	output, err := json.MarshalIndent(apiPolicies, "", "  ")
if err != nil {
    log.Fatal(err)
}

fmt.Println(string(output))


}

