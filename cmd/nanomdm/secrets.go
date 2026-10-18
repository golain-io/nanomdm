package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Environment variables that provide default paths for the *-file flags.
const (
	envAPIKeyFile     = "NANOMDM_API_KEY_FILE"
	envStorageDSNFile = "NANOMDM_STORAGE_DSN_FILE"
	envWebhookURLFile = "NANOMDM_WEBHOOK_URL_FILE"
	envAMQPSConnFile  = "NANOMDM_AMQPS_CONN_FILE"
)

// readSecretFile reads path and returns its content with surrounding
// whitespace (including any trailing newline) removed. Errors name the
// path but never include file content.
func readSecretFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		var pe *os.PathError
		if errors.As(err, &pe) {
			err = pe.Err
		}
		return "", fmt.Errorf("reading file %q: %w", path, err)
	}
	return strings.TrimSpace(string(b)), nil
}

// resolveSecret determines the effective value for a setting.
//
// Precedence: explicit value (valueSet) > file flag > env-var file path.
// Supplying both an explicit value and a file flag is an error. An empty
// file is an error unless allowEmpty is set (in which case the result is
// the empty string, i.e. unset). When valueSet is true, value is returned
// unchanged. When no source is configured the result is "".
func resolveSecret(name, value string, valueSet bool, fileFlag, envFile string, allowEmpty bool) (string, error) {
	if valueSet && fileFlag != "" {
		return "", fmt.Errorf("-%s and -%s-file are mutually exclusive", name, name)
	}
	if valueSet {
		return value, nil
	}
	path := fileFlag
	if path == "" {
		path = envFile
	}
	if path == "" {
		return "", nil
	}
	v, err := readSecretFile(path)
	if err != nil {
		return "", fmt.Errorf("-%s-file: %w", name, err)
	}
	if v == "" && !allowEmpty {
		return "", fmt.Errorf("-%s-file: file %q is empty", name, path)
	}
	return v, nil
}
