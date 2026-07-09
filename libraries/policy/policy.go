// This package handles resolving and updating operation policies for WSO2 APIs.
package policy

import (
	"encoding/json"
	"log"
	"wso2/scripts/libraries/client"
)

func buildPolicyLookupForApi(apiDetail map[string]any, allPolicies []map[string]interface{}) []map[string]interface{} {
	lookup := make([]map[string]interface{}, 0, len(allPolicies))
	lookup = append(lookup, allPolicies...)

	apiPoliciesBlock, ok := apiDetail["apiPolicies"].(map[string]interface{})
	if !ok {
		return lookup
	}

	for _, flow := range []string{"request", "response", "fault"} {
		flowList, ok := apiPoliciesBlock[flow].([]interface{})
		if !ok {
			continue
		}
		for _, item := range flowList {
			policy, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			policyName, ok := policy["policyName"].(string)
			if !ok {
				continue
			}
			policyVersion, _ := policy["policyVersion"].(string)
			policyID, _ := policy["policyId"].(string)
			lookup = append(lookup, map[string]interface{}{
				"id":      policyID,
				"name":    policyName,
				"version": policyVersion,
			})
		}
	}

	return lookup
}

// This function fetches each API, resolves policy IDs by name, and PUTs the result back.
func UpdateApiPolicies(apiPolicies []map[string]any, allPolicies []map[string]interface{}) {
	for _, apiEntry := range apiPolicies {
		apiId := apiEntry["apiId"].(string)

		// Fetch the full API object, will be mutated then PUT back
		var apiDetail map[string]any
		if err := json.Unmarshal(client.GetApiDetailsJsonObject(apiId), &apiDetail); err != nil {
			log.Fatal(err)
		}

		lifecycleStatus, _ := apiDetail["lifeCycleStatus"].(string)
		if lifecycleStatus != "PUBLISHED" {
			log.Printf("skipping API %s because lifecycle status is %s", apiId, lifecycleStatus)
			continue
		}

		modified := false
		lookupPolicies := buildPolicyLookupForApi(apiDetail, allPolicies)

		apiPoliciesBlock, ok := apiDetail["apiPolicies"].(map[string]interface{})
		if ok {
			for _, flow := range []string{"request", "response", "fault"} {
				if resolvePoliciesInFlow(apiPoliciesBlock, flow, lookupPolicies) {
					modified = true
				}
			}
		}

		operations, ok := apiDetail["operations"].([]interface{})
		if !ok {
			continue
		}

		// Walk every operation and resolve policy IDs in each flow.
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
				if resolvePoliciesInFlow(opPolicies, flow, lookupPolicies) {
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
		if err := client.PutApiUpdate(apiId, updatedJson); err != nil {
			log.Printf("failed to update API %s: %v", apiId, err)
			continue
		}

		client.PrepareAndDeployRevision(apiId)
	}
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
			pol["policyId"] = policyId
			pol["policyVersion"] = policyVersion
			changed = true
		}
	}
	return changed
}

// This function looks up a policy by name in the shared operation-policies list
// and returns the id and version of the newest version found.
func findNewestPolicyByName(name string, allPolicies []map[string]interface{}) (string, string, bool) {
	latestId := ""
	latestVersion := ""

	for _, policy := range allPolicies {
		if policy["name"] != name {
			continue
		}
		version, ok := policy["version"].(string)
		if !ok {
			continue
		}
		if version > latestVersion {
			latestVersion = version
			latestId, _ = policy["id"].(string)
		}
	}

	if latestId == "" {
		return "", "", false
	}
	return latestId, latestVersion, true
}
