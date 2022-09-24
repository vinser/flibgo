package stock

import (
	"archive/zip"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/vinser/flibgo/pkg/config"
	"github.com/vinser/flibgo/pkg/database"
	"github.com/vinser/flibgo/pkg/fb2"
	"github.com/vinser/flibgo/pkg/genres"
	"github.com/vinser/flibgo/pkg/model"
	"github.com/vinser/flibgo/pkg/parser"
	"github.com/vinser/flibgo/pkg/rlog"
)

type Handler struct {
	CFG *config.Config
	DB  *database.DB
	GT  *genres.GenresTree
	LOG *rlog.Log
	SY  Sync
}

type Sync struct {
	WG    *sync.WaitGroup
	Quota chan struct{}
}

// Reindex() - recreate book stock database
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
	if reindex || len(dir) == 0 {
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
			h.LOG.E.Printf("file %s from dir has size of zero\n", entry.Name())
			os.Rename(path, filepath.Join(h.CFG.Library.TRASH, entry.Name()))
		case entry.IsDir():
			h.LOG.E.Printf("subdirectory %s has been skipped\n ", path)
			// scanDir(false) // uncomment for recurse
		case ext == ".zip":
			h.SY.WG.Add(1)
			h.SY.Quota <- struct{}{}
			go h.indexFB2Archive(path)
		case ext == ".fb2":
			h.indexFB2SingleFile(path)
		default:
			h.LOG.E.Printf("file %s is of unsupported format \"%s\"\n", path, filepath.Ext(path))
			err = fmt.Errorf("file %s is of unsupported format \"%s\"", path, filepath.Ext(path))
			h.moveFile(path, err)
		}
	}
	h.SY.WG.Wait()
	return nil
}

// Index single FB2 file
func (h *Handler) indexFB2SingleFile(path string) {
	crc32 := fileCRC32(path)
	fInfo, _ := os.Stat(path)
	if h.DB.IsFileInStock(fInfo.Name(), crc32) {
		msg := "file %s is in stock already and has been skipped"
		h.LOG.D.Printf(msg+"\n", path)
		if len(h.CFG.Library.NEW_ACQUISITIONS) > 0 {
			h.moveFile(path, fmt.Errorf(msg, path))
		}
		return
	}
	h.LOG.I.Println("Single file: ", path)
	defer func() {
		if err := recover(); err != nil {
			h.LOG.E.Printf("failed to index single file %s: \n%s\n", path, err)
			h.LOG.D.Println(string(debug.Stack()))
		}
	}()
	f, err := os.Open(path)
	if err != nil {
		h.LOG.E.Printf("failed to open file %s: %s\n", path, err)
		h.moveFile(path, err)
		return
	}
	defer f.Close()

	var p parser.Parser
	p, err = fb2.NewFB2(f)
	if err != nil {
		h.LOG.E.Printf("file %s has errors: %s\n", path, err)
		h.moveFile(path, err)
		return
	}
	h.LOG.D.Println(p)
	book := &model.Book{
		File:     fInfo.Name(),
		CRC32:    crc32,
		Archive:  "",
		Size:     fInfo.Size(),
		Format:   p.GetFormat(),
		Title:    p.GetTitle(),
		Sort:     p.GetSort(),
		Year:     p.GetYear(),
		Plot:     p.GetPlot(),
		Cover:    p.GetCover(),
		Language: p.GetLanguage(),
		Authors:  p.GetAuthors(),
		Genres:   p.GetGenres(),
		Serie:    p.GetSerie(),
		SerieNum: p.GetSerieNumber(),
		Updated:  time.Now().Unix(),
	}
	if !h.acceptLanguage(book.Language.Code) {
		msg := "publication language \"%s\" is configured as not accepted, file %s has been skipped"
		h.LOG.D.Printf(msg+"\n", book.Language.Code, path)
		h.moveFile(path, fmt.Errorf(msg, book.Language.Code, path))
		return
	}
	h.adjustGenges(book)
	h.DB.NewBook(book)
	f.Close()
	h.LOG.D.Printf("file %s has been added\n", path)
	h.moveFile(path, nil)
}

// Index zip archive with FB2 files
func (h *Handler) indexFB2Archive(zipPath string) {
	defer h.SY.WG.Done()
	if h.DB.IsArchiveInStock(filepath.Base(zipPath)) {
		msg := "archive %s is in stock already and has been skipped"
		h.LOG.D.Printf(msg+"\n", zipPath)
		if len(h.CFG.Library.NEW_ACQUISITIONS) > 0 {
			h.moveFile(zipPath, fmt.Errorf(msg, zipPath))
		}
		return
	}
	h.LOG.I.Println("Zip archive: ", zipPath)
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		h.LOG.E.Printf("incorrect zip archive %s\n", zipPath)
		h.moveFile(zipPath, err)
		return
	}
	defer zr.Close()

	for _, file := range zr.File {
		h.indexFB2ArchiveFile(filepath.Base(zipPath), file)

		// runtime.Gosched()
	}
	h.moveFile(zipPath, nil)
	<-h.SY.Quota
}

// Index zip archive with FB2 files
func (h *Handler) indexFB2ArchiveFile(zipName string, file *zip.File) {
	defer func() {
		if err := recover(); err != nil {
			h.LOG.E.Printf("failed to index file %s from archive %s: \n%s\n", zipName, file.Name, err)
			h.LOG.D.Println(string(debug.Stack()))
		}
	}()
	h.LOG.D.Print(ZipEntryInfo(file))
	if filepath.Ext(file.Name) != ".fb2" {
		h.LOG.E.Printf("file %s from %s has not FB2 format\n", file.Name, zipName)
		return
	}
	if h.DB.IsFileInStock(file.Name, file.CRC32) {
		h.LOG.D.Printf("file %s from %s is in stock already and has been skipped\n", file.Name, zipName)
		return
	}
	if file.UncompressedSize == 0 {
		h.LOG.E.Printf("file %s from %s has size of zero\n", file.Name, zipName)
		return
	}
	f, err := file.Open()
	if err != nil {
		h.LOG.E.Printf("archive %s is broken: %s\n", zipName, err.Error())
		return
	}
	defer f.Close()
	var p parser.Parser
	switch filepath.Ext(file.Name) {
	case ".fb2":
		p, err = fb2.NewFB2(f)
		if err != nil {
			h.LOG.E.Printf("file %s from archive %s has error: %s\n", file.Name, zipName, err.Error())
			f.Close()
			return
		}
	default:
		h.LOG.E.Printf("file %s from archive %s is of unsupported format \"%s\"\n", file.Name, zipName, filepath.Ext(file.Name))
	}
	h.LOG.D.Println(p)
	book := &model.Book{
		File:     file.Name,
		CRC32:    file.CRC32,
		Archive:  zipName,
		Size:     int64(file.UncompressedSize),
		Format:   p.GetFormat(),
		Title:    p.GetTitle(),
		Sort:     p.GetSort(),
		Year:     p.GetYear(),
		Plot:     p.GetPlot(),
		Cover:    p.GetCover(),
		Language: p.GetLanguage(),
		Authors:  p.GetAuthors(),
		Genres:   p.GetGenres(),
		Serie:    p.GetSerie(),
		SerieNum: p.GetSerieNumber(),
		Updated:  time.Now().Unix(),
	}
	if !h.acceptLanguage(book.Language.Code) {
		h.LOG.D.Printf("publication language \"%s\" is not accepted, file %s from %s has been skipped\n", book.Language.Code, file.Name, zipName)
		return
	}
	h.adjustGenges(book)
	h.DB.NewBook(book)
	h.LOG.D.Printf("file %s from %s has been added\n", file.Name, zipName)

}

func (h *Handler) adjustGenges(b *model.Book) {
	for i := range b.Genres {
		b.Genres[i] = h.GT.Transfer(b.Genres[i])
	}
}

func (h *Handler) acceptLanguage(lang string) bool {
	return strings.Contains(h.CFG.Database.ACCEPTED_LANGS, lang)
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
	fbytes, err := os.ReadFile(filePath)
	if err != nil {
		return 0
	}
	return crc32.ChecksumIEEE(fbytes)
}

// ===============================
func ZipEntryInfo(e *zip.File) string {
	return "\n===========================================\n" +
		fmt.Sprintln("File               : ", e.Name) +
		fmt.Sprintln("NonUTF8            : ", e.NonUTF8) +
		fmt.Sprintln("Modified           : ", e.Modified) +
		fmt.Sprintln("CRC32              : ", e.CRC32) +
		fmt.Sprintln("UncompressedSize64 : ", e.UncompressedSize) +
		"===========================================\n"
}
