package cs

import (
	"encoding/json"
	"hash/maphash"
	"sync"
)

const logisticsCacheShards = 32
const logisticsCacheEntries = 128
const logisticsCacheMaxKey = 8192

type decodedLogisticsEntry struct {
	job   logisticsJob
	valid bool
}

type logisticsDecodeShard struct {
	sync.RWMutex
	entries map[string]decodedLogisticsEntry
}

var logisticsDecodeSeed = maphash.MakeSeed()
var logisticsDecodeCache [logisticsCacheShards]logisticsDecodeShard

// Key by the complete order, never by fleet identity: direct tag changes and
// fleet copies must see fresh orders. Entries contain values only, and callers
// receive independent copies. No fleet state is mutated by a read.
func decodedLogisticsWork(raw string) *logisticsJob {
	if len(raw) > logisticsCacheMaxKey {
		job := &logisticsJob{}
		if json.Unmarshal([]byte(raw), job) != nil {
			return nil
		}
		return job
	}
	shard := &logisticsDecodeCache[maphash.String(logisticsDecodeSeed, raw)%logisticsCacheShards]
	shard.RLock()
	entry, found := shard.entries[raw]
	shard.RUnlock()
	if !found {
		entry.valid = json.Unmarshal([]byte(raw), &entry.job) == nil
		shard.Lock()
		// Bounded memory, including invalid orders. Eviction affects cost only.
		if len(shard.entries) >= logisticsCacheEntries || shard.entries == nil {
			shard.entries = make(map[string]decodedLogisticsEntry)
		}
		shard.entries[raw] = entry
		shard.Unlock()
	}
	if !entry.valid {
		return nil
	}
	return &entry.job
}
