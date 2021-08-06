package volume

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/algorinfo/rawstore/pkg/jump"
	"github.com/cespare/xxhash"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/unrolled/render"
)

type PutDataRSP struct {
	Namespace string `json:"namespace"`
	Path      string `json:"path"`
	// HashKey   string  `json:"hashKey"`
	GroupBy string `json:"groupBy,omitempty"`
	Bucket  int32  `json:"bucket,omitempty"`
}

type Volume struct {
	Addr     string `json:"address"`
	JumpNode int32  `json:"jumpNode"`
}

// JumpHash Gets a string key and  int bucket and returns the
// bucket number
func JumpHash(key string, b int) int32 {
	dstPath := xxhash.Sum64String(key)
	// fmt.Println("XXHASH :", dstPath)
	bucket := jump.Hash(dstPath, b)
	return bucket
}

// WebApp Main web app
type WebApp struct {
	addr      string
	r         *chi.Mux
	render    *render.Render
	rateLimit int
	// redis      *store.Redis
	dbs        map[string]*sqlx.DB
	namespaces []string
}

// Run main runner
func (wa *WebApp) Run() {
	wa.r.Use(middleware.RequestID)
	wa.r.Use(middleware.RealIP)
	wa.r.Use(middleware.Recoverer)
	wa.r.Use(middleware.Logger)
	wa.r.Use(httprate.LimitByIP(wa.rateLimit, 1*time.Minute))
	wa.r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("welcome"))
	})
	wa.r.Put("/{ns}/{data}", wa.WriteData)
	log.Println("Running web mode on: ", wa.addr)
	http.ListenAndServe(wa.addr, wa.r)
}

type Data struct {
	DataID   int            `db:"data_id"`
	Data     []byte         `db:"data"`
	GroupBy  sql.NullString `db:"group_by"`
	Checksum string         `db:"checksum"`
}

func (wa *WebApp) InsertData(ctx context.Context, key, ns string, data []byte) {
	// wa.dbs[ns].Exec("INSERT INTO data(data_id, data, )")
	wa.dbs[ns].MustExecContext(ctx, "INSERT INTO data (data_id, data) VALUES ($1, $2)", key, data)
}

// WriteData
// Is in charge of assign a bucket for the data sent,
// and send byte data to the actual bucket
func (wa *WebApp) WriteData(w http.ResponseWriter, r *http.Request) {

	dataPath := chi.URLParam(r, "data")
	ns := chi.URLParam(r, "ns")

	// bucket := JumpHash(dataPath, wa.buckets)
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("request", err)
	}

	wa.InsertData(r.Context(), dataPath, ns, buf)

	fmt.Println(buf) // do whatever you want with the binary file buf
	wa.render.JSON(w, http.StatusCreated, &PutDataRSP{
		Namespace: ns,
		Path:      dataPath,
		// Bucket: bucket,
	})
}
