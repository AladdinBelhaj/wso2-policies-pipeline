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

// This function will provide the preview before rolling back
func BuildRollbackPreview(apiIDs []string, target string) string {
	targetLabel := "all APIs"
	if strings.TrimSpace(target) != "" && !strings.EqualFold(target, "all") {
		targetLabel = fmt.Sprintf("API matching %q", target)
	}

	preview := []string{
		fmt.Sprintf("Dry run: %s", targetLabel),
		fmt.Sprintf("%d API(s) will be processed:", len(apiIDs)),
	}

	for _, apiID := range apiIDs {
		preview = append(preview, fmt.Sprintf("- %s", apiID))
	}

	return strings.Join(preview, "\n")
}

// This function prompts the user for confirmation before proceeding with an action.
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
	revisionIDs, err := GetRevisionIds(apiId)
	if err != nil {
		return fmt.Errorf("fetch revisions for API %s: %w", apiId, err)
	}

	if len(revisionIDs) == 1 {
		fmt.Printf("Cannot rollback API %s because there is only 1 revision\n", apiId)
		return nil
	}

	if len(revisionIDs) == 2 {
		fmt.Printf("Cannot rollback API %s because there are only 2 revisions\n", apiId)
		return nil
	}

	targetRevisionID := revisionIDs[len(revisionIDs)-2]
	revisionToRemove := revisionIDs[len(revisionIDs)-1]

	fmt.Printf("Rolling back API %s to revision %s\n", apiId, targetRevisionID)
	if err := DeployRevision(apiId, targetRevisionID); err != nil {
		return fmt.Errorf("deploy rollback target for API %s: %w", apiId, err)
	}

	if len(revisionIDs) > 2 {
		fmt.Printf("Deleting revision %s for API %s\n", revisionToRemove, apiId)
		DeleteOldestRevision(apiId, revisionToRemove)
	} else {
		fmt.Printf("Skipping revision deletion for API %s because there are only 2 revisions\n", apiId)
	}

	return nil
}
