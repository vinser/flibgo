package config

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/yaml.v3"
)

// See config.yml for comments about this struct
type Config struct {
	Library struct {
		BOOK_STOCK       string `yaml:"BOOK_STOCK"`
		NEW_ACQUISITIONS string `yaml:"NEW_ACQUISITIONS"`
		TRASH            string `yaml:"TRASH"`
	}
	Language struct {
		DEFAULT string `yaml:"DEFAULT"`
	}
	Database struct {
		DSN              string `yaml:"DSN"`
		INIT_SCRIPT      string `yaml:"INIT_SCRIPT"`
		DROP_SCRIPT      string `yaml:"DROP_SCRIPT"`
		POLL_PERIOD      int    `yaml:"POLL_PERIOD"`
		MAX_SCAN_THREADS int    `yaml:"MAX_SCAN_THREADS"`
		ACCEPTED_LANGS   string `yaml:"ACCEPTED_LANGS"`
	}
	Genres struct {
		TREE_FILE string `yaml:"TREE_FILE"`
	}
	Logs struct {
		OPDS  string `yaml:"OPDS"`
		SCAN  string `yaml:"SCAN"`
		LEVEL string `yaml:"LEVEL"`
	}
	OPDS struct {
		PORT      int `yaml:"PORT"`
		PAGE_SIZE int `yaml:"PAGE_SIZE"`
	}
}

func LoadConfig(configFile string) *Config {
	f, err := os.Open(configFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}
	c := &Config{}
	if err := yaml.Unmarshal([]byte(b), c); err != nil {
		log.Fatal(err)
	}
	return c
}

func (cfg *Config) MkDirAll() {
	if err := os.MkdirAll(cfg.Library.BOOK_STOCK, 0666); err != nil {
		log.Fatalf("failed to create Library BOOK_STOCK directory %s: %s", cfg.Library.BOOK_STOCK, err)
	}
	if len(cfg.Library.NEW_ACQUISITIONS) > 0 {
		if err := os.MkdirAll(cfg.Library.NEW_ACQUISITIONS, 0666); err != nil {
			log.Fatalf("failed to create Library NEW_ACQUISITIONS directory %s: %s", cfg.Library.NEW_ACQUISITIONS, err)
		}
	}
	if err := os.MkdirAll(cfg.Library.TRASH, 0666); err != nil {
		log.Fatalf("failed to create Library TRASH directory %s: %s", cfg.Library.TRASH, err)
	}
}

func LoadLocales() {
	dir := "locales"
	// dir := "../locales"
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) != ".yml" {
			continue
		}
		yamlFile, err := ioutil.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			log.Fatal(err)
		}
		data := map[string]string{}
		err = yaml.Unmarshal(yamlFile, &data)
		if err != nil {
			log.Fatal(err)
		}

		lang := language.Make(strings.TrimSuffix(f.Name(), ".yml"))
		for key, value := range data {
			message.SetString(lang, key, value)

		}
	}
}
