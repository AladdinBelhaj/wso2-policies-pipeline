package main

import (
	"wso2/scripts/internal/client"
	"wso2/scripts/internal/policy"
	"wso2/scripts/vars"
)

func main() {
	vars.Load()

	apiIds := client.ExtractApiIds()
	apiPolicies := client.ExtractApiPolicies(apiIds)
	allPolicies := client.ExtractOperationPolicies()

	policy.UpdateApiPolicies(apiPolicies, allPolicies)
}
