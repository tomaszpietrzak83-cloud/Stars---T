package generated

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestParallelPlanetReportDecodeMatchesSerialAndKeepsOrder(t *testing.T) {
	reports := [][]byte{[]byte(`[{"num":1,"name":"One","reportAge":-1,"cargo":{"colonists":123}}]`), []byte(`null`), []byte(`[{"num":3,"name":"Three","factories":17,"tags":{"private":"kept"}}]`)}
	serial, parallel := make([]Player, 3), make([]Player, 3)
	for i := range serial {
		serial[i].Num = int64(i + 1)
		parallel[i].Num = int64(i + 1)
	}
	require.NoError(t, decodePlanetReports(context.Background(), serial, reports, 1))
	require.NoError(t, decodePlanetReports(context.Background(), parallel, reports, 8))
	a, _ := json.Marshal(serial)
	b, _ := json.Marshal(parallel)
	require.Equal(t, string(a), string(b))
	reports[1] = []byte(`invalid`)
	require.True(t, strings.Contains(decodePlanetReports(context.Background(), parallel, reports, 8).Error(), "player 2"))
}
