package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadEnvFile loads environment variables from a .env file.
// It only sets variables that are not already set in the environment.
// Lines starting with # are treated as comments.
// Empty lines are ignored.
// Side effects: writes to the process environment via os.Setenv.
func LoadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=value
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		// Remove surrounding quotes if present
		value = strings.Trim(value, `"'`)

		// Only set if not already in environment
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

// LoadEnvFileIfExists loads a .env file if it exists, otherwise does nothing.
// Side effects: writes to the process environment if the file is present.
func LoadEnvFileIfExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return LoadEnvFile(path)
}
