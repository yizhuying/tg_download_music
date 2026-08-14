package download

import (
	"database/sql"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type DownloadRecord struct {
	db *sqlx.DB
}

func NewDownloadRecord(dir string) *DownloadRecord {
	dbPath := filepath.Join(dir, "record.db")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic("failed to create record dir: " + err.Error())
	}

	db, err := sqlx.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		panic("failed to open download record db: " + err.Error())
	}

	db.MustExec(`
		CREATE TABLE IF NOT EXISTS downloads (
			channel  TEXT    NOT NULL,
			msg_id   INTEGER NOT NULL,
			file_name TEXT   NOT NULL DEFAULT '',
			downloaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (channel, msg_id)
		)
	`)

	return &DownloadRecord{db: db}
}

func (dr *DownloadRecord) Close() error {
	return dr.db.Close()
}

func (dr *DownloadRecord) Exists(channel string, msgID int) bool {
	var count int
	dr.db.Get(&count, "SELECT COUNT(1) FROM downloads WHERE channel=? AND msg_id=?", channel, msgID)
	return count > 0
}

func (dr *DownloadRecord) Add(channel string, msgID int, fileName string) {
	dr.db.Exec("INSERT OR IGNORE INTO downloads (channel, msg_id, file_name) VALUES (?, ?, ?)", channel, msgID, fileName)
}

func (dr *DownloadRecord) GetByChannel(channel string) map[int]string {
	rows, err := dr.db.Queryx("SELECT msg_id, file_name FROM downloads WHERE channel=?", channel)
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := make(map[int]string)
	for rows.Next() {
		var msgID int
		var fileName sql.NullString
		rows.Scan(&msgID, &fileName)
		if fileName.Valid {
			result[msgID] = fileName.String
		} else {
			result[msgID] = ""
		}
	}
	return result
}
