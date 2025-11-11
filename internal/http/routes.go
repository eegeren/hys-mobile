package transport

import (
	"log"
	gohttp "net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/hys/backend/internal/handlers"
)

// NewRouter wires routes and middlewares.
func NewRouter(h *handlers.Handler) *mux.Router {
	r := mux.NewRouter()
	r.Use(loggingMiddleware)
	r.Use(recoverMiddleware)
	r.Use(corsMiddleware)

	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/healthz", h.Healthz).Methods(gohttp.MethodGet)
	api.HandleFunc("/personel_detay", h.PersonelList).Methods(gohttp.MethodGet)
	api.HandleFunc("/giris", h.Login).Methods(gohttp.MethodPost)

	admin := api.PathPrefix("/admin").Subrouter()
	admin.HandleFunc("/allowlist", h.AllowlistList).Methods(gohttp.MethodGet)
	admin.HandleFunc("/allowlist", h.AllowlistAdd).Methods(gohttp.MethodPost)
	admin.HandleFunc("/allowlist/{tc}", h.AllowlistDelete).Methods(gohttp.MethodDelete)

	return r
}

type responseRecorder struct {
	gohttp.ResponseWriter
	status int
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.status = code
	rr.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next gohttp.Handler) gohttp.Handler {
	return gohttp.HandlerFunc(func(w gohttp.ResponseWriter, r *gohttp.Request) {
		rr := &responseRecorder{ResponseWriter: w, status: gohttp.StatusOK}
		start := time.Now()
		next.ServeHTTP(rr, r)
		log.Printf("access: %s %s -> %d (%s)", r.Method, r.URL.Path, rr.status, time.Since(start))
	})
}

func recoverMiddleware(next gohttp.Handler) gohttp.Handler {
	return gohttp.HandlerFunc(func(w gohttp.ResponseWriter, r *gohttp.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(gohttp.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"ok":false,"error":"internal server error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next gohttp.Handler) gohttp.Handler {
	return gohttp.HandlerFunc(func(w gohttp.ResponseWriter, r *gohttp.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Role")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if r.Method == gohttp.MethodOptions {
			w.WriteHeader(gohttp.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
