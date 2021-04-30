package stock

import (
	"archive/zip"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vinser/flibgo/pkg/config"
	"github.com/vinser/flibgo/pkg/database"
	"github.com/vinser/flibgo/pkg/fb2"
	"github.com/vinser/flibgo/pkg/genres"
	"github.com/vinser/flibgo/pkg/model"
)

type Op uint32

const (
	Init Op = 1 << iota
	Scan
)

type Handler struct {
	CFG *config.Config
	DB  *database.DB
	GT  *genres.GenresTree
	LOG *config.Log
	SY  Sync
}

type Sync struct {
	WG    *sync.WaitGroup
	Quota chan struct{}
}

func (h *Handler) Reindex() {
	db := h.DB
	db.DropDB(h.CFG.Database.DROP_SCRIPT)
	db.InitDB(h.CFG.Database.INIT_SCRIPT)
	start := time.Now()
	h.LOG.I.Println(">>> Book stock reindex started  >>>>>>>>>>>>>>>>>>>>>>>>>>>")
	h.ScanDir(true)
	finish := time.Now()
	h.LOG.I.Println("<<< Book stock reindex finished <<<<<<<<<<<<<<<<<<<<<<<<<<<")
	elapsed := finish.Sub(start)
	h.LOG.I.Println("Time elapsed: ", elapsed)
}

// Scan
func (h *Handler) ScanDir(reindex bool) error {
	dir := h.CFG.Library.NEW_ACQUISITIONS
	if reindex {
		dir = h.CFG.Library.BOOK_STOCK
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	entries, err := d.Readdir(-1)
	if err != nil {
		return err
	}
	h.SY.WG = &sync.WaitGroup{}
	h.SY.Quota = make(chan struct{}, h.CFG.Database.MAX_SCAN_THREADS)
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch {
		case entry.Size() == 0:
			h.LOG.E.Printf("File %s from dir has size of zero\n", entry.Name())
			os.Rename(path, filepath.Join(h.CFG.Library.TRASH))
		case entry.IsDir():
			h.LOG.I.Printf("Subdirectory %s has been skipped\n ", path)
			// scanDir(false) // uncomment for recurse
		case ext == ".zip":
			h.LOG.I.Println("Zip: ", entry.Name())
			h.SY.WG.Add(1)
			h.SY.Quota <- struct{}{}
			go h.processZip(path)
		case ext == ".fb2":
			h.LOG.I.Println("FB2: ", entry.Name())
			h.processFB2(path)
		}
	}
	h.SY.WG.Wait()
	return nil
}

// Process single FB2 file
func (h *Handler) processFB2(path string) {
	crc32 := fileCRC32(path)
	fInfo, _ := os.Stat(path)
	if h.DB.SkipBook(fInfo.Name(), crc32) {
		msg := "file %s has been skipped"
		h.LOG.I.Printf(msg, path, "\n")
		h.moveFile(path, fmt.Errorf(msg, path))
		return
	}
	f, err := os.Open(path)
	if err != nil {
		h.LOG.E.Printf("Failed to open file %s: %s\n", path, err)
		h.moveFile(path, err)
		return
	}
	defer f.Close()
	fb2, err := fb2.NewFB2(f)
	if err != nil {
		h.LOG.E.Printf("File %s has error: %s\n", path, err)
		h.moveFile(path, err)
		return
	}

	h.LOG.D.Println(fb2)
	book := &model.Book{
		File:     fInfo.Name(),
		CRC32:    crc32,
		Archive:  "",
		Size:     fInfo.Size(),
		Format:   "fb2",
		Title:    fb2.GetTitle(),
		Sort:     fb2.GetSort(),
		Year:     fb2.GetYear(),
		Plot:     fb2.GetPlot(),
		Cover:    fb2.GetCover(),
		Language: fb2.GetLanguage(),
		Authors:  fb2.GetAuthors(),
		Genres:   fb2.GetGenres(),
		Serie:    fb2.GetSerie(),
		SerieNum: fb2.Serie.Number,
		Updated:  time.Now().Unix(),
	}
	h.adjustGenges(book)
	h.DB.NewBook(book)
	f.Close()
	h.LOG.I.Printf("File %s has been added\n", path)
	h.moveFile(path, nil)
}

// Process zip archive with FB2 files
func (h *Handler) processZip(zipPath string) {
	defer h.SY.WG.Done()
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		h.LOG.E.Printf("Incorrect zip archive %s\n", zipPath)
		h.moveFile(zipPath, err)
		return
	}
	defer zr.Close()

	for _, file := range zr.File {
		h.LOG.D.Print(ZipEntryInfo(file))
		if filepath.Ext(file.Name) != ".fb2" {
			h.LOG.E.Printf("File %s from %s has not FB2 format\n", file.Name, filepath.Base(zipPath))
			continue
		}
		if h.DB.SkipBook(file.Name, file.CRC32) {
			h.LOG.I.Printf("File %s from %s has been skipped\n", file.Name, filepath.Base(zipPath))
			continue
		}
		if file.UncompressedSize == 0 {
			h.LOG.E.Printf("File %s from %s has size of zero\n", file.Name, filepath.Base(zipPath))
			continue
		}
		f, _ := file.Open()
		fb2, err := fb2.NewFB2(f)
		if err != nil {
			h.LOG.I.Printf("File %s from %s has error:\n", file.Name, filepath.Base(zipPath))
			h.LOG.E.Println(err)
			f.Close()
			continue
		}
		h.LOG.D.Println(fb2)
		book := &model.Book{
			File:     file.Name,
			CRC32:    file.CRC32,
			Archive:  filepath.Base(zipPath),
			Size:     int64(file.UncompressedSize),
			Format:   "fb2",
			Title:    fb2.GetTitle(),
			Sort:     fb2.GetSort(),
			Year:     fb2.GetYear(),
			Plot:     fb2.GetPlot(),
			Cover:    fb2.GetCover(),
			Language: fb2.GetLanguage(),
			Authors:  fb2.GetAuthors(),
			Genres:   fb2.GetGenres(),
			Serie:    fb2.GetSerie(),
			SerieNum: fb2.Serie.Number,
			Updated:  time.Now().Unix(),
		}
		h.adjustGenges(book)
		h.DB.NewBook(book)
		f.Close()
		h.LOG.I.Printf("File %s from %s has been added\n", file.Name, filepath.Base(zipPath))

		// runtime.Gosched()
	}
	h.moveFile(zipPath, nil)
	<-h.SY.Quota
}

func (h *Handler) adjustGenges(b *model.Book) {
	for i := range b.Genres {
		b.Genres[i] = h.GT.Transfer(b.Genres[i])
	}
}

func (h *Handler) moveFile(filePath string, err error) {
	if err != nil {
		os.Rename(filePath, filepath.Join(h.CFG.Library.TRASH, filepath.Base(filePath)))
		return
	}
	if filepath.Dir(filePath) == h.CFG.Library.BOOK_STOCK {
		return
	}
	os.Rename(filePath, filepath.Join(h.CFG.Library.BOOK_STOCK, filepath.Base(filePath)))
}

// fileCRC32 calculates file CRC32
func fileCRC32(filePath string) uint32 {
	fbytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return 0
	}
	return crc32.ChecksumIEEE(fbytes)
}

//===============================
func ZipEntryInfo(e *zip.File) string {
	return "\n===========================================\n" +
		fmt.Sprintln("File               : ", e.Name) +
		fmt.Sprintln("NonUTF8            : ", e.NonUTF8) +
		fmt.Sprintln("Modified           : ", e.Modified) +
		fmt.Sprintln("CRC32              : ", e.CRC32) +
		fmt.Sprintln("UncompressedSize64 : ", e.UncompressedSize) +
		"===========================================\n"
}
