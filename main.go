package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"wso2/pctl/libraries/client"
	"wso2/pctl/libraries/policy"
	"wso2/pctl/vars"
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

	productIds := client.FindApiProductIdsUsingApis(apiIds)
	preview := buildRollbackPreview(apiIds, productIds)

	if dryRun || !client.ConfirmAction(preview) {
		if dryRun {
			log.Println("dry run: no changes were applied")
		} else {
			log.Println("operation cancelled")
		}
		return
	}

	// Snapshot product state BEFORE the API rollback, since rolling back
	// a revision of the source API wipes the product's real operation-level
	// policy attachments as a side effect.
	var productSnapshots map[string]map[string]any
	if len(productIds) > 0 {
		productSnapshots = client.ExtractApiProductPolicies(productIds)
	}

	for idx, apiId := range apiIds {
		apiName := client.GetApiName(apiId)
		log.Printf("[%d/%d] Rolling back API %s...", idx+1, len(apiIds), apiName)
		if err := client.RollbackApiRevision(apiId); err != nil {
			log.Printf("rollback failed for API %s: %v", apiName, err)
		}
	}

	// Restore product policies
	if len(productIds) > 0 {
		allPolicies := client.ExtractOperationPolicies()
		policy.RestoreApiProductPolicies(productIds, productSnapshots, allPolicies)
	}
}

func buildRollbackPreview(apiIds []string, productIds []string) string {
	preview := "Rollback preview:\n"
	for idx, apiId := range apiIds {
		apiName := client.GetApiName(apiId)
		log.Printf("[%d/%d] Generating rollback preview for API %s...", idx+1, len(apiIds), apiName)
		revisionIDs, err := client.GetRevisionIds(apiId)
		if err != nil {
			log.Printf("failed to fetch revisions for API %s: %v", apiName, err)
			continue
		}
		if len(revisionIDs) <= 2 {
			preview += fmt.Sprintf("  %s: cannot rollback (only %d revision(s))\n", apiName, len(revisionIDs))
			continue
		}

		targetRevisionID := revisionIDs[len(revisionIDs)-2]
		currentPolicies := policy.ListCurrentPolicies(apiId)

		preview += fmt.Sprintf("  %s:\n", apiName)
		preview += fmt.Sprintf("    Target revision: %s\n", targetRevisionID)
		if len(currentPolicies) > 0 {
			preview += "    Current policies that will be reverted:\n"
			for _, p := range currentPolicies {
				preview += p + "\n"
			}
		}
	}

	if len(productIds) > 0 {
		var productNames []string
		for _, pid := range productIds {
			productNames = append(productNames, client.GetApiProductName(pid))
		}
		preview += fmt.Sprintf("  Also affects API Product(s): %s\n", strings.Join(productNames, ", "))
	}

	return preview
}

func executeUpdatePolicies(target string, dryRun bool) {
	apiIds := fetchApiIds(target, false)
	if len(apiIds) == 0 && target != "" {
		return
	}

	policyFilter := promptPolicyChoice()

	allPolicies := client.ExtractOperationPolicies()
	productIds := client.FindApiProductIdsUsingApis(apiIds)

	if dryRun {
		preview := "Dry run: the following policy updates will be applied:\n"
		for idx, apiID := range apiIds {
			apiName := client.GetApiName(apiID)
			log.Printf("[%d/%d] Generating preview for API %s...", idx+1, len(apiIds), apiName)
			changes := policy.PreviewApiPolicyUpdates(apiID, allPolicies, policyFilter)
			if len(changes) == 0 {
				preview += fmt.Sprintf("  %s: no changes\n", apiName)
			} else {
				preview += fmt.Sprintf("  %s:\n", apiName)
				for _, c := range changes {
					preview += c + "\n"
				}
			}
		}
		if len(productIds) > 0 {
			var productNames []string
			for _, pid := range productIds {
				productNames = append(productNames, client.GetApiProductName(pid))
			}
			preview += fmt.Sprintf("  Also affects API Product(s): %s\n", strings.Join(productNames, ", "))
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
	policy.UpdateApiPolicies(apiDetails, allPolicies, policyFilter)

	if len(productIds) > 0 {
		selectedProductIds := promptApiProductSelection(productIds)
		if len(selectedProductIds) > 0 {
			policy.RestoreApiProductPolicies(selectedProductIds, productSnapshots, allPolicies, policyFilter)
		}
	}
}

// promptPolicyChoice asks the user whether to update all policies or a specific one.
// Returns "" for all policies, or the entered policy name for a specific one.
// If stdin is closed/EOF (like in CI/non-interactive settings), it defaults to all policies.
func promptPolicyChoice() string {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Would you like to update:")
	fmt.Println("  1) All policies")
	fmt.Println("  2) A specific policy")
	fmt.Print("Enter choice [1/2]: ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		// Default to all policies on EOF/error
		return ""
	}
	choice = strings.TrimSpace(choice)

	if choice == "2" {
		fmt.Print("Enter the policy name: ")
		name, err := reader.ReadString('\n')
		if err != nil {
			return ""
		}
		name = strings.TrimSpace(name)
		if name == "" {
			log.Fatal("policy name cannot be empty")
		}
		log.Printf("Policy filter active: only updating policy %q", name)
		return name
	}

	return ""
}

// promptApiProductSelection displays each fetched API product and asks the user whether to update it.
// Prompt format: "Update API Product <name> [y/N]: "
func promptApiProductSelection(productIds []string) []string {
	reader := bufio.NewReader(os.Stdin)
	var selected []string

	for _, pid := range productIds {
		pName := client.GetApiProductName(pid)
		fmt.Printf("Update API Product %s [y/N]: ", pName)

		choice, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		choice = strings.TrimSpace(choice)
		if strings.EqualFold(choice, "y") || strings.EqualFold(choice, "yes") {
			selected = append(selected, pid)
		}
	}

	return selected
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
