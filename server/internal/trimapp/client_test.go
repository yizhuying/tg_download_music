package trimapp

import "testing"

func TestParseConvertResultArray(t *testing.T) {
	// Shape observed on a real fnOS device: data is a bare result array.
	data := []byte(`[{"path":"/vol2/1000/music","semanticPath":"存储空间2/admin 的文件/music"}]`)
	got, err := parseConvertResult(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "/vol2/1000/music" || got[0].SemanticPath != "存储空间2/admin 的文件/music" {
		t.Fatalf("unexpected results: %+v", got)
	}
}

func TestParseConvertResultObject(t *testing.T) {
	// Shape from the official docs: {status, result}.
	data := []byte(`{"status":0,"result":[{"path":"/vol1/1000/photo","semanticPath":"存储空间1/admin 的文件/photo"}]}`)
	got, err := parseConvertResult(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "/vol1/1000/photo" {
		t.Fatalf("unexpected results: %+v", got)
	}
}

func TestParseConvertResultObjectError(t *testing.T) {
	if _, err := parseConvertResult([]byte(`{"status":1,"result":[]}`)); err == nil {
		t.Fatal("expected error for non-zero status")
	}
}
