package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
)

type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"
)

type Config struct {
	Env           Environment
	Port          string
	Host          string
	LogLevel      string
	SessionSecret string

	Server ServerConfig
	Quiz   QuizConfig
	CORS   CORSConfig
	Rate   RateConfig
}

type ServerConfig struct {
	ReadTimeout  int
	WriteTimeout int
	IdleTimeout  int
	MaxConns     int
}

type QuizConfig struct {
	MaxPlayers     int
	MaxQuestions   int
	AutoLoadSample bool
	TimerDefault   int
}

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
}

type RateConfig struct {
	Enabled           bool
	RequestsPerSecond int
	Burst             int
}

func Load() *Config {
	env := Environment(getEnv("APP_ENV", string(EnvDevelopment)))

	cfg := &Config{
		Env:           env,
		Port:          getEnv("PORT", "8080"),
		Host:          getEnv("HOST", "0.0.0.0"),
		LogLevel:      getEnv("LOG_LEVEL", defaultLogLevel(env)),
		SessionSecret: getEnv("SESSION_SECRET", generateSessionSecret()),
		Server: ServerConfig{
			ReadTimeout:  getEnvInt("SERVER_READ_TIMEOUT", 30),
			WriteTimeout: getEnvInt("SERVER_WRITE_TIMEOUT", 30),
			IdleTimeout:  getEnvInt("SERVER_IDLE_TIMEOUT", 120),
			MaxConns:     getEnvInt("SERVER_MAX_CONNS", 100),
		},
		Quiz: QuizConfig{
			MaxPlayers:     getEnvInt("MAX_PLAYERS", 100),
			MaxQuestions:   getEnvInt("MAX_QUESTIONS", 50),
			AutoLoadSample: getEnvBool("AUTO_LOAD_SAMPLE", true),
			TimerDefault:   getEnvInt("QUIZ_TIMER_DEFAULT", 30),
		},
	}

	if env == EnvDevelopment {
		cfg.applyDevelopmentDefaults()
	} else {
		cfg.applyProductionDefaults()
	}

	return cfg
}

func (c *Config) applyDevelopmentDefaults() {
	c.LogLevel = getEnv("LOG_LEVEL", "debug")
	c.CORS.AllowedOrigins = []string{"*"}
	c.CORS.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	c.CORS.AllowedHeaders = []string{"*"}
	c.Rate.Enabled = false
}

func (c *Config) applyProductionDefaults() {
	c.LogLevel = getEnv("LOG_LEVEL", "info")
	c.CORS.AllowedOrigins = parseOrigins(getEnv("CORS_ORIGINS", ""))
	c.CORS.AllowedMethods = []string{"GET", "POST", "OPTIONS"}
	c.CORS.AllowedHeaders = []string{"Content-Type", "Authorization"}
	c.Rate.Enabled = getEnvBool("RATE_LIMIT_ENABLED", true)
	c.Rate.RequestsPerSecond = getEnvInt("RATE_LIMIT_RPS", 100)
	c.Rate.Burst = getEnvInt("RATE_LIMIT_BURST", 200)
}

func defaultLogLevel(env Environment) string {
	if env == EnvDevelopment {
		return "debug"
	}
	return "info"
}

func parseOrigins(origins string) []string {
	if origins == "" || origins == "*" {
		return []string{"*"}
	}
	return splitAndTrim(origins, ",")
}

func splitAndTrim(s, sep string) []string {
	if s == "" {
		return nil
	}
	result := make([]string, 0)
	for _, part := range splitString(s, sep) {
		trimmed := trimString(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitString(s, sep string) []string {
	result := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i = start - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimString(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func (c *Config) IsDevelopment() bool {
	return c.Env == EnvDevelopment
}

func (c *Config) IsProduction() bool {
	return c.Env == EnvProduction
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

func generateSessionSecret() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a simple default if crypto/rand fails
		return "quiz-forge-default-secret-change-in-production"
	}
	return hex.EncodeToString(bytes)
}
