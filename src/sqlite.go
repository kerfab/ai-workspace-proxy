package main

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>

static sqlite3_destructor_type transient_destructor() {
	return SQLITE_TRANSIENT;
}
*/
import "C"

import (
	"fmt"
	"sync"
	"time"
	"unsafe"
)

type SQLiteDB struct {
	mu sync.Mutex
	db *C.sqlite3
}

func OpenSQLite(path string) (*SQLiteDB, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	var db *C.sqlite3
	flags := C.int(C.SQLITE_OPEN_READWRITE | C.SQLITE_OPEN_CREATE | C.SQLITE_OPEN_FULLMUTEX)
	if rc := C.sqlite3_open_v2(cpath, &db, flags, nil); rc != C.SQLITE_OK {
		if db != nil {
			_ = C.sqlite3_close(db)
		}
		return nil, fmt.Errorf("sqlite open failed: %s", sqliteErr(db, rc))
	}

	sdb := &SQLiteDB{db: db}
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=5000;",
	} {
		if err := sdb.Exec(pragma); err != nil {
			_ = sdb.Close()
			return nil, err
		}
	}
	return sdb, nil
}

func (s *SQLiteDB) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	if rc := C.sqlite3_close(s.db); rc != C.SQLITE_OK {
		return fmt.Errorf("sqlite close failed: %s", sqliteErr(s.db, rc))
	}
	s.db = nil
	return nil
}

func (s *SQLiteDB) Exec(query string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepare(query)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindArgs(stmt, args...); err != nil {
		return err
	}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return nil
		}
		if rc == C.SQLITE_ROW {
			continue
		}
		return fmt.Errorf("sqlite exec failed: %s", sqliteErr(s.db, rc))
	}
}

func (s *SQLiteDB) Query(query string, args ...any) ([]map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepare(query)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindArgs(stmt, args...); err != nil {
		return nil, err
	}
	return readAllRows(s.db, stmt)
}

func (s *SQLiteDB) QueryOne(query string, args ...any) (map[string]string, error) {
	rows, err := s.Query(query, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (s *SQLiteDB) prepare(query string) (*C.sqlite3_stmt, error) {
	cquery := C.CString(query)
	defer C.free(unsafe.Pointer(cquery))
	var stmt *C.sqlite3_stmt
	if rc := C.sqlite3_prepare_v2(s.db, cquery, -1, &stmt, nil); rc != C.SQLITE_OK {
		return nil, fmt.Errorf("sqlite prepare failed: %s", sqliteErr(s.db, rc))
	}
	return stmt, nil
}

func bindArgs(stmt *C.sqlite3_stmt, args ...any) error {
	for i, arg := range args {
		idx := C.int(i + 1)
		switch v := arg.(type) {
		case nil:
			if rc := C.sqlite3_bind_null(stmt, idx); rc != C.SQLITE_OK {
				return fmt.Errorf("bind null failed: rc=%d", int(rc))
			}
		case string:
			cstr := C.CString(v)
			rc := C.sqlite3_bind_text(stmt, idx, cstr, -1, C.transient_destructor())
			C.free(unsafe.Pointer(cstr))
			if rc != C.SQLITE_OK {
				return fmt.Errorf("bind text failed: rc=%d", int(rc))
			}
		case int:
			if rc := C.sqlite3_bind_int64(stmt, idx, C.sqlite3_int64(v)); rc != C.SQLITE_OK {
				return fmt.Errorf("bind int failed: rc=%d", int(rc))
			}
		case int64:
			if rc := C.sqlite3_bind_int64(stmt, idx, C.sqlite3_int64(v)); rc != C.SQLITE_OK {
				return fmt.Errorf("bind int64 failed: rc=%d", int(rc))
			}
		case bool:
			n := 0
			if v {
				n = 1
			}
			if rc := C.sqlite3_bind_int64(stmt, idx, C.sqlite3_int64(n)); rc != C.SQLITE_OK {
				return fmt.Errorf("bind bool failed: rc=%d", int(rc))
			}
		case time.Time:
			ts := v.UTC().Format(time.RFC3339Nano)
			cstr := C.CString(ts)
			rc := C.sqlite3_bind_text(stmt, idx, cstr, -1, C.transient_destructor())
			C.free(unsafe.Pointer(cstr))
			if rc != C.SQLITE_OK {
				return fmt.Errorf("bind time failed: rc=%d", int(rc))
			}
		default:
			return fmt.Errorf("unsupported sqlite bind type %T", arg)
		}
	}
	return nil
}

func readAllRows(db *C.sqlite3, stmt *C.sqlite3_stmt) ([]map[string]string, error) {
	colCount := int(C.sqlite3_column_count(stmt))
	colNames := make([]string, colCount)
	for i := 0; i < colCount; i++ {
		colNames[i] = C.GoString(C.sqlite3_column_name(stmt, C.int(i)))
	}

	rows := make([]map[string]string, 0)
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			row := make(map[string]string, colCount)
			for i := 0; i < colCount; i++ {
				ctext := C.sqlite3_column_text(stmt, C.int(i))
				if ctext == nil {
					row[colNames[i]] = ""
				} else {
					row[colNames[i]] = C.GoString((*C.char)(unsafe.Pointer(ctext)))
				}
			}
			rows = append(rows, row)
		case C.SQLITE_DONE:
			return rows, nil
		default:
			return nil, fmt.Errorf("sqlite query failed: %s", sqliteErr(db, rc))
		}
	}
}

func sqliteErr(db *C.sqlite3, rc C.int) string {
	if db == nil {
		return fmt.Sprintf("sqlite rc=%d", int(rc))
	}
	return fmt.Sprintf("%s (rc=%d)", C.GoString(C.sqlite3_errmsg(db)), int(rc))
}
