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
		if resolvePoliciesInFlow(apiPoliciesBlock, flow, "API level", policies) {
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
			if resolvePoliciesInFlow(opPolicies, flow, "Operation level", policies) {
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
			changes = append(changes, collectPolicyChanges(apiPoliciesBlock, flow, "API level", apiScopedPolicies)...)
		}
	}

	if operations, ok := apiDetail["operations"].([]interface{}); ok {
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
				changes = append(changes, collectPolicyChanges(opPolicies, flow, "Operation level", apiScopedPolicies)...)
			}
		}
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
			items = append(items, listFlowPolicies(apiPoliciesBlock, flow, "API level")...)
		}
	}

	if operations, ok := apiDetail["operations"].([]interface{}); ok {
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
				items = append(items, listFlowPolicies(opPolicies, flow, "Operation level")...)
			}
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

// This function iterates over the already-fetched API product details, resolves
// operation policy IDs by name, and PUTs the result back with the two-step
// strip/restore dance WSO2 needs to actually pick up the policy change.
func UpdateApiProductPolicies(productDetails map[string]map[string]any, allPolicies []map[string]interface{}) {
	for productId, productDetail := range productDetails {
		processSingleApiProduct(productId, productDetail, allPolicies)
	}
}

func processSingleApiProduct(productId string, productDetail map[string]any, allPolicies []map[string]interface{}) {
	productName, ok := productDetail["name"].(string)
	if !ok {
		log.Println("API product name not found")
		return
	}

	fmt.Printf("Updating API Product: %s\n", productName)

	if !updateApiProductOperationsPoliciesBlock(productDetail, allPolicies) {
		log.Printf("No changes detected for API Product %s. Skipping update.", productId)
		return
	}

	// Step 1: PUT with operations stripped from each `apis[i]` entry.
	strippedJson, err := json.Marshal(stripApiProductOperations(productDetail))
	if err != nil {
		log.Fatal(err)
	}
	if err := client.PutApiProductUpdate(productId, strippedJson); err != nil {
		log.Printf("failed to strip operations for API Product %s: %v", productId, err)
		return
	}

	// Step 2: PUT again with operations (and updated policy IDs/versions) restored.
	updatedJson, err := json.Marshal(productDetail)
	if err != nil {
		log.Fatal(err)
	}
	if err := client.PutApiProductUpdate(productId, updatedJson); err != nil {
		log.Printf("failed to update API Product %s: %v", productId, err)
		return
	}
}

// updateApiProductOperationsPoliciesBlock walks apis[].operations[].operationPolicies
// and resolves policy versions in place, reusing the same resolvePoliciesInFlow logic
// used for regular APIs.
func updateApiProductOperationsPoliciesBlock(productDetail map[string]any, policies []map[string]interface{}) bool {
	apis, ok := productDetail["apis"].([]interface{})
	if !ok {
		return false
	}

	modified := false
	for _, apiRaw := range apis {
		api, ok := apiRaw.(map[string]interface{})
		if !ok {
			continue
		}
		operations, ok := api["operations"].([]interface{})
		if !ok {
			continue
		}
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
				if resolvePoliciesInFlow(opPolicies, flow, "Operation level", policies) {
					modified = true
				}
			}
		}
	}
	return modified
}

// stripApiProductOperations returns a copy of productDetail with "operations"
// removed from every entry in "apis", so the first PUT registers as a real change.
func stripApiProductOperations(productDetail map[string]any) map[string]any {
	stripped := make(map[string]any, len(productDetail))
	for k, v := range productDetail {
		stripped[k] = v
	}

	apis, ok := stripped["apis"].([]interface{})
	if !ok {
		return stripped
	}

	strippedApis := make([]interface{}, 0, len(apis))
	for _, apiRaw := range apis {
		api, ok := apiRaw.(map[string]interface{})
		if !ok {
			strippedApis = append(strippedApis, apiRaw)
			continue
		}
		strippedApi := make(map[string]interface{}, len(api))
		for k, v := range api {
			if k == "operations" {
				continue
			}
			strippedApi[k] = v
		}
		strippedApis = append(strippedApis, strippedApi)
	}
	stripped["apis"] = strippedApis

	return stripped
}
