package main

import (
	"log"
	"os"
	"wso2/scripts/libraries/client"
	"wso2/scripts/libraries/policy"
	"wso2/scripts/vars"
)

func main() {
	vars.Load()

	if len(os.Args) > 1 && os.Args[1] == "rollback" {
		apiIds := client.ExtractApiIds()
		for _, apiId := range apiIds {
			if err := client.RollbackApiRevision(apiId); err != nil {
				log.Printf("rollback failed for API %s: %v", apiId, err)
			}
		}
		return
	}

	apiIds := client.ExtractApiIds()
	apiPolicies := client.ExtractApiPolicies(apiIds)
	allPolicies := client.ExtractOperationPolicies()

	policy.UpdateApiPolicies(apiPolicies, allPolicies)
}
