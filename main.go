// This file executes fetches each API's details and extracts the policies for each operation
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"encoding/json"
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


// updateApiPolicies iterates every API's operations (request/response/fault flows),
// matches each operation-level policy by name against the tenant's shared policies,
// injects the resolved policyId, and PUTs the updated API back to WSO2.
func updateApiPolicies(apiPolicies []map[string]any, allPolicies []map[string]interface{}) {
	for _, apiEntry := range apiPolicies {
		apiId := apiEntry["apiId"].(string)

		// Fetch a fresh, full copy of the API — this is what we'll mutate and PUT back.
		data := getApiDetailsJsonObject(apiId)

		var apiDetail map[string]any
		if err := json.Unmarshal(data, &apiDetail); err != nil {
			log.Fatal(err)
		}

		operations, ok := apiDetail["operations"].([]interface{})
		if !ok {
			continue
		}

		extractedOps, _ := apiEntry["operations"].([]map[string]any)
		modified := false

		for _, extractedOp := range extractedOps {
			target := extractedOp["target"]
			verb := extractedOp["verb"]

			// Find the matching operation inside the freshly fetched apiDetail
			for _, opRaw := range operations {
				op, ok := opRaw.(map[string]interface{})
				if !ok {
					continue
				}
				if op["target"] != target || op["verb"] != verb {
					continue
				}

				opPolicies, ok := op["operationPolicies"].(map[string]interface{})
				if !ok {
					break
				}

				for _, flow := range []string{"request", "response", "fault"} {
					flowList, ok := opPolicies[flow].([]interface{})
					if !ok {
						continue
					}

					for _, polRaw := range flowList {
						pol, ok := polRaw.(map[string]interface{})
						if !ok {
							continue
						}

						policyName, ok := pol["policyName"].(string)
						if !ok {
							continue
						}

						policyId, found := findPolicyIdByName(policyName, allPolicies)
						if !found {
							continue // no equivalent shared policy, skip to next
						}

						pol["policyId"] = policyId
						modified = true
					}
				}

				break // matched operation processed, stop scanning operations
			}
		}

		if !modified {
			continue // nothing changed for this API, don't bother with a PUT
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

// findPolicyIdByName looks up a policy by name in the shared operation-policies list
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

// putApiUpdate sends the updated API JSON back to WSO2 via PUT /apis/{apiId}
func putApiUpdate(apiId string, payload []byte) error {
	tmpFile, err := os.CreateTemp("", "api-update-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(payload); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password,
		"-X", "PUT",
		vars.BaseUrl+"/apis/"+apiId,
		"-H", "Content-Type: application/json",
		"-d", "@"+tmpFile.Name(),
		"-k")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("curl error: %v, output: %s", err, output)
	}

	return nil
}