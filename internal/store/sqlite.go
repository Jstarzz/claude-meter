package store

/*
#cgo LDFLAGS: -lsqlite3
#include <stdlib.h>
#include <sqlite3.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

type sqliteDB struct{ ptr *C.sqlite3 }
type sqliteStmt struct {
	ptr *C.sqlite3_stmt
	db  *sqliteDB
}

func openSQLite(path string) (*sqliteDB, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	var ptr *C.sqlite3
	flags := C.int(C.SQLITE_OPEN_READWRITE | C.SQLITE_OPEN_CREATE | C.SQLITE_OPEN_FULLMUTEX)
	if rc := C.sqlite3_open_v2(cPath, &ptr, flags, nil); rc != C.SQLITE_OK {
		msg := sqliteError(ptr)
		if ptr != nil {
			C.sqlite3_close(ptr)
		}
		return nil, fmt.Errorf("open sqlite: %s", msg)
	}
	return &sqliteDB{ptr: ptr}, nil
}

func (db *sqliteDB) close() error {
	if db == nil || db.ptr == nil {
		return nil
	}
	if rc := C.sqlite3_close(db.ptr); rc != C.SQLITE_OK {
		return fmt.Errorf("close sqlite: %s", sqliteError(db.ptr))
	}
	db.ptr = nil
	return nil
}

func (db *sqliteDB) exec(query string) error {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))
	var errMsg *C.char
	rc := C.sqlite3_exec(db.ptr, cQuery, nil, nil, &errMsg)
	if rc == C.SQLITE_OK {
		return nil
	}
	msg := sqliteError(db.ptr)
	if errMsg != nil {
		msg = C.GoString(errMsg)
		C.sqlite3_free(unsafe.Pointer(errMsg))
	}
	return fmt.Errorf("sqlite exec: %s", msg)
}

func (db *sqliteDB) prepare(query string) (*sqliteStmt, error) {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))
	var ptr *C.sqlite3_stmt
	if rc := C.sqlite3_prepare_v2(db.ptr, cQuery, -1, &ptr, nil); rc != C.SQLITE_OK {
		return nil, fmt.Errorf("sqlite prepare: %s", sqliteError(db.ptr))
	}
	return &sqliteStmt{ptr: ptr, db: db}, nil
}

func (db *sqliteDB) changes() int { return int(C.sqlite3_changes(db.ptr)) }

func (s *sqliteStmt) close() { C.sqlite3_finalize(s.ptr) }
func (s *sqliteStmt) reset() {
	C.sqlite3_reset(s.ptr)
	C.sqlite3_clear_bindings(s.ptr)
}
func (s *sqliteStmt) step() (bool, bool, error) {
	rc := C.sqlite3_step(s.ptr)
	if rc == C.SQLITE_ROW {
		return true, false, nil
	}
	if rc == C.SQLITE_DONE {
		return false, true, nil
	}
	return false, false, fmt.Errorf("sqlite step (%d): %s", int(rc), sqliteError(s.db.ptr))
}
func (s *sqliteStmt) bind(values ...any) error {
	for i, value := range values {
		idx := C.int(i + 1)
		var rc C.int
		switch v := value.(type) {
		case string:
			c := C.CString(v)
			rc = C.sqlite3_bind_text(s.ptr, idx, c, C.int(len(v)), C.SQLITE_TRANSIENT)
			C.free(unsafe.Pointer(c))
		case int:
			rc = C.sqlite3_bind_int64(s.ptr, idx, C.sqlite3_int64(v))
		case int64:
			rc = C.sqlite3_bind_int64(s.ptr, idx, C.sqlite3_int64(v))
		case nil:
			rc = C.sqlite3_bind_null(s.ptr, idx)
		default:
			return fmt.Errorf("unsupported sqlite bind type %T", value)
		}
		if rc != C.SQLITE_OK {
			return fmt.Errorf("sqlite bind parameter %d failed", i+1)
		}
	}
	return nil
}
func (s *sqliteStmt) text(i int) string {
	p := C.sqlite3_column_text(s.ptr, C.int(i))
	if p == nil {
		return ""
	}
	return C.GoString((*C.char)(unsafe.Pointer(p)))
}
func (s *sqliteStmt) int64(i int) int64 { return int64(C.sqlite3_column_int64(s.ptr, C.int(i))) }

func sqliteError(db *C.sqlite3) string {
	if db == nil {
		return "unknown sqlite error"
	}
	return C.GoString(C.sqlite3_errmsg(db))
}
