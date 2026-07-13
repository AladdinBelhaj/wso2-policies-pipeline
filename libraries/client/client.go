// This package handles all HTTP calls to the WSO2 Publisher API.
package client

import (
	"fmt"
	"os/exec"
	"strings"
	"wso2/scripts/vars"
)

type ApiSummary struct {
	ID   string
	Name string
}

// This function creates a new exec.Cmd for a curl command with the provided arguments to interact with WSO2 API Manager.
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
