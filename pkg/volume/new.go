package volume

import (
	"strconv"

	"github.com/algorinfo/rawstore/pkg/store"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"github.com/unrolled/render"
)

type WebOption func(*WebApp)

func WithAddr(a string) WebOption {
	return func(w *WebApp) {
		w.addr = a
	}
}

func WithVolumes(ns []string) WebOption {
	return func(w *WebApp) {
		w.namespaces = ns
	}

}

// New creates a new Node instance
func New(opts ...WebOption) *WebApp {

	rl, _ := strconv.Atoi("10")

	def := store.CreateDB("default")
	dbs := make(map[string]*sqlx.DB)
	dbs["default"] = def

	wa := &WebApp{
		addr:      ":6665",
		r:         chi.NewRouter(),
		render:    render.New(),
		rateLimit: rl,
		dbs:       dbs,
	}

	for _, opt := range opts {
		opt(wa)
	}
	return wa
}
