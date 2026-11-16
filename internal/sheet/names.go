package sheet

import (
	"path/filepath"
	"regexp"
	"strings"
)

// nameSeparator separates the title from the creator in a cover's file name.
const nameSeparator = " - "

// duplicateSuffix matches the "-N" (N ≥ 2) the downloader appends to the
// name of a duplicate file, directly after the name (no space before it).
var duplicateSuffix = regexp.MustCompile(`(\S)-(?:[2-9]|[1-9][0-9]+)$`)

// ParseName reads the title and creator from the file name of a downloaded
// cover, "Title - Creator.ext" (the creator is optional; for movies it is
// the year). The directory and extension are dropped, as is the "-N"
// suffix of duplicates ("Title - Creator-2.ext"). The name is split on its
// last " - ", so a title may contain one; without one, the whole name is the
// title.
//
// A title that itself ends in "-N" with no creator ("Catch-22.jpg") cannot
// be told from a duplicate and loses its suffix ("Catch").
func ParseName(fileName string) (title, creator string) {
	base := filepath.Base(fileName)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	stem = duplicateSuffix.ReplaceAllString(stem, "$1")
	title = stem
	if i := strings.LastIndex(stem, nameSeparator); i >= 0 {
		title, creator = stem[:i], stem[i+len(nameSeparator):]
	}
	return strings.TrimSpace(title), strings.TrimSpace(creator)
}
