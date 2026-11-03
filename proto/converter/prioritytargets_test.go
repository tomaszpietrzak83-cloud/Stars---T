package converter

import (
	"github.com/sirgwain/craig-stars/cs"
	v1 "github.com/sirgwain/craig-stars/proto/gen/craig_stars/v1"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFleetPriorityProtocolRoundTrip(t *testing.T) {
	orders := cs.PlayerOrders{Automation: cs.LogisticsAutomation{PriorityFleets: map[string]bool{"2:17": true, "3:17": true}}}
	encoded, err := C.ConvertCSPlayer(&cs.Player{PlayerOrders: orders}).PlayerOrders.MarshalVT()
	require.NoError(t, err)
	var decoded v1.PlayerOrders
	require.NoError(t, decoded.UnmarshalVT(encoded))
	require.Equal(t, orders.Automation.PriorityFleets, C.ConvertPlayerOrders(&decoded).Automation.PriorityFleets)
}
