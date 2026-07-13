package client

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"wso2/scripts/vars"
)


// This function executes a curl command to fetch the JSON object from the /apis endpoint.
func getApiJsonObject() []byte {
	jsonObject, err := newCurlCmd(vars.BaseURL+"/apis", "-k").Output()
	if err != nil {
		log.Fatal(err)
	}

	return jsonObject
}

// This function iterates through the JSON object and fetches the ID and name of each API.
func ExtractApiSummaries() []ApiSummary {
	jsonObject := getApiJsonObject()

	var data map[string]any
	if err := json.Unmarshal(jsonObject, &data); err != nil {
		log.Fatal(err)
	}

	list := data["list"].([]interface{})
	apis := make([]ApiSummary, 0, len(list))

	for _, item := range list {
		api, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		apiSummary := ApiSummary{}
		if id, ok := api["id"].(string); ok {
			apiSummary.ID = id
		}
		if name, ok := api["name"].(string); ok {
			apiSummary.Name = name
		}
		apis = append(apis, apiSummary)
	}

	return apis
}

// This function iterates through the JSON object and fetches the ID of each API.
func ExtractApiIds() []string {
	apiSummaries := ExtractApiSummaries()
	apiIds := make([]string, 0, len(apiSummaries))
	for _, api := range apiSummaries {
		if api.ID != "" {
			apiIds = append(apiIds, api.ID)
		}
	}
	return apiIds
}

// This function extracts the API IDs from the JSON object returned by the /apis endpoint.
func GetApiDetailsJsonObject(apiId string) []byte {
	jsonObject, err := newCurlCmd(vars.BaseURL+vars.EndpointAPI+apiId, "-k").Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
}

// This function iterates through the list of API IDs and fetches the details for each API, extracting the policies for each operation.
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

// This function fetches all common operation policies.
func getOperationPoliciesJsonObject() []byte {
	jsonObject, err := newCurlCmd(vars.BaseURL+"/operation-policies", "-k").Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
}

// This function extracts the policies and the metadata from the JSON objects.
func ExtractOperationPolicies() []map[string]interface{} {
	jsonObject := getOperationPoliciesJsonObject()

	var data map[string]any
	if err := json.Unmarshal(jsonObject, &data); err != nil {
		log.Fatal(err)
	}

	return normalizePolicyList(data)
}

// This function fetches API level policies.
func getApiLevelPoliciesJsonObject(apiId string) []byte {
	jsonObject, err := newCurlCmd(vars.BaseURL+vars.EndpointAPI+apiId+"/operation-policies", "-k").Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
}

// This function fetches and normalizes the API-specific policies for a single API.
func ExtractApiLevelPolicies(apiId string) []map[string]interface{} {
	jsonObject := getApiLevelPoliciesJsonObject(apiId)

	var data map[string]any
	if err := json.Unmarshal(jsonObject, &data); err != nil {
		log.Fatal(err)
	}

	return normalizePolicyList(data)
}

// This function sends the updated API JSON back to WSO2 via PUT /apis/{apiId}.
func PutApiUpdate(apiId string, payload []byte) error {
	cmd := newCurlCmd(
		"-X", "PUT",
		vars.BaseURL+vars.EndpointAPI+apiId,
		"-H", "Content-Type: application/json",
		"-d", "@-",
		"-k",
		"-s", "-w", "\nHTTP_STATUS:%{http_code}")

	cmd.Stdin = strings.NewReader(string(payload))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("PUT /apis/%s curl error: %v, output: %s", apiId, err, output)
	}

	statusCode, body, err := parseCurlStatus(output, fmt.Sprintf("PUT /apis/%s", apiId))
	if err != nil {
		return err
	}
	if strings.HasPrefix(statusCode, "2") {
		fmt.Printf("PUT /apis/%s: OK (HTTP %s)\n", apiId, statusCode)
		return nil
	}

	fmt.Printf("PUT /apis/%s: FAILED (HTTP %s) - %s\n", apiId, statusCode, body)
	return fmt.Errorf("HTTP %s", statusCode)
}
