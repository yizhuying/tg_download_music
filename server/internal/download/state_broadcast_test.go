package download

import (
	"encoding/json"
	"sync"
	"testing"
)

type captured struct {
	typ string
	raw string
}

func TestMarkDownloadedBroadcastsPerFile(t *testing.T) {
	ds := NewDownloadState()
	msgs := make([]ScannedMessage, 1000)
	for i := range msgs {
		msgs[i] = ScannedMessage{Channel: "c", MsgID: i + 1}
	}
	ds.SetMessages(msgs)

	var mu sync.Mutex
	var got []captured
	ds.SetBroadcaster(func(typ string, data interface{}) {
		raw, _ := json.Marshal(data)
		mu.Lock()
		got = append(got, captured{typ: typ, raw: string(raw)})
		mu.Unlock()
	})

	ds.MarkDownloaded("c", 5)

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0].typ != "file_downloaded" {
		t.Fatalf("expected one file_downloaded broadcast, got %+v", got)
	}
	want := `{"channel":"c","msg_id":5}`
	if got[0].raw != want {
		t.Fatalf("unexpected payload: %s", got[0].raw)
	}
	if !ds.messages[4].Exists {
		t.Fatal("message 5 not marked as exists")
	}
}

func TestIncrementDownloadedBroadcastsSlimStatus(t *testing.T) {
	ds := NewDownloadState()
	for i := 0; i < 500; i++ {
		ds.state.Logs = append(ds.state.Logs, LogEntry{Time: "00:00:00", Message: "log line"})
	}

	var mu sync.Mutex
	var got []captured
	ds.SetBroadcaster(func(typ string, data interface{}) {
		raw, _ := json.Marshal(data)
		mu.Lock()
		got = append(got, captured{typ: typ, raw: string(raw)})
		mu.Unlock()
	})

	ds.IncrementDownloaded()

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0].typ != "download_status" {
		t.Fatalf("expected one download_status broadcast, got %+v", got)
	}
	if got[0].raw != `{"running":false,"total_downloaded":1,"current_channel":""}` {
		t.Fatalf("slim payload should omit logs, got: %s", got[0].raw)
	}
}
