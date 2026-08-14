package download

import (
	"testing"

	"github.com/gotd/td/tg"
)

func audioDoc(fileName, mime string) *tg.Document {
	doc := &tg.Document{MimeType: mime}
	if fileName != "" {
		doc.Attributes = append(doc.Attributes, &tg.DocumentAttributeFilename{FileName: fileName})
	}
	return doc
}

func TestAudioFormatOf(t *testing.T) {
	cases := []struct {
		fileName string
		mime     string
		want     string
	}{
		{"song.MP3", "audio/mpeg", "mp3"},       // 文件名优先，忽略大小写
		{"track.flac", "audio/flac", "flac"},    //
		{"", "audio/mp4", "m4a"},                // 无文件名时回退到 MIME
		{"", "audio/unknown", "bin"},            // 未知 MIME
		{"noext", "audio/mpeg", "mp3"},          // 无扩展名时回退到 MIME
	}
	for _, c := range cases {
		if got := audioFormatOf(audioDoc(c.fileName, c.mime)); got != c.want {
			t.Errorf("audioFormatOf(%q, %q) = %q, want %q", c.fileName, c.mime, got, c.want)
		}
	}
}

func TestFormatAllowed(t *testing.T) {
	mp3 := audioDoc("song.mp3", "audio/mpeg")
	flac := audioDoc("song.flac", "audio/flac")

	if !formatAllowed(nil, mp3) || !formatAllowed([]string{}, mp3) {
		t.Error("empty filter should allow every format")
	}
	if !formatAllowed([]string{"mp3"}, mp3) {
		t.Error("mp3 should be allowed by [mp3]")
	}
	if formatAllowed([]string{"mp3"}, flac) {
		t.Error("flac should be rejected by [mp3]")
	}
	if !formatAllowed([]string{"MP3", "FLAC"}, flac) {
		t.Error("filter matching should be case-insensitive")
	}
}
