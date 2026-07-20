// This file handles resolving and updating operation policies for WSO2 API Products.
// API Products are intentionally NOT handled by the regular API policy code
// (policy.go): their operationPolicies blocks can contain read-only
// policyType:"api" reflection entries (mirroring the source API's own
// API-level policies for display purposes) mixed in alongside real
// operation-level attachments. These reflections must never be echoed back
// in a PUT payload, and must never be mistaken for something to update.
package policy

import (
	"encoding/json"
	"fmt"
	"log"
	"wso2/scripts/libraries/client"
)

var productPolicyFlows = []string{"request", "response", "fault"}

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

	modified := updateApiProductOperations(productDetail, allPolicies)
	sanitizeApiProductOperations(productDetail)

	if !modified {
		log.Printf("No changes detected for API Product %s. Skipping update.", productId)
		return
	}

	updatedJson, err := json.Marshal(productDetail)
	if err != nil {
		log.Fatal(err)
	}
	if err := client.PutApiProductUpdate(productId, updatedJson); err != nil {
		log.Printf("failed to update API Product %s: %v", productId, err)
		return
	}
}

// updateApiProductOperations walks apis[].operations[].operationPolicies and
// resolves real (non-reflected) operation-level policy versions in place.
func updateApiProductOperations(productDetail map[string]any, policies []map[string]interface{}) bool {
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
			for _, flow := range productPolicyFlows {
				if resolveProductPoliciesInFlow(opPolicies, flow, policies) {
					modified = true
				}
			}
		}
	}
	return modified
}

// resolveProductPoliciesInFlow matches each REAL operation-level policy
// (policyType != "api") in a single flow against the shared policies list
// and injects the resolved policyId/policyVersion. Entries with
// policyType:"api" are read-only reflections of the source API's API-level
// policies embedded by WSO2 for display only, and are skipped entirely -
// they are never real attachments on the product and must never be treated
// as updatable.
func resolveProductPoliciesInFlow(
	opPolicies map[string]interface{},
	flow string,
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

		if policyType, _ := pol["policyType"].(string); policyType == "api" {
			continue
		}

		policyName, ok := pol["policyName"].(string)
		if !ok {
			continue
		}

		if policyId, policyVersion, found := findNewestProductPolicyByName(policyName, allPolicies); found {
			currentVersion, _ := pol["policyVersion"].(string)
			currentVersionNumber, err1 := productVersionNumber(currentVersion)
			policyVersionNumber, err2 := productVersionNumber(policyVersion)

			if err1 != nil || err2 != nil {
				log.Printf("Invalid version number for product policy %s: %s", policyName, currentVersion)
				continue
			}

			if currentVersionNumber < policyVersionNumber {
				log.Printf(
					"Updating API Product operation policy [%s flow] %s: %s -> %s",
					flow, policyName, currentVersion, policyVersion,
				)
				pol["policyId"] = policyId
				pol["policyVersion"] = policyVersion
				changed = true
			}
		}
	}

	return changed
}

// findNewestProductPolicyByName looks up a policy by name in the shared
// operation-policies list and returns the id and version of the newest
// version found.
func findNewestProductPolicyByName(name string, allPolicies []map[string]interface{}) (string, string, bool) {
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

		num, err := productVersionNumber(version)
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

func productVersionNumber(version string) (int, error) {
	n := 0
	_, err := fmt.Sscanf(version, "v%d", &n)
	return n, err
}

// sanitizeApiProductOperations strips read-only policyType:"api" reflection
// entries out of every operation's policy flows before the payload is ever
// marshaled for a PUT. These entries are embedded by WSO2 for display only
// and must never be echoed back as if they were real attachments -
// doing so is what corrupted the apis[] entries (dropping other operations)
// in earlier testing.
func sanitizeApiProductOperations(productDetail map[string]any) {
	apis, ok := productDetail["apis"].([]interface{})
	if !ok {
		return
	}

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
			for _, flow := range productPolicyFlows {
				flowList, ok := opPolicies[flow].([]interface{})
				if !ok {
					continue
				}
				filtered := make([]interface{}, 0, len(flowList))
				for _, polRaw := range flowList {
					pol, ok := polRaw.(map[string]interface{})
					if !ok {
						filtered = append(filtered, polRaw)
						continue
					}
					if policyType, _ := pol["policyType"].(string); policyType == "api" {
						continue
					}
					filtered = append(filtered, polRaw)
				}
				opPolicies[flow] = filtered
			}
		}
	}
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
