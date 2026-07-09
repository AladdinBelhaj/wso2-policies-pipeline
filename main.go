package main

import (
	"wso2/scripts/libraries/client"
	// "wso2/scripts/libraries/policy"
	"wso2/scripts/vars"
	"fmt"
)

func main() {
	vars.Load()

	// apiIds := client.ExtractApiIds()
	trueOrFalse := client.reviewRevisionsNumber("2748d335-8e67-4724-b04d-6e6e13a32415")
	fmt.Println(trueOrFalse)
	// apiPolicies := client.ExtractApiPolicies(apiIds)
	// allPolicies := client.ExtractOperationPolicies()

	// policy.UpdateApiPolicies(apiPolicies, allPolicies)
}
