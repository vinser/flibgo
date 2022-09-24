package fb2

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/vinser/flibgo/pkg/model"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type FB2 struct {
	*TitleInfo
}

func NewFB2(rc io.ReadCloser) (*FB2, error) {
	decoder := xml.NewDecoder(rc)
	decoder.CharsetReader = charset.NewReaderLabel
	fb := &FB2{}
TokenLoop:
	for {
		t, err := decoder.Token()
		if err != nil {
			return nil, err
		}

		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "title-info" {
				decoder.DecodeElement(fb, &se)
				break TokenLoop
			}
		default:
		}
	}
	return fb, nil
}

func (fb *FB2) String() string {
	return fmt.Sprint(
		"\n=========FB2===================\n",
		fmt.Sprintf("Authors:    %#v\n", fb.Authors),
		fmt.Sprintf("Title:      %#v\n", fb.Title),
		fmt.Sprintf("Gengre:     %#v\n", fb.Gengres),
		fmt.Sprintf("Annotation: %#v\n", fb.Annotation),
		fmt.Sprintf("Date:       %#v\n", fb.Date),
		fmt.Sprintf("Year:       %#v\n", fb.Year),
		fmt.Sprintf("Lang:       %#v\n", fb.Lang),
		fmt.Sprintf("Serie:      %#v\n", fb.Serie),
		fmt.Sprintf("CoverPage:  %#v\n", fb.CoverPage),
		"===============================\n",
	)
}

type TitleInfo struct {
	Authors    []Author   `xml:"author"`
	Title      string     `xml:"book-title"`
	Gengres    []string   `xml:"genre"`
	Annotation Annotation `xml:"annotation"`
	Date       string     `xml:"date"`
	Year       string     `xml:"year"`
	Lang       string     `xml:"lang"`
	Serie      Serie      `xml:"sequence"`
	CoverPage  Image      `xml:"coverpage>image"`
}

type Author struct {
	FirstName  string `xml:"first-name"`
	MiddleName string `xml:"middle-name"`
	LastName   string `xml:"last-name"`
}

type Annotation struct {
	Text string `xml:",innerxml"`
}

type Serie struct {
	Name   string `xml:"name,attr"`
	Number int    `xml:"number,attr"`
}

type CoverPage struct {
	*Image `xml:"image"`
}

type Image struct {
	Href string `xml:"http://www.w3.org/1999/xlink href,attr"`
}

type Binary struct {
	Id          string `xml:"id,attr"`
	ContentType string `xml:"content-type,attr"`
	Content     []byte `xml:",chardata"`
}

func GetCoverPageBinary(coverLink string, rc io.ReadCloser) (*Binary, error) {
	decoder := xml.NewDecoder(rc)
	decoder.CharsetReader = charset.NewReaderLabel
	b := &Binary{}
	noCover := true
	coverLink = strings.TrimPrefix(coverLink, "#")
TokenLoop:
	for {
		t, _ := decoder.Token()
		if t == nil {
			return nil, errors.New("FB2 xml error")
		}
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "binary" {
				for _, att := range se.Attr {
					if strings.ToLower(att.Name.Local) == "id" && att.Value == coverLink {
						decoder.DecodeElement(b, &se)
						noCover = false
						break TokenLoop
					}
				}
			}
		default:
		}
	}
	if noCover {
		return nil, errors.New("FB2 has no Cover Page")
	}
	return b, nil
}

func (b *Binary) String() string {
	return fmt.Sprintf(
		`CoverPage ----
  Id: %s
  Content-type: %s
================================
%#v
===========(100)================
`, b.Id, b.ContentType, b.Content[:99])
}

func (fb *FB2) GetFormat() string {
	return "fb2"
}

func (fb *FB2) GetTitle() string {
	return strings.Trim(fb.Title, "\n\t ")
}

func (fb *FB2) GetSort() string {
	return strings.ToUpper(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(strings.Trim(fb.Title, "\n\t "), "An "), "A "), "The "))
}

func (fb *FB2) GetYear() string {
	year := fb.Year
	if year == "" {
		year = fb.Date
	}
	rYear := []rune(year)
	if len(rYear) > 4 {
		rYear = rYear[len(rYear)-4:]
	}
	return strings.Trim(string(rYear), "\n\t ")
}

func (fb *FB2) GetPlot() string {
	s := stripNonprintables(fb.Annotation.Text)
	// s = wellFormHTML(s)
	return truncateUTF8String(s, 10000)
}

func (fb *FB2) GetCover() string {
	return strings.TrimPrefix(fb.CoverPage.Href, "#")
}

func (fb *FB2) GetLanguage() *model.Language {
	code := strings.Trim(fb.Lang, "\n\t ")
	base, _ := language.Make(code).Base()
	return &model.Language{Code: fmt.Sprint(base)}
}

func (fb *FB2) GetAuthors() []*model.Author {
	authors := make([]*model.Author, 0, len(fb.Authors))
	if len(fb.Authors) == 1 {
		aLN := strings.Split(fb.Authors[0].LastName, ",")
		if len(aLN) > 1 {
			a := "Авторский коллектив"
			if fb.Lang != "ru" {
				a = "Writing team"
			}
			authors = append(authors, &model.Author{
				Name: a,
				Sort: strings.ToUpper(a),
			})
			return authors
		}
	}
	for _, a := range fb.Authors {
		author := &model.Author{}
		// f := strings.Title(strings.ToLower(strings.Trim(a.FirstName, "\n\t ")))
		// m := strings.Title(strings.ToLower(strings.Trim(a.MiddleName, "\n\t ")))
		// l := strings.Title(strings.ToLower(strings.Trim(a.LastName, "\n\t ")))
		// author.Name = strings.ReplaceAll(fmt.Sprint(f, " ", m, " ", l), "  ", " ")
		// author.Sort = strings.ReplaceAll(fmt.Sprint(l, " ", f, " ", m), "  ", " ")
		f := refineName(a.FirstName, fb.Lang)
		m := refineName(a.MiddleName, fb.Lang)
		l := refineName(a.LastName, fb.Lang)
		author.Name = CollapseSpaces(fmt.Sprintf("%s %s %s", f, m, l))
		author.Sort = CollapseSpaces(fmt.Sprintf("%s, %s %s", l, f, m))
		authors = append(authors, author)
	}
	return authors
}

func (fb *FB2) GetGenres() []string {
	return fb.Gengres
}

func (fb *FB2) GetSerie() *model.Serie {
	return &model.Serie{Name: fb.Serie.Name}
}

func (fb *FB2) GetSerieNumber() int {
	return fb.Serie.Number
}

var rxPrintables = regexp.MustCompile(`(?m)[\p{L}\p{P}\p{N}\n\r\t </>]`)

func stripNonprintables(s string) string {
	return strings.Join(rxPrintables.FindAllString(s, -1), "")
}

func wellFormHTML(s string) string {
	nodes, err := html.ParseFragment(bytes.NewBufferString(s), nil)
	if err != nil {
		return s
	}

	b := new(bytes.Buffer)
	for _, v := range nodes {
		err := html.Render(b, v)
		if err != nil {
			return s
		}
	}
	return strings.TrimSuffix(strings.TrimPrefix(b.String(), "<html><head></head><body>"), "</body></html>")
}

// Truncate UTF8-coded string to the given or less number of bytes to maintain the integrity of string runes
func truncateUTF8String(str string, length int) string {
	if length <= 0 {
		return ""
	}
	if len(str) <= length {
		return str
	}
	// If string is not valuid UTF8
	if !utf8.ValidString(str) {
		return str[:length]
	}
	for len(str) > length {
		_, size := utf8.DecodeLastRuneInString(str)
		str = str[:len(str)-size]
	}
	return str
}

func refineName(n, lang string) string {
	return title(lower(strings.TrimSpace(n), lang), lang)
}

func title(s, lang string) string {
	return cases.Title(getLanguageTag(lang)).String(s)
}

func lower(s, lang string) string {
	return cases.Lower(getLanguageTag(lang)).String(s)
}

func upper(s, lang string) string {
	return cases.Upper(getLanguageTag(lang)).String(s)
}

func getLanguageTag(lang string) language.Tag {
	return language.Make(strings.TrimSpace(lang))
}

// RegExp Remove surplus spaces
var rxSpaces = regexp.MustCompile(`[ \n\r\t]+`)

func CollapseSpaces(s string) string {
	return rxSpaces.ReplaceAllString(s, ` `)
}
