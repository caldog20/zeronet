package controller

import (
	"context"
	"net/http"

	"github.com/caldog20/zeronet/controller/frontend"
)

type HTTPServer struct {
	mux        *http.ServeMux
	server     *http.Server
	controller *Controller
}

func NewHTTPServer(controller *Controller) *HTTPServer {
	mux := http.NewServeMux()
	return &HTTPServer{
		mux:        mux,
		controller: controller,
	}
}

func Cors(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		handler.ServeHTTP(w, r)
	})
}

func (s *HTTPServer) Serve(addr string) error {
	s.mux.Handle("/", frontend.SvelteKitHandler("/"))

	s.server = &http.Server{Addr: addr, Handler: Cors(s.mux)}

	if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (s *HTTPServer) Close(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
