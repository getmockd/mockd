package engine

import (
	"errors"
	"strings"
)

func buildGraphQLSubscriptionPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("graphql path is required when subscriptions are configured")
	}
	if path[len(path)-1] != '/' {
		return path + "/ws", nil
	}
	return path + "ws", nil
}
