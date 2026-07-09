package main

import (
	"log"
	"os"
	"strings"
	"wso2/scripts/libraries/client"
	"wso2/scripts/libraries/policy"
	"wso2/scripts/vars"
)

func main() {
	vars.Load()

	if len(os.Args) > 1 && os.Args[1] == "rollback" {
		target := ""
		if len(os.Args) > 2 {
			target = strings.Join(os.Args[2:], " ")
		}

		apiSummaries := client.ExtractApiSummaries()
		apiIds := client.FilterApiIdsByName(apiSummaries, target)
		if len(apiIds) == 0 {
			log.Printf("no APIs matched rollback target %q", target)
			return
		}

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
