package brain

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/algorinfo/rawstore/pkg/jump"
	"github.com/algorinfo/rawstore/pkg/store"
	"github.com/cespare/xxhash"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/unrolled/render"
)

var (
	rateLimit = "10"
)

// EchoRSP response for /get
type EchoRSP struct {
	Agent   string      `json:"user-agent"`
	Addr    string      `json:"address"`
	Headers interface{} `json:"headers"`
}

type PutDataRSP struct {
	Namespace string `json:"namespace"`
	Path      string `json:"path"`
	// HashKey   string  `json:"hashKey"`
	GroupBy string  `json:"groupBy,omitempty"`
	Volume  *Volume `json:"volume"`
}

type Volume struct {
	Addr     string `json:"address"`
	JumpNode int32  `json:"jumpNode"`
}

// WebApp Main web app
type WebApp struct {
	addr      string
	r         *chi.Mux
	render    *render.Render
	rateLimit int
	redis     *store.Redis
	volumes   []string
}

// JumpHash Gets a string key and  int bucket and returns the
// bucket number
func JumpHash(key string, b int) int32 {
	dstPath := xxhash.Sum64String(key)
	// fmt.Println("XXHASH :", dstPath)
	bucket := jump.Hash(dstPath, b)
	return bucket
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
	wa.r.Get("/auth", wa.simpleEcho)
	wa.r.Put("/{ns}/{data}", wa.WriteData)
	wa.r.Get("/{ns}/{data}", wa.RedirectData)
	log.Println("Running web mode on: ", wa.addr)
	http.ListenAndServe(wa.addr, wa.r)
}

// WriteData
// Is in charge of assign a bucket for the data sent,
// and send byte data to the actual bucket
func (wa *WebApp) WriteData(w http.ResponseWriter, r *http.Request) {

	dataPath := chi.URLParam(r, "data")
	ns := chi.URLParam(r, "ns")

	bucket := JumpHash(dataPath, len(wa.volumes))
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("request", err)
	}
	fmt.Println(buf) // do whatever you want with the binary file buf
	wa.render.JSON(w, http.StatusCreated, &PutDataRSP{
		Namespace: ns,
		Path:      dataPath,
		// HashKey:   strconv.FormatUint(dstPath, 10),
		Volume: &Volume{Addr: wa.volumes[bucket], JumpNode: bucket},
	})
}

// RedirectData a GET method wich return in which volume is the data
func (wa *WebApp) GetNodes(w http.ResponseWriter, r *http.Request) {
	wa.render.JSON(w, http.StatusOK, wa.volumes)
}

// RedirectData a GET method wich return in which volume is the data
func (wa *WebApp) RedirectData(w http.ResponseWriter, r *http.Request) {

	dataPath := chi.URLParam(r, "data")
	ns := chi.URLParam(r, "ns")
	bucket := JumpHash(dataPath, len(wa.volumes))

	fullURL := fmt.Sprintf("%s/%s/%s", wa.volumes[bucket], ns, dataPath)

	http.Redirect(w, r, fullURL, http.StatusMovedPermanently) // 301
}

func (wa *WebApp) simpleEcho(w http.ResponseWriter, r *http.Request) {
	// fmt.Println(r.Header)
	wa.render.JSON(w, http.StatusOK, &EchoRSP{
		Agent:   r.UserAgent(),
		Addr:    r.RemoteAddr,
		Headers: r.Header,
	})

}

/*func (wa *WebApp) AddVolume(w http.ResponseWriter, r *http.Request) {
	redis := wa.redis.GetInstance()
	srvKey := fmt.Sprintf("srv.%s", "test")
	err := redis.Set(r.Context(), srvKey,

}*/
