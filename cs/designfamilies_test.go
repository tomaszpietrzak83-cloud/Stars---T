//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDesignUpgradeFamilyPointsAllPreviousVersionsToLatest(t *testing.T) {
	u := colonizerUniverse(t)
	p := u.Player(1)
	source := p.Designs[0]
	next := *source
	next.Num = 10
	next.Name = "Mark 2"
	require.NoError(t, p.RegisterDesignUpgrade(source, &next))
	require.Equal(t, next.Num, p.DesignMaintenance[source.Num])
	require.Equal(t, 2, next.Version)
	p.Designs = append(p.Designs, &next)
	final := next
	final.Num = 11
	final.Name = "Mark 3"
	require.NoError(t, p.RegisterDesignUpgrade(&next, &final))
	p.Designs = append(p.Designs, &final)
	require.Equal(t, final.Num, p.DesignMaintenance[source.Num])
	require.Equal(t, final.Num, p.DesignMaintenance[next.Num])
	require.Zero(t, p.DesignMaintenance[final.Num])
	require.Equal(t, 3, final.Version)
	require.Equal(t, p.Automation.DesignFamilies[source.Num], p.Automation.DesignFamilies[final.Num])
	require.Less(t, p.Automation.DesignFamilies[final.Num], 0)
	require.NoError(t, p.ValidateDesignMaintenance(p.DesignMaintenance))
	wrong := final
	wrong.Num = 12
	wrong.Hull = "Cruiser"
	require.Error(t, p.RegisterDesignUpgrade(&final, &wrong))
}
