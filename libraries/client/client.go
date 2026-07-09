// This package handles all HTTP calls to the WSO2 Publisher API.
package client

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"wso2/scripts/vars"
)

// This function executes a curl command to fetch the JSON object from the /apis endpoint
func getApiJsonObject() []byte {

	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password, vars.BaseUrl+"/apis", "-k")
	jsonObject, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	return jsonObject
}

// This function iterates through the JSON object and fetches the ID of each API
func ExtractApiIds() []string {

	jsonObject := getApiJsonObject()

	var data map[string]any

	apiIds := make([]string, 0)

	err := json.Unmarshal(jsonObject, &data)

	if err != nil {
		log.Fatal(err)
	}

	list := data["list"].([]interface{})

	for _, item := range list {
		api := item.(map[string]interface{})

		apiIds = append(apiIds, api["id"].(string))
	}

	return apiIds
}

// This function extracts the API IDs from the JSON object returned by the /apis endpoint
func GetApiDetailsJsonObject(apiId string) []byte {

	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password, vars.BaseUrl+"/apis/"+apiId, "-k")
	jsonObject, err := cmd.Output()
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

func getOperationPoliciesJsonObject() []byte {
	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password, vars.BaseUrl+"/operation-policies", "-k")
	jsonObject, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
}

func getApiLevelPoliciesJsonObject(apiId string) []byte {
	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password, vars.BaseUrl+"/apis/"+apiId+"/policies", "-k")
	jsonObject, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
}

func ExtractOperationPolicies() []map[string]interface{} {
	commonJsonObject := getOperationPoliciesJsonObject()
	var commonData map[string]any
	if err := json.Unmarshal(commonJsonObject, &commonData); err != nil {
		log.Fatal(err)
	}

	commonList := commonData["list"].([]interface{})
	allPolicies := make([]map[string]interface{}, 0, len(commonList))

	for _, item := range commonList {
		policy := item.(map[string]interface{})
		allPolicies = append(allPolicies, map[string]interface{}{
			"id":      policy["id"],
			"name":    policy["name"],
			"version": policy["version"],
		})
	}

	apiIds := ExtractApiIds()
	for _, apiId := range apiIds {
		apiJsonObject := getApiLevelPoliciesJsonObject(apiId)
		var apiData map[string]any
		if err := json.Unmarshal(apiJsonObject, &apiData); err != nil {
			log.Fatal(err)
		}

		apiList, ok := apiData["list"].([]interface{})
		if !ok {
			continue
		}

		for _, item := range apiList {
			policy := item.(map[string]interface{})
			allPolicies = append(allPolicies, map[string]interface{}{
				"id":      policy["id"],
				"name":    policy["name"],
				"version": policy["version"],
			})
		}
	}

	return allPolicies
}

// This function sends the updated API JSON back to WSO2 via PUT /apis/{apiId}.
func PutApiUpdate(apiId string, payload []byte) error {
	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password,
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

	outStr := string(output)
	idx := strings.LastIndex(outStr, "HTTP_STATUS:")
	if idx == -1 {
		return fmt.Errorf("PUT /apis/%s: no status code in output: %s", apiId, outStr)
	}

	statusCode := strings.TrimSpace(outStr[idx+len("HTTP_STATUS:"):])
	if strings.HasPrefix(statusCode, "2") {
		fmt.Printf("PUT /apis/%s: OK (HTTP %s)\n", apiId, statusCode)
		return nil
	}

	body := strings.TrimSpace(outStr[:idx])
	fmt.Printf("PUT /apis/%s: FAILED (HTTP %s) - %s\n", apiId, statusCode, body)
	return fmt.Errorf("HTTP %s", statusCode)
}


func ReviewRevisionsNumber(apiId string) (string, bool) {
	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password, vars.BaseUrl+"/apis/"+apiId+"/revisions", "-k")
	jsonObject, err := cmd.Output()
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

func DeleteOldestRevision(apiId string, revisionId string) {
	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password,
		"-X", "DELETE",
		vars.BaseUrl+"/apis/"+apiId+"/revisions/"+revisionId,
		"-k",
		"-s", "-w", "\nHTTP_STATUS:%{http_code}")

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("delete revision curl error: %v, output: %s", err, out)
	}

	outStr := string(out)
	idx := strings.LastIndex(outStr, "HTTP_STATUS:")
	if idx == -1 {
		log.Fatalf("delete revision: no status code in output: %s", outStr)
	}

	statusCode := strings.TrimSpace(outStr[idx+len("HTTP_STATUS:"):])
	if !strings.HasPrefix(statusCode, "2") {
		log.Fatalf("delete revision failed with HTTP %s: %s", statusCode, strings.TrimSpace(outStr[:idx]))
	}

	fmt.Printf("Deleted revision %s successfully\n", revisionId)
}

func CreateRevision(apiId string) string {
	payload := []byte(`{}`)
	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password,
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

	outStr := string(out)
	idx := strings.LastIndex(outStr, "HTTP_STATUS:")
	if idx == -1 {
		log.Fatalf("create revision: no status code in output: %s", outStr)
	}

	statusCode := strings.TrimSpace(outStr[idx+len("HTTP_STATUS:"):])
	if !strings.HasPrefix(statusCode, "2") {
		log.Fatalf("create revision failed with HTTP %s: %s", statusCode, strings.TrimSpace(outStr[:idx]))
	}

	body := strings.TrimSpace(outStr[:idx])
	var data map[string]any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		log.Fatalf("create revision response parse error: %v, body: %s", err, body)
	}

	if revisionID, ok := data["id"].(string); ok {
		return revisionID
	}

	return ""
}

func DeployRevision(apiId string, revisionId string) {
	payload := `[
		{
			"name": "Default",
			"vhost": "localhost",
			"displayOnDevportal": true
		}
	]`

	cmd := exec.Command("curl", "-u", vars.Username+":"+vars.Password,
		"-X", "POST",
		vars.BaseUrl+"/apis/"+apiId+"/deploy-revision?revisionId="+revisionId,
		"-H", "Content-Type: application/json",
		"-d", payload,
		"-k",
		"-s", "-w", "\nHTTP_STATUS:%{http_code}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("deploy revision curl error: %v, output: %s", err, output)
	}

	outStr := string(output)
	idx := strings.LastIndex(outStr, "HTTP_STATUS:")
	if idx == -1 {
		log.Fatalf("deploy revision: no status code in output: %s", outStr)
	}

	statusCode := strings.TrimSpace(outStr[idx+len("HTTP_STATUS:"):])
	if !strings.HasPrefix(statusCode, "2") {
		log.Fatalf("deploy revision failed with HTTP %s: %s", statusCode, strings.TrimSpace(outStr[:idx]))
	}

	fmt.Println("Revision deployed successfully")
}

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
	DeployRevision(apiId, newRevisionID)
}