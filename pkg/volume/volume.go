package volume

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/algorinfo/rawstore/pkg/jump"
	"github.com/algorinfo/rawstore/pkg/store"
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
	GroupBy string `json:"groupBy,omitempty"` // to be deprecated
	Bucket  int32  `json:"bucket,omitempty"`  // to be deprecated
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

/* Config
Main config it has a rateLimit
Addr: Full address to listen to ":6667" by default
RateLimit: how many rq per ip per minute
NSDir: namespace dir where files will be stored
*/
type Config struct {
	Addr      string
	RateLimit int
	NSDir     string
}

// WebApp Main web app
type WebApp struct {
	r      *chi.Mux
	render *render.Render
	// redis      *store.Redis
	dbs        map[string]*sqlx.DB
	namespaces []string
	cfg        *Config
}

// Run main runner
func (wa *WebApp) Run() {
	wa.r.Use(middleware.RequestID)
	wa.r.Use(middleware.RealIP)
	wa.r.Use(middleware.Recoverer)
	wa.r.Use(middleware.Logger)
	wa.r.Use(httprate.LimitByIP(wa.cfg.RateLimit, 1*time.Minute))
	wa.r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("welcome"))
	})
	wa.r.Route("/v1", func(r chi.Router) {
		r.Get("/namespace", wa.AllNS)
		r.Get("/namespace/{ns}/_backup", wa.NSBackup)
		r.Post("/namespace", wa.CreateNS)
	})

	workDir, _ := os.Getwd()
	filesDir := http.Dir(filepath.Join(workDir, wa.cfg.NSDir))
	FileServer(wa.r, "/files", filesDir)

	wa.r.Put("/{ns}/{data}", wa.PutData)
	wa.r.Get("/{ns}/{data}", wa.GetOneData)
	wa.r.Delete("/{ns}/{data}", wa.DelOneData)
	wa.r.Get("/{ns}/", wa.GetAllData)
	log.Println("Running web mode on: ", wa.cfg.Addr)
	http.ListenAndServe(wa.cfg.Addr, wa.r)
}

/* DataModel
Main data model in the sqlite store
each namespace will share the same model
*/
type DataModel struct {
	DataID string `db:"data_id"`
	Data   []byte `db:"data"`
	// GroupBy   sql.NullString `db:"group_by"`
	// Checksum  sql.NullString `db:"checksum"`
	CreatedAt string `db:"created_at"`
}

/* Namespace
Right now is a thin wrapper. In the future
it could have other annotations.
*/
type Namespace struct {
	Name string `json:"name"`
}

// NSBackup, endpoint to start a backup in place of a namespace
func (wa *WebApp) NSBackup(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "ns")
	fullSrc := fmt.Sprintf("%s%s.db", wa.cfg.NSDir, ns)
	fullDst := fmt.Sprintf("%s%s.backup.db", wa.cfg.NSDir, ns)
	store.Backup(fullSrc, fullDst)

	wa.render.JSON(w, http.StatusOK, map[string]string{"ok": "done"})

}

// CreateNS, endpoint which creates a new namespace
func (wa *WebApp) CreateNS(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var ns Namespace
	err = json.Unmarshal(b, &ns)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if _, ok := wa.dbs[ns.Name]; ok {
		wa.render.JSON(w, http.StatusOK, &wa.namespaces)
		return
	}
	CreateNS(wa, ns.Name)

	wa.render.JSON(w, http.StatusCreated, &wa.namespaces)

}

// AllNS list all namespaces
func (wa *WebApp) AllNS(w http.ResponseWriter, r *http.Request) {

	wa.render.JSON(w, http.StatusOK, &wa.namespaces)
}

// InsertData insert data in the store
func (wa *WebApp) InsertData(ctx context.Context, key, ns string, data []byte) error {
	// wa.dbs[ns].Exec("INSERT INTO data(data_id, data, )")
	_, err := wa.dbs[ns].ExecContext(ctx, "INSERT INTO data (data_id, data) VALUES ($1, $2)", key, data)
	if err != nil {
		return err
	}
	return nil
}

// WriteData
// Is in charge of assign a bucket for the data sent,
// and send byte data to the actual bucket
func (wa *WebApp) PutData(w http.ResponseWriter, r *http.Request) {

	dataPath := chi.URLParam(r, "data")
	ns := chi.URLParam(r, "ns")

	var zdata bytes.Buffer
	zw := zlib.NewWriter(&zdata)

	// bucket := JumpHash(dataPath, wa.buckets)
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		wa.render.JSON(w, http.StatusInternalServerError,
			map[string]string{"error": fmt.Sprintf("%s", err)})
		return

	}
	zw.Write(buf)
	zw.Close()

	err = wa.InsertData(r.Context(), dataPath, ns, zdata.Bytes())
	if err != nil {
		wa.render.JSON(w, http.StatusInternalServerError,
			map[string]string{"error": fmt.Sprintf("%s", err)})
		return

	}

	wa.render.JSON(w, http.StatusCreated, &PutDataRSP{
		Namespace: ns,
		Path:      dataPath,
		// Bucket: bucket,
	})
}

func (wa *WebApp) GetOneData(w http.ResponseWriter, r *http.Request) {

	dataPath := chi.URLParam(r, "data")
	ns := chi.URLParam(r, "ns")

	oneData := DataModel{}
	err := wa.dbs[ns].Get(&oneData, "SELECT * FROM data where data_id = ?", dataPath)
	if err != nil {
		wa.render.JSON(w, http.StatusNotFound, map[string]string{"error": "Data not found"})
		return
	}

	//buff := []byte{120, 156, 202, 72, 205, 201, 201, 215, 81, 40, 207,
	//	47, 202, 73, 225, 2, 4, 0, 0, 255, 255, 33, 231, 4, 147}

	rb := bytes.NewReader(oneData.Data)
	zr, err := zlib.NewReader(rb)
	if err != nil {
		wa.render.JSON(w, http.StatusInternalServerError,
			map[string]string{"error": fmt.Sprintf("%s", err)})
		return
	}
	data, _ := ioutil.ReadAll(zr)

	w.Write(data)
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
	Rows  []DataModel `json:"rows"`
	Next  int         `json:"next"`
	Total int         `json:"total"`
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
	ns := chi.URLParam(r, "ns")

	ad := []DataModel{}
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
