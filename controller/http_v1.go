package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/caldog20/zeronet/controller/auth"
	"github.com/caldog20/zeronet/controller/frontend"
)

type HTTPServer struct {
	mux            *http.ServeMux
	server         *http.Server
	controller     *Controller
	tokenValidator *auth.TokenValidator
}

func NewHTTPServer(controller *Controller, validator *auth.TokenValidator) *HTTPServer {
	mux := http.NewServeMux()
	return &HTTPServer{
		mux:            mux,
		controller:     controller,
		tokenValidator: validator,
	}
}

func (s *HTTPServer) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := strings.Split(r.Header.Get("Authorization"), "Bearer ")
		if len(header) != 2 {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Malformed Token"))
			return
		}

		token := header[1]
		fmt.Printf("TOKEN: %s\n", token)
		err := s.tokenValidator.ValidateAccessToken(token)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Invalid Token"))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

func Cors(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		handler.ServeHTTP(w, r)
	})
}

func (s *HTTPServer) Serve(addr string) error {
	s.mux.Handle("/", frontend.SvelteKitHandler("/"))

	s.mux.Handle("GET /api/peers", s.Middleware(http.HandlerFunc(s.GetPeers)))

	s.server = &http.Server{Addr: addr, Handler: Cors(s.mux)}

	if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (s *HTTPServer) GetPeers(w http.ResponseWriter, r *http.Request) {
	peers, err := s.controller.db.GetPeers()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(peers)
	}
}

func (s *HTTPServer) Close(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
