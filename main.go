package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/algorinfo/rawstore/pkg/brain"
	"github.com/algorinfo/rawstore/pkg/store"
	"github.com/algorinfo/rawstore/pkg/volume"
)

// Env get a environment variable adding a defaultValue
func Env(key, defaultValue string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	return val
}

var (
	rateLimit    = Env("RD_RATE_LIMIT", "1000")
	listenAddr   = Env("RD_LISTEN_ADDR", ":6667")
	nsDir        = Env("RD_NS_DIR", "data/")
	redisAddr    = Env("RD_REDIS_ADDR", "localhost:6379")
	redisPass    = Env("RD_REDIS_PASS", "")
	redisDB      = Env("RD_REDIS_DB", "0")
	streamNo     = Env("RD_STREAM", "false")
	eStreamLimit = Env("RD_STREAM_LIMIT", "1000")
	streamNS     = Env("RD_STREAM_NS", "RD")
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	streamB, _ := strconv.ParseBool(streamNo)

	// commands
	brainCmd := flag.NewFlagSet("brain", flag.ExitOnError)
	volumeCmd := flag.NewFlagSet("volume", flag.ExitOnError)

	// Params
	listen := brainCmd.String("listen", ":6665", "Address to listen")
	listenV := volumeCmd.String("listen", listenAddr, "Address to listen")
	pnsDir := volumeCmd.String("namespace", nsDir, "Namespace dir")
	stream := volumeCmd.Bool("stream", streamB, "Stream data added")
	streamLimit := volumeCmd.String("stream-limit", eStreamLimit, "How many message by stream")
	streamNSC := volumeCmd.String("stream-ns", streamNS, "Which namespace use for redis")

	flag.Parse()
	if len(os.Args) < 2 {
		fmt.Println("Command Error: `brain` is required")
	}

	switch os.Args[1] {
	case "brain":
		err := brainCmd.Parse(os.Args[2:])
		redis := store.NewRedis(store.WithDefaults())
		if err != nil {
			log.Fatal("Error parsing args")
		}
		brain := brain.New(
			brain.WithAddr(*listen),
			brain.WithRedis(redis),
			brain.WithVolumes(
				[]string{"localhost:888", "localhost:999"},
			),
		)
		brain.Run()

	case "volume":
		err := volumeCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatal("Error parsing args")
		}

		rt, _ := strconv.Atoi(rateLimit)

		cfg := &volume.Config{
			Addr:      *listenV,
			RateLimit: rt,
			NSDir:     *pnsDir,
		}

		// store.UseDB()

		if *stream {
			intDb, _ := strconv.Atoi(redisDB)
			maxLen, _ := strconv.ParseInt(*streamLimit, 10, 64)
			p := store.NewProducer(store.WithRedis(
				&store.Redis{Conn: &store.Connection{
					Addr:     redisAddr,
					Password: redisPass,
					DB:       intDb,
				},
				},
			),
				store.WithMaxLen(maxLen),
			)
			p.Namespace = *streamNSC
			p.RDB.Connect()
			vol := volume.New(
				volume.WithConfig(cfg),
				volume.WithProducer(p),
			)
			vol.Run()
		} else {
			vol := volume.New(
				volume.WithConfig(cfg),
			)
			vol.Run()
		}

	default:
		fmt.Printf("Please use 'web' or 'volume' command")
	}

}
