// This package handles all HTTP calls to the WSO2 Publisher API.
package client

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"wso2/pctl/vars"
)

type ApiSummary struct {
	ID   string
	Name string
}

type ApiProductSummary struct {
	ID   string
	Name string
}

type Deployment struct {
	Name               string `json:"name"`
	Vhost              string `json:"vhost"`
	DisplayOnDevportal bool   `json:"displayOnDevportal"`
}

const contentTypeJSON = "application/json"

// httpClient is shared across all requests to the WSO2 Publisher API.
// InsecureSkipVerify mirrors the previous curl -k behavior.
var httpClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

// newRequest builds an HTTP request with basic auth configured for the WSO2 API.
func newRequest(method, url string, body []byte) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(vars.Username, vars.Password)
	if body != nil {
		req.Header.Set("Content-Type", contentTypeJSON)
	}

	return req, nil
}

// doRequest executes an HTTP request against the WSO2 API and returns the
// status code and response body.
func doRequest(method, url string, payload []byte) (int, []byte, error) {
	req, err := newRequest(method, url, payload)
	if err != nil {
		return 0, nil, fmt.Errorf("%s %s: failed to build request: %v", method, url, err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%s %s: request failed: %v", method, url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("%s %s: failed to read response: %v", method, url, err)
	}

	return resp.StatusCode, respBody, nil
}

// This function extracts policies from the JSON object and stores them in a map
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
			"id":          policy["id"],
			"name":        policy["name"],
			"displayName": policy["displayName"],
			"version":     policy["version"],
		})
	}

	return policies
}
