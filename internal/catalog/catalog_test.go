package catalog

import (
	"net/http"
	"reflect"
	"testing"
)

func TestDefault(t *testing.T) {
	reg := Default(http.DefaultClient)
	books, ok := reg.Get("books")
	if !ok {
		t.Fatal("books media type not registered")
	}
	if books.Label != "Books" || books.CreatorLabel != "Author" {
		t.Errorf("books = %+v", books)
	}
	var names []string
	for _, p := range books.Providers {
		names = append(names, p.Name())
	}
	if want := []string{"apple", "openlibrary", "amazon"}; !reflect.DeepEqual(names, want) {
		t.Errorf("providers = %q, want %q", names, want)
	}
	if Default(nil) == nil {
		t.Error("Default(nil) should fall back to a default client")
	}
}
