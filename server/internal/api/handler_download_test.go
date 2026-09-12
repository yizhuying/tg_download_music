package api

import (
	"reflect"
	"testing"
)

func TestShareDirsSkipsLogs(t *testing.T) {
	t.Setenv("TRIM_DATA_SHARE_PATHS", "/vol1/1000/TuneGram/music:/vol1/1000/TuneGram/logs")

	got := shareDirs()
	want := []string{"/vol1/1000/TuneGram/music"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestShareDirsNoEnv(t *testing.T) {
	t.Setenv("TRIM_DATA_SHARE_PATHS", "")

	if got := shareDirs(); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestAppendUnique(t *testing.T) {
	// Authorized dirs are appended after the default; a trailing-slash
	// duplicate is skipped.
	got := appendUnique([]string{"/vol1/1000/TuneGram/music"},
		[]string{"/vol2/1000/music", "/vol1/1000/TuneGram/music/"})
	want := []string{"/vol1/1000/TuneGram/music", "/vol2/1000/music"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
