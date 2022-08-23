ALTER DATABASE flibgo DEFAULT CHARACTER SET utf8mb4 DEFAULT COLLATE utf8mb4_general_ci;

SET FOREIGN_KEY_CHECKS=0;
-- START TRANSACTION;

DROP TABLE IF EXISTS languages;
CREATE TABLE languages (
    id INTEGER   PRIMARY KEY AUTO_INCREMENT,
    code VARCHAR(8) NOT NULL,
    name VARCHAR(16) NULL
);
CREATE INDEX languages_code_idx ON languages (code);
CREATE INDEX languages_name_idx ON languages (name);

DROP TABLE IF EXISTS authors;
CREATE TABLE authors (
    id INTEGER   PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(128) NOT NULL,
    sort VARCHAR(128) NOT NULL
);
CREATE INDEX authots_name_idx ON authors (name);
CREATE INDEX authots_sort_idx ON authors (sort);

DROP TABLE IF EXISTS books;
CREATE TABLE books (
    id INTEGER   PRIMARY KEY AUTO_INCREMENT,
    file VARCHAR(256) NOT NULL,
    crc32 BIGINT NOT NULL DEFAULT 0,
    archive VARCHAR(256) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    format VARCHAR(8) NOT NULL,
    title VARCHAR(512) NOT NULL,
    sort VARCHAR(512) NOT NULL,
    year VARCHAR(4) NOT NULL,
    language_id INTEGER NOT NULL,
    plot VARCHAR(10000) NOT NULL,
    cover VARCHAR(256),
    updated BIGINT NOT NULL DEFAULT 0,
    FOREIGN KEY (language_id) REFERENCES languages (id) ON DELETE CASCADE
);
CREATE INDEX book_file_idx ON books (file);
CREATE INDEX book_archive_idx ON books (archive);
CREATE INDEX book_title_idx ON books (title);
CREATE INDEX book_sort_idx ON books (sort);
CREATE INDEX book_updated_idx ON books (updated);

DROP TABLE IF EXISTS series;
CREATE TABLE series (
    id INTEGER   PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(256) NOT NULL
);
CREATE INDEX series_name_idx ON series (name);

DROP TABLE IF EXISTS books_authors;
CREATE TABLE books_authors (
    id INTEGER   PRIMARY KEY AUTO_INCREMENT,
    book_id INTEGER NOT NULL,
    author_id INTEGER NOT NULL,
    FOREIGN KEY (book_id) REFERENCES books (id) ON DELETE CASCADE,
    FOREIGN KEY (author_id) REFERENCES authors (id) ON DELETE CASCADE
);
CREATE INDEX books_authors_book_idx ON books_authors (book_id);
CREATE INDEX books_authors_author_idx ON books_authors (author_id);

DROP TABLE IF EXISTS books_genres;
CREATE TABLE books_genres (
    id INTEGER   PRIMARY KEY AUTO_INCREMENT,
    book_id INTEGER NOT NULL,
    genre_code VARCHAR(64) NOT NULL,
    FOREIGN KEY (book_id) REFERENCES books (id) ON DELETE CASCADE
);
CREATE INDEX books_genres_genre_code_idx ON books_genres (genre_code);
CREATE INDEX books_genres_book_idx ON books_genres (book_id);

DROP TABLE IF EXISTS books_series;
CREATE TABLE books_series (
    id INTEGER   PRIMARY KEY AUTO_INCREMENT,
    serie_num INTEGER NOT NULL DEFAULT 0,
    book_id INTEGER NOT NULL,
    serie_id INTEGER NOT NULL,
    FOREIGN KEY (book_id) REFERENCES books (id) ON DELETE CASCADE,
    FOREIGN KEY (serie_id) REFERENCES series (id) ON DELETE CASCADE
);
CREATE INDEX books_series_book_idx ON books_series (book_id);
CREATE INDEX books_series_serie_idx ON books_series (serie_id);


-- COMMIT;
SET FOREIGN_KEY_CHECKS=1;