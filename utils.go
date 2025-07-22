package main

import (
	"bytes"
	"github.com/google/uuid"
	"log/slog"
	"math/rand"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz" + "0123456789"

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func stringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func GetStringId(length int) string {
	return stringWithCharset(length, charset)
}

func GetUuid() string {
	generatedId := uuid.New()
	return generatedId.String()
}

// CreateLogString generate a log formated as desired.
// Accept "json" or "console" as formats.
// Attributes are passed as key-value pairs: "key1", value1, "key2", value2, ...
func CreateLogString(format string, msg string, attrs ...interface{}) string {
	var output bytes.Buffer
	var handler slog.Handler

	//
	switch format {
	case "json":
		handler = slog.NewJSONHandler(&output, nil)
	default:
		handler = slog.NewTextHandler(&output, nil)
	}

	//
	logger := slog.New(handler)
	logger.Info(msg, attrs...)

	return output.String()
}
