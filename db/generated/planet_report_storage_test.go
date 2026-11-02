package generated

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func TestPlanetReportWriterReuseOwnsOutput(t *testing.T) {
	for worker := range 12 {
		t.Run(fmt.Sprint(worker), func(t *testing.T) {
			t.Parallel()
			original := []string{fmt.Sprint(worker), string(bytes.Repeat([]byte("report "), 2000))}
			first, err := valuePlanetReports(original)
			require.NoError(t, err)
			retained := bytes.Clone(first.([]byte))
			for i := range 20 {
				_, err := valuePlanetReports([]string{fmt.Sprint(i), original[1]})
				require.NoError(t, err)
			}
			require.Equal(t, retained, first, "later encodes must not overwrite earlier outputs")
			var decoded []string
			require.NoError(t, scanPlanetReports(first, &decoded))
			require.Equal(t, original, decoded)
		})
	}
}
