// This package handles resolving and updating operation policies for WSO2 APIs.
package policy

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"wso2/scripts/libraries/client"
)

const (
	LevelAPI       = "API level"
	LevelOperation = "Operation level"
)

var policyFlows = []string{"request", "response", "fault"}

// This function iterates over the already-fetched API details, resolves policy IDs by name, and PUTs the result back.
func UpdateApiPolicies(apiDetails map[string]map[string]any, allPolicies []map[string]interface{}) {
	for apiId, apiDetail := range apiDetails {
		processSingleApi(apiId, apiDetail, allPolicies)
	}
}

func processSingleApi(apiId string, apiDetail map[string]any, allPolicies []map[string]interface{}) {
	apiName, ok := apiDetail["name"].(string)
	if !ok {
		log.Println("API name not found")
		return
	}

	fmt.Printf("Updating API: %s\n", apiName)

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
		if resolvePoliciesInFlow(apiPoliciesBlock, flow, LevelAPI, policies) {
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
			if resolvePoliciesInFlow(opPolicies, flow, LevelOperation, policies) {
				modified = true
			}
		}
	}
	return modified
}

// This function matches each policy in a single flow (request for example)
// by name against the shared policies list and injects the resolved policyId
// and policyVersion.
// Returns true if any policy version was updated.
func resolvePoliciesInFlow(
	opPolicies map[string]interface{},
	flow string,
	level string,
	allPolicies []map[string]interface{},
) bool {
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

			currentVersion, _ := pol["policyVersion"].(string)

			currentVersionNumber, err1 := versionNumber(currentVersion)

			policyVersionNumber, err2 := versionNumber(policyVersion)

			if err1 != nil || err2 != nil {
				log.Printf("Invalid version number for policy %s: %s", policyName, currentVersion)
				continue
			}

			if currentVersionNumber < policyVersionNumber {
				log.Printf(
					"Updating %s policy [%s flow] %s: %s -> %s",
					level,
					flow,
					policyName,
					currentVersion,
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

		num, err := versionNumber(version)
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

func versionNumber(version string) (int, error) {
	return strconv.Atoi(strings.TrimPrefix(version, "v"))
}

func collectOperationsPolicyChanges(operations []interface{}, apiScopedPolicies []map[string]interface{}) []string {
	var changes []string
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
			changes = append(changes, collectPolicyChanges(opPolicies, flow, LevelOperation, apiScopedPolicies)...)
		}
	}
	return changes
}

// PreviewApiPolicyUpdates returns human-readable descriptions of the policy
// changes that would be applied to a single API, without modifying anything.
func PreviewApiPolicyUpdates(apiId string, allPolicies []map[string]interface{}) []string {
	var apiDetail map[string]any
	if err := json.Unmarshal(client.GetApiDetailsJsonObject(apiId), &apiDetail); err != nil {
		log.Fatal(err)
	}

	apiScopedPolicies := append(
		append([]map[string]interface{}{}, allPolicies...),
		client.ExtractApiLevelPolicies(apiId)...,
	)

	var changes []string

	if apiPoliciesBlock, ok := apiDetail["apiPolicies"].(map[string]interface{}); ok {
		for _, flow := range policyFlows {
			changes = append(changes, collectPolicyChanges(apiPoliciesBlock, flow, LevelAPI, apiScopedPolicies)...)
		}
	}

	if operations, ok := apiDetail["operations"].([]interface{}); ok {
		changes = append(changes, collectOperationsPolicyChanges(operations, apiScopedPolicies)...)
	}

	return changes
}

// collectPolicyChanges is a read-only variant of resolvePoliciesInFlow.
// Instead of mutating the policy map, it returns a description for each
// policy that would be updated.
func collectPolicyChanges(
	opPolicies map[string]interface{},
	flow string,
	level string,
	allPolicies []map[string]interface{},
) []string {
	flowList, ok := opPolicies[flow].([]interface{})
	if !ok {
		return nil
	}

	var changes []string

	for _, polRaw := range flowList {
		pol, ok := polRaw.(map[string]interface{})
		if !ok {
			continue
		}

		policyName, ok := pol["policyName"].(string)
		if !ok {
			continue
		}

		if _, policyVersion, found := findNewestPolicyByName(policyName, allPolicies); found {
			currentVersion, _ := pol["policyVersion"].(string)
			currentVersionNumber, err1 := versionNumber(currentVersion)
			policyVersionNumber, err2 := versionNumber(policyVersion)

			if err1 != nil || err2 != nil {
				continue
			}

			if currentVersionNumber < policyVersionNumber {
				changes = append(changes, fmt.Sprintf(
					"  Updating %s policy [%s flow] %s: %s -> %s",
					level, flow, policyName, currentVersion, policyVersion,
				))
			}
		}
	}

	return changes
}

// ListCurrentPolicies returns human-readable descriptions of all policies
// currently attached to an API, with their levels, flows, and versions.
func ListCurrentPolicies(apiId string) []string {
	var apiDetail map[string]any
	if err := json.Unmarshal(client.GetApiDetailsJsonObject(apiId), &apiDetail); err != nil {
		log.Fatal(err)
	}

	var items []string

	if apiPoliciesBlock, ok := apiDetail["apiPolicies"].(map[string]interface{}); ok {
		for _, flow := range policyFlows {
			items = append(items, listFlowPolicies(apiPoliciesBlock, flow, LevelAPI)...)
		}
	}

	if operations, ok := apiDetail["operations"].([]interface{}); ok {
		items = append(items, listOperationsPolicies(operations)...)
	}

	return items
}

func listOperationsPolicies(operations []interface{}) []string {
	var items []string
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
			items = append(items, listFlowPolicies(opPolicies, flow, LevelOperation)...)
		}
	}
	return items
}

// listFlowPolicies lists all policies in a single flow with their current versions.
func listFlowPolicies(opPolicies map[string]interface{}, flow string, level string) []string {
	flowList, ok := opPolicies[flow].([]interface{})
	if !ok {
		return nil
	}

	var items []string
	for _, polRaw := range flowList {
		pol, ok := polRaw.(map[string]interface{})
		if !ok {
			continue
		}
		policyName, _ := pol["policyName"].(string)
		policyVersion, _ := pol["policyVersion"].(string)
		if policyName != "" {
			items = append(items, fmt.Sprintf(
				"    %s policy [%s flow] %s: %s",
				level, flow, policyName, policyVersion,
			))
		}
	}
	return items
}
