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
	"strings"

	"github.com/algorinfo/rawstore/pkg/jump"
	"github.com/algorinfo/rawstore/pkg/store"
	"github.com/cespare/xxhash"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/docgen"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/unrolled/render"
)

// PutDataRSP main entity to receive raw data
type PutDataRSP struct {
	Namespace string `json:"namespace"`
	Path      string `json:"path"`
	GroupBy   string `json:"groupBy,omitempty"` // to be deprecated
	Bucket    int32  `json:"bucket,omitempty"`  // to be deprecated
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

/*Config Main config it has a rateLimit
Addr: Full address to listen to ":6667" by default
RateLimit: how many rq per ip per minute
NSDir: namespace dir where files will be stored
*/
type Config struct {
	Addr   string
	NSDir  string
	Stream bool
	/*RedisAddress string
	RedisPass    string
	RedisDB      int*/
}

// WebApp Main web app
type WebApp struct {
	r      *chi.Mux
	render *render.Render
	// redis      *store.Redis
	dbs        map[string]*sqlx.DB
	namespaces []string
	cfg        *Config
	producer   *store.Producer
}

// RegisterRoutes Register routes for the router and docs
func (wa *WebApp) RegisterRoutes() {
	wa.r.Use(middleware.RequestID)
	wa.r.Use(middleware.RealIP)
	wa.r.Use(middleware.Recoverer)
	wa.r.Use(middleware.Logger)
	// wa.r.Use(httprate.LimitByIP(wa.cfg.RateLimit, 1*time.Minute))
	wa.r.Get("/status", wa.Status)
	wa.r.Route("/v1", func(r chi.Router) {
		r.Get("/namespace", wa.AllNS)
		r.Get("/namespace/{ns}/_backup", wa.NSBackup)
		r.Post("/namespace", wa.CreateNS)
		r.Get("/data/{ns}/_list", wa.GetIDData)
		r.Get("/data/{ns}", wa.GetAllData)
	})

	workDir, _ := os.Getwd()
	filesDir := http.Dir(filepath.Join(workDir, wa.cfg.NSDir))
	FileServer(wa.r, "/files", filesDir)

	wa.r.Put("/{ns}/{data}", wa.PutData)
	wa.r.Post("/{ns}/{data}", wa.PostData)
	wa.r.Get("/{ns}/{data}", wa.GetOneData)
	wa.r.Delete("/{ns}/{data}", wa.DelOneData)
	wa.r.Get("/{ns}", wa.GetAllData)
	log.Println("Running web mode on: ", wa.cfg.Addr)
	// http.ListenAndServe(wa.cfg.Addr, wa.r)
}

// Run run main worker
func (wa *WebApp) Run() {
	http.ListenAndServe(wa.cfg.Addr, wa.r)
}

/*Docs generation for chi
Sparsed information from:
- https://github.com/go-chi/docgen/issues/8
- https://github.com/go-chi/docgen/blob/master/funcinfo.go
- https://github.com/go-chi/chi/blob/master/_examples/router-walk/main.go

*/
func (wa *WebApp) Docs() {

	/*
		fmt.Println(docgen.MarkdownRoutesDoc(wa.r, docgen.MarkdownOpts{
			ProjectPath: "github.com/go-chi/chi/v5",
			Intro:       "Welcome to the chi/_examples/rest generated docs.",
		}))*/

	walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		route = strings.Replace(route, "/*/", "/", -1)
		handlerInfo := docgen.GetFuncInfo(handler)
		fmt.Printf("%s %s %s\n", method, route, handlerInfo.Comment)
		return nil
	}

	if err := chi.Walk(wa.r, walkFunc); err != nil {
		fmt.Printf("Logging err: %s\n", err.Error())
	}

}

/*DataModel Main data model in the sqlite store
each namespace will share the same model
*/
type DataModel struct {
	DataID string `db:"data_id" json:"dataID"`
	Data   []byte `db:"data" json:"data"`
	// GroupBy   sql.NullString `db:"group_by"`
	// Checksum  sql.NullString `db:"checksum"`
	CreatedAt string `db:"created_at" json:"createdAt"`
}

/*Namespace Right now is a thin wrapper. In the future
it could have other annotations.
*/
type Namespace struct {
	Name        string `json:"name"`
	Stream      bool   `json:"stream,omitempty"`
	StreamLimit int    `json:"stream_limit,omitempty"`
}

type StatusResponse struct {
	Stream         bool     `json:"stream"`
	StreamLimit    int64    `json:"streamLimit,string"`
	RedisNamespace string   `json:"redisNamespace"`
	Namespaces     []string `json:"namespaces"`
}

// NSBackup, endpoint to start a backup in place of a namespace
func (wa *WebApp) Status(w http.ResponseWriter, r *http.Request) {
	var stream bool
	if wa.producer != nil {
		stream = true
	}
	sr := &StatusResponse{
		Stream:         stream,
		StreamLimit:    wa.producer.MaxLenApprox,
		RedisNamespace: wa.producer.Namespace,
		Namespaces:     wa.namespaces,
	}

	wa.render.JSON(w, http.StatusOK, sr)

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
	CreateNS(wa, dataSchemaV1, ns.Name)

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

// UpsertData insert data in the store
func (wa *WebApp) UpsertData(ctx context.Context, key, ns string, data []byte) error {
	// wa.dbs[ns].Exec("INSERT INTO data(data_id, data, )")
	_, err := wa.dbs[ns].ExecContext(ctx, "INSERT INTO data (data_id, data) VALUES ($1, $2) ON CONFLICT(data_id) DO UPDATE SET data=$2", key, data)
	if err != nil {
		return err
	}
	return nil
}

// PostData Write data to the sqlite file
// If the path already exist will fail
func (wa *WebApp) PostData(w http.ResponseWriter, r *http.Request) {

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

	if wa.producer != nil {
		nsStream := fmt.Sprintf("RD.%s", ns)
		wa.producer.SendTo(r.Context(), nsStream, map[string]interface{}{
			"namespace": ns,
			"path":      dataPath,
		})

	}

	wa.render.JSON(w, http.StatusCreated, &PutDataRSP{
		Namespace: ns,
		Path:      dataPath,
		// Bucket: bucket,
	})
}

// PutData Write data to the sqlite store
// If the path already exist, will replace the data.
// TODO: date will remain as origin.
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

	err = wa.UpsertData(r.Context(), dataPath, ns, zdata.Bytes())
	if err != nil {
		wa.render.JSON(w, http.StatusInternalServerError,
			map[string]string{"error": fmt.Sprintf("%s", err)})
		return

	}

	if wa.producer != nil {
		nsStream := fmt.Sprintf("%s.%s", wa.producer.Namespace, ns)
		wa.producer.SendTo(r.Context(), nsStream, map[string]interface{}{
			"namespace": ns,
			"path":      dataPath,
		})

	}

	wa.render.JSON(w, http.StatusCreated, &PutDataRSP{
		Namespace: ns,
		Path:      dataPath,
		// Bucket: bucket,
	})
}

// GetOneData get one element
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

func getStringQueryParam(value *string, r *http.Request, key string) error {
	sp := r.URL.Query().Get(key)
	if sp != "" {
		*value = sp
	}

	return nil
}

func getNumberQueryParam(value *int, r *http.Request, key string) error {
	sp := r.URL.Query().Get(key)
	if sp != "" {
		pg, err := strconv.Atoi(sp)
		if err != nil {
			return err
		}
		*value = pg
		return nil
	}

	return nil
}

// GetAllData Returns data with base64 encoding and uncompressed
func (wa *WebApp) GetAllData(w http.ResponseWriter, r *http.Request) {

	page := 1
	limit := 50

	err1 := getNumberQueryParam(&page, r, "page")
	err2 := getNumberQueryParam(&limit, r, "limit")
	if err1 != nil || err2 != nil {
		wa.render.JSON(w, http.StatusInternalServerError,
			map[string]string{"error": "bad param"})
		return

	}

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

type DataID struct {
	DataID    string `db:"data_id" json:"dataID"`
	CreatedAt string `db:"created_at" json:"createdAt"`
}

type DataIDResponse struct {
	Rows  []DataID `json:"rows"`
	Total int      `json:"total"`
	Next  int      `json:"next"`
}

func (wa *WebApp) GetIDData(w http.ResponseWriter, r *http.Request) {

	page := 1
	limit := 50

	err1 := getNumberQueryParam(&page, r, "page")
	err2 := getNumberQueryParam(&limit, r, "limit")
	if err1 != nil || err2 != nil {
		wa.render.JSON(w, http.StatusInternalServerError,
			map[string]string{"error": "bad param"})
		return

	}

	offset := limit * (page - 1)
	ns := chi.URLParam(r, "ns")

	ad := []DataID{}
	var total int
	row := wa.dbs[ns].QueryRow("SELECT count(*) FROM data;")
	_ = row.Scan(&total)

	nextPage := page + 1
	nextOffset := limit * page
	if nextOffset >= total {
		nextPage = -1
	}

	err := wa.dbs[ns].Select(&ad, "SELECT data_id, created_at FROM data ORDER BY created_at desc LIMIT ? OFFSET ?;", limit, offset)
	// err := wa.dbs[ns].Select(&ad, "SELECT * FROM data")
	if err != nil {
		fmt.Println("Error geting value ", err)
		wa.render.JSON(w, http.StatusInternalServerError, map[string]string{"error": "Cannot get data"})
		return

	}

	// err = wa.render.JSON(w, http.StatusOK, map[string][]Data{"rows": ad})
	wa.render.JSON(w,
		http.StatusOK,
		&DataIDResponse{Rows: ad, Next: nextPage, Total: total},
	)
}
