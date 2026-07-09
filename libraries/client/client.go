// This package handles all HTTP calls to the WSO2 Publisher API.
package client

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"wso2/scripts/vars"
)

// This function executes a curl command to fetch the JSON object from the /apis endpoint
func getApiJsonObject() []byte {

	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password, vars.BaseUrl+"/apis", "-k")
	jsonObject, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	return jsonObject
}

// This function iterates through the JSON object and fetches the ID of each API
func ExtractApiIds() []string {

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
func GetApiDetailsJsonObject(apiId string) []byte {

	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password, vars.BaseUrl+"/apis/"+apiId, "-k")
	jsonObject, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
}

// This function iterates through the list of API IDs and fetches the details for each API, extracting the policies for each operation
func ExtractApiPolicies(apiIds []string) []map[string]any {

	apiPolicies := make([]map[string]any, 0)

	for _, apiId := range apiIds {
		var api map[string]any
		data := GetApiDetailsJsonObject(apiId)
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

	return apiPolicies

}

func getOperationPoliciesJsonObject() []byte {

	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password, vars.BaseUrl+"/operation-policies", "-k")
	jsonObject, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
}

func ExtractOperationPolicies() []map[string]interface{} {
	jsonObject := getOperationPoliciesJsonObject()

	var data map[string]any

	err := json.Unmarshal(jsonObject, &data)
	if err != nil {
		log.Fatal(err)
	}

	list := data["list"].([]interface{})

	var allPolicies []map[string]interface{}

	for _, item := range list {
		policy := item.(map[string]interface{})

		allPolicies = append(allPolicies, map[string]interface{}{
			"id":      policy["id"],
			"name":    policy["name"],
			"version": policy["version"],
		})

	}

	return allPolicies
}

// This function sends the updated API JSON back to WSO2 via PUT /apis/{apiId}.
func PutApiUpdate(apiId string, payload []byte) error {
	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password,
		"-X", "PUT",
		vars.BaseUrl+"/apis/"+apiId,
		"-H", "Content-Type: application/json",
		"-d", "@-",
		"-k",
		"-s", "-w", "\nHTTP_STATUS:%{http_code}")

	cmd.Stdin = strings.NewReader(string(payload))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("PUT /apis/%s curl error: %v, output: %s", apiId, err, output)
	}

	outStr := string(output)
	idx := strings.LastIndex(outStr, "HTTP_STATUS:")
	if idx == -1 {
		return fmt.Errorf("PUT /apis/%s: no status code in output: %s", apiId, outStr)
	}

	statusCode := strings.TrimSpace(outStr[idx+len("HTTP_STATUS:"):])
	if strings.HasPrefix(statusCode, "2") {
		fmt.Printf("PUT /apis/%s: OK (HTTP %s)\n", apiId, statusCode)
		return nil
	}

	body := strings.TrimSpace(outStr[:idx])
	fmt.Printf("PUT /apis/%s: FAILED (HTTP %s) - %s\n", apiId, statusCode, body)
	return fmt.Errorf("HTTP %s", statusCode)
}


func ReviewRevisionsNumber(apiId string) {
	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password, vars.BaseUrl+ "/apis/" + apiId + "/revisions", "-k")
	jsonObject, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	
	var data map[string]any

	json.Unmarshal(jsonObject, &data)

	// count := data["count"].(float64)

	// if count == 5 {
	// 	return true
	// } else{
	// 	return false
	// }

	list := data["list"].([]interface{})
	fmt.Println(list[0].(map[string]interface{})["id"])
}


// func deleteRevision(apiId string, revisionId string) error {

// }

