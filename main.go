package main

import (
	"wso2/scripts/libraries/client"
	"wso2/scripts/libraries/policy"
	"wso2/scripts/vars"
)

func main() {
	vars.Load()


	apiIds := client.ExtractApiIds()
	apiPolicies := client.ExtractApiPolicies(apiIds)
	allPolicies := client.ExtractOperationPolicies()

	policy.UpdateApiPolicies(apiPolicies, allPolicies)
}
