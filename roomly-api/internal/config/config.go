package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
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

	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
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
	if !strings.EqualFold(cfg.DBTimezone, "UTC") {
		return nil, fmt.Errorf("DB_TIMEZONE must be UTC; booking timestamps are stored in UTC")
	}
	cfg.DBTimezone = "UTC"
	cfg.JWTSecret = getStringEnv("JWT_SECRET", "")
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be set to at least 32 characters")
	}

	return cfg, nil
}

// DSN builds the MySQL DSN string used by the database package.
func (c *Config) DSN() string {
	dsn := mysql.NewConfig()
	dsn.User = c.DBUser
	dsn.Passwd = c.DBPassword
	dsn.Net = "tcp"
	dsn.Addr = net.JoinHostPort(c.DBHost, strconv.Itoa(c.DBPort))
	dsn.DBName = c.DBName
	dsn.ParseTime = true
	dsn.Loc = time.UTC
	dsn.Params = map[string]string{"time_zone": "'+00:00'"}
	return dsn.FormatDSN()
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
