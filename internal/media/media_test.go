package media

import "testing"

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	for _, id := range []string{"books", "movies", "music"} {
		if err := r.Register(MediaType{ID: id, Label: id}); err != nil {
			t.Fatalf("Register(%q): %v", id, err)
		}
	}
	if err := r.Register(MediaType{ID: "books"}); err == nil {
		t.Error("duplicate Register should fail")
	}
	if err := r.Register(MediaType{}); err == nil {
		t.Error("empty ID Register should fail")
	}

	got, ok := r.Get("movies")
	if !ok || got.ID != "movies" {
		t.Errorf("Get(movies) = %+v, %v", got, ok)
	}
	if _, ok := r.Get("games"); ok {
		t.Error("Get(games) should not be found")
	}

	list := r.List()
	want := []string{"books", "movies", "music"}
	if len(list) != len(want) {
		t.Fatalf("List() len = %d, want %d", len(list), len(want))
	}
	for i, id := range want {
		if list[i].ID != id {
			t.Errorf("List()[%d] = %q, want %q", i, list[i].ID, id)
		}
	}
	list[0].ID = "mutated"
	if r.List()[0].ID != "books" {
		t.Error("List() must return a copy")
	}
}
