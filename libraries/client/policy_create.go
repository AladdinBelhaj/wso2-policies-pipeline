package client

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"wso2/pctl/vars"
)

const PathOperationPoliciesCreate = "/operation-policies"

// CreateOperationPolicy publishes a new operation policy (a new version of
// an existing policy, or a brand new one) to WSO2 by POSTing the policy
// spec JSON together with the Synapse mediation definition (.j2) file as
// multipart/form-data to /operation-policies. policyName is used only for
// logging/failure-log context.
func CreateOperationPolicy(specJSON []byte, synapseDefPath string, policyName string) error {
	url := vars.BaseURL + PathOperationPoliciesCreate

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	specPart, err := writer.CreateFormFile("policySpecFile", "policy-spec.json")
	if err != nil {
		return fmt.Errorf("failed to create policySpecFile part: %w", err)
	}
	if _, err := specPart.Write(specJSON); err != nil {
		return fmt.Errorf("failed to write policySpecFile content: %w", err)
	}

	defFile, err := os.Open(synapseDefPath)
	if err != nil {
		return fmt.Errorf("failed to open synapse definition file %s: %w", synapseDefPath, err)
	}
	defer defFile.Close()

	defPart, err := writer.CreateFormFile("synapsePolicyDefinitionFile", filepath.Base(synapseDefPath))
	if err != nil {
		return fmt.Errorf("failed to create synapsePolicyDefinitionFile part: %w", err)
	}
	if _, err := io.Copy(defPart, defFile); err != nil {
		return fmt.Errorf("failed to write synapse definition content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to finalize multipart body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("failed to build POST %s request: %w", url, err)
	}
	req.SetBasicAuth(vars.Username, vars.Password)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s error: %v", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("POST %s: failed to read response: %v", url, err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("POST %s: OK (HTTP %d)\n", PathOperationPoliciesCreate, resp.StatusCode)
		return nil
	}

	fmt.Printf("POST %s: FAILED (HTTP %d) - %s\n", PathOperationPoliciesCreate, resp.StatusCode, respBody)
	LogUpdateFailure("Operation Policy", policyName, resp.StatusCode, respBody, nil)
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}
