package database

import (
	"testing"
	"testing/fstest"
)

func TestLoadMigrationsOrdersAndHashes(t *testing.T) {
	fsys := fstest.MapFS{
		"0002_second.sql": {Data: []byte("SELECT 2;")},
		"0001_first.sql":  {Data: []byte("SELECT 1;")},
		"0010_tenth.sql":  {Data: []byte("-- oddsvault:no-transaction\nSELECT 10;")},
		"notes.txt":       {Data: []byte("ignored")},
	}

	migs, err := LoadMigrations(fsys)
	if err != nil {
		t.Fatalf("LoadMigrations: %v", err)
	}
	if len(migs) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(migs))
	}

	wantOrder := []int64{1, 2, 10}
	for i, want := range wantOrder {
		if migs[i].Version != want {
			t.Fatalf("position %d: expected version %d, got %d", i, want, migs[i].Version)
		}
	}
	if migs[0].Name != "first" {
		t.Fatalf("expected name %q, got %q", "first", migs[0].Name)
	}
	if !migs[2].NoTx {
		t.Fatalf("expected 0010 to opt out of a transaction")
	}
	if migs[0].Checksum == "" || migs[0].Checksum == migs[1].Checksum {
		t.Fatalf("expected distinct non-empty checksums")
	}
}

func TestLoadMigrationsRejectsBadFiles(t *testing.T) {
	tests := []struct {
		name string
		fsys fstest.MapFS
	}{
		{
			name: "duplicate versions",
			fsys: fstest.MapFS{
				"0001_a.sql": {Data: []byte("SELECT 1;")},
				"1_b.sql":    {Data: []byte("SELECT 1;")},
			},
		},
		{
			name: "missing underscore",
			fsys: fstest.MapFS{"0001.sql": {Data: []byte("SELECT 1;")}},
		},
		{
			name: "non numeric version",
			fsys: fstest.MapFS{"abc_init.sql": {Data: []byte("SELECT 1;")}},
		},
		{
			name: "empty file",
			fsys: fstest.MapFS{"0001_empty.sql": {Data: []byte("   \n")}},
		},
		{
			name: "missing name",
			fsys: fstest.MapFS{"0001_.sql": {Data: []byte("SELECT 1;")}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadMigrations(tc.fsys); err == nil {
				t.Fatalf("expected an error")
			}
		})
	}
}
