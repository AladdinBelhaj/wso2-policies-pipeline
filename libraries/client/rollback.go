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
	lastRevisionID := revisionIDs[len(revisionIDs)-1]

	fmt.Printf("Rolling back API %s to revision %s\n", apiId, targetRevisionID)
	if err := RestoreRevision(apiId, targetRevisionID); err != nil {
		return fmt.Errorf("restore rollback target for API %s: %w", apiId, err)
	}

	if err := UndeployRevision(apiId, lastRevisionID); err != nil {
		return fmt.Errorf("\nundeploy oldest revision for API %s: %w", apiId, err)
	}

	if err := DeleteRevision(apiId, lastRevisionID); err != nil {
		return fmt.Errorf("\ndelete oldest revision for API %s: %w", apiId, err)
	}

	if err := DeployRevision(apiId, targetRevisionID); err != nil {
		return fmt.Errorf("deploy new revision for API %s: %w", apiId, err)
	}

	// if oldestRevision, found := ReviewRevisionsNumber(apiId); found {
	// 	fmt.Printf("Found 5 revisions; deleting oldest revision %s to make room for new revision\n", oldestRevision)
	// 	if err := DeleteRevision(apiId, oldestRevision); err != nil {
	// 		return fmt.Errorf("delete oldest revision for API %s: %w", apiId, err)
	// 	}
	// }

	// newRevisionID := CreateRevision(apiId)
	// if newRevisionID == "" {
	// 	return fmt.Errorf("failed to create a new revision for API %s", apiId)
	// }

	// fmt.Printf("Created new revision %s from restored state; deploying it\n", newRevisionID)
	// if err := DeployRevision(apiId, newRevisionID); err != nil {
	// 	return fmt.Errorf("deploy new revision for API %s: %w", apiId, err)
	// }

	return nil
}
