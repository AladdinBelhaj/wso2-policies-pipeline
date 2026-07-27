package client

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"wso2/pctl/vars"
)

const (
	PathAPI               = "/apis/"
	PathOperationPolicies = "/operation-policies?limit=100"
)

// This function executes a curl command to fetch the JSON object from the /apis endpoint.
func getApiJsonObject() []byte {
	jsonObject, err := newCurlCmd(vars.BaseURL+"/apis", "-k").Output()
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

// This function extracts the API IDs from the JSON object returned by the /apis endpoint.
func GetApiDetailsJsonObject(apiId string) []byte {
	jsonObject, err := newCurlCmd(vars.BaseURL+PathAPI+apiId, "-k").Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
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
	jsonObject, err := newCurlCmd(vars.BaseURL+PathOperationPolicies, "-k").Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
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
	jsonObject, err := newCurlCmd(vars.BaseURL+PathAPI+apiId+PathOperationPolicies, "-k").Output()
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
		vars.BaseURL+PathAPI+apiId,
		"-H", contentTypeJSON,
		"-d", "@-",
		"-k",
		"-s", "-w", httpStatusFormat)

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

func getApiProductJsonObject() []byte {
	jsonObject, err := newCurlCmd(vars.BaseURL+"/api-products", "-k").Output()
	if err != nil {
		log.Fatal(err)
	}

	return jsonObject
}

func GetApiProductDetailsJsonObject(apiProductId string) []byte {
	jsonObject, err := newCurlCmd(vars.BaseURL+"/api-products/"+apiProductId, "-k").Output()
	if err != nil {
		log.Fatal(err)
	}
	return jsonObject
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

func ExtractApiProductPolicies(productIds []string) map[string]map[string]any {
	details := make(map[string]map[string]any, len(productIds))
	for idx, id := range productIds {
		log.Printf("[%d/%d] Fetching details for API Product %s...", idx+1, len(productIds), GetApiProductName(id))
		var product map[string]any
		if err := json.Unmarshal(GetApiProductDetailsJsonObject(id), &product); err != nil {
			log.Fatal(err)
		}
		details[id] = product
	}
	return details
}

func PutApiProductUpdate(apiProductId string, payload []byte) error {
	cmd := newCurlCmd(
		"-X", "PUT",
		vars.BaseURL+"/api-products/"+apiProductId,
		"-H", contentTypeJSON,
		"-d", "@-",
		"-k",
		"-s", "-w", httpStatusFormat)

	cmd.Stdin = strings.NewReader(string(payload))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("PUT /api-products/%s curl error: %v, output: %s", apiProductId, err, output)
	}

	statusCode, body, err := parseCurlStatus(output, fmt.Sprintf("PUT /api-products/%s", apiProductId))
	if err != nil {
		return err
	}
	if strings.HasPrefix(statusCode, "2") {
		fmt.Printf("PUT /api-products/%s: OK (HTTP %s)\n", apiProductId, statusCode)
		return nil
	}

	fmt.Printf("PUT /api-products/%s: FAILED (HTTP %s) - %s\n", apiProductId, statusCode, body)
	return fmt.Errorf("HTTP %s", statusCode)
}

func productReferencesApis(product map[string]any, idSet map[string]bool) bool {
	apis, ok := product["apis"].([]interface{})
	if !ok {
		return false
	}

	for _, apiRaw := range apis {
		api, ok := apiRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if apiId, ok := api["apiId"].(string); ok && idSet[apiId] {
			return true
		}
	}
	return false
}

// This function returns the IDs of API products that reference any of the given API IDs
// in their apis[].apiId list.
func FindApiProductIdsUsingApis(apiIds []string) []string {
	if len(apiIds) == 0 {
		return nil
	}

	idSet := make(map[string]bool, len(apiIds))
	for _, id := range apiIds {
		idSet[id] = true
	}

	productSummaries := ExtractApiProductSummaries()
	var matched []string

	for idx, summary := range productSummaries {
		log.Printf("[%d/%d] Scanning API Product %s to check API references...", idx+1, len(productSummaries), summary.Name)
		var product map[string]any
		data := GetApiProductDetailsJsonObject(summary.ID)
		if err := json.Unmarshal(data, &product); err != nil {
			log.Printf("failed to fetch details for API product %s: %v", summary.ID, err)
			continue
		}

		if productReferencesApis(product, idSet) {
			matched = append(matched, summary.ID)
		}
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
