package client

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wso2/pctl/vars"
)

// LogUpdateFailure appends a single entry to <LogsDir>/log.txt whenever a
// policy update PUT request fails (non-2xx, e.g. HTTP 400/500). Each entry
// records the exact timestamp, the environment in use, the API/API Product
// targeted, and the exact policies that were being attempted, so failed
// updates can be diagnosed later without re-running the command.
func LogUpdateFailure(entityKind, entityName string, statusCode int, responseBody []byte, policies []string) {
	policyList := "none"
	if len(policies) > 0 {
		policyList = strings.Join(policies, ", ")
	}

	entry := fmt.Sprintf(
		"[%s] env=%s %s=%q status=%d policies=[%s] response=%s\n",
		time.Now().Format(time.RFC3339),
		vars.CurrentEnv,
		entityKind,
		entityName,
		statusCode,
		policyList,
		strings.TrimSpace(string(responseBody)),
	)

	if err := os.MkdirAll(vars.LogsDir, 0755); err != nil {
		log.Printf("failed to create logs directory %s: %v", vars.LogsDir, err)
		return
	}

	logPath := filepath.Join(vars.LogsDir, "log.txt")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("failed to open log file %s: %v", logPath, err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(entry); err != nil {
		log.Printf("failed to write to log file %s: %v", logPath, err)
	}
}
