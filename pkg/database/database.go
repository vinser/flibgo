package database

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/vinser/flibgo/pkg/model"

	_ "github.com/go-sql-driver/mysql"
)

type DB struct {
	*sql.DB
}

// Books
func (db *DB) NewBook(b *model.Book) int64 {
	bookId := db.FindBook(b)
	if bookId != 0 {
		return bookId
	}
	languageId := db.NewLanguage(b.Language)

	q := "INSERT INTO books (file, crc32, archive, size, format, title, sort, year,language_id, plot, cover, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	res, err := db.Exec(q,
		b.File,
		b.CRC32,
		b.Archive,
		b.Size,
		b.Format,
		b.Title,
		b.Sort,
		b.Year,
		languageId,
		b.Plot,
		b.Cover,
		b.Updated,
	)
	if err != nil {
		log.Panic(err)
	}

	bookId, err = res.LastInsertId()
	if err != nil {
		log.Println(err)
		return 0
	}

	for _, author := range b.Authors {
		authorId := db.NewAuthor(author)
		q = "INSERT INTO books_authors (book_id, author_id) VALUES (?, ?)"
		_, err = db.Exec(q, bookId, authorId)
	}
	if err != nil {
		log.Println(err)
	}

	for _, genre := range b.Genres {
		q = "INSERT INTO books_genres (book_id, genre_code) VALUES (?, ?)"
		_, err = db.Exec(q, bookId, genre)
	}
	if err != nil {
		log.Println(err)
	}

	serieId := db.NewSerie(b.Serie)
	if serieId != 0 {
		q = "INSERT INTO books_series (serie_num, book_id, serie_id) VALUES (?, ?, ?)"
		_, err = db.Exec(q, b.SerieNum, bookId, serieId)
		if err != nil {
			log.Println(err)
		}
	}

	return bookId
}

func (db *DB) FindBook(b *model.Book) int64 {
	var id int64 = 0
	q := "SELECT id FROM books WHERE sort LIKE ? and crc32=?"
	err := db.QueryRow(q, b.Sort, b.CRC32).Scan(&id)
	if err == sql.ErrNoRows {
		return 0
	}
	return id
}

func (db *DB) FindBookById(id int64) *model.Book {
	b := &model.Book{}
	q := "SELECT file, archive, format, title, cover FROM books WHERE id=?"
	err := db.QueryRow(q, id).Scan(&b.File, &b.Archive, &b.Format, &b.Title, &b.Cover)
	if err == sql.ErrNoRows {
		return nil
	}
	return b
}

func (db *DB) IsFileInStock(file string, crc32 uint32) bool {
	var id int64
	q := "SELECT id FROM books WHERE file=? AND crc32=?"
	err := db.QueryRow(q, file, crc32).Scan(&id)
	return err != sql.ErrNoRows
}

func (db *DB) IsArchiveInStock(archive string) bool {
	var id int64
	q := "SELECT id FROM books WHERE archive=?"
	err := db.QueryRow(q, archive).Scan(&id)
	return err != sql.ErrNoRows
}

// Languages
func (db *DB) NewLanguage(l *model.Language) int64 {
	id := db.FindLanguage(l)
	if id != 0 {
		return id
	}
	q := "INSERT INTO languages (code, name) VALUES (?, ?)"
	res, _ := db.Exec(q, l.Code, l.Code)
	id, _ = res.LastInsertId()
	return id
}

func (db *DB) FindLanguage(l *model.Language) int64 {
	var id int64 = 0
	q := "SELECT id FROM languages WHERE code LIKE ?"
	err := db.QueryRow(q, l.Code).Scan(&id)
	if err == sql.ErrNoRows {
		return 0
	}
	return id
}

// Authors
func (db *DB) NewAuthor(a *model.Author) int64 {
	id := db.FindAuthor(a)
	if id != 0 {
		return id
	}
	q := "INSERT INTO authors (name, sort) VALUES (?, ?)"
	res, _ := db.Exec(q, a.Name, a.Sort)
	id, _ = res.LastInsertId()
	return id
}

func (db *DB) ListAuthors(prefix, language string) []*model.Author {
	var order1, order2 string
	switch language {
	case "ru":
		order1 = "а" // Cyrilic 'а'
		order2 = "a" // Latin 'a'
	default:
		order1 = "a" // Latin 'a'
		order2 = "а" // Cyrilic 'а'
	}
	l := utf8.RuneCountInString(prefix) + 1
	var (
		rows *sql.Rows
		err  error
	)
	if l == 1 {
		q := fmt.Sprint(`SELECT id, name, substr(sort,1,1) as s, count(*) as c FROM authors GROUP BY s ORDER BY s<'`, order1, `', s<'`, order2, `',s`)
		rows, err = db.Query(q)
	} else {
		q := fmt.Sprint(`SELECT id, name, substr(sort,1,`, fmt.Sprint(l), `) as s, count(*) as c FROM authors WHERE sort LIKE ? GROUP BY s`)
		rows, err = db.Query(q, prefix+"%")
	}
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	authors := []*model.Author{}

	for rows.Next() {
		var a *model.Author = &model.Author{}
		if err := rows.Scan(&a.ID, &a.Name, &a.Sort, &a.Count); err != nil {
			log.Fatal(err)
		}
		authors = append(authors, a)
	}
	return authors
}

func (db *DB) ListAuthorWithTotals(prefix string) []*model.Author {
	authors := []*model.Author{}
	q := `SELECT a.id, a.name, a.sort, count(*) FROM authors as a, books_authors as ba WHERE sort LIKE ? AND a.id=ba.author_id GROUP BY a.sort`
	rows, err := db.Query(q, prefix+"%")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var a *model.Author = &model.Author{}
		if err := rows.Scan(&a.ID, &a.Name, &a.Sort, &a.Count); err != nil {
			log.Fatal(err)
		}
		authors = append(authors, a)
	}
	return authors
}

func (db *DB) ListAuthorBooks(authorId, serieId int64, limit, offset int) []*model.Book {
	var (
		q    string
		rows *sql.Rows
		err  error
	)
	if serieId == 0 {
		q = `SELECT b.id, b.title, b.plot, b.cover FROM books as b, books_authors as ba WHERE ba.author_id=? AND b.id=ba.book_id ORDER BY b.sort`
		rows, err = db.pageQuery(q, limit, offset, authorId)
	} else {
		q = `SELECT b.id, b.title, b.plot, b.cover FROM books as b, books_authors as ba, series as s, books_series as bs WHERE ba.author_id=? AND ba.book_id=b.id AND bs.book_id=b.id AND bs.serie_id=? GROUP BY b.title`
		rows, err = db.pageQuery(q, limit, offset, authorId, serieId)
	}
	if err != nil {
		log.Println("DB page query error: ", err.Error())
	}
	defer rows.Close()
	books := []*model.Book{}

	for rows.Next() {
		b := &model.Book{}
		if err = rows.Scan(&b.ID, &b.Title, &b.Plot, &b.Cover); err != nil {
			log.Fatal(err)
		}
		books = append(books, b)
	}
	return books
}

func (db *DB) AuthorBookSeries(id int64) []*model.Serie {
	series := []*model.Serie{}
	q := `SELECT s.id, s.name FROM books_authors as ba, books as b, books_series as bs, series as s WHERE ba.author_id=? AND b.id=ba.book_id AND b.id=bs.book_id AND s.id=bs.serie_id GROUP BY s.name`
	rows, err := db.Query(q, id)
	if err != nil {
		return series
	}
	defer rows.Close()

	for rows.Next() {
		s := &model.Serie{}
		if err := rows.Scan(&s.ID, &s.Name); err != nil {
			return series
		}
		series = append(series, s)
	}
	return series
}

func (db *DB) AuthorByID(id int64) *model.Author {
	author := &model.Author{}
	q := "SELECT name, sort FROM authors WHERE id=?"
	err := db.QueryRow(q, id).Scan(&author.Name, &author.Sort)
	if err == sql.ErrNoRows {
		return nil
	}
	return author
}

func (db *DB) FindAuthor(a *model.Author) int64 {
	var id int64 = 0
	q := "SELECT id FROM authors WHERE sort LIKE ?"
	err := db.QueryRow(q, a.Sort).Scan(&id)
	if err == sql.ErrNoRows {
		return 0
	}
	return id
}

func (db *DB) AuthorsByBookId(bookId int64) []*model.Author {
	authors := []*model.Author{}
	q := `SELECT a.id, a.name FROM authors as a, books_authors as ba WHERE ba.book_id=? AND ba.author_id=a.id ORDER BY a.sort`
	rows, err := db.Query(q, bookId)
	if err != nil {
		return authors
	}
	defer rows.Close()

	for rows.Next() {
		a := &model.Author{}
		if err := rows.Scan(&a.ID, &a.Name); err != nil {
			return authors
		}
		authors = append(authors, a)
	}
	return authors
}

// Genres

func (db *DB) ListGenreBooks(genreCode string, limit, offset int) []*model.Book {
	q := `SELECT b.id, b.title, b.plot, b.cover FROM books as b, books_genres as bg WHERE bg.genre_code=? AND b.id=bg.book_id ORDER BY b.sort`
	rows, err := db.pageQuery(q, limit, offset, genreCode)
	if err != nil {
		log.Println("DB page query error: ", err.Error())
	}
	defer rows.Close()
	books := []*model.Book{}

	for rows.Next() {
		b := &model.Book{}
		if err = rows.Scan(&b.ID, &b.Title, &b.Plot, &b.Cover); err != nil {
			log.Fatal(err)
		}
		books = append(books, b)
	}
	return books
}

func (db *DB) CountGenreBooks(genreCode string) int64 {
	var c int64 = 0
	q := "SELECT count(*) FROM books_genres as bg WHERE bg.genre_code=?"
	err := db.QueryRow(q, genreCode).Scan(&c)
	if err == sql.ErrNoRows {
		return 0
	}
	return c
}

// Series
func (db *DB) NewSerie(s *model.Serie) int64 {
	if s.Name == "" {
		return 0
	}
	id := db.FindSerie(s)
	if id != 0 {
		return id
	}
	q := "INSERT INTO series (name) VALUES (?)"
	res, _ := db.Exec(q, s.Name)
	id, _ = res.LastInsertId()
	return id
}

func (db *DB) ListSerieBooks(id int64, limit, offset int) []*model.Book {
	q := `SELECT b.id, b.title, b.plot, b.cover FROM books as b, books_series as bs WHERE bs.serie_id=? AND b.id=bs.book_id ORDER BY bs.serie_num`
	rows, err := db.pageQuery(q, limit, offset, id)
	if err != nil {
		log.Println("DB page query error: ", err.Error())
	}
	defer rows.Close()
	books := []*model.Book{}

	for rows.Next() {
		b := &model.Book{}
		if err = rows.Scan(&b.ID, &b.Title, &b.Plot, &b.Cover); err != nil {
			log.Fatal(err)
		}
		books = append(books, b)
	}
	return books
}

// select id, substr(name,1,1) as s, count(*) as c FROM series group by s order by name<'а', `name`<'a',`name`;

func (db *DB) ListSeries(prefix, language string) []*model.Serie {
	var order1, order2 string
	switch language {
	case "ru":
		order1 = "а" // Cyrilic 'а'
		order2 = "a" // Latin 'a'
	default:
		order1 = "a" // Latin 'a'
		order2 = "а" // Cyrilic 'а'
	}
	l := utf8.RuneCountInString(prefix) + 1
	var (
		rows *sql.Rows
		err  error
	)
	if l == 1 {
		q := fmt.Sprint(`SELECT s2.id, substr(s2.n,1,1) as n2, count(*) as c2 FROM (SELECT s.id as id, s.name as n, count(*) as c FROM series as s, books_series as bs WHERE s.id=bs.serie_id GROUP BY n HAVING c>2) as s2 GROUP BY n2 ORDER BY n2<'`, order1, `', n2<'`, order2, `', n2`)
		rows, err = db.Query(q)
	} else {
		q := fmt.Sprint(`SELECT s2.id, substr(s2.n,1,`, fmt.Sprint(l), `) as n2, count(*) as c2 FROM (SELECT s.id as id, s.name as n, count(*) as c FROM series as s, books_series as bs WHERE s.id=bs.serie_id GROUP BY n HAVING c>2) as s2 WHERE s2.n LIKE ? GROUP BY n2;
		`)
		rows, err = db.Query(q, prefix+"%")
	}
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	series := []*model.Serie{}
	for rows.Next() {
		a := &model.Serie{}
		if err := rows.Scan(&a.ID, &a.Name, &a.Count); err != nil {
			log.Fatal(err)
		}
		series = append(series, a)
	}
	return series
}

func (db *DB) ListSeriesWithTotals(prefix string) []*model.Serie {
	series := []*model.Serie{}
	q := `SELECT s.id, s.name, count(*) as c FROM series as s, books_series as bs WHERE s.name LIKE ? AND s.id=bs.serie_id GROUP BY s.Name HAVING c>2`
	rows, err := db.Query(q, prefix+"%")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		s := &model.Serie{}
		if err := rows.Scan(&s.ID, &s.Name, &s.Count); err != nil {
			log.Fatal(err)
		}
		series = append(series, s)
	}
	return series
}

func (db *DB) SerieByID(id int64) *model.Serie {
	serie := &model.Serie{}
	q := "SELECT name FROM series WHERE id=?"
	err := db.QueryRow(q, id).Scan(&serie.Name)
	if err == sql.ErrNoRows {
		return nil
	}
	return serie
}

func (db *DB) FindSerie(s *model.Serie) int64 {
	var id int64 = 0
	q := "SELECT id FROM series WHERE name LIKE ?"
	err := db.QueryRow(q, s.Name).Scan(&id)
	if err == sql.ErrNoRows {
		return 0
	}
	return id
}

// Search
func (db *DB) SearchBooks(pattern string) []*model.Book {
	q := `SELECT id, title, plot, cover FROM books WHERE title Like ? ORDER BY sort`
	rows, err := db.Query(q, fmt.Sprint("%", pattern, "%"))
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	books := []*model.Book{}

	for rows.Next() {
		b := &model.Book{}
		if err := rows.Scan(&b.ID, &b.Title, &b.Plot, &b.Cover); err != nil {
			log.Fatal(err)
		}
		books = append(books, b)
	}
	return books
}

func (db *DB) PageSearchedBooks(pattern string, limit, offset int) []*model.Book {
	q := `SELECT id, title, plot, cover FROM books WHERE title LIKE ? ORDER BY sort`
	rows, err := db.pageQuery(q, limit, offset, fmt.Sprint("%", pattern, "%"))
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	books := []*model.Book{}

	for rows.Next() {
		b := &model.Book{}
		if err := rows.Scan(&b.ID, &b.Title, &b.Plot, &b.Cover); err != nil {
			log.Fatal(err)
		}
		books = append(books, b)
	}
	return books
}

func (db *DB) SearchAuthors(pattern string) []*model.Author {
	q := `SELECT id, name, sort FROM authors WHERE sort LIKE ? ORDER BY sort`
	rows, err := db.Query(q, fmt.Sprint(pattern, "%"))
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	authors := []*model.Author{}

	for rows.Next() {
		a := &model.Author{}
		if err := rows.Scan(&a.ID, &a.Name, &a.Sort); err != nil {
			log.Fatal(err)
		}
		authors = append(authors, a)
	}
	return authors
}

// ==================================
func NewDB(dsn string) *DB {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(10)
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	return &DB{db}
}

func (db *DB) InitDB(initSQL string) {
	if !db.IsReady() {
		db.execFile(initSQL)
	}
}

func (db *DB) DropDB(dropSQL string) {
	if db.IsReady() {
		db.execFile(dropSQL)
	}
}

func (db *DB) IsReady() bool {
	var err error
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	return rows.Next()
}

func (db *DB) execFile(sqlFile string) {
	file, err := os.Open(sqlFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	q := ""

	for scanner.Scan() {
		q += scanner.Text()
		if strings.Contains(q, ";") {
			_, err := db.Exec(q)
			q = ""
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func (db *DB) pageQuery(query string, limit, offset int, args ...interface{}) (*sql.Rows, error) {
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	if offset > 0 {
		query += " OFFSET ?"
		args = append(args, offset)
	}
	// log.Println("query: ", query, " args: ", args)
	rows, err := db.Query(query, args...)
	return rows, err
}
