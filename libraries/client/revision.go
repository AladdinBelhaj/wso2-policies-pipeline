package client

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"wso2/pctl/vars"
)

const (
	PathRevision         = "/revisions/"
	PathUndeployRevision = "/undeploy-revision?revisionId="
	PathDeployRevision   = "/deploy-revision?revisionId="
	PathRestoreRevision  = "/restore-revision?revisionId="
)

// This function verifies if the revisions number has reached 5 or not.
func ReviewRevisionsNumber(apiId string) (string, bool) {
	body := getJSON(vars.BaseURL + PathAPI + apiId + PathRevision)

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
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
	url := vars.BaseURL + PathAPI + apiId + PathRevision
	statusCode, body, err := doRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("GET %s failed with HTTP %d: %s", url, statusCode, body)
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
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

// This function performs a revision action request, checks for a 2xx
// status, and returns the response body or an error.
func executeRevisionRequest(method, url string, payload []byte, actionDesc string) (string, error) {
	statusCode, body, err := doRequest(method, url, payload)
	if err != nil {
		return "", fmt.Errorf("%s error: %v", actionDesc, err)
	}
	if statusCode < 200 || statusCode >= 300 {
		return "", fmt.Errorf("%s failed with HTTP %d: %s", actionDesc, statusCode, body)
	}

	return string(body), nil
}

// This function undeploys a revision before deleting it
func UndeployRevision(apiId string, revisionId string) error {
	payload := []byte(`[{"name":"Default","displayOnDevportal":false}]`)

	url := vars.BaseURL + PathAPI + apiId + PathUndeployRevision + revisionId
	if _, err := executeRevisionRequest(http.MethodPost, url, payload, fmt.Sprintf("undeploy revision %s", revisionId)); err != nil {
		return err
	}

	fmt.Printf("Undeployed revision %s successfully\n", revisionId)
	return nil
}

// This function deletes a revision without attempting to undeploy it first
func DeleteRevision(apiId string, revisionId string) error {
	url := vars.BaseURL + PathAPI + apiId + PathRevision + revisionId
	if _, err := executeRevisionRequest(http.MethodDelete, url, nil, fmt.Sprintf("delete revision %s", revisionId)); err != nil {
		return err
	}

	fmt.Printf("Deleted revision %s successfully\n", revisionId)
	return nil
}

// This function creates a revision.
func CreateRevision(apiId string) string {
	payload := []byte(`{}`)
	url := vars.BaseURL + PathAPI + apiId + PathRevision

	statusCode, body, err := doRequest(http.MethodPost, url, payload)
	if err != nil {
		log.Fatalf("create revision request error: %v", err)
	}
	if statusCode < 200 || statusCode >= 300 {
		log.Fatalf("create revision failed with HTTP %d: %s", statusCode, body)
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		log.Fatalf("create revision response parse error: %v, body: %s", err, body)
	}

	if revisionID, ok := data["id"].(string); ok {
		return revisionID
	}

	return ""
}

// This function deploys a revision.
func DeployRevision(apiId string, revisionId string) error {
	payload := []byte(fmt.Sprintf(`[
    {
        "name": "Default",
        "vhost": "%s",
        "displayOnDevportal": true
    }
]`, vars.Vhost))

	url := vars.BaseURL + PathAPI + apiId + PathDeployRevision + revisionId
	if _, err := executeRevisionRequest(http.MethodPost, url, payload, fmt.Sprintf("deploy revision %s", revisionId)); err != nil {
		return err
	}

	fmt.Println("Revision deployed successfully")
	return nil
}

// This function restores a previous revision to the API.
func RestoreRevision(apiId string, revisionId string) error {
	// Empty (non-nil) payload preserves the original curl behavior of sending
	// the Content-Type header with no body.
	payload := []byte{}

	url := vars.BaseURL + PathAPI + apiId + PathRestoreRevision + revisionId
	if _, err := executeRevisionRequest(http.MethodPost, url, payload, fmt.Sprintf("restore revision %s", revisionId)); err != nil {
		return err
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

// ReviewProductRevisionsNumber verifies if the product revisions number has reached 5.
func ReviewProductRevisionsNumber(productId string) (string, bool) {
	url := vars.BaseURL + "/api-products/" + productId + "/revisions"
	body := getJSON(url)

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		log.Fatal(err)
	}

	count, ok := data["count"].(float64)
	if !ok {
		return "", false
	}

	if count == 5 {
		list, ok := data["list"].([]interface{})
		if ok && len(list) > 0 {
			if first, ok := list[0].(map[string]interface{}); ok {
				if id, ok := first["id"].(string); ok {
					return id, true
				}
			}
		}
	}

	return "", false
}

// GetProductRevisionIds fetches the ID of each revision for an API Product.
func GetProductRevisionIds(productId string) ([]string, error) {
	url := vars.BaseURL + "/api-products/" + productId + "/revisions"
	statusCode, body, err := doRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("GET %s failed with HTTP %d: %s", url, statusCode, body)
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	list, ok := data["list"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected product revisions response format for Product %s", productId)
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

// UndeployProductRevision undeploys an API Product revision before deleting it
func UndeployProductRevision(productId string, revisionId string) error {
	payload := []byte(`[{"name":"Default","displayOnDevportal":false}]`)
	url := vars.BaseURL + "/api-products/" + productId + "/undeploy-revision?revisionId=" + revisionId
	if _, err := executeRevisionRequest(http.MethodPost, url, payload, fmt.Sprintf("undeploy product revision %s", revisionId)); err != nil {
		return err
	}

	fmt.Printf("Undeployed API Product revision %s successfully\n", revisionId)
	return nil
}

// DeleteProductRevision deletes an API Product revision
func DeleteProductRevision(productId string, revisionId string) error {
	url := vars.BaseURL + "/api-products/" + productId + "/revisions/" + revisionId
	if _, err := executeRevisionRequest(http.MethodDelete, url, nil, fmt.Sprintf("delete product revision %s", revisionId)); err != nil {
		return err
	}

	fmt.Printf("Deleted API Product revision %s successfully\n", revisionId)
	return nil
}

// CreateProductRevision creates a new revision for an API Product.
func CreateProductRevision(productId string) string {
	payload := []byte(`{}`)
	url := vars.BaseURL + "/api-products/" + productId + "/revisions"

	statusCode, body, err := doRequest(http.MethodPost, url, payload)
	if err != nil {
		log.Fatalf("create product revision request error: %v", err)
	}
	if statusCode < 200 || statusCode >= 300 {
		log.Fatalf("create product revision failed with HTTP %d: %s", statusCode, body)
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		log.Fatalf("create product revision response parse error: %v, body: %s", err, body)
	}

	if revisionID, ok := data["id"].(string); ok {
		return revisionID
	}

	return ""
}

// DeployProductRevision deploys an API Product revision.
func DeployProductRevision(productId string, revisionId string) error {
	payload := []byte(fmt.Sprintf(`[
    {
        "name": "Default",
        "vhost": "%s",
        "displayOnDevportal": true
    }
]`, vars.Vhost))

	url := vars.BaseURL + "/api-products/" + productId + "/deploy-revision?revisionId=" + revisionId
	if _, err := executeRevisionRequest(http.MethodPost, url, payload, fmt.Sprintf("deploy product revision %s", revisionId)); err != nil {
		return err
	}

	fmt.Println("API Product revision deployed successfully")
	return nil
}

// RestoreProductRevision restores a previous revision for an API Product.
func RestoreProductRevision(productId string, revisionId string) error {
	payload := []byte{}
	url := vars.BaseURL + "/api-products/" + productId + "/restore-revision?revisionId=" + revisionId
	if _, err := executeRevisionRequest(http.MethodPost, url, payload, fmt.Sprintf("restore product revision %s", revisionId)); err != nil {
		return err
	}

	fmt.Println("API Product revision restored successfully")
	return nil
}

// PrepareAndDeployProductRevision handles revision limit check (5 max) and creates/deploys a new revision for an API Product.
func PrepareAndDeployProductRevision(productId string) {
	if oldestRevision, found := ReviewProductRevisionsNumber(productId); found {
		fmt.Printf("Found 5 product revisions; deleting oldest revision %s\n", oldestRevision)
		DeleteProductRevision(productId, oldestRevision)
	}

	newRevisionID := CreateProductRevision(productId)
	if newRevisionID == "" {
		log.Fatal("failed to create a new product revision")
	}

	fmt.Printf("Created product revision %s; deploying it\n", newRevisionID)
	if err := DeployProductRevision(productId, newRevisionID); err != nil {
		log.Fatal(err)
	}
}
