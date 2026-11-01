package generated

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPlanetReportStorageLosslessAndLegacy(t *testing.T) {
	original := []string{string(bytes.Repeat([]byte("planet repeated text "), 10000)), "Gravity 0.24; 温度; \u2605"}
	encoded, err := valuePlanetReports(original)
	require.NoError(t, err)
	raw := encoded.([]byte)
	require.True(t, bytes.HasPrefix(raw, planetReportMagic))
	plain, err := json.Marshal(original)
	require.NoError(t, err)
	require.Less(t, len(raw), len(plain)/5)
	for _, source := range []any{raw, plain, string(plain)} {
		var decoded []string
		require.NoError(t, scanPlanetReports(source, &decoded))
		require.Equal(t, original, decoded)
	}
	var decoded []string
	require.Error(t, scanPlanetReports(append(append([]byte{}, planetReportMagic...), 1, 2), &decoded))
	require.NoError(t, scanPlanetReports(nil, &decoded))
}
