package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Common errors for configuration loading/saving.
var (
	ErrFileNotFound     = errors.New("configuration file not found")
	ErrPermissionDenied = errors.New("permission denied")
	ErrInvalidJSON      = errors.New("invalid JSON syntax")
	ErrEmptyFile        = errors.New("configuration file is empty")
)

// LoadFromFile reads a MockCollection from a JSON file.
// Returns wrapped errors for common failure cases.
func LoadFromFile(path string) (*MockCollection, error) {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, path)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("%w: %s", ErrPermissionDenied, path)
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Check if it's a regular file
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", path)
	}

	// Open and read file
	file, err := os.Open(path)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("%w: %s", ErrPermissionDenied, path)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrEmptyFile, path)
	}

	// Validate JSON syntax first
	if !json.Valid(data) {
		return nil, fmt.Errorf("%w in file: %s", ErrInvalidJSON, path)
	}

	return ParseJSON(data)
}

// SaveToFile writes a MockCollection to a JSON file using atomic rename.
// Creates parent directories if they don't exist.
func SaveToFile(path string, collection *MockCollection) error {
	if collection == nil {
		return errors.New("collection cannot be nil")
	}

	// Convert to JSON
	data, err := ToJSON(collection)
	if err != nil {
		return fmt.Errorf("failed to marshal collection: %w", err)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write to temporary file first (atomic write pattern)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		// Clean up temp file on failure
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return nil
}

// ParseJSON parses JSON bytes into a MockCollection with validation.
func ParseJSON(data []byte) (*MockCollection, error) {
	var collection MockCollection

	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate the collection
	if err := collection.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &collection, nil
}

// ToJSON marshals a MockCollection to formatted JSON bytes.
func ToJSON(collection *MockCollection) ([]byte, error) {
	if collection == nil {
		return nil, errors.New("collection cannot be nil")
	}

	data, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	// Add trailing newline for better file formatting
	data = append(data, '\n')

	return data, nil
}

// LoadMocksFromFile is a convenience function that loads mocks from a file
// and returns just the mock configurations slice.
func LoadMocksFromFile(path string) ([]*MockConfiguration, error) {
	collection, err := LoadFromFile(path)
	if err != nil {
		return nil, err
	}

	return collection.Mocks, nil
}

// SaveMocksToFile is a convenience function that saves mock configurations
// to a file, wrapping them in a MockCollection.
func SaveMocksToFile(path string, mocks []*MockConfiguration, name string) error {
	collection := &MockCollection{
		Version: "1.0",
		Name:    name,
		Mocks:   mocks,
	}

	return SaveToFile(path, collection)
}
