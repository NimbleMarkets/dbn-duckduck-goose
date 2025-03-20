// Copyright (c) 2025 Neomantra Corp

package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetEnvDefault retrieves the value of an environment variable, returning the fallback value if the variable is not set.
// https://stackoverflow.com/a/40326580
func GetEnvDefault(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		value = fallback
	}
	return value
}

// InternalError responds to a request with an http.StatusInternalServerError and status
func InternalError(c *gin.Context, message string, err error) {
	c.Error(err)
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": message})
}

// BadRequestError responds to a request with an error and status
func BadRequestError(c *gin.Context, err error) {
	c.Error(err)
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
}

// ValidatePositiveNonzeroInteger checks if the input string is a positive non-zero integer.
// Returns a descriptive error if the input is not valid.
func ValidatePositiveNonzeroInteger(str string) (int, error) {
	num, err := strconv.Atoi(str)
	if err != nil {
		return 0, fmt.Errorf("input is not a valid integer: %w", err)
	}

	if num <= 0 {
		return 0, fmt.Errorf("input must be greater than 0")
	}

	return num, nil
}
