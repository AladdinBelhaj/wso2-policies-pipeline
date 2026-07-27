package client

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// This function will filter each API ID by its exact name.
func FilterApiIdsByName(apis []ApiSummary, target string) []string {
	target = strings.TrimSpace(target)
	if target == "" || strings.EqualFold(target, "all") {
		ids := make([]string, 0, len(apis))
		for _, api := range apis {
			if api.ID != "" {
				ids = append(ids, api.ID)
			}
		}
		return ids
	}

	matchedIDs := make([]string, 0)
	for _, api := range apis {
		if api.ID == "" {
			continue
		}
		if strings.EqualFold(api.Name, target) {
			matchedIDs = append(matchedIDs, api.ID)
		}
	}

	return matchedIDs
}

// FilterApiProductIdsByName filters API Product IDs by exact name match.
func FilterApiProductIdsByName(products []ApiProductSummary, target string) []string {
	target = strings.TrimSpace(target)
	if target == "" || strings.EqualFold(target, "all") {
		ids := make([]string, 0, len(products))
		for _, p := range products {
			if p.ID != "" {
				ids = append(ids, p.ID)
			}
		}
		return ids
	}

	matchedIDs := make([]string, 0)
	for _, p := range products {
		if p.ID == "" {
			continue
		}
		if strings.EqualFold(p.Name, target) {
			matchedIDs = append(matchedIDs, p.ID)
		}
	}

	return matchedIDs
}

// This function prompts the user for confirmation before proceeding with an action (rollback)
func ConfirmAction(preview string) bool {
	fmt.Println(preview)
	fmt.Print("Proceed? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

// This function rolls back to the previous revision.
func RollbackApiRevision(apiId string) error {
	apiName := GetApiName(apiId)
	revisionIDs, err := GetRevisionIds(apiId)
	if err != nil {
		return fmt.Errorf("fetch revisions for API %s: %w", apiId, err)
	}

	if len(revisionIDs) <= 1 {
		fmt.Printf("Cannot rollback API %s because there is only %d revision\n", apiName, len(revisionIDs))
		return nil
	}

	targetRevisionID := revisionIDs[len(revisionIDs)-2]
	lastRevisionID := revisionIDs[len(revisionIDs)-1]

	fmt.Printf("Rolling back API %s to revision %s\n", apiName, targetRevisionID)
	// 1. Restore state of target revision (e.g. revision 3)
	if err := RestoreRevision(apiId, targetRevisionID); err != nil {
		return fmt.Errorf("restore rollback target for API %s: %w", apiId, err)
	}

	// 2. Undeploy current revision (e.g. revision 4)
	if err := UndeployRevision(apiId, lastRevisionID); err != nil {
		return fmt.Errorf("undeploy current revision for API %s: %w", apiId, err)
	}

	// 3. Deploy target revision (e.g. revision 3)
	if err := DeployRevision(apiId, targetRevisionID); err != nil {
		return fmt.Errorf("deploy target revision for API %s: %w", apiId, err)
	}

	// 4. Delete current revision (e.g. revision 4)
	if err := DeleteRevision(apiId, lastRevisionID); err != nil {
		return fmt.Errorf("delete rolled back revision for API %s: %w", apiId, err)
	}

	return nil
}

// RollbackProductRevision rolls back an API Product to the previous revision.
func RollbackProductRevision(productId string) error {
	productName := GetApiProductName(productId)
	revisionIDs, err := GetProductRevisionIds(productId)
	if err != nil {
		return fmt.Errorf("fetch revisions for API Product %s: %w", productId, err)
	}

	if len(revisionIDs) <= 1 {
		fmt.Printf("Cannot rollback API Product %s because there is only %d revision\n", productName, len(revisionIDs))
		return nil
	}

	targetRevisionID := revisionIDs[len(revisionIDs)-2]
	lastRevisionID := revisionIDs[len(revisionIDs)-1]

	fmt.Printf("Rolling back API Product %s to revision %s\n", productName, targetRevisionID)
	// 1. Restore state of target revision
	if err := RestoreProductRevision(productId, targetRevisionID); err != nil {
		return fmt.Errorf("restore rollback target for API Product %s: %w", productId, err)
	}

	// 2. Undeploy current revision
	if err := UndeployProductRevision(productId, lastRevisionID); err != nil {
		return fmt.Errorf("undeploy current revision for API Product %s: %w", productId, err)
	}

	// 3. Deploy target revision
	if err := DeployProductRevision(productId, targetRevisionID); err != nil {
		return fmt.Errorf("deploy target revision for API Product %s: %w", productId, err)
	}

	// 4. Delete current revision
	if err := DeleteProductRevision(productId, lastRevisionID); err != nil {
		return fmt.Errorf("delete rolled back revision for API Product %s: %w", productId, err)
	}

	return nil
}
