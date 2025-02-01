package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Port string
	FQDN string
	ClientID string
	ClientSecret string
	Environment string
	RepoOwner string
	RepoName string
	RepoBranch string
	RepoPath string
}

const (
	portEnv         = "PORT"
	fqdnEnv         = "FQDN"
	clientIDEnv     = "CLIENT_ID"
	clientSecretEnv = "CLIENT_SECRET"
	environmentEnv  = "ENVIRONMENT"
	repoOwnerEnv    = "REPO_OWNER"
	repoNameEnv     = "REPO_NAME"
	repoBranchEnv   = "REPO_BRANCH"
	repoPathEnv     = "REPO_PATH"
)

func New() (*Config, error) {
	if err := loadEnv(); err != nil {
		return nil, fmt.Errorf("failed to load .env file: %w", err)
	}

	requiredVars := map[string]string{
		portEnv:         os.Getenv(portEnv),
		fqdnEnv:         os.Getenv(fqdnEnv),
		clientIDEnv:     os.Getenv(clientIDEnv),
		clientSecretEnv: os.Getenv(clientSecretEnv),
		repoOwnerEnv:    os.Getenv(repoOwnerEnv),
		repoNameEnv:     os.Getenv(repoNameEnv),
		repoBranchEnv:   os.Getenv(repoBranchEnv),
		repoPathEnv:     os.Getenv(repoPathEnv),
	}

	var missingVars []string
	for envVar, value := range requiredVars {
		if value == "" {
			missingVars = append(missingVars, envVar)
		}
	}

	if len(missingVars) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missingVars, ", "))
	}

	fqdn := strings.TrimSuffix(requiredVars[fqdnEnv], "/")

	env := os.Getenv(environmentEnv)
	if env == "" {
		env = "production"
	}

	return &Config{
		Port:         requiredVars[portEnv],
		FQDN:         fqdn,
		ClientID:     requiredVars[clientIDEnv],
		ClientSecret: requiredVars[clientSecretEnv],
		Environment:  env,
		RepoOwner:    requiredVars[repoOwnerEnv],
		RepoName:     requiredVars[repoNameEnv],
		RepoBranch:   requiredVars[repoBranchEnv],
		RepoPath:     requiredVars[repoPathEnv],
	}, nil
}

func loadEnv() error {
	envPath, err := findEnvFile()
	if err != nil {
		return fmt.Errorf("error finding .env file: %w", err)
	}

	if envPath == "" {
		return nil
	}

	file, err := os.Open(envPath)
	if err != nil {
		return fmt.Errorf("error opening .env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid format in .env file line %d: %s", lineNum, line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		value = strings.Trim(value, `"'`)

		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading .env file: %w", err)
	}

	return nil
}

func findEnvFile() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

func (c *Config) IsDevelopment() bool {
	return strings.ToLower(c.Environment) == "development"
}

func (c *Config) IsProduction() bool {
	return strings.ToLower(c.Environment) == "production"
}