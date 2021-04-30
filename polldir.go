package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	pollPeriod := 3 * time.Second
	scanDir := "scan"
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt)
	stopScan := make(chan bool)
	go func() {
		defer func() { stopScan <- true }()
		for {
			poll(scanDir)
			time.Sleep(pollPeriod)
			select {
			case <-stopScan:
				return
			default:
				continue
			}
		}

	}()
	<-done
	fmt.Println("\nstopping...")
	time.Sleep(2 * time.Second)
	stopScan <- true
	<-stopScan
}

func poll(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	entries, err := d.Readdir(-1)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No files found")
		return nil
	}
	for _, entry := range entries {
		scan := filepath.Join(dir, entry.Name())
		trash := filepath.Join("trash", entry.Name())
		archive := filepath.Join("archive", entry.Name())
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch {
		// case entry.Size() == 0:
		// 	fmt.Printf("File %s from dir had size of zero and was moved to trash\n", entry.Name())
		// 	os.Rename(scan, trash)
		case entry.IsDir():
			fmt.Printf("Subdirectory %s has been skipped\n ", scan)
			// scanDir(path) // uncomment for recurse
		case ext == ".zip":
			fmt.Println("Zip: ", entry.Name())
			process(scan)
			os.Rename(scan, archive)
		case ext == ".fb2":
			fmt.Println("FB2: ", entry.Name())
			process(scan)
			os.Rename(scan, archive)
		default:
			fmt.Printf("File %s from dir had unknown type and was moved to trash\n", entry.Name())
			os.Rename(scan, trash)
		}
	}
	return nil
}

func process(path string) {
	time.Sleep(5 * time.Second)
	fmt.Printf("file: %s was processed\n", path)
}
