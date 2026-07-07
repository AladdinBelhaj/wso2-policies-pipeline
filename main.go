// This file executes fetches each API's details and extracts the policies for each operation
package main

import (
	"fmt"
	"log"
	"os/exec"
	"encoding/json"
	"wso2/scripts/vars"
)
func main() {

	vars.Load()
	// apiIds := extractApiIds()
	// apiPolicies := extractApiPolicies(apiIds)

	// marshalledData, err := json.MarshalIndent(apiPolicies, "", "  ")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// fmt.Println(string(marshalledData))
	updateOperationPolicies()

}

// This function executes a curl command to fetch the JSON object from the /apis endpoint
func getApiJsonObject() []byte {

    cmd := exec.Command("curl", "-u", vars.Username + ":" + vars.Password, vars.BaseUrl + "/apis", "-k")
	jsonObject, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	return jsonObject
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


// This function extracts the API IDs from the JSON object returned by the /apis endpoint
func getApiDetailsJsonObject(apiId string) []byte {

    cmd := exec.Command("curl", "-u", vars.Username + ":" + vars.Password, vars.BaseUrl + "/apis/" + apiId, "-k")
	jsonObject, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
}

// This function iterates through the list of API IDs and fetches the details for each API, extracting the policies for each operation
func extractApiPolicies(apiIds []string) []map[string]any{

	apiPolicies := make([]map[string]any, 0)

	for _, apiId := range apiIds {
		var api map[string]any
		data := getApiDetailsJsonObject(apiId)
		err := json.Unmarshal(data, &api)
		if err != nil {
			log.Fatal(err)
		}

	}

	return apiPolicies

}



func getOperationPoliciesJsonObject() []byte{

	cmd := exec.Command("curl", "-u", vars.Username + ":" + vars.Password, vars.BaseUrl + "/operation-policies", "-k")
	jsonObject, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
}

func extractOperationPolicies() []map[string]interface{} {
	jsonObject := getOperationPoliciesJsonObject()

	var data map[string]any

	err := json.Unmarshal(jsonObject, &data)
	if err != nil {
		log.Fatal(err)
	}

	list := data["list"].([]interface{})

	var policies []map[string]interface{}

	for _, item := range list {
		policy := item.(map[string]interface{})

		policies = append(policies, map[string]interface{}{
			"id":      policy["id"],
			"name":    policy["name"],
			"type":    policy["type"],
			"version": policy["version"],
		})

		fmt.Println(policy["id"], policy["name"], policy["type"], policy["version"])
	}

	return policies
}