package policy

import (
	"encoding/json"
	"fmt"
	"strconv"
	"wso2/pctl/libraries/client"
)

// PolicySpec mirrors the policySpecFile JSON structure WSO2 expects when
// creating/publishing an operation policy.
type PolicySpec struct {
	Category          string        `json:"category"`
	Name              string        `json:"name"`
	DisplayName       string        `json:"displayName"`
	Version           string        `json:"version"`
	Description       string        `json:"description"`
	ApplicableFlows   []string      `json:"applicableFlows"`
	SupportedApiTypes []string      `json:"supportedApiTypes"`
	SupportedGateways []string      `json:"supportedGateways"`
	PolicyAttributes  []interface{} `json:"policyAttributes"`
}

// PublishPolicyOptions holds the values used to build the policySpecFile
// that WSO2 requires alongside the Synapse mediation definition. Only Name
// and DefinitionPath are required; everything else falls back to a sane
// default for a request/response/fault Synapse mediation policy.
type PublishPolicyOptions struct {
	Name           string
	DefinitionPath string

	DisplayName       string // defaults to Name
	Category          string // defaults to "Mediation"
	Description       string
	ApplicableFlows   []string // defaults to ["request", "response", "fault"]
	SupportedApiTypes []string // defaults to ["HTTP"]
	SupportedGateways []string // defaults to ["Synapse"]
}

// PublishPolicy builds a policySpecFile from opts (no spec file is read
// from disk), looks up the newest version of that policy currently
// registered in WSO2 by name, bumps it by one (v1 -> v2, absent -> v1),
// and POSTs the spec together with the Synapse mediation definition (.j2)
// to WSO2 to publish the new version. This mirrors the WSO2 UI "create
// policy" flow, which is also how new *versions* of an existing policy are
// added - there is no separate "update version" endpoint.
func PublishPolicy(opts PublishPolicyOptions) error {
	if opts.Name == "" {
		return fmt.Errorf("policy name is required")
	}

	spec := PolicySpec{
		Category:          opts.Category,
		Name:              opts.Name,
		DisplayName:       opts.DisplayName,
		Description:       opts.Description,
		ApplicableFlows:   opts.ApplicableFlows,
		SupportedApiTypes: opts.SupportedApiTypes,
		SupportedGateways: opts.SupportedGateways,
		PolicyAttributes:  []interface{}{},
	}

	if spec.DisplayName == "" {
		spec.DisplayName = spec.Name
	}
	if spec.Category == "" {
		spec.Category = "Mediation"
	}
	if len(spec.ApplicableFlows) == 0 {
		spec.ApplicableFlows = []string{"request", "response", "fault"}
	}
	if len(spec.SupportedApiTypes) == 0 {
		spec.SupportedApiTypes = []string{"HTTP"}
	}
	if len(spec.SupportedGateways) == 0 {
		spec.SupportedGateways = []string{"Synapse"}
	}

	nextVersion, err := resolveNextVersion(spec.Name)
	if err != nil {
		return fmt.Errorf("failed to resolve next version for policy %q: %w", spec.Name, err)
	}
	spec.Version = nextVersion

	fmt.Printf("Publishing policy %q as %s\n", spec.Name, nextVersion)

	specJSON, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal policy spec: %w", err)
	}

	if err := client.CreateOperationPolicy(specJSON, opts.DefinitionPath, spec.Name); err != nil {
		return fmt.Errorf("failed to publish policy %q: %w", spec.Name, err)
	}

	fmt.Printf("Policy %q published successfully as %s\n", spec.Name, nextVersion)
	return nil
}

// resolveNextVersion looks up the newest existing version of a policy (by
// clean name) already registered in WSO2, and returns the next version
// string. If no existing version is found, it starts at "v1".
func resolveNextVersion(name string) (string, error) {
	allPolicies := client.ExtractOperationPolicies()

	cleanTarget := CleanPolicyName(name)
	latestNumber := -1

	for _, p := range allPolicies {
		pName, _ := p["name"].(string)
		if pName == "" {
			pName, _ = p["displayName"].(string)
		}
		if CleanPolicyName(pName) != cleanTarget {
			continue
		}

		version, ok := p["version"].(string)
		if !ok {
			continue
		}

		num, err := versionNumber(version)
		if err != nil {
			continue
		}

		if num > latestNumber {
			latestNumber = num
		}
	}

	if latestNumber < 0 {
		return "v1", nil
	}

	return "v" + strconv.Itoa(latestNumber+1), nil
}
