package main

import (
	"log/slog"

	"github.com/google/uuid"
	"github.com/picosh/pgs"
	"github.com/picosh/pgs/db"
	"github.com/picosh/pgs/db/memory"
	"github.com/picosh/pgs/minimal"
	"github.com/picosh/pgs/storage"
)

func main() {
	logger := slog.Default()
	// connect to database -- in this case we use in-memory
	// we just need the struct to adhere to the `db.DB` interface
	dbpool := memory.NewDBMemory(logger)
	// setting up test data so it's easy to get running
	dbpool.SetupTestData()
	// add a public key so we can connect to ssh server
	addPubkey(dbpool)

	// connect to object storage -- in this case we use in-memory
	// we just need the struct to adhere to the `storage.StorageServ` interface
	st, err := storage.NewStorageMemory(map[string]map[string]string{})
	if err != nil {
		panic(err)
	}
	// central configuration and also carries db, obj store, and logger
	cfg := pgs.NewConfigSite(logger, dbpool, st)

	// noop
	ch := make(chan error)
	// start web server
	go minimal.StartMinimalWebServer(cfg)
	// start ssh server
	minimal.StartMinimalSshServer(cfg, ch)
}

func addPubkey(dbpool *memory.MemoryDB) {
	pk := &db.PublicKey{
		ID:     uuid.NewString(),
		UserID: dbpool.Users[0].ID,
		Name:   "main",
		Key:    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHRRo6TbaxWiynXhSGiHAdM7ZQ1rGcZ8DEBOskE6l7vs",
	}
	dbpool.Pubkeys = append(dbpool.Pubkeys, pk)
}
