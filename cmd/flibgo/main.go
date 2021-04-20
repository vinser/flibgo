package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/vinser/flibgo/pkg/config"
	"github.com/vinser/flibgo/pkg/database"
	"github.com/vinser/flibgo/pkg/genres"
	"github.com/vinser/flibgo/pkg/opds"
	"github.com/vinser/flibgo/pkg/stock"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func main() {

	opdsCmd := flag.NewFlagSet("opds", flag.ExitOnError)
	opdsConfig := opdsCmd.String("config", "config/config.yml", "configuration file")
	// opdsConfig := opdsCmd.String("config", "../config/config.yml", "config file")

	dbCmd := flag.NewFlagSet("database", flag.ExitOnError)
	dbConfig := dbCmd.String("config", "config/config.yml", "config file")
	dbScan := dbCmd.Bool("scan", false, "scan book stock directory and add new books to database")
	dbInit := dbCmd.Bool("init", false, "empty book database without scanning book stock directory")

	if len(os.Args) < 2 {
		startOPDS(*opdsConfig)
		// startOPDS("config/config.yml")
		// startOPDS("../config/config.yml") //for debug
	} else {
		switch os.Args[1] {
		case "opds":
			opdsCmd.Parse(os.Args[2:])
			startOPDS(*opdsConfig)
		case "db":
			dbCmd.Parse(os.Args[2:])
			maintainDB(*dbConfig, *dbScan, *dbInit)
		default:
			fmt.Println("expected 'opds' or 'db' subcommands")
			os.Exit(1)
		}
	}
}

func startOPDS(configFile string) {
	cfg := config.GetConfig(configFile)
	db := database.NewDB(cfg.Database.DSN)
	defer db.Close()

	gt := genres.NewGenresTree(cfg.Genres.TREE_FILE)
	lg := config.NewLog(cfg.Logs.OPDS, cfg.Logs.DEBUG)
	defer lg.File.Close()
	langTag := language.Make(cfg.Language.DEFAULT)
	handler := &opds.Handler{
		CFG: cfg,
		DB:  db,
		GT:  gt,
		P:   message.NewPrinter(langTag),
		LOG: lg,
	}
	config.LoadLocales()

	if !db.IsReady() {
		db.InitDB(cfg.Database.INIT_SCRIPT)
		lg.I.Println("Empty database was inited. You can add new books by flibgo db -scan")
		return
	}

	portString := fmt.Sprint(":", cfg.OPDS.PORT)
	lg.I.Printf("server on http://localhost%s is listening\n", portString)
	log.Printf("server on http://localhost%s is listening\n", portString)

	// err := http.ListenAndServe(portString, handler)

	server := &http.Server{Addr: portString, Handler: handler}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop
		err := server.Shutdown(context.Background())
		if err != nil {
			lg.E.Printf("error during shutdown: %v\n", err)
			log.Printf("error during shutdown: %v\n", err)
		}
		wg.Done()
	}()

	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		log.Println("commencing server shutdown...")
		wg.Wait()
		lg.I.Printf("server on http://localhost%s was shut down successfully\n", portString)
		log.Printf("server on http://localhost%s was shut down successfully\n", portString)
	} else if err != nil {
		lg.E.Printf("server error: %v\n", err)
		log.Printf("server error: %v\n", err)
	}
}

func maintainDB(configFile string, scan, init bool) {
	cfg := config.GetConfig(configFile)
	gt := genres.NewGenresTree(cfg.Genres.TREE_FILE)
	db := database.NewDB(cfg.Database.DSN)
	defer db.Close()
	lg := config.NewLog(cfg.Logs.SCAN, cfg.Logs.DEBUG)
	defer lg.File.Close()
	handler := &stock.Handler{
		CFG: cfg,
		DB:  db,
		GT:  gt,
		LOG: lg,
	}
	handler.Do(scan, init)
}
