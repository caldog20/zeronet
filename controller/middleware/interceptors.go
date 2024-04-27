package middleware

import (
	"context"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/caldog20/zeronet/controller/auth"
)

func methodBypass(method string) bool {
	switch method {
	case "/proto.ControllerService/GetPKCEAuthInfo":
		return true
	case "/proto.ControllerService/LoginPeer":
		return true
	default:
		return false
	}
}

func authorizeJwt(ctx context.Context, method string) error {
	if methodBypass(method) {
		return nil
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.Unauthenticated, "metadata missing from request context")
	}

	values := md["authorization"]
	if len(values) == 0 {
		return status.Errorf(codes.Unauthenticated, "authorization token is not provided")
	}
	jwtToken := values[0]

	_, err := auth.ParseJwtWithClaims(jwtToken)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "jwt token is invalid: %v", err)
	}
	return nil
}

func NewUnaryAuthInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		log.Println("--> unary auth interceptor: ", info.FullMethod)
		err := authorizeJwt(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}
