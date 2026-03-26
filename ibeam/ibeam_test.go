package ibeam

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		t.Skip("no cache dir")
	}
	dbPath := filepath.Join(cacheDir, "gritt", "dyalog-docs.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Skip("no docs DB cached")
	}
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		t.Skip("can't open docs DB")
	}
	return db
}

func TestSearchByNumber(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	results := Search(db, "220")
	if len(results) == 0 {
		t.Fatal("no results for 220")
	}
	if results[0].Number != 220 {
		t.Fatalf("got number %d, want 220", results[0].Number)
	}
	if results[0].Name != "Serialise/Deserialise Array" {
		t.Fatalf("got name %q", results[0].Name)
	}
	if results[0].Source != "docs" {
		t.Fatalf("got source %q, want docs", results[0].Source)
	}
}

func TestSearchByName(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	results := Search(db, "UUID")
	found := false
	for _, r := range results {
		if r.Number == 120 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("120⌶ (UUID) not found searching for 'UUID'")
	}
}

func TestAll(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	all := All(db)
	if len(all) < 30 {
		t.Fatalf("expected 30+ I-beams, got %d", len(all))
	}
	// Verify sorted by number
	for i := 1; i < len(all); i++ {
		if all[i].Number < all[i-1].Number {
			t.Fatalf("not sorted: %d after %d", all[i].Number, all[i-1].Number)
		}
	}
}

func TestLookup(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	e := Lookup(db, 220)
	if e == nil {
		t.Fatal("220 not found")
	}
	if e.Name != "Serialise/Deserialise Array" {
		t.Fatalf("got %q", e.Name)
	}

	e = Lookup(db, 99999)
	if e != nil {
		t.Fatal("99999 should not exist")
	}
}

func TestParseDocEntry(t *testing.T) {
	cases := []struct {
		title, wantName, wantSig string
		wantNum                  int
	}{
		{"Serialise/Deserialise Array R←X(220⌶)Y", "Serialise/Deserialise Array", "R←X(220⌶)Y", 220},
		{"Generate UUID R←120⌶Y", "Generate UUID", "R←120⌶Y", 120},
		{"Log Use of Deprecated Features {R}←(13⌶)Y", "Log Use of Deprecated Features", "{R}←(13⌶)Y", 13},
		{"Called Monadically? R←900⌶Y", "Called Monadically?", "R←900⌶Y", 900},
	}
	for _, tc := range cases {
		e := parseDocEntry(tc.title, "")
		if e.Number != tc.wantNum {
			t.Errorf("%q: number=%d, want %d", tc.title, e.Number, tc.wantNum)
		}
		if e.Name != tc.wantName {
			t.Errorf("%q: name=%q, want %q", tc.title, e.Name, tc.wantName)
		}
		if e.Signature != tc.wantSig {
			t.Errorf("%q: sig=%q, want %q", tc.title, e.Signature, tc.wantSig)
		}
	}
}
