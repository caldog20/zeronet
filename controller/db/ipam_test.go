package db

import (
	"fmt"
	"log"
	"net/netip"
	"os"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/caldog20/zeronet/controller/types"
)

var gdb *gorm.DB

func GetDB() *Store {
	return &Store{
		db: gdb.Begin(),
	}
}

func TestMain(m *testing.M) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_journal_mode=WAL"),
		&gorm.Config{PrepareStmt: true},
	)

	if err != nil {
		log.Fatal(err)
	}

	db.AutoMigrate(&types.Peer{})

	gdb = db

	os.Exit(m.Run())
}

func createTestPeers(db *gorm.DB, count int) {
	peers := []*types.Peer{}

	for i := range count {
		peer := &types.Peer{
			MachineID:      uuid.New().String(),
			NoisePublicKey: "asdkflj3j2klj3r2312ea",
			Prefix:         "100.70.0.0/24",
			IP:             fmt.Sprintf("100.70.0.%d", i+1),
		}
		peers = append(peers, peer)
	}

	err := db.Create(peers).Error
	if err != nil {
		log.Fatal(err)
	}
}

func Test_GetAllocatedIPs(t *testing.T) {
	store := GetDB()

	defer store.db.Rollback()
	createTestPeers(store.db, 5)

	ips, err := store.GetAllocatedIPs()
	assert.Nil(t, err)
	assert.NotNil(t, ips)
	assert.Equal(t, len(ips), 5)
}

func Test_AllocateIPFirstPeer(t *testing.T) {
	store := GetDB()
	defer store.db.Rollback()
	ip, err := store.AllocatePeerIP(netip.MustParsePrefix("100.70.0.0/24"))
	assert.Nil(t, err)
	assert.EqualValues(t, ip, "100.70.0.1")
}

func Test_AllocateIPSecondPeer(t *testing.T) {
	store := GetDB()
	defer store.db.Rollback()
	createTestPeers(store.db, 1)
	ip, err := store.AllocatePeerIP(netip.MustParsePrefix("100.70.0.0/24"))
	assert.Nil(t, err)
	assert.EqualValues(t, ip, "100.70.0.2")
}

func Test_AllocateIPLastPeer(t *testing.T) {
	store := GetDB()
	defer store.db.Rollback()
	createTestPeers(store.db, 253)
	ip, err := store.AllocatePeerIP(netip.MustParsePrefix("100.70.0.0/24"))
	assert.Nil(t, err)
	assert.EqualValues(t, ip, "100.70.0.254")
}
