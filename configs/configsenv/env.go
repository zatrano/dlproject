package configsenv

import (
	"os"
	"strconv"
)

func GetEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func GetEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	valueInt, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return valueInt
}

func IsProduction() bool {
	return os.Getenv("APP_ENV") == "production"
}
