package policy

import (
	"encoding/json"
	"fmt"
	"log"
	"wso2/pctl/libraries/client"
)

var productPolicyFlows = []string{"request", "response", "fault"}

// This function iterates over the already-fetched API product details, resolves
// operation policy IDs by name, and PUTs the result back with the two-step
// strip/restore dance WSO2 needs to actually pick up the policy change.
func UpdateApiProductPolicies(productDetails map[string]map[string]any, allPolicies []map[string]interface{}, policyFilter ...string) {
	for productId, productDetail := range productDetails {
		processSingleApiProduct(productId, productDetail, allPolicies, policyFilter...)
	}
}

func processSingleApiProduct(productId string, productDetail map[string]any, allPolicies []map[string]interface{}, policyFilter ...string) {
	productName, ok := productDetail["name"].(string)
	if !ok {
		log.Println("API product name not found")
		return
	}

	fmt.Printf("Updating API Product: %s\n", productName)

	modified := updateApiProductOperations(productDetail, allPolicies, policyFilter...)
	sanitizeApiProductOperations(productDetail)

	if modified {
		// Perform a single PUT update with operations and updated policies included.
		updatedJson, err := json.Marshal(productDetail)
		if err != nil {
			log.Fatalf("failed to marshal API Product JSON: %v", err)
		}
		if err := client.PutApiProductUpdate(productId, updatedJson); err != nil {
			log.Printf("failed to update API Product %s: %v", productName, err)
			return
		}
	}

	client.PrepareAndDeployProductRevision(productId)
}

// updateApiProductOperations walks apis[].operations[].operationPolicies and
// resolves real (non-reflected) operation-level policy versions in place.
func updateApiProductOperations(productDetail map[string]any, policies []map[string]interface{}, policyFilter ...string) bool {
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
		if updateSingleApiOperations(api, policies, policyFilter...) {
			modified = true
		}
	}
	return modified
}

func updateSingleApiOperations(api map[string]interface{}, policies []map[string]interface{}, policyFilter ...string) bool {
	operations, ok := api["operations"].([]interface{})
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
		for _, flow := range productPolicyFlows {
			if resolveProductPoliciesInFlow(opPolicies, flow, policies, policyFilter...) {
				modified = true
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
	policyFilter ...string,
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
		if resolveSingleProductPolicy(pol, flow, allPolicies, policyFilter...) {
			changed = true
		}
	}

	return changed
}

func resolveSingleProductPolicy(pol map[string]interface{}, flow string, allPolicies []map[string]interface{}, policyFilter ...string) bool {
	if policyType, _ := pol["policyType"].(string); policyType == "api" {
		return false
	}

	policyName, ok := pol["policyName"].(string)
	if !ok {
		return false
	}

	filter := ""
	if len(policyFilter) > 0 {
		filter = policyFilter[0]
	}
	if filter != "" && policyName != filter {
		return false
	}

	policyId, policyVersion, found := findNewestProductPolicyByName(policyName, allPolicies)
	if !found {
		return false
	}

	currentVersion, _ := pol["policyVersion"].(string)
	currentVersionNumber, err1 := productVersionNumber(currentVersion)
	policyVersionNumber, err2 := productVersionNumber(policyVersion)

	if err1 != nil || err2 != nil {
		log.Printf("Invalid version number for product policy %s: %s", policyName, currentVersion)
		return false
	}

	if currentVersionNumber < policyVersionNumber {
		log.Printf(
			"Updating API Product operation policy [%s flow] %s: %s -> %s",
			flow, policyName, currentVersion, policyVersion,
		)
		pol["policyId"] = policyId
		pol["policyVersion"] = policyVersion
		return true
	}

	return false
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
		sanitizeSingleApiOperations(api)
	}
}

func sanitizeSingleApiOperations(api map[string]interface{}) {
	operations, ok := api["operations"].([]interface{})
	if !ok {
		return
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
		sanitizeSingleOperationFlows(opPolicies)
	}
}

func sanitizeSingleOperationFlows(opPolicies map[string]interface{}) {
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

// RestoreApiProductPolicies projects each source API's current (post-update)
// operation-level policies onto the matching API Product operations, using
// the API as ground truth rather than the product's own embedded copy. This
// is deliberate: the product's own copy can be wiped either by a source API
// redeploy or by rolling back the product itself to an older revision, so it
// must never be trusted as the source of truth for what "real" policies
// should exist.
func RestoreApiProductPolicies(productIds []string, apiDetails map[string]map[string]any, allPolicies []map[string]interface{}, policyFilter ...string) {
	for idx, productId := range productIds {
		productName := client.GetApiProductName(productId)
		log.Printf("[%d/%d] Restoring policies for API Product %s...", idx+1, len(productIds), productName)
		restoreSingleApiProduct(productId, apiDetails)
	}
}

func restoreSingleApiProduct(productId string, apiDetails map[string]map[string]any) {
	productName := client.GetApiProductName(productId)

	fresh := client.GetApiProductDetailsJsonObject(productId)
	var freshDetail map[string]any
	if err := json.Unmarshal(fresh, &freshDetail); err != nil {
		log.Printf("failed to fetch current API Product %s: %v", productName, err)
		return
	}

	if !projectApiOperationPolicies(freshDetail, apiDetails) {
		log.Printf("No operation-level policy changes to project onto API Product %s", productName)
		return
	}

	updatedJson, err := json.Marshal(freshDetail)
	if err != nil {
		log.Fatal(err)
	}
	if err := client.PutApiProductUpdate(productId, updatedJson); err != nil {
		log.Printf("failed to restore policies for API Product %s: %v", productName, err)
		return
	}
	fmt.Printf("Updated policies for API Product %s\n", productName)
	client.PrepareAndDeployProductRevision(productId)
}

type operationKey struct {
	Verb   string
	Target string
}

// projectApiOperationPolicies overwrites each product operation's policy
// flows with the current policies from the matching source API operation
// (matched by apiId, then target+verb). Returns true if anything actually
// changed relative to what the freshly-fetched product currently has.
func projectApiOperationPolicies(productDetail map[string]any, apiDetails map[string]map[string]any) bool {
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
		apiId, _ := api["apiId"].(string)
		sourceApiDetail, ok := apiDetails[apiId]
		if !ok {
			// This product operation belongs to an API not part of the
			// current update run; leave it untouched.
			continue
		}
		if projectSingleApiOperations(api, sourceApiDetail) {
			modified = true
		}
	}
	return modified
}

func projectSingleApiOperations(productApi map[string]interface{}, sourceApiDetail map[string]any) bool {
	productOps, ok := productApi["operations"].([]interface{})
	if !ok {
		return false
	}
	sourceOps, ok := sourceApiDetail["operations"].([]interface{})
	if !ok {
		return false
	}

	sourceIndex := indexOperationsByVerbTarget(sourceOps)

	modified := false
	for _, opRaw := range productOps {
		op, ok := opRaw.(map[string]interface{})
		if !ok {
			continue
		}
		target, _ := op["target"].(string)
		verb, _ := op["verb"].(string)

		sourceOp, found := sourceIndex[operationKey{Verb: verb, Target: target}]
		if !found {
			continue
		}

		if projectSingleOperationPolicies(op, sourceOp) {
			modified = true
		}
	}
	return modified
}

func indexOperationsByVerbTarget(operations []interface{}) map[operationKey]map[string]interface{} {
	index := make(map[operationKey]map[string]interface{}, len(operations))
	for _, opRaw := range operations {
		op, ok := opRaw.(map[string]interface{})
		if !ok {
			continue
		}
		target, _ := op["target"].(string)
		verb, _ := op["verb"].(string)
		index[operationKey{Verb: verb, Target: target}] = op
	}
	return index
}

// projectSingleOperationPolicies replaces the product operation's policy
// list, per flow, with the source API operation's current policy list.
// This wholesale replacement is intentional: it both refreshes real
// attachments and drops any stale "policyType":"api" reflection entries,
// since the source API's operationPolicies never contain those reflections
// in the first place.
func projectSingleOperationPolicies(productOp map[string]interface{}, sourceOp map[string]interface{}) bool {
	productOpPolicies, ok := productOp["operationPolicies"].(map[string]interface{})
	if !ok {
		return false
	}
	sourceOpPolicies, ok := sourceOp["operationPolicies"].(map[string]interface{})
	if !ok {
		return false
	}

	modified := false
	for _, flow := range productPolicyFlows {
		sourceList, ok := sourceOpPolicies[flow].([]interface{})
		if !ok {
			sourceList = []interface{}{}
		}

		if !policyListsEqual(productOpPolicies[flow], sourceList) {
			modified = true
		}
		productOpPolicies[flow] = sourceList
	}
	return modified
}

// policyListsEqual compares two policy-entry lists by JSON content so
// unrelated field ordering doesn't cause false positives.
func policyListsEqual(a interface{}, b []interface{}) bool {
	aList, _ := a.([]interface{})
	if len(aList) != len(b) {
		return false
	}
	aJson, err1 := json.Marshal(aList)
	bJson, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(aJson) == string(bJson)
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
