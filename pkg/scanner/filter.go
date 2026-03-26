package scanner

import (
	"strings"

	"github.com/devports/devpt/pkg/models"
)

// IsDevProcess checks if a process is likely a development server
func IsDevProcess(record *models.ProcessRecord, commandInfo string) bool {
	if record == nil {
		return false
	}

	// Extract process name from command
	cmd := strings.ToLower(commandInfo)

	ignorePatterns := []string{
		"/.cursor/",
		"cursor.app",
		"cursor-server",
		"/.vscode/",
		"code helper",
		"com.microsoft.vscode",
	}
	for _, pattern := range ignorePatterns {
		if strings.Contains(cmd, pattern) {
			return false
		}
	}

	// Check for known dev tools and frameworks
	devPatterns := []string{
		"node",
		"npm",
		"yarn",
		"pnpm",
		"python",
		"python3",
		"ruby",
		"rails",
		"go",
		"java",
		"mvn",
		"gradle",
		"cargo",
		"rust",
		"php",
		"laravel",
		"symfony",
		"dotnet",
		"flask",
		"django",
		"fastapi",
		"uvicorn",
		"gunicorn",
		"express",
		"next",
		"nuxt",
		"vite",
		"webpack",
		"parcel",
		"gulp",
		"deno",
		"bun",
		"rspec",
		"pytest",
		"jest",
		"vitest",
		"cloudflared", // Cloudflare tunnel for dev exposure
	}

	for _, pattern := range devPatterns {
		if strings.Contains(cmd, pattern) {
			return true
		}
	}

	return false
}

// FilterDevProcesses keeps only development-related processes
func FilterDevProcesses(records []*models.ProcessRecord, commandMap map[int]string) []*models.ProcessRecord {
	filtered := make([]*models.ProcessRecord, 0)

	for _, record := range records {
		if record == nil {
			continue
		}

		cmd := commandMap[record.PID]
		if IsDevProcess(record, cmd) {
			filtered = append(filtered, record)
		}
	}

	return filtered
}
