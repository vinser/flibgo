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

	dbCmd := flag.NewFlagSet("database", flag.ExitOnError)
	dbScan := dbCmd.Bool("scan", false, "scan book stock directory and add new books to database")
	dbInit := dbCmd.Bool("init", false, "empty book database without scanning book stock directory")

	const configFile = "config/config.yml"
	// const configFile = "../config/config.yml" // for debug
	if len(os.Args) < 2 {
		startOPDS(configFile)
	} else {
		switch os.Args[1] {
		case "opds":
			opdsCmd.Parse(os.Args[2:])
			startOPDS(configFile)
		case "db":
			dbCmd.Parse(os.Args[2:])
			var op stock.Op
			if *dbInit {
				op |= stock.Init
			}
			if *dbScan {
				op |= stock.Scan
			}
			maintainStock(configFile, op)
		default:
			fmt.Println("'opds' or 'db' subcommand is expected ")
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
		f := "Empty database was inited. You can add new books by flibgo db -scan"
		lg.I.Println(f)
		log.Println(f)
		return
	}

	portString := fmt.Sprint(":", cfg.OPDS.PORT)
	f := "server on http://localhost%s is listening\n"
	lg.I.Printf(f, portString)
	log.Printf(f, portString)

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
			f := "error during shutdown: %v\n"
			lg.E.Printf(f, err)
			log.Printf(f, err)
		}
		wg.Done()
	}()

	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		log.Println("commencing server shutdown...")
		wg.Wait()
		f := "server on http://localhost%s was shut down successfully\n"
		lg.I.Printf(f, portString)
		log.Printf(f, portString)
	} else if err != nil {
		f := "server error: %v\n"
		lg.E.Printf(f, err)
		log.Printf(f, err)
	}
}

func maintainStock(configFile string, op stock.Op) {
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
	handler.Do(op)
}
