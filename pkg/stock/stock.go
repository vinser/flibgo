package stock

import (
	"archive/zip"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vinser/flibgo/pkgflibgo/pkg/config"
	"github.com/vinser/flibgo/pkgflibgo/pkg/database"
	"github.com/vinser/flibgo/pkgflibgo/pkg/fb2"
	"github.com/vinser/flibgo/pkgflibgo/pkg/genres"
	"github.com/vinser/flibgo/pkgflibgo/pkg/model"
)

type Handler struct {
	CFG *config.Config
	DB  *database.DB
	GT  *genres.GenresTree
	LOG *config.Log
}

func (h *Handler) Do(scan, init bool) {
	db := h.DB
	defer db.Close()
	if init {
		db.DropDB(h.CFG.Database.DROP_SCRIPT)
		db.InitDB(h.CFG.Database.INIT_SCRIPT)
	}
	if scan {
		start := time.Now()
		h.LOG.I.Println(">>> Book stock scan started  >>>>>>>>>>>>>>>>>>>>>>>>>>>")
		h.scanDir(h.CFG.Library.BOOK_STOCK)
		finish := time.Now()
		h.LOG.I.Println("<<< Book stock scan finished <<<<<<<<<<<<<<<<<<<<<<<<<<<")
		elapsed := finish.Sub(start)
		h.LOG.I.Println("Time elapsed: ", elapsed)
	}
}

// Scan dir
func (h *Handler) scanDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	entries, err := f.Readdir(-1)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch {
		case entry.Size() == 0:
			h.LOG.E.Printf("File %s from dir has size of zero\n", entry.Name())
		case entry.IsDir():
			h.LOG.I.Println("=== ", path)
			// scanDir(path) // Recurse
		case ext == ".zip":
			h.LOG.I.Println("Zip: ", entry.Name())
			h.processZip(path)
		case ext == ".fb2":
			h.LOG.I.Println("FB2: ", entry.Name())
			crc32 := FileCRC32(path)
			if h.DB.SkipBook(entry.Name(), crc32) {
				h.LOG.I.Printf("File %s from dir skipped\n", entry.Name())
				continue
			}
			f, _ := os.Open(path)
			fb2, err := fb2.NewFB2(f)
			if err != nil {
				h.LOG.I.Printf("File %s from dir has error:\n", entry.Name())
				h.LOG.E.Println(err)
				f.Close()
				continue
			}
			h.LOG.D.Println(fb2)
			book := &model.Book{
				File:     entry.Name(),
				CRC32:    crc32,
				Archive:  "",
				Size:     entry.Size(),
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
			h.LOG.I.Printf("File %s from dir added\n", entry.Name())
		}
	}
	return nil
}

// Get zip
func (h *Handler) processZip(zipPath string) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		panic(err)
	}
	defer zr.Close()

	for _, file := range zr.File {
		h.LOG.D.Print(ZipEntryInfo(file))
		if h.DB.SkipBook(file.Name, file.CRC32) {
			h.LOG.I.Printf("File %s from %s skipped\n", file.Name, filepath.Base(zipPath))
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
		h.LOG.I.Printf("File %s from %s added\n", file.Name, filepath.Base(zipPath))
	}
}

func (h *Handler) adjustGenges(b *model.Book) {
	for i := range b.Genres {
		b.Genres[i] = h.GT.Transfer(b.Genres[i])
	}
}

// FileCRC32 calculates file CRC32
func FileCRC32(filePath string) uint32 {
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
