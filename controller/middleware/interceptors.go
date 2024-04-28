package middleware

import (
	"context"
	"strings"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
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
	// TODO: Verify JWT matches current Peer JWT even if it's valid
	// This prevents a leaked JWT from being used with another peer
	_, err := auth.ParseJwtWithClaims(jwtToken)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "jwt token is invalid: %v", err)
	}
	return nil
}

func NewUnaryLogInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		p, _ := peer.FromContext(ctx)
		splitMethod := strings.Split(info.FullMethod, "/")
		var method string
		if len(splitMethod) < 3 {
			method = info.FullMethod
		} else {
			method = splitMethod[2]
		}
		log.Printf("--> unary log interceptor: %s - peer: %s", method, p.Addr.String())
		return handler(ctx, req)
	}
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
