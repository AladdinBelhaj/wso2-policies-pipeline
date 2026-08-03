package client

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"wso2/pctl/vars"
)

const (
	PathAPI               = "/apis/"
	PathOperationPolicies = "/operation-policies?limit=100"
	PathAPIProduct        = "/api-products/"
)

type ApiProductRef struct {
	ProductID string
	ApiIDs    []string
}

// FindApiProductsUsingApis returns, for each API Product that references at
// least one of apiIds, the subset of apiIds it actually references.
func FindApiProductsUsingApis(apiIds []string) []ApiProductRef {
	if len(apiIds) == 0 {
		return nil
	}

	productSummaries := ExtractApiProductSummaries()
	return collectApiProductRefsFromSummaries(apiIds, productSummaries, func(productID string) ([]byte, error) {
		return GetApiProductDetailsJsonObject(productID), nil
	}, func(productID string, matched []string) ApiProductRef {
		return ApiProductRef{ProductID: productID, ApiIDs: matched}
	})
}

func referencedApiIds(product map[string]any, idSet map[string]bool) []string {
	apis, ok := product["apis"].([]interface{})
	if !ok {
		return nil
	}

	var matched []string
	for _, apiRaw := range apis {
		api, ok := apiRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if apiId, ok := api["apiId"].(string); ok && idSet[apiId] {
			matched = append(matched, apiId)
		}
	}
	return matched
}

func collectApiProductRefsFromSummaries(apiIds []string, productSummaries []ApiProductSummary, fetchProduct func(productID string) ([]byte, error), buildRef func(productID string, matched []string) ApiProductRef) []ApiProductRef {
	idSet := make(map[string]bool, len(apiIds))
	for _, id := range apiIds {
		idSet[id] = true
	}

	var refs []ApiProductRef

	for idx, summary := range productSummaries {
		log.Printf("[%d/%d] Scanning API Product %s to check API references...", idx+1, len(productSummaries), summary.Name)

		data, err := fetchProduct(summary.ID)
		if err != nil {
			log.Printf("failed to fetch details for API product %s: %v", summary.ID, err)
			continue
		}

		var product map[string]any
		if err := json.Unmarshal(data, &product); err != nil {
			log.Printf("failed to fetch details for API product %s: %v", summary.ID, err)
			continue
		}

		if matched := referencedApiIds(product, idSet); len(matched) > 0 {
			refs = append(refs, buildRef(summary.ID, matched))
		}
	}

	return refs
}

// This function executes an HTTP GET to fetch the JSON object from the /apis endpoint.
func getApiJsonObject() []byte {
	return getJSON(vars.BaseURL + "/apis")
}

// getJSON performs a GET request and fatals with the status code and body if
// the response was not a 2xx, instead of returning a body that will fail to
// parse in a confusing way further down the call chain.
func getJSON(url string) []byte {
	statusCode, body, err := doRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}
	if statusCode < 200 || statusCode >= 300 {
		log.Fatalf("GET %s failed with HTTP %d: %s", url, statusCode, body)
	}
	return body
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

// This function extracts the API IDs from the JSON object returned by the /apis endpoint.
func GetApiDetailsJsonObject(apiId string) []byte {
	return getJSON(vars.BaseURL + PathAPI + apiId)
}

// GetApiDeployments fetches the deployments list for an API from GET /apis/{apiId}/deployments
func GetApiDeployments(apiId string) []Deployment {
	url := vars.BaseURL + PathAPI + apiId + "/deployments"
	statusCode, body, err := doRequest(http.MethodGet, url, nil)
	if err != nil || statusCode < 200 || statusCode >= 300 {
		return defaultDeployments()
	}

	var list []map[string]any
	if err := json.Unmarshal(body, &list); err != nil || len(list) == 0 {
		return defaultDeployments()
	}

	return parseDeploymentsList(list)
}

// This function iterates through the list of API IDs and fetches the full details for each API.
func ExtractApiPolicies(apiIds []string) map[string]map[string]any {
	apiDetails := make(map[string]map[string]any, len(apiIds))

	for idx, apiId := range apiIds {
		log.Printf("[%d/%d] Fetching details for API %s...", idx+1, len(apiIds), GetApiName(apiId))
		var api map[string]any
		data := GetApiDetailsJsonObject(apiId)
		err := json.Unmarshal(data, &api)
		if err != nil {
			log.Fatal(err)
		}

		apiDetails[apiId] = api
	}

	return apiDetails
}

// This function fetches all common operation policies.
func getOperationPoliciesJsonObject() []byte {
	return getJSON(vars.BaseURL + PathOperationPolicies)
}

// This function extracts the policies and the metadata from the JSON objects.
func ExtractOperationPolicies() []map[string]interface{} {
	jsonObject := getOperationPoliciesJsonObject()

	var data map[string]any
	if err := json.Unmarshal(jsonObject, &data); err != nil {
		log.Fatal(err)
	}

	return normalizePolicyList(data)
}

// This function fetches API level policies.
func getApiLevelPoliciesJsonObject(apiId string) []byte {
	return getJSON(vars.BaseURL + PathAPI + apiId + PathOperationPolicies)
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
// policies lists the exact operation/API-level policies that were attempted
// in this update, so a failure can be logged with full context.
func PutApiUpdate(apiId string, payload []byte, policies []string) error {
	url := vars.BaseURL + PathAPI + apiId
	statusCode, body, err := doRequest(http.MethodPut, url, payload)
	if err != nil {
		return fmt.Errorf("PUT /apis/%s error: %v", apiId, err)
	}

	if statusCode >= 200 && statusCode < 300 {
		fmt.Printf("PUT /apis/%s: OK (HTTP %d)\n", apiId, statusCode)
		return nil
	}

	fmt.Printf("PUT /apis/%s: FAILED (HTTP %d) - %s\n", apiId, statusCode, body)
	LogUpdateFailure("API", GetApiName(apiId), statusCode, body, policies)
	return fmt.Errorf("HTTP %d", statusCode)
}

func getApiProductJsonObject() []byte {
	return getJSON(vars.BaseURL + "/api-products")
}

func GetApiProductDetailsJsonObject(apiProductId string) []byte {
	return getJSON(vars.BaseURL + PathAPIProduct + apiProductId)
}

// GetApiProductDeployments fetches the deployments list for an API product from GET /api-products/{apiProductId}/deployments
func GetApiProductDeployments(apiProductId string) []Deployment {
	url := vars.BaseURL + PathAPIProduct + apiProductId + "/deployments"
	statusCode, body, err := doRequest(http.MethodGet, url, nil)
	if err != nil || statusCode < 200 || statusCode >= 300 {
		return defaultDeployments()
	}

	var list []map[string]any
	if err := json.Unmarshal(body, &list); err != nil || len(list) == 0 {
		return defaultDeployments()
	}

	return parseDeploymentsList(list)
}

func defaultDeployments() []Deployment {
	return []Deployment{
		{
			Name:               "Default",
			Vhost:              "localhost",
			DisplayOnDevportal: true,
		},
	}
}

func parseDeploymentsList(list []map[string]any) []Deployment {
	deployments := make([]Deployment, 0, len(list))
	for _, item := range list {
		d := Deployment{
			Name:               "Default",
			DisplayOnDevportal: true,
		}
		if name, ok := item["name"].(string); ok && name != "" {
			d.Name = name
		}
		if vhost, ok := item["vhost"].(string); ok && vhost != "" {
			d.Vhost = vhost
		}
		if display, ok := item["displayOnDevportal"].(bool); ok {
			d.DisplayOnDevportal = display
		}
		if d.Vhost == "" {
			d.Vhost = "localhost"
		}
		deployments = append(deployments, d)
	}
	if len(deployments) == 0 {
		return defaultDeployments()
	}
	return deployments
}

// This function iterates through the JSON object and fetches the ID and name of each API product.
func ExtractApiProductSummaries() []ApiProductSummary {
	jsonObject := getApiProductJsonObject()

	var data map[string]any
	if err := json.Unmarshal(jsonObject, &data); err != nil {
		log.Fatal(err)
	}

	list := data["list"].([]interface{})
	products := make([]ApiProductSummary, 0, len(list))

	for _, item := range list {
		product, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		summary := ApiProductSummary{}
		if id, ok := product["id"].(string); ok {
			summary.ID = id
		}
		if name, ok := product["name"].(string); ok {
			summary.Name = name
		}
		products = append(products, summary)
	}

	return products
}

// This function iterates through the JSON object and fetches the ID of each API product.
func ExtractApiProductIds() []string {
	summaries := ExtractApiProductSummaries()
	ids := make([]string, 0, len(summaries))
	for _, p := range summaries {
		if p.ID != "" {
			ids = append(ids, p.ID)
		}
	}
	return ids
}

// policies lists the exact operation-level policies that were attempted in
// this update, so a failure can be logged with full context.
func PutApiProductUpdate(apiProductId string, payload []byte, policies []string) error {
	url := vars.BaseURL + PathAPIProduct + apiProductId
	statusCode, body, err := doRequest(http.MethodPut, url, payload)
	if err != nil {
		return fmt.Errorf("PUT /api-products/%s error: %v", apiProductId, err)
	}

	if statusCode >= 200 && statusCode < 300 {
		fmt.Printf("PUT /api-products/%s: OK (HTTP %d)\n", apiProductId, statusCode)
		return nil
	}

	fmt.Printf("PUT /api-products/%s: FAILED (HTTP %d) - %s\n", apiProductId, statusCode, body)
	LogUpdateFailure("API Product", GetApiProductName(apiProductId), statusCode, body, policies)
	return fmt.Errorf("HTTP %d", statusCode)
}

// This function returns the IDs of API products that reference any of the given API IDs
// in their apis[].apiId list.
func FindApiProductIdsUsingApis(apiIds []string) []string {
	if len(apiIds) == 0 {
		return nil
	}

	productSummaries := ExtractApiProductSummaries()
	refs := collectApiProductRefsFromSummaries(apiIds, productSummaries, func(productID string) ([]byte, error) {
		return GetApiProductDetailsJsonObject(productID), nil
	}, func(productID string, _ []string) ApiProductRef {
		return ApiProductRef{ProductID: productID}
	})

	matched := make([]string, 0, len(refs))
	for _, ref := range refs {
		matched = append(matched, ref.ProductID)
	}
	return matched
}

var apiNameCache map[string]string

func GetApiName(apiId string) string {
	if apiNameCache == nil {
		apiNameCache = make(map[string]string)
		for _, api := range ExtractApiSummaries() {
			apiNameCache[api.ID] = api.Name
		}
	}
	if name, ok := apiNameCache[apiId]; ok {
		return name
	}
	return apiId
}

var apiProductNameCache map[string]string

func GetApiProductName(productId string) string {
	if apiProductNameCache == nil {
		apiProductNameCache = make(map[string]string)
		for _, p := range ExtractApiProductSummaries() {
			apiProductNameCache[p.ID] = p.Name
		}
	}
	if name, ok := apiProductNameCache[productId]; ok {
		return name
	}
	return productId
}
