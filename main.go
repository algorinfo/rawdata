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
	rateLimit  = Env("RATE_LIMIT", "100")
	redisAddr  = Env("REDIS", "localhost:6379")
	listenAddr = Env("LISTEN_ADDR", ":6667")
	nsDir      = Env("NS_DIR", "data/")
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// commands
	brainCmd := flag.NewFlagSet("brain", flag.ExitOnError)
	volumeCmd := flag.NewFlagSet("volume", flag.ExitOnError)

	// Params
	listen := brainCmd.String("listen", ":6665", "Address to listen")
	listenV := volumeCmd.String("listen", ":6667", "Address to listen")

	flag.Parse()
	if len(os.Args) < 2 {
		fmt.Println("Command Error: `brain` is required")
	}

	switch os.Args[1] {
	case "brain":
		err := brainCmd.Parse(os.Args[2:])
		redis := store.NewRedis(&store.RedisOptions{
			Addr: redisAddr,
		})
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

		cfg := volume.DefaultConfig()
		cfg.Addr = *listenV
		cfg.RateLimit = rt
		cfg.NSDir = nsDir

		// store.UseDB()
		vol := volume.New(
			volume.WithConfig(cfg),
		)

		vol.Run()

	default:
		fmt.Printf("Please use 'web' or 'volume' command")
	}

}
