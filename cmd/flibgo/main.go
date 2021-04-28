package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vinser/flibgo/pkg/config"
	"github.com/vinser/flibgo/pkg/database"
	"github.com/vinser/flibgo/pkg/genres"
	"github.com/vinser/flibgo/pkg/opds"
	"github.com/vinser/flibgo/pkg/stock"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func main() {
	const configFile = "config/config.yml"

	cfg := config.GetConfig(configFile)
	db := database.NewDB(cfg.Database.DSN)
	defer db.Close()

	gt := genres.NewGenresTree(cfg.Genres.TREE_FILE)

	reindex := flag.Bool("reindex", false, "empty catalog database and then scan book stock directory to add new books to catalog")
	flag.Parse()
	lScan := config.NewLog(cfg.Logs.SCAN, cfg.Logs.DEBUG)
	defer lScan.File.Close()
	hScan := &stock.Handler{
		CFG: cfg,
		DB:  db,
		GT:  gt,
		LOG: lScan,
	}
	if *reindex {
		hScan.Do(stock.Init | stock.Scan)
		return
	}

	lOPDS := config.NewLog(cfg.Logs.OPDS, cfg.Logs.DEBUG)
	defer lOPDS.File.Close()
	config.LoadLocales()
	langTag := language.Make(cfg.Language.DEFAULT)
	hOPDS := &opds.Handler{
		CFG: cfg,
		DB:  db,
		GT:  gt,
		P:   message.NewPrinter(langTag),
		LOG: lOPDS,
	}

	if !db.IsReady() {
		db.InitDB(cfg.Database.INIT_SCRIPT)
		f := "Catalog was inited. Tables were created in empty database"
		lOPDS.I.Println(f)
		return
	}
	portString := fmt.Sprint(":", cfg.OPDS.PORT)
	server := &http.Server{
		Addr:    portString,
		Handler: hOPDS,
	}
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	f := "server on http://localhost%s is listening\n"
	lOPDS.I.Printf(f, portString)
	log.Printf(f, portString)

	<-done
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		// extra handling here
		cancel()
	}()

	if err := server.Shutdown(ctx); err != nil {
		f := "shutdown error: %v\n"
		lOPDS.E.Printf(f, err)
		log.Fatalf(f, err)
	}
	f = "server on http://localhost%s was shut down successfully\n"
	lOPDS.I.Printf(f, portString)
	log.Printf(f, portString)
}
