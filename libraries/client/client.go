// This package handles all HTTP calls to the WSO2 Publisher API.
package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"wso2/scripts/vars"
)

type ApiSummary struct {
	ID   string
	Name string
}

// This function creates a new exec.Cmd for a curl command with the provided arguments to interact with WSO2 API Manager
func newCurlCmd(args ...string) *exec.Cmd {
	curlArgs := append([]string{"-u", vars.Username + ":" + vars.Password}, args...)
	return exec.Command("curl", curlArgs...)
}

// This function parses the output of a curl command to extract the HTTP status code and response body.
func parseCurlStatus(output []byte, action string) (string, string, error) {
	outStr := string(output)
	idx := strings.LastIndex(outStr, "HTTP_STATUS:")
	if idx == -1 {
		return "", "", fmt.Errorf("%s: no status code in output: %s", action, strings.TrimSpace(outStr))
	}

	statusCode := strings.TrimSpace(outStr[idx+len("HTTP_STATUS:"):])
	body := strings.TrimSpace(outStr[:idx])
	return statusCode, body, nil
}


func normalizePolicyList(data map[string]any) []map[string]interface{} {
	list, ok := data["list"].([]interface{})
	if !ok {
		return nil
	}

	policies := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		policy, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		policies = append(policies, map[string]interface{}{
			"id":      policy["id"],
			"name":    policy["name"],
			"version": policy["version"],
		})
	}

	return policies
}

// This function executes a curl command to fetch the JSON object from the /apis endpoint
func getApiJsonObject() []byte {
	jsonObject, err := newCurlCmd(vars.BaseUrl+"/apis", "-k").Output()
	if err != nil {
		log.Fatal(err)
	}

	return jsonObject
}

// This function iterates through the JSON object and fetches the ID and name of each API.
func ExtractApiSummaries() []ApiSummary {
	jsonObject := getApiJsonObject()

	var data map[string]any
	if err := json.Unmarshal(jsonObject, &data); err != nil {
		log.Fatal(err)
	}

	list := data["list"].([]interface{})
	apis := make([]ApiSummary, 0, len(list))

	for _, item := range list {
		api, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		apiSummary := ApiSummary{}
		if id, ok := api["id"].(string); ok {
			apiSummary.ID = id
		}
		if name, ok := api["name"].(string); ok {
			apiSummary.Name = name
		}
		apis = append(apis, apiSummary)
	}

	return apis
}

// This function iterates through the JSON object and fetches the ID of each API.
func ExtractApiIds() []string {
	apiSummaries := ExtractApiSummaries()
	apiIds := make([]string, 0, len(apiSummaries))
	for _, api := range apiSummaries {
		if api.ID != "" {
			apiIds = append(apiIds, api.ID)
		}
	}
	return apiIds
}

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

	normalizedTarget := strings.ToLower(target)
	matchedIDs := make([]string, 0)
	for _, api := range apis {
		if api.ID == "" {
			continue
		}
		if strings.Contains(strings.ToLower(api.Name), normalizedTarget) {
			matchedIDs = append(matchedIDs, api.ID)
		}
	}

	return matchedIDs
}

func BuildRollbackPreview(apiIDs []string, target string) string {
	targetLabel := "all APIs"
	if strings.TrimSpace(target) != "" && !strings.EqualFold(target, "all") {
		targetLabel = fmt.Sprintf("APIs matching %q", target)
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

// This function extracts the API IDs from the JSON object returned by the /apis endpoint
func GetApiDetailsJsonObject(apiId string) []byte {
	jsonObject, err := newCurlCmd(vars.BaseUrl+"/apis/"+apiId, "-k").Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
}

// This function iterates through the list of API IDs and fetches the details for each API, extracting the policies for each operation
func ExtractApiPolicies(apiIds []string) []map[string]any {

	apiPolicies := make([]map[string]any, 0)

	for _, apiId := range apiIds {
		var api map[string]any
		data := GetApiDetailsJsonObject(apiId)
		err := json.Unmarshal(data, &api)
		if err != nil {
			log.Fatal(err)
		}

		operations := make([]map[string]any, 0)

		for _, operation := range api["operations"].([]interface{}) {
			op := operation.(map[string]interface{})

			operationObject := map[string]any{
				"target":            op["target"],
				"verb":              op["verb"],
				"operationPolicies": op["operationPolicies"],
			}

			operations = append(operations, operationObject)
		}

		apiObject := map[string]any{
			"apiId":      api["id"],
			"operations": operations,
		}

		apiPolicies = append(apiPolicies, apiObject)
	}

	return apiPolicies

}

// This function fetches all common operation policies
func getOperationPoliciesJsonObject() []byte {
	jsonObject, err := newCurlCmd(vars.BaseUrl+"/operation-policies", "-k").Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
}

// This function extracts the policies and the metadata from the JSON objects
func ExtractOperationPolicies() []map[string]interface{} {
	jsonObject := getOperationPoliciesJsonObject()

	var data map[string]any
	if err := json.Unmarshal(jsonObject, &data); err != nil {
		log.Fatal(err)
	}

	return normalizePolicyList(data)
}

// This function fetches API level policies
func getApiLevelPoliciesJsonObject(apiId string) []byte {
	jsonObject, err := newCurlCmd(vars.BaseUrl+"/apis/"+apiId+"/operation-policies", "-k").Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
}

// This function fetches and normalizes the API-specific policies for a single API.
func ExtractApiLevelPolicies(apiId string) []map[string]interface{} {
	jsonObject := getApiLevelPoliciesJsonObject(apiId)

	var data map[string]any
	if err := json.Unmarshal(jsonObject, &data); err != nil {
		log.Fatal(err)
	}

	return normalizePolicyList(data)
}

// This function sends the updated API JSON back to WSO2 via PUT /apis/{apiId}.
func PutApiUpdate(apiId string, payload []byte) error {
	cmd := newCurlCmd(
		"-X", "PUT",
		vars.BaseUrl+"/apis/"+apiId,
		"-H", "Content-Type: application/json",
		"-d", "@-",
		"-k",
		"-s", "-w", "\nHTTP_STATUS:%{http_code}")

	cmd.Stdin = strings.NewReader(string(payload))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("PUT /apis/%s curl error: %v, output: %s", apiId, err, output)
	}

	statusCode, body, err := parseCurlStatus(output, fmt.Sprintf("PUT /apis/%s", apiId))
	if err != nil {
		return err
	}
	if strings.HasPrefix(statusCode, "2") {
		fmt.Printf("PUT /apis/%s: OK (HTTP %s)\n", apiId, statusCode)
		return nil
	}

	fmt.Printf("PUT /apis/%s: FAILED (HTTP %s) - %s\n", apiId, statusCode, body)
	return fmt.Errorf("HTTP %s", statusCode)
}

// This function verifies if the revisions number has reached 5 or not
func ReviewRevisionsNumber(apiId string) (string, bool) {
	jsonObject, err := newCurlCmd(vars.BaseUrl+"/apis/"+apiId+"/revisions", "-k").Output()
	if err != nil {
		log.Fatal(err)
	}

	var data map[string]any

	if err := json.Unmarshal(jsonObject, &data); err != nil {
		log.Fatal(err)
	}

	count := data["count"].(float64)

	if count == 5 {
		list := data["list"].([]interface{})
		return list[0].(map[string]interface{})["id"].(string), true
	}

	return "", false
}

// This function fetches the ID of each revision
func GetRevisionIds(apiId string) ([]string, error) {
	jsonObject, err := newCurlCmd(vars.BaseUrl+"/apis/"+apiId+"/revisions", "-k").Output()
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if err := json.Unmarshal(jsonObject, &data); err != nil {
		return nil, err
	}

	list, ok := data["list"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected revisions response format for API %s", apiId)
	}

	revisions := make([]string, 0, len(list))
	for _, item := range list {
		revision, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if revisionID, ok := revision["id"].(string); ok {
			revisions = append(revisions, revisionID)
		}
	}

	return revisions, nil
}

// This function rolls back to the previous revision
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

// This function deletes the oldest revision (before rolling back)
func DeleteOldestRevision(apiId string, revisionId string) {
	cmd := newCurlCmd(
		"-X", "DELETE",
		vars.BaseUrl+"/apis/"+apiId+"/revisions/"+revisionId,
		"-k",
		"-s", "-w", "\nHTTP_STATUS:%{http_code}")

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("delete revision curl error: %v, output: %s", err, out)
	}

	statusCode, body, err := parseCurlStatus(out, fmt.Sprintf("delete revision %s", revisionId))
	if err != nil {
		log.Fatal(err)
	}
	if !strings.HasPrefix(statusCode, "2") {
		log.Fatalf("delete revision failed with HTTP %s: %s", statusCode, body)
	}

	fmt.Printf("Deleted revision %s successfully\n", revisionId)
}

// This function creates a revision
func CreateRevision(apiId string) string {
	payload := []byte(`{}`)
	cmd := newCurlCmd(
		"-X", "POST",
		vars.BaseUrl+"/apis/"+apiId+"/revisions",
		"-H", "Content-Type: application/json",
		"-d", string(payload),
		"-k",
		"-s", "-w", "\nHTTP_STATUS:%{http_code}")

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("create revision curl error: %v, output: %s", err, out)
	}

	statusCode, body, err := parseCurlStatus(out, fmt.Sprintf("create revision for API %s", apiId))
	if err != nil {
		log.Fatal(err)
	}
	if !strings.HasPrefix(statusCode, "2") {
		log.Fatalf("create revision failed with HTTP %s: %s", statusCode, body)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		log.Fatalf("create revision response parse error: %v, body: %s", err, body)
	}

	if revisionID, ok := data["id"].(string); ok {
		return revisionID
	}

	return ""
}

// This function deploys a revision

func DeployRevision(apiId string, revisionId string) error {
	payload := `[
		{
			"name": "Default",
			"vhost": "localhost",
			"displayOnDevportal": true
		}
	]`

	cmd := newCurlCmd(
		"-X", "POST",
		vars.BaseUrl+"/apis/"+apiId+"/deploy-revision?revisionId="+revisionId,
		"-H", "Content-Type: application/json",
		"-d", payload,
		"-k",
		"-s", "-w", "\nHTTP_STATUS:%{http_code}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("deploy revision curl error: %v, output: %s", err, output)
	}

	statusCode, body, err := parseCurlStatus(output, fmt.Sprintf("deploy revision %s", revisionId))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(statusCode, "2") {
		return fmt.Errorf("deploy revision failed with HTTP %s: %s", statusCode, body)
	}

	fmt.Println("Revision deployed successfully")
	return nil
}

// This function deletes the oldest revision (if number of revisions == 5), creates and deploys a new revision
func PrepareAndDeployRevision(apiId string) {
	if oldestRevision, found := ReviewRevisionsNumber(apiId); found {
		fmt.Printf("Found 5 revisions; deleting oldest revision %s\n", oldestRevision)
		DeleteOldestRevision(apiId, oldestRevision)
	}

	newRevisionID := CreateRevision(apiId)
	if newRevisionID == "" {
		log.Fatal("failed to create a new revision")
	}

	fmt.Printf("Created revision %s; deploying it\n", newRevisionID)
	if err := DeployRevision(apiId, newRevisionID); err != nil {
		log.Fatal(err)
	}
}
