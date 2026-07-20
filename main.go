package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"wso2/scripts/libraries/client"
	"wso2/scripts/libraries/policy"
	"wso2/scripts/vars"
)

func main() {
	vars.Load()

	dryRun, isRollback, target := parseArguments()

	if isRollback {
		executeRollback(target, dryRun)
	} else {
		executeUpdatePolicies(target, dryRun)
	}
}

func parseArguments() (dryRun bool, isRollback bool, target string) {
	if len(os.Args) > 1 && os.Args[1] == "rollback" {
		isRollback = true
	}

	var targetParts []string
	for _, arg := range os.Args[1:] {
		if arg == "--dry-run" || arg == "-n" {
			dryRun = true
			continue
		}
		if arg == "rollback" && isRollback {
			continue
		}
		targetParts = append(targetParts, arg)
	}
	return dryRun, isRollback, strings.Join(targetParts, " ")
}

func executeRollback(target string, dryRun bool) {
	apiIds := fetchApiIds(target, true)
	if len(apiIds) == 0 {
		return
	}

	preview := "Rollback preview:\n"
	for _, apiId := range apiIds {
		revisionIDs, err := client.GetRevisionIds(apiId)
		if err != nil {
			log.Printf("failed to fetch revisions for API %s: %v", apiId, err)
			continue
		}
		if len(revisionIDs) <= 2 {
			preview += fmt.Sprintf("  %s: cannot rollback (only %d revision(s))\n", apiId, len(revisionIDs))
			continue
		}

		targetRevisionID := revisionIDs[len(revisionIDs)-2]
		currentPolicies := policy.ListCurrentPolicies(apiId)

		preview += fmt.Sprintf("  %s:\n", apiId)
		preview += fmt.Sprintf("    Target revision: %s\n", targetRevisionID)
		if len(currentPolicies) > 0 {
			preview += "    Current policies that will be reverted:\n"
			for _, p := range currentPolicies {
				preview += p + "\n"
			}
		}
	}

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
}

func executeUpdatePolicies(target string, dryRun bool) {
	apiIds := fetchApiIds(target, false)
	if len(apiIds) == 0 && target != "" {
		return
	}

	allPolicies := client.ExtractOperationPolicies()
	productIds := client.FindApiProductIdsUsingApis(apiIds)

	if dryRun {
		preview := "Dry run: the following policy updates will be applied:\n"
		for _, apiID := range apiIds {
			changes := policy.PreviewApiPolicyUpdates(apiID, allPolicies)
			if len(changes) == 0 {
				preview += fmt.Sprintf("  %s: no changes\n", apiID)
			} else {
				preview += fmt.Sprintf("  %s:\n", apiID)
				for _, c := range changes {
					preview += c + "\n"
				}
			}
		}
		if len(productIds) > 0 {
			preview += fmt.Sprintf("  Also affects API Product(s): %s\n", strings.Join(productIds, ", "))
		}
		if !client.ConfirmAction(preview) {
			log.Println("operation cancelled")
			return
		}
	}

	// Snapshot product state BEFORE the API update/redeploy, since deploying
	// a new revision of the source API wipes the product's real operation-level
	// policy attachments as a side effect.
	var productSnapshots map[string]map[string]any
	if len(productIds) > 0 {
		productSnapshots = client.ExtractApiProductPolicies(productIds)
	}

	apiDetails := client.ExtractApiPolicies(apiIds)
	policy.UpdateApiPolicies(apiDetails, allPolicies)

	if len(productIds) > 0 {
		policy.RestoreApiProductPolicies(productIds, productSnapshots, allPolicies)
	}
}

func fetchApiIds(target string, isRollback bool) []string {
	apiSummaries := client.ExtractApiSummaries()
	apiIds := client.FilterApiIdsByName(apiSummaries, target)

	if len(apiIds) == 0 {
		if isRollback || target != "" {
			log.Printf("API %q does not exist", target)
		}
	}

	return apiIds
}