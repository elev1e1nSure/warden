package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

func dbPath() string {
	if override := os.Getenv("WARDEN_MEMORY_DB"); override != "" {
		return override
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".warden", "memory.db")
}

type MemoryEntry struct {
	ID              int     `json:"id"`
	SessionID       string  `json:"session_id"`
	Timestamp       string  `json:"timestamp"`
	Category        string  `json:"category"`
	Key             string  `json:"key"`
	Value           string  `json:"value"`
	Confidence      float64 `json:"confidence"`
	SourceMessageID string  `json:"source_message_id"`
}

type MemoryStore struct {
	DBPath string
	db     *sql.DB
}

func NewMemoryStore(customPath string) *MemoryStore {
	path := customPath
	if path == "" {
		path = dbPath()
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	return &MemoryStore{DBPath: path}
}

func (s *MemoryStore) open() (*sql.DB, error) {
	if s.db != nil {
		return s.db, nil
	}
	db, err := sql.Open("sqlite", s.DBPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s.db = db
	return db, nil
}

func (s *MemoryStore) Init() error {
	db, err := s.open()
	if err != nil {
		return err
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS memory_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS memory_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			category TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			confidence REAL NOT NULL DEFAULT 1.0,
			source_message_id TEXT
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_entries_cat_key ON memory_entries(category, key)`,
		`CREATE TABLE IF NOT EXISTS memory_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			data TEXT NOT NULL
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("init db: %w", err)
		}
	}
	return nil
}

func (s *MemoryStore) SetEnabled(enabled bool) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	val := "0"
	if enabled {
		val = "1"
	}
	_, err = db.Exec(`
		INSERT INTO memory_state(key, value, updated_at)
		VALUES ('enabled', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP
	`, val)
	return err
}

func (s *MemoryStore) GetEnabled() bool {
	db, err := s.open()
	if err != nil {
		return false
	}
	var val string
	err = db.QueryRow("SELECT value FROM memory_state WHERE key = 'enabled'").Scan(&val)
	if err != nil {
		return false
	}
	return val == "1"
}

func (s *MemoryStore) UpsertEntry(sessionID, category, key, value string, confidence float64, sourceMsgID string) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	var sid *string
	if sourceMsgID != "" {
		sid = &sourceMsgID
	}
	_, err = db.Exec(`
		INSERT INTO memory_entries
		(session_id, category, key, value, confidence, source_message_id)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(category, key) DO UPDATE SET
			session_id = excluded.session_id,
			value = excluded.value,
			confidence = excluded.confidence,
			source_message_id = excluded.source_message_id,
			timestamp = CURRENT_TIMESTAMP
	`, sessionID, category, key, value, confidence, sid)
	return err
}

func (s *MemoryStore) GetEntries(sessionID, category string) ([]MemoryEntry, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}

	query := `SELECT id, session_id, timestamp, category, key, value, confidence, source_message_id FROM memory_entries`
	var conds []string
	var args []interface{}

	if sessionID != "" {
		conds = append(conds, "session_id = ?")
		args = append(args, sessionID)
	}
	if category != "" {
		conds = append(conds, "category = ?")
		args = append(args, category)
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY timestamp DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		var sourceMsgID sql.NullString
		err := rows.Scan(&e.ID, &e.SessionID, &e.Timestamp, &e.Category, &e.Key, &e.Value, &e.Confidence, &sourceMsgID)
		if err != nil {
			return nil, err
		}
		if sourceMsgID.Valid {
			e.SourceMessageID = sourceMsgID.String
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []MemoryEntry{}
	}
	return entries, rows.Err()
}

func (s *MemoryStore) ClearEntries(sessionID string) (int, error) {
	db, err := s.open()
	if err != nil {
		return 0, err
	}
	var res sql.Result
	if sessionID != "" {
		res, err = db.Exec("DELETE FROM memory_entries WHERE session_id = ?", sessionID)
	} else {
		res, err = db.Exec("DELETE FROM memory_entries")
	}
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (s *MemoryStore) SaveSnapshot(sessionID string, data map[string]interface{}) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO memory_snapshots(session_id, data) VALUES (?, ?)", sessionID, string(jsonData))
	return err
}

func (s *MemoryStore) GetLatestSnapshot() (map[string]interface{}, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	var data string
	err = db.QueryRow("SELECT data FROM memory_snapshots ORDER BY id DESC LIMIT 1").Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *MemoryStore) DeleteEntry(key string) (int, error) {
	db, err := s.open()
	if err != nil {
		return 0, err
	}
	res, err := db.Exec("DELETE FROM memory_entries WHERE key = ?", key)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (s *MemoryStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *MemoryStore) GetContextText(sessionID string, minConfidence float64, maxEntries int) string {
	snapshot, _ := s.GetLatestSnapshot()
	entries, _ := s.GetEntries(sessionID, "")

	var filtered []MemoryEntry
	for _, e := range entries {
		if e.Confidence >= minConfidence {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) > maxEntries {
		filtered = filtered[:maxEntries]
	}

	if snapshot == nil && len(filtered) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "[Memory]")

	if snapshot != nil {
		if u, ok := snapshot["user"].(map[string]interface{}); ok {
			for k, v := range u {
				lines = append(lines, fmt.Sprintf("- user %s: %v", k, v))
			}
		}
		if projs, ok := snapshot["projects"].([]interface{}); ok {
			for _, p := range projs {
				if pm, ok := p.(map[string]interface{}); ok {
					name := ""
					if n, ok := pm["name"].(string); ok {
						name = n
					}
					if ts, ok := pm["tech_stack"].([]interface{}); ok {
						var stack []string
						for _, t := range ts {
							if s, ok := t.(string); ok {
								stack = append(stack, s)
							}
						}
						if name != "" {
							lines = append(lines, fmt.Sprintf("- project %s (stack: %s)", name, strings.Join(stack, ", ")))
						}
					} else if name != "" {
						lines = append(lines, fmt.Sprintf("- project: %s", name))
					}
				}
			}
		}
		if prefs, ok := snapshot["preferences"].(map[string]interface{}); ok {
			for k, v := range prefs {
				lines = append(lines, fmt.Sprintf("- preference %s: %v", k, v))
			}
		}
		if ts, ok := snapshot["tech_stack"].([]interface{}); ok {
			var stack []string
			for _, t := range ts {
				if s, ok := t.(string); ok {
					stack = append(stack, s)
				}
			}
			if len(stack) > 0 {
				lines = append(lines, fmt.Sprintf("- tech stack: %s", strings.Join(stack, ", ")))
			}
		}
	}

	if len(filtered) > 0 {
		byCat := make(map[string]map[string]string)
		for _, e := range filtered {
			if byCat[e.Category] == nil {
				byCat[e.Category] = make(map[string]string)
			}
			byCat[e.Category][e.Key] = e.Value
		}

		for k, v := range byCat["user"] {
			lines = append(lines, fmt.Sprintf("- user %s: %s (current session)", k, v))
		}

		var tech []string
		for _, v := range byCat["tech_stack"] {
			tech = append(tech, v)
		}
		sort.Strings(tech)
		if len(tech) > 0 {
			lines = append(lines, fmt.Sprintf("- tech stack: %s (current session)", strings.Join(tech, ", ")))
		}

		for k, v := range byCat["preference"] {
			lines = append(lines, fmt.Sprintf("- preference %s: %s (current session)", k, v))
		}
		for _, v := range byCat["project"] {
			lines = append(lines, fmt.Sprintf("- project: %s (current session)", v))
		}
	}

	if len(lines) <= 1 {
		return ""
	}
	return strings.Join(lines, "\n")
}

type MemoryStats struct {
	Enabled   bool  `json:"enabled"`
	Entries   int   `json:"entries"`
	Snapshots int   `json:"snapshots"`
	DBSize    int64 `json:"db_size"`
}

func (s *MemoryStore) GetStats() MemoryStats {
	db, err := s.open()
	if err != nil {
		return MemoryStats{}
	}

	var stats MemoryStats
	stats.Enabled = s.GetEnabled()

	db.QueryRow("SELECT COUNT(*) FROM memory_entries").Scan(&stats.Entries)
	db.QueryRow("SELECT COUNT(*) FROM memory_snapshots").Scan(&stats.Snapshots)

	if fi, err := os.Stat(s.DBPath); err == nil {
		stats.DBSize = fi.Size()
	}

	return stats
}
