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

// resolveProductPoliciesInFlow matches each operation-level policy against shared list
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

// OperationPolicySnapshot captures the real (non-reflection) operation-level
// policies attached to one operation at a point in time, so they can be
// restored later after something (e.g. a source API revision redeploy)
// wipes the product's operation policy state.
type OperationPolicySnapshot struct {
	Verb   string
	Target string
	Flows  map[string][]map[string]interface{} // flow -> real (non "api" type) policy entries
}

// SnapshotApiProductRealPolicies must be called BEFORE the source API is
// updated/redeployed. It captures only genuine attachments (policyType !=
// "api") per operation, keyed by the owning apiId, so they can be resolved
// to newer versions and written back after the redeploy wipes them.
func SnapshotApiProductRealPolicies(productDetail map[string]any) map[string][]OperationPolicySnapshot {
	result := make(map[string][]OperationPolicySnapshot)

	apis, ok := productDetail["apis"].([]interface{})
	if !ok {
		return result
	}

	for _, apiRaw := range apis {
		api, ok := apiRaw.(map[string]interface{})
		if !ok {
			continue
		}
		apiId, _ := api["apiId"].(string)

		operations, ok := api["operations"].([]interface{})
		if !ok {
			continue
		}

		var snaps []OperationPolicySnapshot
		for _, opRaw := range operations {
			op, ok := opRaw.(map[string]interface{})
			if !ok {
				continue
			}
			target, _ := op["target"].(string)
			verb, _ := op["verb"].(string)

			opPolicies, ok := op["operationPolicies"].(map[string]interface{})
			if !ok {
				continue
			}

			snap := OperationPolicySnapshot{Verb: verb, Target: target, Flows: map[string][]map[string]interface{}{}}

			for _, flow := range productPolicyFlows {
				flowList, ok := opPolicies[flow].([]interface{})
				if !ok {
					continue
				}
				var real []map[string]interface{}
				for _, polRaw := range flowList {
					pol, ok := polRaw.(map[string]interface{})
					if !ok {
						continue
					}
					if policyType, _ := pol["policyType"].(string); policyType == "api" {
						continue
					}
					real = append(real, pol)
				}
				if len(real) > 0 {
					snap.Flows[flow] = real
				}
			}

			snaps = append(snaps, snap)
		}

		result[apiId] = snaps
	}

	return result
}

// RestoreApiProductPolicies resolves each pre-update snapshot's policies to
// their newest versions, re-fetches the CURRENT (post-redeploy) product
// state, merges the resolved real policies back into the matching
// operations, and PUTs the result back so real attachments survive the
// source API's redeploy instead of being silently dropped.
func RestoreApiProductPolicies(productIds []string, preUpdateSnapshots map[string]map[string]any, allPolicies []map[string]interface{}) {
	for _, productId := range productIds {
		snapshotDetail, ok := preUpdateSnapshots[productId]
		if !ok {
			continue
		}
		restoreSingleApiProduct(productId, snapshotDetail, allPolicies)
	}
}

func restoreSingleApiProduct(productId string, snapshotDetail map[string]any, allPolicies []map[string]interface{}) {
	realByApi := SnapshotApiProductRealPolicies(snapshotDetail)

	hasAny := false
	for _, snaps := range realByApi {
		for _, s := range snaps {
			if len(s.Flows) > 0 {
				hasAny = true
			}
		}
	}
	if !hasAny {
		log.Printf("No real operation-level policies to restore for API Product %s", productId)
		return
	}

	// Resolve each snapshotted policy to its newest version before writing it back.
	for _, snaps := range realByApi {
		for i := range snaps {
			for flow, policies := range snaps[i].Flows {
				wrapped := map[string]interface{}{flow: toPolicyInterfaceSlice(policies)}
				resolveProductPoliciesInFlow(wrapped, flow, allPolicies)
				snaps[i].Flows[flow] = fromPolicyInterfaceSlice(wrapped[flow].([]interface{}))
			}
		}
	}

	fresh := client.GetApiProductDetailsJsonObject(productId)
	var freshDetail map[string]any
	if err := json.Unmarshal(fresh, &freshDetail); err != nil {
		log.Printf("failed to fetch current API Product %s: %v", productId, err)
		return
	}

	mergeRealOperationPolicies(freshDetail, realByApi)

	// Step 1: PUT with operations stripped - mirrors the UI's "remove
	// operations, save" action.
	strippedJson, err := json.Marshal(stripApiProductOperations(freshDetail))
	if err != nil {
		log.Fatal(err)
	}
	if err := client.PutApiProductUpdate(productId, strippedJson); err != nil {
		log.Printf("failed to strip operations for API Product %s: %v", productId, err)
		return
	}

	// Step 2: PUT with operations (carrying the resolved real policies)
	// restored - mirrors the UI's "add operations back, save" action.
	updatedJson, err := json.Marshal(freshDetail)
	if err != nil {
		log.Fatal(err)
	}
	if err := client.PutApiProductUpdate(productId, updatedJson); err != nil {
		log.Printf("failed to restore policies for API Product %s: %v", productId, err)
		return
	}
	fmt.Printf("Restored real operation policies for API Product %s\n", productId)
}

// mergeRealOperationPolicies writes the resolved real policies back into the
// matching operation (matched by apiId + target + verb) of a freshly
// fetched product detail.
func mergeRealOperationPolicies(productDetail map[string]any, realByApi map[string][]OperationPolicySnapshot) {
	apis, ok := productDetail["apis"].([]interface{})
	if !ok {
		return
	}

	for _, apiRaw := range apis {
		api, ok := apiRaw.(map[string]interface{})
		if !ok {
			continue
		}
		apiId, _ := api["apiId"].(string)
		snaps, ok := realByApi[apiId]
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
			target, _ := op["target"].(string)
			verb, _ := op["verb"].(string)

			for _, snap := range snaps {
				if snap.Target != target || snap.Verb != verb {
					continue
				}
				opPolicies, ok := op["operationPolicies"].(map[string]interface{})
				if !ok {
					continue
				}
				for flow, real := range snap.Flows {
					existing, _ := opPolicies[flow].([]interface{})
					opPolicies[flow] = append(existing, toPolicyInterfaceSlice(real)...)
				}
			}
		}
	}
}

func toPolicyInterfaceSlice(policies []map[string]interface{}) []interface{} {
	out := make([]interface{}, len(policies))
	for i, p := range policies {
		out[i] = p
	}
	return out
}

func fromPolicyInterfaceSlice(items []interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
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
