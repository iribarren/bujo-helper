// Package batch converts between user-supplied lists (text lines, CSV files)
// and media items.
package batch

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"

	"cover-downloader/internal/match"
	"cover-downloader/internal/media"
)

// ParseLines parses one "Title, Creator" item per line. The split happens on
// the last comma so titles may contain commas; a line without a comma is a
// title alone. Blank lines and lines starting with '#' are skipped.
func ParseLines(text string) []media.Item {
	var items []media.Item
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var item media.Item
		if i := strings.LastIndexByte(line, ','); i >= 0 {
			item = media.Item{
				Title:   strings.TrimSpace(line[:i]),
				Creator: strings.TrimSpace(line[i+1:]),
			}
		} else {
			item = media.Item{Title: line}
		}
		if item.Title != "" {
			items = append(items, item)
		}
	}
	return items
}

// FormatLines is the inverse of ParseLines. A title containing a comma with
// no creator gets a trailing comma so it parses back unchanged.
func FormatLines(items []media.Item) string {
	var b strings.Builder
	for _, it := range items {
		b.WriteString(it.Title)
		switch {
		case it.Creator != "":
			b.WriteString(", ")
			b.WriteString(it.Creator)
		case strings.Contains(it.Title, ","):
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// headerNames are normalized first-column names that mark a header row.
var headerNames = map[string]bool{
	"title": true, "book": true, "libro": true,
	"titulo": true, "name": true, "nombre": true,
}

// ReadCSV reads items from a CSV file: column 1 is the title, column 2 the
// optional creator, and further columns are ignored. It accepts UTF-8 (with
// or without BOM) or Windows-1252, comma or semicolon delimiters, and an
// optional header row.
func ReadCSV(r io.Reader) ([]media.Item, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(data) {
		if data, err = charmap.Windows1252.NewDecoder().Bytes(data); err != nil {
			return nil, fmt.Errorf("decode csv: %w", err)
		}
	}

	cr := csv.NewReader(bytes.NewReader(data))
	cr.Comma = detectDelimiter(string(data))
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true

	var items []media.Item
	first := true
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse csv: %w", err)
		}
		title := cell(rec, 0)
		if title == "" {
			continue
		}
		if first {
			first = false
			if headerNames[match.Normalize(title)] {
				continue
			}
		}
		items = append(items, media.Item{Title: title, Creator: cell(rec, 1)})
	}
	return items, nil
}

// detectDelimiter picks ';' when the first non-empty line has more
// semicolons than commas, else ','.
func detectDelimiter(text string) rune {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Count(line, ";") > strings.Count(line, ",") {
			return ';'
		}
		return ','
	}
	return ','
}

func cell(rec []string, i int) string {
	if i < len(rec) {
		return strings.TrimSpace(rec[i])
	}
	return ""
}
