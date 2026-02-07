package auth

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

const (
	serviceName   = "focst"
	geminiAccount = "gemini-api-key"
	openaiAccount = "openai-api-key"
	geminiEnvVar  = "GEMINI_API_KEY"
	openaiEnvVar  = "OPENAI_API_KEY"
)

// GetKey retrieves the API key for a specific service (gemini or openai).
// If allowEnv is false, environment variables are ignored.
func GetKey(service string, allowEnv bool) (string, string) {
	account := geminiAccount
	envVar := geminiEnvVar
	if service == "openai" {
		account = openaiAccount
		envVar = openaiEnvVar
	}

	// 1. Try Keychain
	key, err := keyring.Get(serviceName, account)
	if err == nil && key != "" {
		return strings.TrimSpace(key), "Keychain"
	}

	if allowEnv {
		// 2. Try Env Var (optional)
		key = os.Getenv(envVar)
		if key != "" {
			return strings.TrimSpace(key), "Environment Variable"
		}
	}

	return "", ""
}

// SaveKey saves the key for a specific service to the OS Keychain.
func SaveKey(service, key string) error {
	account := geminiAccount
	if service == "openai" {
		account = openaiAccount
	}
	return keyring.Set(serviceName, account, strings.TrimSpace(key))
}

// DeleteKey removes the key for a specific service from the OS Keychain.
func DeleteKey(service string) error {
	account := geminiAccount
	if service == "openai" {
		account = openaiAccount
	}
	return keyring.Delete(serviceName, account)
}

// GetStatus returns whether a key exists for a specific service in the keychain.
func GetStatus(service string) bool {
	account := geminiAccount
	if service == "openai" {
		account = openaiAccount
	}
	key, err := keyring.Get(serviceName, account)
	if err != nil || key == "" {
		return false
	}
	return true
}

// PromptForAPIKey securely prompts the user for their API key.
func PromptForAPIKey(prompt string) (string, error) {
	fmt.Print(prompt)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	fmt.Println() // Add newline after password input
	return strings.TrimSpace(string(bytePassword)), nil
}

// GetEnvKey retrieves the key from environment variables only.
func GetEnvKey(service string) (string, bool) {
	envVar := geminiEnvVar
	if service == "openai" {
		envVar = openaiEnvVar
	}
	key := strings.TrimSpace(os.Getenv(envVar))
	if key == "" {
		return "", false
	}
	return key, true
}
