package volume

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/algorinfo/rawstore/pkg/store"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"github.com/unrolled/render"
)

type WebOption func(*WebApp)

func WithVolumes(ns []string) WebOption {
	return func(w *WebApp) {
		w.namespaces = ns
	}

}

func WithProducer(p *store.Producer) WebOption {
	return func(w *WebApp) {
		w.producer = p
	}

}

func WithConfig(c *Config) WebOption {
	return func(w *WebApp) {
		w.cfg = c
	}
}

func DefaultConfig() *Config {
	return &Config{
		Addr:   "6667",
		NSDir:  "data/",
		Stream: false,
	}

}

// LoadNS load namespace from the filesystem
func LoadNS(wa *WebApp) error {

	entries, err := os.ReadDir(wa.cfg.NSDir)
	if err != nil {
		return err
	}

	for _, e := range entries {

		nsName := strings.Split(e.Name(), ".db")[0]
		log.Printf("NS Loading for %s", nsName)

		if nsName != "default" {

			fullPath := fmt.Sprintf("%s/%s", wa.cfg.NSDir, nsName)
			wa.namespaces = append(wa.namespaces, nsName)
			wa.dbs[nsName] = store.CreateDB(fullPath, dataSchemaV1)
		}
	}
	return nil
}

func CreateNS(wa *WebApp, schema, ns string) error {

	defPath := fmt.Sprintf("%s/%s", wa.cfg.NSDir, ns)
	def := store.CreateDB(defPath, schema)
	wa.dbs[ns] = def
	wa.namespaces = append(wa.namespaces, ns)
	return nil
}

// New creates a new Node instance
func New(opts ...WebOption) *WebApp {

	dbs := make(map[string]*sqlx.DB)

	wa := &WebApp{
		r:      chi.NewRouter(),
		render: render.New(),
		dbs:    dbs,
		cfg:    DefaultConfig(),
	}

	for _, opt := range opts {
		opt(wa)
	}

	CreateNS(wa, dataSchemaV1, "default")

	if LoadNS(wa) != nil {
		log.Printf("Error with dir %s", wa.cfg.NSDir)
	}

	currDir, _ := os.Getwd()

	log.Printf("Starting from %s", currDir)
	if wa.producer != nil {
		log.Printf("With stream enabled")
	} else {
		log.Printf("With stream disabled")
	}

	wa.RegisterRoutes()

	return wa
}
