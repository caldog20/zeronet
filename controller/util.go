package controller

import (
	"context"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const MachineIDLen = 64

var alphanumeric = regexp.MustCompile("^[a-zA-Z0-9_]*$")

func validateMachineID(id string) bool {
	return true
	if !alphanumeric.MatchString(id) {
		return false
	}
	if len(id) < MachineIDLen {
		return false
	}

	return true
}

func extractTokenMetadata(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "metadata missing from request context")
	}

	values := md["authorization"]
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "invalid or missing access token")
	}
	token := strings.Split(values[0], "Bearer ")

	if len(token) < 2 {
		return "", status.Error(codes.Unauthenticated, "invalid or missing access token")
	}

	return token[1], nil
}
