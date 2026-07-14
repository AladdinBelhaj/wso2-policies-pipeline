// This package handles resolving and updating operation policies for WSO2 APIs.
package policy

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"wso2/scripts/libraries/client"
)

var policyFlows = []string{"request", "response", "fault"}

// This function fetches each API, resolves policy IDs by name, and PUTs the result back.
func UpdateApiPolicies(apiPolicies []map[string]any, allPolicies []map[string]interface{}) {
	for _, apiEntry := range apiPolicies {
		apiId := apiEntry["apiId"].(string)
		processSingleApi(apiId, allPolicies)
	}
}

func processSingleApi(apiId string, allPolicies []map[string]interface{}) {
	var apiDetail map[string]any
	if err := json.Unmarshal(client.GetApiDetailsJsonObject(apiId), &apiDetail); err != nil {
		log.Fatal(err)
	}

	apiScopedPolicies := append(
		append([]map[string]interface{}{}, allPolicies...),
		client.ExtractApiLevelPolicies(apiId)...,
	)

	modified := updateApiLevelPoliciesBlock(apiDetail, apiScopedPolicies)
	if updateOperationsPoliciesBlock(apiDetail, apiScopedPolicies) {
		modified = true
	}

	if !modified {
		log.Printf("No changes detected for API %s. Skipping update and deployment.", apiId)
		return
	}

	updatedJson, err := json.Marshal(apiDetail)
	if err != nil {
		log.Fatal(err)
	}
	if err := client.PutApiUpdate(apiId, updatedJson); err != nil {
		log.Printf("failed to update API %s: %v", apiId, err)
		return
	}

	client.PrepareAndDeployRevision(apiId)
}

func updateApiLevelPoliciesBlock(apiDetail map[string]any, policies []map[string]interface{}) bool {
	apiPoliciesBlock, ok := apiDetail["apiPolicies"].(map[string]interface{})
	if !ok {
		return false
	}

	modified := false
	for _, flow := range policyFlows {
		if resolvePoliciesInFlow(apiPoliciesBlock, flow, policies) {
			modified = true
		}
	}
	return modified
}

func updateOperationsPoliciesBlock(apiDetail map[string]any, policies []map[string]interface{}) bool {
	operations, ok := apiDetail["operations"].([]interface{})
	if !ok {
		return false
	}

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
		for _, flow := range policyFlows {
			if resolvePoliciesInFlow(opPolicies, flow, policies) {
				modified = true
			}
		}
	}
	return modified
}

// This function matches each policy in a single flow (request for example) by name
// against the shared policies list and injects the resolved policyId and policyVersion.
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

		if policyId, policyVersion, found := findNewestPolicyByName(policyName, allPolicies); found {
			currentId, _ := pol["policyId"].(string)
			currentVersion, _ := pol["policyVersion"].(string)

			if currentVersion != policyVersion {
				log.Printf(
					"Updating policy %s: %s/%s -> %s/%s",
					policyName,
					currentId,
					currentVersion,
					policyId,
					policyVersion,
				)

				pol["policyId"] = policyId
				pol["policyVersion"] = policyVersion
				changed = true
			}
		}
	}

	return changed
}

// This function looks up a policy by name in the shared operation-policies list
// and returns the id and version of the newest version found.
func findNewestPolicyByName(name string, allPolicies []map[string]interface{}) (string, string, bool) {
	latestId := ""
	latestVersion := ""
	latestNumber := -1

	for _, policy := range allPolicies {
		if policy["name"] != name {
			continue
		}

		version, ok := policy["version"].(string)
		if !ok {
			continue
		}

		num, err := strconv.Atoi(strings.TrimPrefix(version, "v"))
		if err != nil {
			continue
		}

		if num > latestNumber {
			latestNumber = num
			latestVersion = version
			latestId, _ = policy["id"].(string)
		}
	}

	if latestId == "" {
		return "", "", false
	}

	return latestId, latestVersion, true
}
