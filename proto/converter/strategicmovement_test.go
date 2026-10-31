package converter

import (
	"testing"

	"github.com/sirgwain/craig-stars/cs"
	v1 "github.com/sirgwain/craig-stars/proto/gen/craig_stars/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestStrategicEngineAndFleetSurviveAPIAndWasmRoundTrip(t *testing.T) {
	part := cs.StaticTechStore.GetHullComponent("EEE Galactic Drive")
	encoded, err := C.ConvertCSTechHullComponentP(part).MarshalVT()
	require.NoError(t, err)
	var receivedPart v1.TechHullComponent
	require.NoError(t, receivedPart.UnmarshalVT(encoded))
	require.Equal(t, 3.0, C.ConvertTechHullComponentP(&receivedPart).StrategicSpeedFactor)

	fleet := &cs.Fleet{ObservedSpeed: 300, WarpSpeed: 10, Spec: cs.FleetSpec{ShipDesignSpec: cs.ShipDesignSpec{Engine: part.Engine}}}
	for _, wasm := range []bool{false, true} {
		wire := C.ConvertCSFleet(fleet)
		var data []byte
		var err error
		if wasm {
			data, err = wire.MarshalVT()
		} else {
			data, err = proto.Marshal(wire)
		}
		require.NoError(t, err)
		var received v1.Fleet
		if wasm {
			err = received.UnmarshalVT(data)
		} else {
			err = proto.Unmarshal(data, &received)
		}
		require.NoError(t, err)
		decoded := C.ConvertFleet(&received)
		require.Equal(t, 300.0, decoded.ObservedSpeed)
		require.Equal(t, 300.0, decoded.StrategicSpeed(10))
	}
}
