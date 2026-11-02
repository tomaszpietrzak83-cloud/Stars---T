// Hand-maintained SQL storage companion: API and archives remain ordinary JSON.
package generated

import (
	"bytes"
	"compress/gzip"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

var planetReportMagic = []byte{'T', 'S', 'J', 1, 0}

// A writer owns sizeable compression tables. Reuse those tables, never the
// returned report bytes, across independent report-encoding workers.
var planetReportWriters = sync.Pool{New: func() any {
	z, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
	return z
}}

func valuePlanetReports(item any) (driver.Value, error) {
	raw, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	if len(raw) < 4096 {
		return raw, nil
	}
	var out bytes.Buffer
	out.Write(planetReportMagic)
	z := planetReportWriters.Get().(*gzip.Writer)
	z.Reset(&out)
	defer func() {
		z.Reset(io.Discard)
		planetReportWriters.Put(z)
	}()
	if _, err = z.Write(raw); err != nil {
		return nil, err
	}
	if err = z.Close(); err != nil {
		return nil, err
	}
	if out.Len()*10 >= len(raw)*9 {
		return raw, nil
	}
	return out.Bytes(), nil
}
func scanPlanetReports(src any, dest any) error {
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return scanJSON(src, dest)
	}
	if !bytes.HasPrefix(raw, planetReportMagic) {
		return scanJSON(src, dest)
	}
	z, err := gzip.NewReader(bytes.NewReader(raw[len(planetReportMagic):]))
	if err != nil {
		return fmt.Errorf("compressed planet reports: %w", err)
	}
	defer z.Close()
	const limit = 256 << 20
	decoded, err := io.ReadAll(io.LimitReader(z, limit+1))
	if err != nil {
		return fmt.Errorf("compressed planet reports: %w", err)
	}
	if len(decoded) > limit {
		return fmt.Errorf("compressed planet reports exceed %d bytes", limit)
	}
	return json.Unmarshal(decoded, dest)
}
