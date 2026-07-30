package client

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"wso2/pctl/vars"
)

const (
	PathRevision         = "/revisions/"
	PathUndeployRevision = "/undeploy-revision?revisionId="
	PathDeployRevision   = "/deploy-revision?revisionId="
	PathRestoreRevision  = "/restore-revision?revisionId="
)

type revisionItem struct {
	id          string
	displayName string
	createdTime string
	num         int
}

func extractNumber(s string) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err == nil {
		return n
	}
	if _, err := fmt.Sscanf(s, "v%d", &n); err == nil {
		return n
	}
	if _, err := fmt.Sscanf(s, "rev-%d", &n); err == nil {
		return n
	}
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(s)
	if match != "" {
		if val, err := strconv.Atoi(match); err == nil {
			return val
		}
	}
	return -1
}

func parseAndSortRevisions(list []interface{}) []string {
	items := make([]revisionItem, 0, len(list))
	for _, raw := range list {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if id == "" {
			continue
		}
		displayName, _ := m["displayName"].(string)
		createdTime, _ := m["createdTime"].(string)

		num := extractNumber(displayName)
		if num < 0 {
			num = extractNumber(id)
		}

		items = append(items, revisionItem{
			id:          id,
			displayName: displayName,
			createdTime: createdTime,
			num:         num,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].num >= 0 && items[j].num >= 0 && items[i].num != items[j].num {
			return items[i].num < items[j].num
		}
		if items[i].createdTime != "" && items[j].createdTime != "" && items[i].createdTime != items[j].createdTime {
			return items[i].createdTime < items[j].createdTime
		}
		return items[i].id < items[j].id
	})

	result := make([]string, len(items))
	for i, item := range items {
		result[i] = item.id
	}
	return result
}

// This function verifies if the revisions number has reached 5 or not.
func ReviewRevisionsNumber(apiId string) (string, bool) {
	revisions, err := GetRevisionIds(apiId)
	if err != nil || len(revisions) < 5 {
		return "", false
	}
	return revisions[0], true
}

// This function fetches the ID of each revision sorted from oldest to newest.
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

	return parseAndSortRevisions(list), nil
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
	deployments := GetApiDeployments(apiId)
	type undeployItem struct {
		Name               string `json:"name"`
		DisplayOnDevportal bool   `json:"displayOnDevportal"`
	}
	var undeployList []undeployItem
	for _, d := range deployments {
		undeployList = append(undeployList, undeployItem{
			Name:               d.Name,
			DisplayOnDevportal: false,
		})
	}
	payload, err := json.Marshal(undeployList)
	if err != nil {
		payload = []byte(`[{"name":"Default","displayOnDevportal":false}]`)
	}

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
	deployments := GetApiDeployments(apiId)
	payload, err := json.Marshal(deployments)
	if err != nil {
		return fmt.Errorf("failed to marshal deploy payload for API %s: %w", apiId, err)
	}

	url := vars.BaseURL + PathAPI + apiId + PathDeployRevision + revisionId
	if _, err := executeRevisionRequest(http.MethodPost, url, payload, fmt.Sprintf("deploy revision %s", revisionId)); err != nil {
		return err
	}

	fmt.Printf("Revision %s deployed successfully\n", revisionId)
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

	fmt.Printf("Revision %s restored successfully\n", revisionId)
	return nil
}

// This function deletes the oldest revision (if number of revisions >= 5), creates and deploys a new revision.
func PrepareAndDeployRevision(apiId string) {
	if oldestRevision, found := ReviewRevisionsNumber(apiId); found {
		fmt.Printf("Found 5 revisions; deleting oldest revision %s\n", oldestRevision)
		DeleteRevision(apiId, oldestRevision)
	}

	newRevisionID := CreateRevision(apiId)
	if newRevisionID == "" {
		log.Fatalf("failed to create a new revision for API %s", apiId)
	}

	fmt.Printf("Created revision %s; deploying it\n", newRevisionID)
	if err := DeployRevision(apiId, newRevisionID); err != nil {
		log.Fatalf("failed to deploy revision %s for API %s: %v", newRevisionID, apiId, err)
	}
}

// ReviewProductRevisionsNumber verifies if the product revisions number has reached 5.
func ReviewProductRevisionsNumber(productId string) (string, bool) {
	revisions, err := GetProductRevisionIds(productId)
	if err != nil || len(revisions) < 5 {
		return "", false
	}
	return revisions[0], true
}

// GetProductRevisionIds fetches the ID of each revision for an API Product sorted from oldest to newest.
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

	return parseAndSortRevisions(list), nil
}

// UndeployProductRevision undeploys an API Product revision before deleting it
func UndeployProductRevision(productId string, revisionId string) error {
	deployments := GetApiProductDeployments(productId)
	type undeployItem struct {
		Name               string `json:"name"`
		DisplayOnDevportal bool   `json:"displayOnDevportal"`
	}
	var undeployList []undeployItem
	for _, d := range deployments {
		undeployList = append(undeployList, undeployItem{
			Name:               d.Name,
			DisplayOnDevportal: false,
		})
	}
	payload, err := json.Marshal(undeployList)
	if err != nil {
		payload = []byte(`[{"name":"Default","displayOnDevportal":false}]`)
	}
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
	deployments := GetApiProductDeployments(productId)
	payload, err := json.Marshal(deployments)
	if err != nil {
		return fmt.Errorf("failed to marshal deploy payload for API Product %s: %w", productId, err)
	}

	url := vars.BaseURL + "/api-products/" + productId + "/deploy-revision?revisionId=" + revisionId
	if _, err := executeRevisionRequest(http.MethodPost, url, payload, fmt.Sprintf("deploy product revision %s", revisionId)); err != nil {
		return err
	}

	fmt.Printf("API Product revision %s deployed successfully\n", revisionId)
	return nil
}

// RestoreProductRevision restores a previous revision for an API Product.
func RestoreProductRevision(productId string, revisionId string) error {
	payload := []byte{}
	url := vars.BaseURL + "/api-products/" + productId + "/restore-revision?revisionId=" + revisionId
	if _, err := executeRevisionRequest(http.MethodPost, url, payload, fmt.Sprintf("restore product revision %s", revisionId)); err != nil {
		return err
	}

	fmt.Printf("API Product revision %s restored successfully\n", revisionId)
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
		log.Fatalf("failed to create a new product revision")
	}

	fmt.Printf("Created product revision %s; deploying it\n", newRevisionID)
	if err := DeployProductRevision(productId, newRevisionID); err != nil {
		log.Fatalf("failed to deploy product revision %s: %v", err)
	}
}
