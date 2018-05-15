package database

import (
	"github.com/boltdb/bolt"
	"github.com/gelembjuk/democoin/lib/utils"
)

type Cursor struct {
	bucket  string
	DB      *BoltDB
	cursor  *bolt.Cursor
	lastkey []byte
}

func (i *Cursor) Next() ([]byte, []byte, error) {
	if i.cursor == nil {
		// create cursor
		err := i.DB.db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(i.bucket))

			if b == nil {
				return NewBucketNotFoundDBError()
			}
			i.cursor = b.Cursor()

			if i.cursor == nil {
				return NewCursorDBError()
			}

			return nil
		})

		if err != nil {
			return nil, nil, err
		}
	}

	var k, v []byte

	if i.lastkey == nil {
		k, v = i.cursor.First()
	} else {
		k, v = i.cursor.Next()
	}
	if k != nil {
		i.lastkey = k

		key := utils.CopyBytes(k)
		value := utils.CopyBytes(v)

		return key, value, nil
	}
	return nil, nil, nil
}

func (i *Cursor) Count() (int, error) {
	totalnumber := 0

	for {
		k, _, err := i.Next()

		if err != nil {
			return 0, err
		}

		if k == nil {
			break
		}

		totalnumber++
	}

	return totalnumber, nil
}
