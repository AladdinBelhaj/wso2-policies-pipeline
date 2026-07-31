package report

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wso2/pctl/libraries/client"
	"wso2/pctl/vars"
)

// ApiUpdateDetail holds extensive details for an API update.
type ApiUpdateDetail struct {
	ID               string
	Name             string
	Modified         bool
	Status           string // e.g. "MODIFIED_AND_DEPLOYED", "NO_CHANGES", "FAILED"
	Error            string
	PreviousRevision string
	NewRevision      string
	Deployments      []client.Deployment
	PlannedChanges   []string
	CurrentPolicies  []string
}

// ProductUpdateDetail holds details for an API Product update.
type ProductUpdateDetail struct {
	ID               string
	Name             string
	Updated          bool
	Status           string
	PreviousRevision string
	NewRevision      string
	Deployments      []client.Deployment
}

// UpdateReportData contains all parameters and results for a policy update execution.
type UpdateReportData struct {
	Timestamp    time.Time
	Environment  string
	BaseURL      string
	TargetFilter string
	PolicyFilter string
	IsDryRun     bool
	Status       string // "SUCCESS", "NO_CHANGES", "DRY_RUN_CONFIRMED", "CANCELLED", "FAILED"
	Summary      string
	APIs         []ApiUpdateDetail
	Products     []ProductUpdateDetail
}

// ApiRollbackDetail holds details for an API revision rollback.
type ApiRollbackDetail struct {
	ID                   string
	Name                 string
	Success              bool
	Status               string // e.g. "ROLLED_BACK", "CANNOT_ROLLBACK", "FAILED"
	Error                string
	RolledBackFrom       string // Deleted revision (e.g. rev-4)
	RolledBackTo         string // Restored revision (e.g. rev-3)
	TotalRevisions       int
	InitialPolicies      []string
	PostRollbackPolicies []string
	Deployments          []client.Deployment
}

// ProductRollbackDetail holds details for an API Product revision rollback.
type ProductRollbackDetail struct {
	ID             string
	Name           string
	Success        bool
	Status         string
	Error          string
	RolledBackFrom string
	RolledBackTo   string
	TotalRevisions int
	Deployments    []client.Deployment
}

// RollbackReportData contains all parameters and results for a rollback execution.
type RollbackReportData struct {
	Timestamp    time.Time
	Environment  string
	BaseURL      string
	TargetFilter string
	IsDryRun     bool
	Status       string // "SUCCESS", "DRY_RUN", "CANCELLED", "FAILED"
	Summary      string
	APIs         []ApiRollbackDetail
	Products     []ProductRollbackDetail
}

// GenerateUpdateReport creates an extensive PDF report and text log for a policy update.
func GenerateUpdateReport(data UpdateReportData) (string, string) {
	if data.Timestamp.IsZero() {
		data.Timestamp = time.Now()
	}

	timeStr := data.Timestamp.Format("20060102_150405")
	pdfFileName := fmt.Sprintf("update_report_%s.pdf", timeStr)
	txtFileName := fmt.Sprintf("update_report_%s.txt", timeStr)

	pdfPath := filepath.Join(vars.LogsDir, pdfFileName)
	txtPath := filepath.Join(vars.LogsDir, txtFileName)

	// Build PDF Document
	pw := NewPDFWriter("WSO2 POLICY PIPELINE REPORT", "POLICY UPDATE REPORT")

	// Metadata Box
	modeStr := "Live Update"
	if data.IsDryRun {
		modeStr = "Dry-Run / Preview"
	}

	pw.AddMetadataBox([]KeyVal{
		{Key: "Execution Date", Val: data.Timestamp.Format("2006-01-02 15:04:05 MST")},
		{Key: "Environment", Val: data.Environment},
		{Key: "Base URL", Val: data.BaseURL},
		{Key: "Execution Mode", Val: modeStr},
		{Key: "API Target Filter", Val: data.TargetFilter},
		{Key: "Policy Filter", Val: data.PolicyFilter},
	}, data.Status)

	// Stat Cards
	modifiedCount := 0
	for _, a := range data.APIs {
		if a.Modified {
			modifiedCount++
		}
	}
	prodUpdatedCount := 0
	for _, p := range data.Products {
		if p.Updated {
			prodUpdatedCount++
		}
	}

	pw.AddStatCards([]StatCard{
		{Label: "Total APIs Targeted", Value: fmt.Sprintf("%d", len(data.APIs))},
		{Label: "APIs Modified", Value: fmt.Sprintf("%d", modifiedCount)},
		{Label: "API Products Targeted", Value: fmt.Sprintf("%d", len(data.Products))},
		{Label: "Products Updated", Value: fmt.Sprintf("%d", prodUpdatedCount)},
	})

	// APIs Details
	if len(data.APIs) > 0 {
		pw.AddSectionHeader("APIs UPDATE BREAKDOWN")
		for _, api := range data.APIs {
			pw.AddEntityHeader("API", api.Name, api.ID, api.Status)

			// Revision & Deployment summary table
			prevRev := api.PreviousRevision
			if prevRev == "" {
				prevRev = "N/A"
			}
			newRev := api.NewRevision
			if newRev == "" {
				if api.Modified {
					newRev = "Created & Deployed"
				} else {
					newRev = prevRev
				}
			}

			vhosts := extractVhosts(api.Deployments)

			pw.AddTable(
				[]string{"Previous Revision", "Active/New Revision", "Deployment Environments", "Update Status"},
				[][]string{
					{prevRev, newRev, vhosts, api.Status},
				},
				[]float64{1.2, 1.4, 2.0, 1.4},
			)

			// Planned Changes
			if len(api.PlannedChanges) > 0 {
				pw.AddTextLine("Detected Policy Changes:", true)
				pw.AddBulletList(api.PlannedChanges)
			} else {
				pw.AddTextLine("Detected Policy Changes: None (All policies are up to date)", false)
			}

			// Current Attached Policies
			if len(api.CurrentPolicies) > 0 {
				pw.AddTextLine("Current Attached Policies (Post-Update):", true)
				pw.AddBulletList(api.CurrentPolicies)
			}
		}
	}

	// API Products Details
	if len(data.Products) > 0 {
		pw.AddSectionHeader("API PRODUCTS UPDATE BREAKDOWN")
		for _, prod := range data.Products {
			pw.AddEntityHeader("API Product", prod.Name, prod.ID, prod.Status)

			prevRev := prod.PreviousRevision
			if prevRev == "" {
				prevRev = "N/A"
			}
			newRev := prod.NewRevision
			if newRev == "" {
				if prod.Updated {
					newRev = "Created & Deployed"
				} else {
					newRev = prevRev
				}
			}

			vhosts := extractVhosts(prod.Deployments)

			pw.AddTable(
				[]string{"Previous Revision", "Active/New Revision", "Deployment Environments", "Status"},
				[][]string{
					{prevRev, newRev, vhosts, prod.Status},
				},
				[]float64{1.2, 1.4, 2.0, 1.4},
			)
		}
	}

	// Save PDF
	if err := pw.SaveToFile(pdfPath); err != nil {
		log.Printf("Error saving PDF report to %s: %v", pdfPath, err)
	}

	// Generate and Save Text Summary Log
	txtContent := formatUpdateTextReport(data)
	if err := os.WriteFile(txtPath, []byte(txtContent), 0644); err != nil {
		log.Printf("Error saving text log to %s: %v", txtPath, err)
	}

	// Append to centralized log.txt
	appendToCentralLog("UPDATE", data.Environment, data.Status, fmt.Sprintf("APIs modified: %d/%d, PDF: %s", modifiedCount, len(data.APIs), pdfFileName))

	log.Printf("================================================================================")
	log.Printf("Extensive PDF Report generated successfully:")
	log.Printf("  [PDF Report] -> %s", pdfPath)
	log.Printf("  [Text Log]   -> %s", txtPath)
	log.Printf("================================================================================")

	return pdfPath, txtPath
}

// GenerateRollbackReport creates an extensive PDF report and text log for a revision rollback.
func GenerateRollbackReport(data RollbackReportData) (string, string) {
	if data.Timestamp.IsZero() {
		data.Timestamp = time.Now()
	}

	timeStr := data.Timestamp.Format("20060102_150405")
	pdfFileName := fmt.Sprintf("rollback_report_%s.pdf", timeStr)
	txtFileName := fmt.Sprintf("rollback_report_%s.txt", timeStr)

	pdfPath := filepath.Join(vars.LogsDir, pdfFileName)
	txtPath := filepath.Join(vars.LogsDir, txtFileName)

	// Build PDF Document
	pw := NewPDFWriter("WSO2 POLICY PIPELINE REPORT", "REVISION ROLLBACK REPORT")

	modeStr := "Live Rollback"
	if data.IsDryRun {
		modeStr = "Dry-Run / Preview"
	}

	// Metadata Box
	pw.AddMetadataBox([]KeyVal{
		{Key: "Execution Date", Val: data.Timestamp.Format("2006-01-02 15:04:05 MST")},
		{Key: "Environment", Val: data.Environment},
		{Key: "Base URL", Val: data.BaseURL},
		{Key: "Execution Mode", Val: modeStr},
		{Key: "Target Filter", Val: data.TargetFilter},
		{Key: "Overall Status", Val: data.Status},
	}, data.Status)

	// Stat Cards
	rolledBackApis := 0
	for _, a := range data.APIs {
		if a.Success {
			rolledBackApis++
		}
	}
	rolledBackProds := 0
	for _, p := range data.Products {
		if p.Success {
			rolledBackProds++
		}
	}

	pw.AddStatCards([]StatCard{
		{Label: "Total APIs Targeted", Value: fmt.Sprintf("%d", len(data.APIs))},
		{Label: "APIs Rolled Back", Value: fmt.Sprintf("%d", rolledBackApis)},
		{Label: "API Products Targeted", Value: fmt.Sprintf("%d", len(data.Products))},
		{Label: "Products Rolled Back", Value: fmt.Sprintf("%d", rolledBackProds)},
	})

	// APIs Breakdown
	if len(data.APIs) > 0 {
		pw.AddSectionHeader("APIs ROLLBACK BREAKDOWN")
		for _, api := range data.APIs {
			pw.AddEntityHeader("API", api.Name, api.ID, api.Status)

			fromRev := api.RolledBackFrom
			if fromRev == "" {
				fromRev = "N/A"
			}
			toRev := api.RolledBackTo
			if toRev == "" {
				toRev = "N/A"
			}

			vhosts := extractVhosts(api.Deployments)

			pw.AddTable(
				[]string{"Deleted/Reverted Rev", "Restored Active Rev", "Total Revisions", "Environments", "Status"},
				[][]string{
					{fromRev, toRev, fmt.Sprintf("%d", api.TotalRevisions), vhosts, api.Status},
				},
				[]float64{1.4, 1.4, 1.0, 1.4, 1.2},
			)

			if len(api.InitialPolicies) > 0 {
				pw.AddTextLine("Initial Policies Before Rollback:", true)
				pw.AddBulletList(api.InitialPolicies)
			}

			if len(api.PostRollbackPolicies) > 0 {
				pw.AddTextLine("Restored Policies After Rollback:", true)
				pw.AddBulletList(api.PostRollbackPolicies)
			}

			if api.Error != "" {
				pw.AddTextLine(fmt.Sprintf("Error: %s", api.Error), true)
			}
		}
	}

	// API Products Breakdown
	if len(data.Products) > 0 {
		pw.AddSectionHeader("API PRODUCTS ROLLBACK BREAKDOWN")
		for _, prod := range data.Products {
			pw.AddEntityHeader("API Product", prod.Name, prod.ID, prod.Status)

			fromRev := prod.RolledBackFrom
			if fromRev == "" {
				fromRev = "N/A"
			}
			toRev := prod.RolledBackTo
			if toRev == "" {
				toRev = "N/A"
			}

			vhosts := extractVhosts(prod.Deployments)

			pw.AddTable(
				[]string{"Deleted/Reverted Rev", "Restored Active Rev", "Total Revisions", "Environments", "Status"},
				[][]string{
					{fromRev, toRev, fmt.Sprintf("%d", prod.TotalRevisions), vhosts, prod.Status},
				},
				[]float64{1.4, 1.4, 1.0, 1.4, 1.2},
			)

			if prod.Error != "" {
				pw.AddTextLine(fmt.Sprintf("Error: %s", prod.Error), true)
			}
		}
	}

	// Save PDF
	if err := pw.SaveToFile(pdfPath); err != nil {
		log.Printf("Error saving PDF report to %s: %v", pdfPath, err)
	}

	// Generate and Save Text Summary Log
	txtContent := formatRollbackTextReport(data)
	if err := os.WriteFile(txtPath, []byte(txtContent), 0644); err != nil {
		log.Printf("Error saving text log to %s: %v", txtPath, err)
	}

	// Append to centralized log.txt
	appendToCentralLog("ROLLBACK", data.Environment, data.Status, fmt.Sprintf("APIs rolled back: %d/%d, PDF: %s", rolledBackApis, len(data.APIs), pdfFileName))

	log.Printf("================================================================================")
	log.Printf("Extensive PDF Report generated successfully:")
	log.Printf("  [PDF Report] -> %s", pdfPath)
	log.Printf("  [Text Log]   -> %s", txtPath)
	log.Printf("================================================================================")

	return pdfPath, txtPath
}

func extractVhosts(deployments []client.Deployment) string {
	if len(deployments) == 0 {
		return "Default"
	}
	var hosts []string
	for _, d := range deployments {
		if d.Vhost != "" {
			hosts = append(hosts, d.Vhost)
		} else if d.Name != "" {
			hosts = append(hosts, d.Name)
		}
	}
	if len(hosts) == 0 {
		return "Default"
	}
	return strings.Join(hosts, ", ")
}

func formatUpdateTextReport(data UpdateReportData) string {
	var sb strings.Builder
	sb.WriteString("================================================================================\n")
	sb.WriteString("                     WSO2 POLICY PIPELINE UPDATE REPORT                         \n")
	sb.WriteString("================================================================================\n")
	sb.WriteString(fmt.Sprintf("Execution Time  : %s\n", data.Timestamp.Format("2006-01-02 15:04:05 MST")))
	sb.WriteString(fmt.Sprintf("Environment     : %s\n", data.Environment))
	sb.WriteString(fmt.Sprintf("Base URL        : %s\n", data.BaseURL))
	sb.WriteString(fmt.Sprintf("Target Filter   : %s\n", data.TargetFilter))
	sb.WriteString(fmt.Sprintf("Policy Filter   : %s\n", data.PolicyFilter))
	sb.WriteString(fmt.Sprintf("Execution Mode  : %s\n", map[bool]string{true: "Dry-Run", false: "Live Update"}[data.IsDryRun]))
	sb.WriteString(fmt.Sprintf("Overall Status  : %s\n", data.Status))
	sb.WriteString("================================================================\n\n")

	sb.WriteString("--- APIs UPDATE DETAILS ---\n")
	for idx, api := range data.APIs {
		sb.WriteString(fmt.Sprintf("[%d/%d] API Name: %s (ID: %s)\n", idx+1, len(data.APIs), api.Name, api.ID))
		sb.WriteString(fmt.Sprintf("  Status           : %s\n", api.Status))
		sb.WriteString(fmt.Sprintf("  Previous Rev     : %s\n", api.PreviousRevision))
		sb.WriteString(fmt.Sprintf("  Active/New Rev   : %s\n", api.NewRevision))
		sb.WriteString(fmt.Sprintf("  Environments     : %s\n", extractVhosts(api.Deployments)))

		if len(api.PlannedChanges) > 0 {
			sb.WriteString("  Detected Policy Changes:\n")
			for _, c := range api.PlannedChanges {
				sb.WriteString(fmt.Sprintf("    - %s\n", c))
			}
		} else {
			sb.WriteString("  Detected Policy Changes: None\n")
		}

		if len(api.CurrentPolicies) > 0 {
			sb.WriteString("  Current Attached Policies:\n")
			for _, p := range api.CurrentPolicies {
				sb.WriteString(fmt.Sprintf("    - %s\n", p))
			}
		}
		sb.WriteString("\n")
	}

	if len(data.Products) > 0 {
		sb.WriteString("--- API PRODUCTS UPDATE DETAILS ---\n")
		for idx, prod := range data.Products {
			sb.WriteString(fmt.Sprintf("[%d/%d] API Product: %s (ID: %s)\n", idx+1, len(data.Products), prod.Name, prod.ID))
			sb.WriteString(fmt.Sprintf("  Status           : %s\n", prod.Status))
			sb.WriteString(fmt.Sprintf("  Previous Rev     : %s\n", prod.PreviousRevision))
			sb.WriteString(fmt.Sprintf("  Active/New Rev   : %s\n", prod.NewRevision))
			sb.WriteString(fmt.Sprintf("  Environments     : %s\n", extractVhosts(prod.Deployments)))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func formatRollbackTextReport(data RollbackReportData) string {
	var sb strings.Builder
	sb.WriteString("================================================================================\n")
	sb.WriteString("                    WSO2 POLICY PIPELINE ROLLBACK REPORT                        \n")
	sb.WriteString("================================================================================\n")
	sb.WriteString(fmt.Sprintf("Execution Time  : %s\n", data.Timestamp.Format("2006-01-02 15:04:05 MST")))
	sb.WriteString(fmt.Sprintf("Environment     : %s\n", data.Environment))
	sb.WriteString(fmt.Sprintf("Base URL        : %s\n", data.BaseURL))
	sb.WriteString(fmt.Sprintf("Target Filter   : %s\n", data.TargetFilter))
	sb.WriteString(fmt.Sprintf("Execution Mode  : %s\n", map[bool]string{true: "Dry-Run", false: "Live Rollback"}[data.IsDryRun]))
	sb.WriteString(fmt.Sprintf("Overall Status  : %s\n", data.Status))
	sb.WriteString("================================================================\n\n")

	sb.WriteString("--- APIs ROLLBACK DETAILS ---\n")
	for idx, api := range data.APIs {
		sb.WriteString(fmt.Sprintf("[%d/%d] API Name: %s (ID: %s)\n", idx+1, len(data.APIs), api.Name, api.ID))
		sb.WriteString(fmt.Sprintf("  Status           : %s\n", api.Status))
		sb.WriteString(fmt.Sprintf("  Deleted/Reverted : %s\n", api.RolledBackFrom))
		sb.WriteString(fmt.Sprintf("  Restored Active  : %s\n", api.RolledBackTo))
		sb.WriteString(fmt.Sprintf("  Total Revisions  : %d\n", api.TotalRevisions))
		sb.WriteString(fmt.Sprintf("  Environments     : %s\n", extractVhosts(api.Deployments)))

		if len(api.InitialPolicies) > 0 {
			sb.WriteString("  Initial Policies Before Rollback:\n")
			for _, p := range api.InitialPolicies {
				sb.WriteString(fmt.Sprintf("    - %s\n", p))
			}
		}

		if len(api.PostRollbackPolicies) > 0 {
			sb.WriteString("  Restored Policies After Rollback:\n")
			for _, p := range api.PostRollbackPolicies {
				sb.WriteString(fmt.Sprintf("    - %s\n", p))
			}
		}

		if api.Error != "" {
			sb.WriteString(fmt.Sprintf("  Error            : %s\n", api.Error))
		}
		sb.WriteString("\n")
	}

	if len(data.Products) > 0 {
		sb.WriteString("--- API PRODUCTS ROLLBACK DETAILS ---\n")
		for idx, prod := range data.Products {
			sb.WriteString(fmt.Sprintf("[%d/%d] API Product: %s (ID: %s)\n", idx+1, len(data.Products), prod.Name, prod.ID))
			sb.WriteString(fmt.Sprintf("  Status           : %s\n", prod.Status))
			sb.WriteString(fmt.Sprintf("  Deleted/Reverted : %s\n", prod.RolledBackFrom))
			sb.WriteString(fmt.Sprintf("  Restored Active  : %s\n", prod.RolledBackTo))
			sb.WriteString(fmt.Sprintf("  Total Revisions  : %d\n", prod.TotalRevisions))
			sb.WriteString(fmt.Sprintf("  Environments     : %s\n", extractVhosts(prod.Deployments)))
			if prod.Error != "" {
				sb.WriteString(fmt.Sprintf("  Error            : %s\n", prod.Error))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func appendToCentralLog(actionKind, envName, status, details string) {
	if err := os.MkdirAll(vars.LogsDir, 0755); err != nil {
		return
	}
	entry := fmt.Sprintf("[%s] env=%s action=%s status=%s %s\n", time.Now().Format(time.RFC3339), envName, actionKind, status, details)
	logPath := filepath.Join(vars.LogsDir, "log.txt")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(entry)
}
