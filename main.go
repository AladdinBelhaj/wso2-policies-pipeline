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

	dryRun := false
	if len(os.Args) > 1 {
		for _, arg := range os.Args[1:] {
			if arg == "--dry-run" || arg == "-n" {
				dryRun = true
			}
		}
	}

	if len(os.Args) > 1 && os.Args[1] == "rollback" {
		target := ""
		if len(os.Args) > 2 {
			target = strings.Join(os.Args[2:], " ")
		}

		apiSummaries := client.ExtractApiSummaries()
		apiIds := client.FilterApiIdsByName(apiSummaries, target)
		if len(apiIds) == 0 {
			log.Printf("API %q does not exist", target)
			return
		}

		preview := client.BuildRollbackPreview(apiIds, target)
		if dryRun || !client.ConfirmAction(preview) {
			if dryRun {
				log.Println("dry run: no changes were applied")
			} else {
				log.Println("operation cancelled")
			}
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

	if dryRun {
		preview := "Dry run: policy updates for the following APIs will be applied:\n"
		for _, apiID := range apiIds {
			preview += "- " + apiID + "\n"
		}
		if !client.ConfirmAction(preview) {
			log.Println("operation cancelled")
			return
		}
	}

	policy.UpdateApiPolicies(apiPolicies, allPolicies)
}
