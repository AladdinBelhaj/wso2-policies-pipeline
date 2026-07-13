package client

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"wso2/scripts/vars"
)

// This function verifies if the revisions number has reached 5 or not.
func ReviewRevisionsNumber(apiId string) (string, bool) {
	jsonObject, err := newCurlCmd(vars.BaseUrl+vars.Endpoint+apiId+vars.PathRevision, "-k").Output()
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

// This function fetches the ID of each revision.
func GetRevisionIds(apiId string) ([]string, error) {
	jsonObject, err := newCurlCmd(vars.BaseUrl+vars.Endpoint+apiId+vars.PathRevision, "-k").Output()
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

// This function undeploys a revision before deleting it
func UndeployRevision(apiId string, revisionId string) error {
	cmd := newCurlCmd(
		"-X", "POST",
		vars.BaseURL+vars.Endpoint+apiId+"/undeploy-revision?revisionId="+revisionId,
		"-H", "Content-Type: application/json",
		"-k",
		"-s", "-w", "\nHTTP_STATUS:%{http_code}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("undeploy revision curl error: %v, output: %s", err, output)
	}

	statusCode, body, err := parseCurlStatus(output, fmt.Sprintf("undeploy revision %s", revisionId))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(statusCode, "2") {
		return fmt.Errorf("undeploy revision failed with HTTP %s: %s", statusCode, body)
	}

	fmt.Printf("Undeployed revision %s successfully\n", revisionId)
	return nil
}


// This function deletes a revision without attempting to undeploy it first
func DeleteRevision(apiId string, revisionId string) error {
	cmd := newCurlCmd(
		"-X", "DELETE",
		vars.BaseURL+vars.Endpoint+apiId+vars.PathRevision+revisionId,
		"-k",
		"-s", "-w", "\nHTTP_STATUS:%{http_code}")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("delete revision curl error: %v, output: %s", err, out)
	}

	statusCode, body, err := parseCurlStatus(out, fmt.Sprintf("delete revision %s", revisionId))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(statusCode, "2") {
		return fmt.Errorf("delete revision failed with HTTP %s: %s", statusCode, body)
	}

	fmt.Printf("Deleted revision %s successfully\n", revisionId)
	return nil
}

// This function creates a revision.
func CreateRevision(apiId string) string {
	payload := []byte(`{}`)
	cmd := newCurlCmd(
		"-X", "POST",
		vars.BaseURL+vars.Endpoint+apiId+vars.PathRevision,
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

// This function deploys a revision.
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
		vars.BaseURL+vars.Endpoint+apiId+"/deploy-revision?revisionId="+revisionId,
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

// This function restores a previous revision to the API.
func RestoreRevision(apiId string, revisionId string) error {
	
	cmd := newCurlCmd(
		"-X", "POST",
		vars.BaseURL+vars.Endpoint+apiId+"/restore-revision?revisionId="+revisionId,
		"-H", "Content-Type: application/json",
		"-k",
		"-s", "-w", "\nHTTP_STATUS:%{http_code}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restore revision curl error: %v, output: %s", err, output)
	}

	statusCode, body, err := parseCurlStatus(output, fmt.Sprintf("restore revision %s", revisionId))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(statusCode, "2") {
		return fmt.Errorf("restore revision failed with HTTP %s: %s", statusCode, body)
	}

	fmt.Println("Revision restored successfully")
	return nil
}

// This function deletes the oldest revision (if number of revisions == 5), creates and deploys a new revision.
func PrepareAndDeployRevision(apiId string) {
	if oldestRevision, found := ReviewRevisionsNumber(apiId); found {
		fmt.Printf("Found 5 revisions; deleting oldest revision %s\n", oldestRevision)
		DeleteRevision(apiId, oldestRevision)
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
