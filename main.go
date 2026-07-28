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

	"github.com/spf13/cobra"
)

var (
	dryRun bool
)

var rootCmd = &cobra.Command{
	Use:   "pctl",
	Short: "pctl is a CLI tool for WSO2 policy deployment and management",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		vars.Load()
	},
}

var updateCmd = &cobra.Command{
	Use:   "update [API_NAME]",
	Short: "Update policies for all APIs or a specific API",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := ""
		if len(args) > 0 {
			target = args[0]
		}
		executeUpdatePolicies(target, dryRun)
	},
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback [API_NAME]",
	Short: "Rollback API revisions to previous state",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := ""
		if len(args) > 0 {
			target = args[0]
		}
		executeRollback(target, dryRun)
	},
}

func init() {
	updateCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Preview changes without applying them")
	rollbackCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Preview rollback without applying")

	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(rollbackCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func executeRollback(target string, dryRun bool) {
	apiIds := fetchApiIds(target, true)
	productIds := client.FindApiProductIdsUsingApis(apiIds)

	if len(apiIds) == 0 && target != "" {
		productSummaries := client.ExtractApiProductSummaries()
		targetProductIds := client.FilterApiProductIdsByName(productSummaries, target)
		if len(targetProductIds) > 0 {
			productIds = targetProductIds
		} else {
			log.Printf("API or API Product %q does not exist", target)
			return
		}
	}

	if len(apiIds) == 0 && len(productIds) == 0 {
		return
	}

	preview := buildRollbackPreview(apiIds, productIds)

	if dryRun || !client.ConfirmAction(preview) {
		if dryRun {
			log.Println("dry run: no changes were applied")
		} else {
			log.Println("operation cancelled")
		}
		return
	}

	for idx, apiId := range apiIds {
		apiName := client.GetApiName(apiId)
		log.Printf("[%d/%d] Rolling back API %s...", idx+1, len(apiIds), apiName)
		if err := client.RollbackApiRevision(apiId); err != nil {
			log.Printf("rollback failed for API %s: %v", apiName, err)
		}
	}

	if len(productIds) > 0 {
		for idx, productId := range productIds {
			productName := client.GetApiProductName(productId)
			log.Printf("[%d/%d] Rolling back API Product %s...", idx+1, len(productIds), productName)
			if err := client.RollbackProductRevision(productId); err != nil {
				log.Printf("rollback failed for API Product %s: %v", productName, err)
			}
		}
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
		if len(revisionIDs) <= 1 {
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
		for _, pid := range productIds {
			pName := client.GetApiProductName(pid)
			revisions, err := client.GetProductRevisionIds(pid)
			if err != nil || len(revisions) <= 1 {
				preview += fmt.Sprintf("    %s: cannot rollback (only %d revision(s))\n", pName, len(revisions))
			} else {
				targetRev := revisions[len(revisions)-2]
				preview += fmt.Sprintf("    %s: target revision %s\n", pName, targetRev)
			}
		}
	}

	return preview
}

func executeUpdatePolicies(target string, dryRun bool) {
	apiIds := fetchApiIds(target, false)
	productRefs := client.FindApiProductsUsingApis(apiIds)
	productIds := productIdsFromRefs(productRefs)

	if len(apiIds) == 0 && target != "" {
		productSummaries := client.ExtractApiProductSummaries()
		targetProductIds := client.FilterApiProductIdsByName(productSummaries, target)
		if len(targetProductIds) > 0 {
			productIds = targetProductIds
		} else {
			log.Printf("API or API Product %q does not exist", target)
			return
		}
	}

	if len(apiIds) == 0 && len(productIds) == 0 {
		return
	}

	allPolicies := client.ExtractOperationPolicies()
	policyFilter := promptPolicyChoice(allPolicies, apiIds)

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

	apiDetails := client.ExtractApiPolicies(apiIds)
	modifiedApis := policy.UpdateApiPolicies(apiDetails, allPolicies, policyFilter)

	if len(productIds) > 0 {
		updatableProductIds := filterProductsWithChanges(productRefs, modifiedApis)
		if len(updatableProductIds) > 0 {
			selectedProductIds := promptApiProductSelection(updatableProductIds)
			if len(selectedProductIds) > 0 {
				policy.RestoreApiProductPolicies(selectedProductIds, apiDetails, allPolicies, policyFilter)
			}
		}
	}
}

// productIdsFromRefs extracts the plain list of API Product IDs from the
// API-reference mapping returned by client.FindApiProductsUsingApis.
func productIdsFromRefs(refs []client.ApiProductRef) []string {
	ids := make([]string, 0, len(refs))
	for _, r := range refs {
		ids = append(ids, r.ProductID)
	}
	return ids
}

// filterProductsWithChanges drops any API Product whose referenced APIs
// (within the current run) had no actual policy changes deployed, so the
// user is never prompted to update a product that has nothing new to pull in.
func filterProductsWithChanges(productRefs []client.ApiProductRef, modifiedApis map[string]bool) []string {
	var updatable []string
	for _, ref := range productRefs {
		hasChange := false
		for _, apiId := range ref.ApiIDs {
			if modifiedApis[apiId] {
				hasChange = true
				break
			}
		}
		if hasChange {
			updatable = append(updatable, ref.ProductID)
		} else {
			log.Printf("Skipping API Product %s: no policy changes in referenced API(s)", client.GetApiProductName(ref.ProductID))
		}
	}
	return updatable
}

// promptPolicyChoice asks the user whether to update all policies or a specific one.
// Returns "" for all policies, or the entered policy name for a specific one.
// If stdin is closed/EOF (like in CI/non-interactive settings), it defaults to all policies.
func promptPolicyChoice(allPolicies []map[string]interface{}, apiIds []string) string {
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
		for {
			fmt.Print("Enter the policy name: ")
			name, err := reader.ReadString('\n')
			if err != nil {
				return ""
			}
			name = strings.TrimSpace(name)
			resolvedName, found := policy.ResolvePolicyName(name, allPolicies, apiIds)
			if name == "" || !found {
				fmt.Println("This policy does not exist, try again")
				continue
			}
			log.Printf("Policy filter active: only updating policy %q (matched from input %q)", resolvedName, name)
			return resolvedName
		}
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
