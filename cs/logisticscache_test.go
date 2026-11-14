package cs

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestLogisticsDecodePreservesOrders(t *testing.T) {
	fleet := &Fleet{}
	for _, raw := range []string{"", "null", "{}", "invalid", "[]", `{"kind":"supply","load":{"ironium":12},"position":{"x":42,"y":7}}`, `{"kind":"first","kind":"last"}`, `{"targetNum":"bad"}`, `{"status":"` + strings.Repeat("x", 9000) + `"}`} {
		fleet.SetTag(logisticsTag, raw)
		var want *logisticsJob
		if raw != "" {
			want = &logisticsJob{}
			if json.Unmarshal([]byte(raw), want) != nil {
				want = nil
			}
		}
		for repeat := 0; repeat < 3; repeat++ {
			got := logisticsWork(fleet)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("decode mismatch for %q", raw)
			}
			if got != nil {
				got.Status = "caller mutation"
				got.Load.Ironium = 999
				got.Position.X = 999
			}
		}
	}
	fleet.SetTag(logisticsTag, `{"kind":"original"}`)
	clone := *fleet
	clone.Tags = map[string]string{logisticsTag: `{"kind":"clone"}`}
	if logisticsWork(&clone).Kind != "clone" || logisticsWork(fleet).Kind != "original" {
		t.Fatal("copy shares decoded order")
	}
	delete(fleet.Tags, logisticsTag)
	if logisticsWork(fleet) != nil {
		t.Fatal("deleted order retained")
	}
}

func TestLogisticsDecodeConcurrentAndBounded(t *testing.T) {
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 6000; i++ {
				raw := fmt.Sprintf(`{"targetNum":%d}`, i)
				job := decodedLogisticsWork(raw)
				if job == nil || job.TargetNum != i {
					t.Error("concurrent decode mismatch")
					return
				}
			}
		}()
	}
	wg.Wait()
	for i := range logisticsDecodeCache {
		s := &logisticsDecodeCache[i]
		s.RLock()
		n := len(s.entries)
		s.RUnlock()
		if n > logisticsCacheEntries {
			t.Fatal("unbounded cache", n)
		}
	}
}

func BenchmarkLogisticsDecode(b *testing.B) {
	raw := `{"kind":"supply","targetNum":324,"load":{"ironium":120,"boranium":45,"germanium":90},"position":{"x":425,"y":728},"status":"Returning to shipyard"}`
	for _, cached := range []bool{false, true} {
		name := "original"
		if cached {
			name = "cached"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if cached {
					decodedLogisticsWork(raw)
				} else {
					job := &logisticsJob{}
					json.Unmarshal([]byte(raw), job)
				}
			}
		})
	}
}

func TestCachedLogisticsReservationsUseCurrentCargoAndOrders(t *testing.T) {
	p := &Player{}
	p.Num = 1
	f := &Fleet{}
	f.Num, f.PlayerNum = 1, 1
	gen := &turnGenerator{game: &FullGame{Universe: &Universe{Fleets: []*Fleet{f}}}}
	check := func(want int) {
		t.Helper()
		if got := gen.pendingSupplyMinerals(p, 7, false, 0).Ironium; got != want {
			t.Fatalf("reserved ironium: got %d, want %d", got, want)
		}
	}
	saveLogisticsWork(f, &logisticsJob{Kind: "return", DeliveryNum: 7})
	f.Cargo.Ironium = 30
	check(30)
	f.Cargo.Ironium = 9
	check(9)
	saveLogisticsWork(f, &logisticsJob{Kind: "planet", DeliveryNum: 7, Load: Mineral{Ironium: 21}})
	check(21)
	job := logisticsWork(f)
	job.Load.Ironium = 15
	check(21) // A caller's uncommitted edits must not leak into reservations.
	saveLogisticsWork(f, job)
	check(15)
	f.Delete = true
	check(0)
}
