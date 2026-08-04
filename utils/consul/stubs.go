package consul

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Consul keys structures
type apiKeysStruct struct {
	GeminiCredsVB map[string]interface{}
}

// ApiKeys is a global stub variable representing credentials stored in Consul.
// These mock values activate the conversational simulator if no live keys are configured.
var ApiKeys = apiKeysStruct{
	GeminiCredsVB: map[string]interface{}{
		"project_id":   "mock-bard-pilot-project-id",
		"private_key":  "-----BEGIN PRIVATE KEY-----\nMOCK\n-----END PRIVATE KEY-----",
		"client_email": "mock@mock-bard-pilot.iam.gserviceaccount.com",
	},
}

func init() {
	// Traverse current and parent folders to locate and load a .env file if it exists
	loadEnv(".env")
	loadEnv("../.env")
	loadEnv("../../.env")

	// 1. Try loading credentials from a Google Cloud Service Account JSON file path
	if credsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); credsPath != "" {
		cleanPath := filepath.Clean(credsPath)
		if fileBytes, err := os.ReadFile(cleanPath); err == nil { // #nosec G304, G703
			var credsMap map[string]interface{}
			if err := json.Unmarshal(fileBytes, &credsMap); err == nil {
				ApiKeys.GeminiCredsVB = credsMap
				return
			}
		}
	}

	// 2. Try loading individual service account credentials variables directly
	if projID := os.Getenv("GCP_PROJECT_ID"); projID != "" {
		ApiKeys.GeminiCredsVB["project_id"] = projID
	}
	if privKey := os.Getenv("GCP_PRIVATE_KEY"); privKey != "" {
		// Keep newlines intact for RSA private key headers and feet
		formattedKey := strings.ReplaceAll(privKey, "\\n", "\n")
		ApiKeys.GeminiCredsVB["private_key"] = formattedKey
	}
	if email := os.Getenv("GCP_CLIENT_EMAIL"); email != "" {
		ApiKeys.GeminiCredsVB["client_email"] = email
	}
}

// loadEnv reads key=value configuration lines and registers them as environment variables
func loadEnv(filename string) {
	cleanName := filepath.Clean(filename)
	file, err := os.Open(cleanName) // #nosec G304
	if err != nil {
		return // File does not exist, ignore silently
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Strip quotes if wrapped around values
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		// Save variable if not already established in the shell
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}
