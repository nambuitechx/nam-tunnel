package relay_models

import (
	"database/sql"
	"time"
)

func unixNow() int64 {
	return time.Now().UTC().Unix()
}

func timeFromEpoch(sec int64) time.Time {
	return time.Unix(sec, 0).UTC()
}

func timeFromNullEpoch(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := timeFromEpoch(v.Int64)
	return &t
}
