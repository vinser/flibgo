package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	pollPeriod := 5 * time.Second

	go func(dir string, p time.Duration) {
		for {
			poll(dir)
			time.Sleep(p)
		}

	}("scan", pollPeriod)
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
		fmt.Printf("No files found")
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
			fmt.Printf("File %s from dir had unknown type and was moved to trash", entry.Name())
			os.Rename(scan, trash)
		}
	}
	return nil
}

func process(path string) {
	time.Sleep(30 * time.Second)
	fmt.Printf("file: %s was processed", path)
}
