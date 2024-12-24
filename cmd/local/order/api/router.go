package api

import (
	"net/http"

	"orchestration/cmd/local/orchestrator/appcontext"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Router struct {
	handlers OrderHandlers
}

func NewRouter(handlers OrderHandlers) *Router {
	return &Router{
		handlers: handlers,
	}
}

func (rr *Router) Build() *chi.Mux {
	router := chi.NewRouter()
	router.Use(requestIDMiddleware)
	router.Route("/v1", func(r chi.Router) {
		r.Get("/health", rr.handlers.Health)
		r.Get("/orders", rr.handlers.ListAll)
		r.Get("/orders/{id}", rr.handlers.GetByID)
	})
	return router
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		hReqID := r.Header.Get("X-Request-ID")
		if hReqID != "" {
			requestID = hReqID
		}
		ctx := appcontext.WithRequestID(r.Context(), requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
