package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"wso2/pctl/libraries/client"
	"wso2/pctl/libraries/policy"
	"wso2/pctl/libraries/report"
	"wso2/pctl/vars"

	"github.com/spf13/cobra"
)

var (
	dryRun  bool
	envFlag string

	addEnvBaseURL  string
	addEnvUsername string
	addEnvPassword string

	publishDisplayName string
	publishCategory    string
	publishDescription string
	publishFlows       []string
	publishApiTypes    []string
	publishGateways    []string
)

var rootCmd = &cobra.Command{
	Use:   "pctl",
	Short: "pctl is a CLI tool for WSO2 policy deployment and management",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		vars.EnvOverride = envFlag
		vars.LoadEnvironments()
	},
}

var updateCmd = &cobra.Command{
	Use:   "update [API_NAME]",
	Short: "Update policies for all APIs or a specific API",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := vars.ResolveEnv(); err != nil {
			log.Fatal(err)
		}
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
		if err := vars.ResolveEnv(); err != nil {
			log.Fatal(err)
		}
		target := ""
		if len(args) > 0 {
			target = args[0]
		}
		executeRollback(target, dryRun)
	},
}

var publishPolicyCmd = &cobra.Command{
	Use:   "publish-policy [POLICY_NAME] [DEFINITION_FILE.j2]",
	Short: "Publish a new version of an operation policy from a Synapse .j2 definition to WSO2",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := vars.ResolveEnv(); err != nil {
			log.Fatal(err)
		}
		opts := policy.PublishPolicyOptions{
			Name:              args[0],
			DefinitionPath:    args[1],
			DisplayName:       publishDisplayName,
			Category:          publishCategory,
			Description:       publishDescription,
			ApplicableFlows:   publishFlows,
			SupportedApiTypes: publishApiTypes,
			SupportedGateways: publishGateways,
		}
		if err := policy.PublishPolicy(opts); err != nil {
			log.Fatal(err)
		}
	},
}

var setEnvCmd = &cobra.Command{
	Use:   "set-env [ENV_NAME]",
	Short: "Persist the environment pctl should use for subsequent commands",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := vars.SetEnv(args[0]); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Environment set to %q (saved to %s)\n", args[0], vars.ConfigPath)
	},
}

var addEnvCmd = &cobra.Command{
	Use:   "add-env [ENV_NAME]",
	Short: "Add a new environment to the config file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		env, err := resolveNewEnvConfig(cmd)
		if err != nil {
			log.Fatal(err)
		}
		if err := vars.AddEnv(name, env); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Environment %q added (saved to %s)\n", name, vars.ConfigPath)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&envFlag, "env", "e", "", "Environment to use for this command (overrides the persisted current_env, without saving it)")

	updateCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Preview changes without applying them")
	rollbackCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Preview rollback without applying")

	addEnvCmd.Flags().StringVar(&addEnvBaseURL, "burl", "", "Base URL for the new environment")
	addEnvCmd.Flags().StringVar(&addEnvUsername, "username", "", "Username for the new environment")
	addEnvCmd.Flags().StringVar(&addEnvPassword, "password", "", "Password for the new environment")

	publishPolicyCmd.Flags().StringVar(&publishDisplayName, "display-name", "", "Display name shown in the WSO2 UI (defaults to POLICY_NAME)")
	publishPolicyCmd.Flags().StringVar(&publishCategory, "category", "Mediation", "Policy category")
	publishPolicyCmd.Flags().StringVar(&publishDescription, "description", "", "Policy description")
	publishPolicyCmd.Flags().StringSliceVar(&publishFlows, "flows", []string{"request", "response", "fault"}, "Applicable flows")
	publishPolicyCmd.Flags().StringSliceVar(&publishApiTypes, "api-types", []string{"HTTP"}, "Supported API types")
	publishPolicyCmd.Flags().StringSliceVar(&publishGateways, "gateways", []string{"Synapse"}, "Supported gateways")

	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(publishPolicyCmd)
	rootCmd.AddCommand(setEnvCmd)
	rootCmd.AddCommand(addEnvCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func executeRollback(target string, dryRun bool) {
	reportData := report.RollbackReportData{
		Timestamp:    time.Now(),
		Environment:  vars.CurrentEnv,
		BaseURL:      vars.BaseURL,
		TargetFilter: target,
		IsDryRun:     dryRun,
	}
	if target == "" {
		reportData.TargetFilter = "All APIs"
	}

	apiIds := fetchApiIds(target, true)
	productIds := client.FindApiProductIdsUsingApis(apiIds)

	if len(apiIds) == 0 && target != "" {
		productSummaries := client.ExtractApiProductSummaries()
		targetProductIds := client.FilterApiProductIdsByName(productSummaries, target)
		if len(targetProductIds) > 0 {
			productIds = targetProductIds
		} else {
			log.Printf("API or API Product %q does not exist", target)
			reportData.Status = "FAILED"
			reportData.Summary = fmt.Sprintf("API or API Product %q does not exist", target)
			report.GenerateRollbackReport(reportData)
			return
		}
	}

	if len(apiIds) == 0 && len(productIds) == 0 {
		reportData.Status = "NO_TARGETS"
		reportData.Summary = "No matching APIs or API Products found"
		report.GenerateRollbackReport(reportData)
		return
	}

	for _, apiId := range apiIds {
		apiName := client.GetApiName(apiId)
		revs, _ := client.GetRevisionIds(apiId)
		currentPolicies := policy.ListCurrentPolicies(apiId)
		deployments := client.GetApiDeployments(apiId)

		detail := report.ApiRollbackDetail{
			ID:              apiId,
			Name:            apiName,
			TotalRevisions:  len(revs),
			InitialPolicies: currentPolicies,
			Deployments:     deployments,
		}

		if len(revs) <= 1 {
			detail.Status = "CANNOT_ROLLBACK"
			detail.Error = fmt.Sprintf("Only %d revision(s) available", len(revs))
		} else {
			detail.RolledBackFrom = revs[len(revs)-1]
			detail.RolledBackTo = revs[len(revs)-2]
		}
		reportData.APIs = append(reportData.APIs, detail)
	}

	for _, pid := range productIds {
		pName := client.GetApiProductName(pid)
		revs, _ := client.GetProductRevisionIds(pid)
		deployments := client.GetApiProductDeployments(pid)

		pDetail := report.ProductRollbackDetail{
			ID:             pid,
			Name:           pName,
			TotalRevisions: len(revs),
			Deployments:    deployments,
		}
		if len(revs) <= 1 {
			pDetail.Status = "CANNOT_ROLLBACK"
			pDetail.Error = fmt.Sprintf("Only %d revision(s) available", len(revs))
		} else {
			pDetail.RolledBackFrom = revs[len(revs)-1]
			pDetail.RolledBackTo = revs[len(revs)-2]
		}
		reportData.Products = append(reportData.Products, pDetail)
	}

	preview := buildRollbackPreview(apiIds, productIds)

	if dryRun || !client.ConfirmAction(preview) {
		if dryRun {
			log.Println("dry run: no changes were applied")
			reportData.Status = "DRY_RUN"
			reportData.Summary = "Dry run rollback preview generated"
		} else {
			log.Println("operation cancelled")
			reportData.Status = "CANCELLED"
			reportData.Summary = "Rollback operation cancelled by user"
		}
		report.GenerateRollbackReport(reportData)
		return
	}

	for i, api := range reportData.APIs {
		if api.Status == "CANNOT_ROLLBACK" {
			continue
		}
		log.Printf("[%d/%d] Rolling back API %s...", i+1, len(reportData.APIs), api.Name)
		if err := client.RollbackApiRevision(api.ID); err != nil {
			log.Printf("rollback failed for API %s: %v", api.Name, err)
			reportData.APIs[i].Status = "FAILED"
			reportData.APIs[i].Error = err.Error()
		} else {
			reportData.APIs[i].Success = true
			reportData.APIs[i].Status = "ROLLED_BACK"
			reportData.APIs[i].PostRollbackPolicies = policy.ListCurrentPolicies(api.ID)
		}
	}

	if len(reportData.Products) > 0 {
		for i, prod := range reportData.Products {
			if prod.Status == "CANNOT_ROLLBACK" {
				continue
			}
			log.Printf("[%d/%d] Rolling back API Product %s...", i+1, len(reportData.Products), prod.Name)
			if err := client.RollbackProductRevision(prod.ID); err != nil {
				log.Printf("rollback failed for API Product %s: %v", prod.Name, err)
				reportData.Products[i].Status = "FAILED"
				reportData.Products[i].Error = err.Error()
			} else {
				reportData.Products[i].Success = true
				reportData.Products[i].Status = "ROLLED_BACK"
			}
		}
	}

	reportData.Status = "SUCCESS"
	reportData.Summary = "Rollback operation completed"
	report.GenerateRollbackReport(reportData)
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
	reportData := report.UpdateReportData{
		Timestamp:    time.Now(),
		Environment:  vars.CurrentEnv,
		BaseURL:      vars.BaseURL,
		TargetFilter: target,
		IsDryRun:     dryRun,
	}
	if target == "" {
		reportData.TargetFilter = "All APIs"
	}

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
			reportData.Status = "FAILED"
			reportData.Summary = fmt.Sprintf("API or API Product %q does not exist", target)
			report.GenerateUpdateReport(reportData)
			return
		}
	}

	if len(apiIds) == 0 && len(productIds) == 0 {
		reportData.Status = "NO_TARGETS"
		reportData.Summary = "No matching APIs or API Products found"
		report.GenerateUpdateReport(reportData)
		return
	}

	allPolicies := client.ExtractOperationPolicies()
	policyFilter := promptPolicyChoice(allPolicies, apiIds)
	reportData.PolicyFilter = policyFilter
	if policyFilter == "" {
		reportData.PolicyFilter = "All Policies"
	}

	for _, apiId := range apiIds {
		apiName := client.GetApiName(apiId)
		revs, _ := client.GetRevisionIds(apiId)
		deployments := client.GetApiDeployments(apiId)
		currentPolicies := policy.ListCurrentPolicies(apiId)
		plannedChanges := policy.PreviewApiPolicyUpdates(apiId, allPolicies, policyFilter)

		lastRev := ""
		if len(revs) > 0 {
			lastRev = revs[len(revs)-1]
		}

		apiDetail := report.ApiUpdateDetail{
			ID:               apiId,
			Name:             apiName,
			PreviousRevision: lastRev,
			Deployments:      deployments,
			PlannedChanges:   plannedChanges,
			CurrentPolicies:  currentPolicies,
		}
		reportData.APIs = append(reportData.APIs, apiDetail)
	}

	for _, pid := range productIds {
		pName := client.GetApiProductName(pid)
		revs, _ := client.GetProductRevisionIds(pid)
		deployments := client.GetApiProductDeployments(pid)
		lastRev := ""
		if len(revs) > 0 {
			lastRev = revs[len(revs)-1]
		}
		pDetail := report.ProductUpdateDetail{
			ID:               pid,
			Name:             pName,
			PreviousRevision: lastRev,
			Deployments:      deployments,
		}
		reportData.Products = append(reportData.Products, pDetail)
	}

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
			reportData.Status = "CANCELLED"
			reportData.Summary = "Dry run preview cancelled by user"
			report.GenerateUpdateReport(reportData)
			return
		}
		reportData.Status = "DRY_RUN_CONFIRMED"
		reportData.Summary = "Dry run preview confirmed"
		report.GenerateUpdateReport(reportData)
		return
	}

	apiDetails := client.ExtractApiPolicies(apiIds)
	modifiedApis := policy.UpdateApiPolicies(apiDetails, allPolicies, policyFilter)

	for i, api := range reportData.APIs {
		if modifiedApis[api.ID] {
			reportData.APIs[i].Modified = true
			reportData.APIs[i].Status = "MODIFIED_AND_DEPLOYED"
			newRevs, _ := client.GetRevisionIds(api.ID)
			if len(newRevs) > 0 {
				reportData.APIs[i].NewRevision = newRevs[len(newRevs)-1]
			}
			reportData.APIs[i].CurrentPolicies = policy.ListCurrentPolicies(api.ID)
		} else {
			reportData.APIs[i].Status = "NO_CHANGES"
		}
	}

	if len(productIds) > 0 {
		updatableProductIds := filterProductsWithChanges(productRefs, modifiedApis)
		if len(updatableProductIds) > 0 {
			selectedProductIds := promptApiProductUpdateMode(updatableProductIds)
			if len(selectedProductIds) > 0 {
				policy.RestoreApiProductPolicies(selectedProductIds, apiDetails, allPolicies, policyFilter)
				for i, prod := range reportData.Products {
					for _, selId := range selectedProductIds {
						if prod.ID == selId {
							reportData.Products[i].Updated = true
							reportData.Products[i].Status = "UPDATED_AND_DEPLOYED"
							newRevs, _ := client.GetProductRevisionIds(prod.ID)
							if len(newRevs) > 0 {
								reportData.Products[i].NewRevision = newRevs[len(newRevs)-1]
							}
						}
					}
				}
			}
		}
	}

	reportData.Status = "SUCCESS"
	reportData.Summary = "Policy updates successfully processed"
	report.GenerateUpdateReport(reportData)
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

// promptApiProductUpdateMode asks whether to update all eligible API
// Products at once or decide one by one, then returns the resulting
// selection. On EOF/error, defaults to no selection (nothing updated).
func promptApiProductUpdateMode(productIds []string) []string {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Update API Products:")
	fmt.Println("  1) All API Products")
	fmt.Println("  2) Choose specific API Products")
	fmt.Print("Enter choice [1/2]: ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return nil
	}
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1":
		return productIds
	case "2":
		return promptApiProductSelectionOneByOne(reader, productIds)
	default:
		fmt.Println("Invalid choice, defaulting to ONE by ONE")
		return promptApiProductSelectionOneByOne(reader, productIds)
	}
}

// promptApiProductSelectionOneByOne displays each fetched API product and asks the user whether to update it.
// Prompt format: "Update API Product <name> [y/N]: "
func promptApiProductSelectionOneByOne(reader *bufio.Reader, productIds []string) []string {
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

// resolveNewEnvConfig determines the settings for a new environment being
// added via `add-env`. If none of the config flags were passed, it prompts
// interactively for each value. If some but not all three were passed, it
// errors out and asks the user to provide all of them together.
func resolveNewEnvConfig(cmd *cobra.Command) (vars.Env, error) {
	flagsSet := 0
	for _, name := range []string{"burl", "username", "password"} {
		if cmd.Flags().Changed(name) {
			flagsSet++
		}
	}

	if flagsSet == 0 {
		return promptNewEnvConfig()
	}

	if flagsSet < 3 {
		return vars.Env{}, fmt.Errorf("please provide all flags together (--burl, --username, --password), or none of them to enter values interactively")
	}

	return vars.Env{
		BaseURL:  addEnvBaseURL,
		Username: addEnvUsername,
		Password: addEnvPassword,
	}, nil
}

// promptNewEnvConfig interactively collects the environment values
// from stdin, used when `add-env [ENV_NAME]` is run with no config flags.
func promptNewEnvConfig() (vars.Env, error) {
	reader := bufio.NewReader(os.Stdin)
	var env vars.Env
	var err error

	fmt.Print("Enter base_url: ")
	if env.BaseURL, err = readTrimmedLine(reader); err != nil {
		return vars.Env{}, err
	}

	fmt.Print("Enter username: ")
	if env.Username, err = readTrimmedLine(reader); err != nil {
		return vars.Env{}, err
	}

	fmt.Print("Enter password: ")
	if env.Password, err = readTrimmedLine(reader); err != nil {
		return vars.Env{}, err
	}

	return env, nil
}

func readTrimmedLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
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
