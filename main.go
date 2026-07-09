package main

import (
	"wso2/scripts/libraries/client"
	// "wso2/scripts/libraries/policy"
	"wso2/scripts/vars"
)

func main() {
	vars.Load()

	// apiIds := client.ExtractApiIds()
	// client.ReviewRevisionsNumber("2748d335-8e67-4724-b04d-6e6e13a32415")
	client.CreateRevision("2748d335-8e67-4724-b04d-6e6e13a32415")
	// apiPolicies := client.ExtractApiPolicies(apiIds)
	// allPolicies := client.ExtractOperationPolicies()

	// policy.UpdateApiPolicies(apiPolicies, allPolicies)
}
