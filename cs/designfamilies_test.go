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

func TestDesignUpgradeMigratesActiveCompositionButDoesNotRestartCompletedOrder(t *testing.T) {
	u := colonizerUniverse(t)
	p := u.Player(1)
	source := p.Designs[0]
	next := *source
	next.Num = 10
	p.Automation.FleetAssemblies = map[int]FleetAssembly{
		1: {Name: "Pending fleet", Revision: 2, Designs: map[int]int{source.Num: 3, next.Num: 2}},
		2: {Name: "Historical fleet", Revision: 1, CompletedRevision: 1, Designs: map[int]int{source.Num: 4}},
	}
	require.NoError(t, p.RegisterDesignUpgrade(source, &next))
	p.Designs = append(p.Designs, &next)
	plan := p.Automation.FleetAssemblies[1]
	require.Equal(t, map[int]int{next.Num: 5}, plan.Designs)
	require.Equal(t, 3, plan.Revision)
	require.Zero(t, plan.CompletedRevision)
	require.Equal(t, map[int]int{source.Num: 4}, p.Automation.FleetAssemblies[2].Designs)
	require.Equal(t, 1, p.Automation.FleetAssemblies[2].Revision)
	require.NoError(t, p.ValidateFleetAssemblies(p.Automation.FleetAssemblies, nil, nil))
	third := next
	third.Num = 11
	require.NoError(t, p.RegisterDesignUpgrade(&next, &third))
	require.Equal(t, map[int]int{third.Num: 5}, p.Automation.FleetAssemblies[1].Designs)
	require.Equal(t, 4, p.Automation.FleetAssemblies[1].Revision)
}
