package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config contains the application configuration values loaded from environment variables.
type Config struct {
	AppPort    int
	AppEnv     string
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBTimezone string
	JWTSecret  string
}

// Load reads configuration values from environment variables and a local .env file.
func Load() (*Config, error) {
	cfg := &Config{
		AppPort:    8080,
		AppEnv:     "development",
		DBHost:     "127.0.0.1",
		DBPort:     3306,
		DBUser:     "root",
		DBPassword: "",
		DBName:     "roomly_db",
		DBTimezone: "UTC",
	}

	if err := loadDotEnv(".env"); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	cfg.AppPort = getIntEnv("APP_PORT", cfg.AppPort)
	cfg.AppEnv = getStringEnv("APP_ENV", cfg.AppEnv)
	cfg.DBHost = getStringEnv("DB_HOST", cfg.DBHost)
	cfg.DBPort = getIntEnv("DB_PORT", cfg.DBPort)
	cfg.DBUser = getStringEnv("DB_USER", cfg.DBUser)
	cfg.DBPassword = getStringEnv("DB_PASSWORD", cfg.DBPassword)
	cfg.DBName = getStringEnv("DB_NAME", cfg.DBName)
	cfg.DBTimezone = getStringEnv("DB_TIMEZONE", cfg.DBTimezone)
	cfg.JWTSecret = getStringEnv("JWT_SECRET", "")
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be set to at least 32 characters")
	}

	return cfg, nil
}

// DSN builds the MySQL DSN string used by the database package.
func (c *Config) DSN() string {
	loc := url.QueryEscape(c.DBTimezone)
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=%s", c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, loc)
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
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
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, strings.Trim(value, "\"'")); err != nil {
				return err
			}
		}
	}

	return scanner.Err()
}

func getStringEnv(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func getIntEnv(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}
