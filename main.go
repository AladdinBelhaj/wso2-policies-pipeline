// This file executes fetches each API's details and extracts the policies for each operation
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"wso2/scripts/vars"
)

func main() {

	vars.Load()
	apiIds := extractApiIds()
	apiPolicies := extractApiPolicies(apiIds)
	allPolicies := extractOperationPolicies()

	updateApiPolicies(apiPolicies, allPolicies)
}

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

	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password, vars.BaseUrl+"/apis/"+apiId, "-k")
	jsonObject, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
}

// This function iterates through the list of API IDs and fetches the details for each API, extracting the policies for each operation
func extractApiPolicies(apiIds []string) []map[string]any {

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

func extractOperationPolicies() []map[string]interface{} {
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

// This function fetches each API, resolves policy IDs by name, and PUTs the result back.
func updateApiPolicies(apiPolicies []map[string]any, allPolicies []map[string]interface{}) {
	for _, apiEntry := range apiPolicies {
		apiId := apiEntry["apiId"].(string)

		// Fetch the full API object, will be mutated then PUT back
		var apiDetail map[string]any
		if err := json.Unmarshal(getApiDetailsJsonObject(apiId), &apiDetail); err != nil {
			log.Fatal(err)
		}

		operations, ok := apiDetail["operations"].([]interface{})
		if !ok {
			continue
		}

		// Walk every operation and resolve policy IDs in each flow.
		modified := false
		for _, opRaw := range operations {
			op, ok := opRaw.(map[string]interface{})
			if !ok {
				continue
			}
			opPolicies, ok := op["operationPolicies"].(map[string]interface{})
			if !ok {
				continue
			}
			for _, flow := range []string{"request", "response", "fault"} {
				if resolvePoliciesInFlow(opPolicies, flow, allPolicies) {
					modified = true
				}
			}
		}

		if !modified {
			continue
		}

		updatedJson, err := json.Marshal(apiDetail)
		if err != nil {
			log.Fatal(err)
		}
		if err := putApiUpdate(apiId, updatedJson); err != nil {
			log.Printf("failed to update API %s: %v", apiId, err)
		}
	}
}

// This function matches each policy in a single flow (request for example) by name
// against the shared policies list and injects the resolved policyId.
// Returns true if any policy was updated.
func resolvePoliciesInFlow(opPolicies map[string]interface{}, flow string, allPolicies []map[string]interface{}) bool {
	flowList, ok := opPolicies[flow].([]interface{})
	if !ok {
		return false
	}

	changed := false
	for _, polRaw := range flowList {
		pol, ok := polRaw.(map[string]interface{})
		if !ok {
			continue
		}
		policyName, ok := pol["policyName"].(string)
		if !ok {
			continue
		}
		if policyId, found := findPolicyIdByName(policyName, allPolicies); found {
			pol["policyId"] = policyId
			changed = true
		}
	}
	return changed
}

// This function looks up a policy by name in the shared operation-policies list
// and returns its id if found.
func findPolicyIdByName(name string, allPolicies []map[string]interface{}) (string, bool) {
	for _, policy := range allPolicies {
		if policy["name"] == name {
			id, ok := policy["id"].(string)
			return id, ok
		}
	}
	return "", false
}

// This function sends the updated API JSON back to WSO2 via PUT /apis/{apiId}.
func putApiUpdate(apiId string, payload []byte) error {
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
