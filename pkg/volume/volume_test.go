package volume

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/algorinfo/rawstore/pkg/store"
	"github.com/stretchr/testify/assert"
)

func init() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

/*func TestCFG(t *testing.T) {
	cfg := DefaultConfig()
	addr := cfg.Addr
	vol := New(WithConfig(cfg))
	assert.Equal(t, vol.cfg.Addr, addr)
}*/

func TestLoadNS(t *testing.T) {

	//log.Println(os.Getwd())
	dirName := "./temp"
	os.Mkdir(dirName, 0755)
	defer os.RemoveAll(dirName)

	cfg := DefaultConfig()
	cfg.NSDir = dirName
	log.Println("DirName...:", dirName)

	fn := fmt.Sprintf("%s/%s", dirName, "test")
	store.CreateDB(fn)

	vol := New(WithConfig(cfg))
	LoadNS(vol)
	assert.Equal(t, len(vol.namespaces), 2)
	assert.Equal(t, vol.namespaces[1], "test.db")
}
