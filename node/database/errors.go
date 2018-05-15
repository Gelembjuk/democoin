package database

// Custom errors

import (
	"fmt"
)

const TXVerifyErrorNoInput = "noinput"

type DBError struct {
	err  string
	kind string
}

func (e *DBError) Error() string {
	return fmt.Sprintf("Database Error: %s", e.err)
}

func NewDBError(err string, kind string) error {
	return &DBError{err, kind}
}

func NewBucketNotFoundDBError() error {
	return &DBError{"Bucket is not found", "bucket"}
}

func NewCursorDBError() error {
	return &DBError{"Can not get cursor", "cursor"}
}

func NewNotFoundDBError(kind string) error {
	return &DBError{"Not found", kind}
}

func NewDBIsNotReadyError() error {
	return &DBError{"Database is not ready", "database"}
}
