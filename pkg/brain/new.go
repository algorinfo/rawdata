package brain

import (
	"strconv"

	"github.com/algorinfo/rawstore/pkg/store"
	"github.com/go-chi/chi/v5"
	"github.com/unrolled/render"
)

type WebOption func(*WebApp)

func WithRedis(r *store.Redis) WebOption {
	return func(w *WebApp) {
		w.redis = r
	}
}

func WithAddr(a string) WebOption {
	return func(w *WebApp) {
		w.addr = a
	}
}

func WithVolumes(vols []string) WebOption {
	return func(w *WebApp) {
		w.volumes = vols
	}

}

// New creates a new Node instance
func New(opts ...WebOption) *WebApp {

	rl, _ := strconv.Atoi(rateLimit)

	wa := &WebApp{
		addr:      ":6665",
		r:         chi.NewRouter(),
		render:    render.New(),
		rateLimit: rl,
	}

	for _, opt := range opts {
		opt(wa)
	}
	return wa
}
