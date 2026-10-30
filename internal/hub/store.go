package hub

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // store choice: modernc.org/sqlite, pure Go, no cgo
)

// Store is the local task state. It replaces progress.md and survives restarts.
type Store struct {
	db *sql.DB
}

const schema = `CREATE TABLE IF NOT EXISTS tasks (
	number       INTEGER PRIMARY KEY,
	epic         INTEGER,
	claimed_by   TEXT,
	state        TEXT,
	batch        INTEGER,
	revise_count INTEGER NOT NULL DEFAULT 0,
	worktree     TEXT,
	pane         TEXT
);
CREATE TABLE IF NOT EXISTS events (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	received_at TEXT NOT NULL,
	hook        TEXT NOT NULL,
	role        TEXT NOT NULL,
	pane        TEXT NOT NULL,
	session     TEXT,
	payload     TEXT
);
CREATE TABLE IF NOT EXISTS reports (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TEXT NOT NULL,
	issue      INTEGER NOT NULL,
	role       TEXT NOT NULL,
	status     TEXT NOT NULL,
	summary    TEXT NOT NULL
)`

// OpenStore opens or creates the SQLite file at path.
func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=locking_mode(EXCLUSIVE)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	// One connection holds the exclusive file lock for the life of the Store,
	// so a second process (or a second OpenStore) gets SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	// BEGIN EXCLUSIVE takes the write lock now; CREATE IF NOT EXISTS alone only reads an existing file.
	if _, err := db.Exec("BEGIN EXCLUSIVE; " + schema + "; COMMIT"); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// Claim sets owner on a sub-issue if nobody holds it. A single UPDATE is atomic in SQLite,
// so of several concurrent claims exactly one returns true.
func (s *Store) Claim(number, epic int, owner string) (bool, error) {
	// A row made first by IncrementRevise or SetTask has no epic yet; fill it in, never overwrite.
	if _, err := s.db.Exec(`INSERT INTO tasks (number, epic) VALUES (?, ?)
		ON CONFLICT(number) DO UPDATE SET epic = excluded.epic WHERE epic IS NULL`, number, epic); err != nil {
		return false, err
	}
	res, err := s.db.Exec(`UPDATE tasks SET claimed_by = ? WHERE number = ? AND claimed_by IS NULL`, owner, number)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// Release clears the claim, but only when owner holds it.
func (s *Store) Release(number int, owner string) error {
	_, err := s.db.Exec(`UPDATE tasks SET claimed_by = NULL WHERE number = ? AND claimed_by = ?`, number, owner)
	return err
}

// Claims returns the current claim owner of every claimed sub-issue.
func (s *Store) Claims() (map[int]string, error) {
	rows, err := s.db.Query(`SELECT number, claimed_by FROM tasks WHERE claimed_by IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]string{}
	for rows.Next() {
		var number int
		var owner string
		if err := rows.Scan(&number, &owner); err != nil {
			return nil, err
		}
		out[number] = owner
	}
	return out, rows.Err()
}

// IncrementRevise adds one REVISE attempt and returns the new count.
func (s *Store) IncrementRevise(number int) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR IGNORE INTO tasks (number) VALUES (?)`, number); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`UPDATE tasks SET revise_count = revise_count + 1 WHERE number = ?`, number); err != nil {
		return 0, err
	}
	var count int
	if err := tx.QueryRow(`SELECT revise_count FROM tasks WHERE number = ?`, number).Scan(&count); err != nil {
		return 0, err
	}
	return count, tx.Commit()
}

// Task is the per-sub-issue state the hub tracks besides the claim.
type Task struct {
	State    string
	Batch    int
	Worktree string
	Pane     string
}

// SetTask writes state, batch, worktree and pane for a sub-issue.
func (s *Store) SetTask(number int, t Task) error {
	_, err := s.db.Exec(`INSERT INTO tasks (number, state, batch, worktree, pane) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(number) DO UPDATE SET state = excluded.state, batch = excluded.batch,
		worktree = excluded.worktree, pane = excluded.pane`, number, t.State, t.Batch, t.Worktree, t.Pane)
	return err
}

// Task returns state, batch, worktree and pane for a sub-issue; unset fields are zero.
func (s *Store) Task(number int) (Task, error) {
	var t Task
	err := s.db.QueryRow(`SELECT COALESCE(state, ''), COALESCE(batch, 0), COALESCE(worktree, ''), COALESCE(pane, '')
		FROM tasks WHERE number = ?`, number).Scan(&t.State, &t.Batch, &t.Worktree, &t.Pane)
	if err == sql.ErrNoRows {
		return Task{}, nil
	}
	return t, err
}

// Event is one hook event as the hub stored it.
type Event struct {
	ReceivedAt time.Time
	Hook       string
	Role       string
	Pane       string
	Session    string
	Payload    string
}

// RecordEvent stores one hook event.
func (s *Store) RecordEvent(e Event) error {
	_, err := s.db.Exec(`INSERT INTO events (received_at, hook, role, pane, session, payload) VALUES (?, ?, ?, ?, ?, ?)`,
		e.ReceivedAt.UTC().Format(time.RFC3339Nano), e.Hook, e.Role, e.Pane, e.Session, e.Payload)
	return err
}

// Events returns the stored events for one pane, oldest first.
func (s *Store) Events(pane string) ([]Event, error) {
	rows, err := s.db.Query(`SELECT received_at, hook, role, pane, COALESCE(session, ''), COALESCE(payload, '')
		FROM events WHERE pane = ? ORDER BY id`, pane)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var at string
		if err := rows.Scan(&at, &e.Hook, &e.Role, &e.Pane, &e.Session, &e.Payload); err != nil {
			return nil, err
		}
		if e.ReceivedAt, err = time.Parse(time.RFC3339Nano, at); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// RecordReport stores one status report a session made with the report tool.
func (s *Store) RecordReport(issue int, role, status, summary string) error {
	_, err := s.db.Exec(`INSERT INTO reports (created_at, issue, role, status, summary) VALUES (?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339Nano), issue, role, status, summary)
	return err
}

// Reports returns the stored report count for one sub-issue.
func (s *Store) Reports(issue int) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM reports WHERE issue = ?`, issue).Scan(&n)
	return n, err
}

// LatestPerPane returns the most recent stored event of each pane.
func (s *Store) LatestPerPane() ([]Event, error) {
	rows, err := s.db.Query(`SELECT received_at, hook, role, pane, COALESCE(session, ''), COALESCE(payload, '')
		FROM events WHERE id IN (SELECT MAX(id) FROM events GROUP BY pane) ORDER BY pane`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var at string
		if err := rows.Scan(&at, &e.Hook, &e.Role, &e.Pane, &e.Session, &e.Payload); err != nil {
			return nil, err
		}
		if e.ReceivedAt, err = time.Parse(time.RFC3339Nano, at); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SubIssueForPane returns the sub-issue whose task runs in pane, or 0 if none does.
func (s *Store) SubIssueForPane(pane string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT number FROM tasks WHERE pane = ? ORDER BY number LIMIT 1`, pane).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return n, err
}
