package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FrameworkInfo holds detected framework/language information
type FrameworkInfo struct {
	Language    string // "Node", "Python", "Go", "Ruby", "PHP", "Java", "Rust", etc.
	Framework   string // "Express", "Django", "Gin", "Rails", "Laravel", etc.
	Version     string // e.g., "18.12.0", "3.9.1"
	PackageJson string // Path to package.json if found
	Confidence  string // "high", "medium", "low"
}

// DetectFramework analyzes a process to identify its framework and language
func DetectFramework(pid int, command string, cwd string) *FrameworkInfo {
	info := &FrameworkInfo{Confidence: "low"}

	// Try to detect from command line first
	cmdLower := strings.ToLower(command)

	// Node.js detection
	if strings.Contains(cmdLower, "node") || strings.Contains(cmdLower, "npm") || strings.Contains(cmdLower, "yarn") {
		info.Language = "Node.js"
		info.Framework = detectNodeFramework(command, cwd)
		info.Version = extractNodeVersion(pid)
		info.Confidence = "high"
		return info
	}

	// Python detection
	if strings.Contains(cmdLower, "python") {
		info.Language = "Python"
		info.Framework = detectPythonFramework(command, cwd)
		info.Version = extractPythonVersion(pid)
		info.Confidence = "high"
		return info
	}

	// Go detection
	if strings.Contains(cmdLower, "go run") {
		info.Language = "Go"
		info.Framework = "Go (custom)"
		info.Version = extractGoVersion()
		info.Confidence = "high"
		return info
	}

	// Ruby detection
	if strings.Contains(cmdLower, "ruby") || strings.Contains(cmdLower, "rails") {
		info.Language = "Ruby"
		info.Framework = detectRubyFramework(command)
		info.Version = extractRubyVersion(pid)
		info.Confidence = "high"
		return info
	}

	// Java detection
	if strings.Contains(cmdLower, "java") {
		info.Language = "Java"
		info.Framework = detectJavaFramework(command)
		info.Version = extractJavaVersion(pid)
		info.Confidence = "medium"
		return info
	}

	// PHP detection
	if strings.Contains(cmdLower, "php") {
		info.Language = "PHP"
		info.Framework = "PHP"
		info.Version = extractPHPVersion(pid)
		info.Confidence = "high"
		return info
	}

	// Rust detection
	if strings.Contains(cmdLower, "cargo") {
		info.Language = "Rust"
		info.Framework = "Rust (custom)"
		info.Version = extractRustVersion()
		info.Confidence = "high"
		return info
	}

	// If we couldn't identify, set to unknown
	info.Language = "Unknown"
	info.Confidence = "low"
	return info
}

func detectNodeFramework(command string, cwd string) string {
	cmdLower := strings.ToLower(command)

	// Check for known frameworks in command
	if strings.Contains(cmdLower, "express") {
		return "Express"
	}
	if strings.Contains(cmdLower, "next") {
		return "Next.js"
	}
	if strings.Contains(cmdLower, "nuxt") {
		return "Nuxt"
	}
	if strings.Contains(cmdLower, "vue") {
		return "Vue"
	}
	if strings.Contains(cmdLower, "react") {
		return "React"
	}
	if strings.Contains(cmdLower, "gatsby") {
		return "Gatsby"
	}
	if strings.Contains(cmdLower, "vite") {
		return "Vite"
	}
	if strings.Contains(cmdLower, "webpack") {
		return "Webpack"
	}

	// Check package.json for dependencies
	pkgPath := filepath.Join(cwd, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		content := string(data)
		if strings.Contains(content, "express") {
			return "Express"
		}
		if strings.Contains(content, "next") {
			return "Next.js"
		}
		if strings.Contains(content, "nuxt") {
			return "Nuxt"
		}
		if strings.Contains(content, "fastify") {
			return "Fastify"
		}
		if strings.Contains(content, "koa") {
			return "Koa"
		}
		if strings.Contains(content, "hapi") {
			return "Hapi"
		}
	}

	return "Node.js (generic)"
}

func detectPythonFramework(command string, cwd string) string {
	cmdLower := strings.ToLower(command)

	// Check for known frameworks
	if strings.Contains(cmdLower, "django") || strings.Contains(cmdLower, "manage.py") {
		return "Django"
	}
	if strings.Contains(cmdLower, "flask") {
		return "Flask"
	}
	if strings.Contains(cmdLower, "fastapi") {
		return "FastAPI"
	}
	if strings.Contains(cmdLower, "uvicorn") {
		return "FastAPI (uvicorn)"
	}
	if strings.Contains(cmdLower, "gunicorn") {
		return "Gunicorn"
	}
	if strings.Contains(cmdLower, "pyramid") {
		return "Pyramid"
	}
	if strings.Contains(cmdLower, "starlette") {
		return "Starlette"
	}

	// Check for requirements.txt
	if _, err := os.Stat(filepath.Join(cwd, "requirements.txt")); err == nil {
		if data, err := os.ReadFile(filepath.Join(cwd, "requirements.txt")); err == nil {
			content := string(data)
			if strings.Contains(content, "django") {
				return "Django"
			}
			if strings.Contains(content, "flask") {
				return "Flask"
			}
			if strings.Contains(content, "fastapi") {
				return "FastAPI"
			}
		}
	}

	return "Python (generic)"
}

func detectRubyFramework(command string) string {
	cmdLower := strings.ToLower(command)

	if strings.Contains(cmdLower, "rails") {
		return "Rails"
	}
	if strings.Contains(cmdLower, "sinatra") {
		return "Sinatra"
	}
	if strings.Contains(cmdLower, "hanami") {
		return "Hanami"
	}

	return "Ruby (generic)"
}

func detectJavaFramework(command string) string {
	cmdLower := strings.ToLower(command)

	if strings.Contains(cmdLower, "spring") {
		return "Spring"
	}
	if strings.Contains(cmdLower, "quarkus") {
		return "Quarkus"
	}
	if strings.Contains(cmdLower, "micronaut") {
		return "Micronaut"
	}
	if strings.Contains(cmdLower, "dropwizard") {
		return "Dropwizard"
	}

	return "Java (generic)"
}

// Version extraction helpers
func extractNodeVersion(pid int) string {
	out, _ := exec.Command("node", "--version").Output()
	return strings.TrimSpace(string(out))
}

func extractPythonVersion(pid int) string {
	out, _ := exec.Command("python3", "--version").Output()
	if len(out) == 0 {
		out, _ = exec.Command("python", "--version").Output()
	}
	return strings.TrimSpace(string(out))
}

func extractGoVersion() string {
	out, _ := exec.Command("go", "version").Output()
	parts := strings.Fields(string(out))
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

func extractRubyVersion(pid int) string {
	out, _ := exec.Command("ruby", "--version").Output()
	parts := strings.Fields(string(out))
	if len(parts) > 0 {
		return parts[1]
	}
	return ""
}

func extractJavaVersion(pid int) string {
	out, _ := exec.Command("java", "-version").CombinedOutput()
	return strings.TrimSpace(string(out))
}

func extractPHPVersion(pid int) string {
	out, _ := exec.Command("php", "--version").Output()
	parts := strings.Fields(string(out))
	if len(parts) > 0 {
		return parts[1]
	}
	return ""
}

func extractRustVersion() string {
	out, _ := exec.Command("rustc", "--version").Output()
	return strings.TrimSpace(string(out))
}
