// This package handles all HTTP calls to the WSO2 Publisher API.
package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
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

type PolicyLogInfo struct {
	Name    string
	Version string
	ID      string
	Flow    string
	Level   string
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

	if resp.StatusCode == 400 {
		log400Error(method, url, payload, resp.StatusCode, respBody)
	}

	return resp.StatusCode, respBody, nil
}

var loggingErrorGuard bool

func log400Error(method, url string, payload []byte, statusCode int, respBody []byte) {
	if loggingErrorGuard {
		return
	}
	loggingErrorGuard = true
	defer func() { loggingErrorGuard = false }()

	logsDir := vars.LogsDir
	if logsDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		if runtime.GOOS == "windows" {
			logsDir = filepath.Join(home, ".pctl", "logs")
		} else {
			logsDir = filepath.Join(home, ".config", "pctl", "logs")
		}
	}
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		log.Printf("Warning: could not create logs directory %s: %v", logsDir, err)
	}

	now := time.Now()
	timestampStr := now.Format(time.RFC3339)

	var targetType string
	var targetId string
	var targetName string

	if strings.Contains(url, "/apis/") {
		parts := strings.Split(url, "/apis/")
		if len(parts) > 1 {
			rem := parts[1]
			id := strings.Split(strings.Split(rem, "/")[0], "?")[0]
			if id != "" && id != "operation-policies" {
				targetType = "API"
				targetId = id
				targetName = GetApiName(id)
			}
		}
	} else if strings.Contains(url, "/api-products/") {
		parts := strings.Split(url, "/api-products/")
		if len(parts) > 1 {
			rem := parts[1]
			id := strings.Split(strings.Split(rem, "/")[0], "?")[0]
			if id != "" {
				targetType = "API Product"
				targetId = id
				targetName = GetApiProductName(id)
			}
		}
	}

	var policies []PolicyLogInfo
	if len(payload) > 0 {
		var data map[string]any
		if err := json.Unmarshal(payload, &data); err == nil {
			policies = extractPoliciesFromMap(data)
		}
	}

	if len(policies) == 0 && targetId != "" {
		if targetType == "API" {
			details := GetApiDetailsJsonObject(targetId)
			var apiData map[string]any
			if err := json.Unmarshal(details, &apiData); err == nil {
				policies = extractPoliciesFromMap(apiData)
			}
		} else if targetType == "API Product" {
			details := GetApiProductDetailsJsonObject(targetId)
			var prodData map[string]any
			if err := json.Unmarshal(details, &prodData); err == nil {
				policies = extractPoliciesFromMap(prodData)
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("================================================================================\n")
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", timestampStr))
	sb.WriteString(fmt.Sprintf("HTTP Status: %d (Bad Request)\n", statusCode))
	sb.WriteString(fmt.Sprintf("Method: %s\n", method))
	sb.WriteString(fmt.Sprintf("URL: %s\n", url))
	if targetType != "" {
		sb.WriteString(fmt.Sprintf("Concerned %s: %s (ID: %s)\n", targetType, targetName, targetId))
	} else {
		sb.WriteString("Concerned Target: N/A\n")
	}
	sb.WriteString("Exact Policies Concerned:\n")
	if len(policies) == 0 {
		sb.WriteString("  None / No policies found\n")
	} else {
		for _, p := range policies {
			info := fmt.Sprintf("  - Policy: %s", p.Name)
			if p.Version != "" {
				info += fmt.Sprintf(" (Version: %s)", p.Version)
			}
			if p.ID != "" {
				info += fmt.Sprintf(" (ID: %s)", p.ID)
			}
			if p.Flow != "" {
				info += fmt.Sprintf(" (Flow: %s)", p.Flow)
			}
			if p.Level != "" {
				info += fmt.Sprintf(" (Level: %s)", p.Level)
			}
			sb.WriteString(info + "\n")
		}
	}
	if len(respBody) > 0 {
		sb.WriteString(fmt.Sprintf("Response Body: %s\n", string(respBody)))
	}
	sb.WriteString("================================================================================\n\n")

	logContent := sb.String()

	// Append to error.log
	logFilePath := filepath.Join(logsDir, "error.log")
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		_, _ = f.WriteString(logContent)
		_ = f.Close()
	}

	// Also write to a timestamped log file in the logs directory
	timeLogFile := filepath.Join(logsDir, fmt.Sprintf("error_%s.log", now.Format("20060102_150405_000000000")))
	_ = os.WriteFile(timeLogFile, []byte(logContent), 0644)
}

func extractPoliciesFromMap(data map[string]any) []PolicyLogInfo {
	if data == nil {
		return nil
	}
	var policies []PolicyLogInfo
	seen := make(map[string]bool)

	addPolicy := func(name, version, id, flow, level string) {
		if name == "" {
			return
		}
		key := fmt.Sprintf("%s|%s|%s|%s|%s", name, version, id, flow, level)
		if seen[key] {
			return
		}
		seen[key] = true
		policies = append(policies, PolicyLogInfo{
			Name:    name,
			Version: version,
			ID:      id,
			Flow:    flow,
			Level:   level,
		})
	}

	if apiPolicies, ok := data["apiPolicies"].(map[string]interface{}); ok {
		for _, flow := range []string{"request", "response", "fault"} {
			if flowList, ok := apiPolicies[flow].([]interface{}); ok {
				for _, pRaw := range flowList {
					if p, ok := pRaw.(map[string]interface{}); ok {
						pName, _ := p["policyName"].(string)
						if pName == "" {
							pName, _ = p["name"].(string)
						}
						pVer, _ := p["policyVersion"].(string)
						if pVer == "" {
							pVer, _ = p["version"].(string)
						}
						pId, _ := p["policyId"].(string)
						if pId == "" {
							pId, _ = p["id"].(string)
						}
						addPolicy(pName, pVer, pId, flow, "API level")
					}
				}
			}
		}
	}

	if operations, ok := data["operations"].([]interface{}); ok {
		for _, opRaw := range operations {
			if op, ok := opRaw.(map[string]interface{}); ok {
				if opPolicies, ok := op["operationPolicies"].(map[string]interface{}); ok {
					for _, flow := range []string{"request", "response", "fault"} {
						if flowList, ok := opPolicies[flow].([]interface{}); ok {
							for _, pRaw := range flowList {
								if p, ok := pRaw.(map[string]interface{}); ok {
									pName, _ := p["policyName"].(string)
									if pName == "" {
										pName, _ = p["name"].(string)
									}
									pVer, _ := p["policyVersion"].(string)
									if pVer == "" {
										pVer, _ = p["version"].(string)
									}
									pId, _ := p["policyId"].(string)
									if pId == "" {
										pId, _ = p["id"].(string)
									}
									addPolicy(pName, pVer, pId, flow, "Operation level")
								}
							}
						}
					}
				}
			}
		}
	}

	if apis, ok := data["apis"].([]interface{}); ok {
		for _, apiRaw := range apis {
			if apiMap, ok := apiRaw.(map[string]interface{}); ok {
				if ops, ok := apiMap["operations"].([]interface{}); ok {
					for _, opRaw := range ops {
						if op, ok := opRaw.(map[string]interface{}); ok {
							if opPolicies, ok := op["operationPolicies"].(map[string]interface{}); ok {
								for _, flow := range []string{"request", "response", "fault"} {
									if flowList, ok := opPolicies[flow].([]interface{}); ok {
										for _, pRaw := range flowList {
											if p, ok := pRaw.(map[string]interface{}); ok {
												pName, _ := p["policyName"].(string)
												if pName == "" {
													pName, _ = p["name"].(string)
												}
												pVer, _ := p["policyVersion"].(string)
												if pVer == "" {
													pVer, _ = p["version"].(string)
												}
												pId, _ := p["policyId"].(string)
												if pId == "" {
													pId, _ = p["id"].(string)
												}
												addPolicy(pName, pVer, pId, flow, "Operation level")
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return policies
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
