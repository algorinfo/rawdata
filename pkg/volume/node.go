package volume

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
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
	wa.r.Put("/{ns}/{data}", wa.PutData)
	wa.r.Get("/{ns}/{data}", wa.GetOneData)
	wa.r.Delete("/{ns}/{data}", wa.DelOneData)
	wa.r.Get("/{ns}/", wa.GetAllData)
	log.Println("Running web mode on: ", wa.addr)
	http.ListenAndServe(wa.addr, wa.r)
}

type Data struct {
	DataID    string         `db:"data_id"`
	Data      []byte         `db:"data"`
	GroupBy   sql.NullString `db:"group_by"`
	Checksum  sql.NullString `db:"checksum"`
	CreatedAt string         `db:"created_at"`
}

func (wa *WebApp) InsertData(ctx context.Context, key, ns string, data []byte) {
	// wa.dbs[ns].Exec("INSERT INTO data(data_id, data, )")
	wa.dbs[ns].ExecContext(ctx, "INSERT INTO data (data_id, data) VALUES ($1, $2)", key, data)
}

// WriteData
// Is in charge of assign a bucket for the data sent,
// and send byte data to the actual bucket
func (wa *WebApp) PutData(w http.ResponseWriter, r *http.Request) {

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

func (wa *WebApp) GetOneData(w http.ResponseWriter, r *http.Request) {

	dataPath := chi.URLParam(r, "data")
	ns := chi.URLParam(r, "ns")

	oneData := Data{}
	err := wa.dbs[ns].Get(&oneData, "SELECT * FROM data where data_id = ?", dataPath)
	if err != nil {
		wa.render.JSON(w, http.StatusNotFound, map[string]string{"error": "Data not found"})
		return
	}

	// wa.render.JSON(w, http.StatusOK, oneData)
	w.Write(oneData.Data)
}

func (wa *WebApp) DelOneData(w http.ResponseWriter, r *http.Request) {

	dataPath := chi.URLParam(r, "data")
	ns := chi.URLParam(r, "ns")

	_, err := wa.dbs[ns].ExecContext(r.Context(), "DELETE FROM data where data_id  = ?", dataPath)
	if err != nil {
		wa.render.JSON(w, http.StatusInternalServerError, map[string]string{"error": "Cannot delete data"})
		return
	}

	wa.render.JSON(w, http.StatusOK, map[string]string{"msg": "ok"})
}

type AllData struct {
	Rows  []Data `json:"rows"`
	Next  int    `json:"next"`
	Total int    `json:"total"`
}

func (wa *WebApp) GetAllData(w http.ResponseWriter, r *http.Request) {

	page := 1
	sp := r.URL.Query().Get("page")
	if sp != "" {
		pg, err := strconv.Atoi(sp)
		if err != nil {
			wa.render.JSON(w, http.StatusInternalServerError, map[string]string{"error": "bad param"})
			return
		}
		page = pg
	}
	limit := 2
	offset := limit * (page - 1)
	fmt.Println(offset)
	ns := chi.URLParam(r, "ns")

	ad := []Data{}
	var total int
	row := wa.dbs[ns].QueryRow("SELECT count(*) FROM data;")
	_ = row.Scan(&total)

	nextPage := page + 1
	nextOffset := limit * page
	if nextOffset >= total {
		nextPage = -1
	}

	err := wa.dbs[ns].Select(&ad, "SELECT * FROM data LIMIT ? OFFSET ?;", limit, offset)
	// err := wa.dbs[ns].Select(&ad, "SELECT * FROM data")
	if err != nil {
		fmt.Println("Error geting value ", err)
		wa.render.JSON(w, http.StatusInternalServerError, map[string]string{"error": "Cannot get data"})
		return

	}

	// err = wa.render.JSON(w, http.StatusOK, map[string][]Data{"rows": ad})
	err = wa.render.JSON(w, http.StatusOK, &AllData{Rows: ad, Next: nextPage, Total: total})
	if err != nil {
		fmt.Println(err)
	}
	// wa.render.JSON(w, http.StatusOK, ad)
}
