//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

// A synthetic native campaign tests the entire controller sequence independently
// of the optional private database campaign. No manual orders after setup.
func TestNativeRecaptureCampaign(t *testing.T) {
	runNativeRecaptureCampaign(t, false)
}
func TestNativeRecaptureCampaignFinishesBombingBeforePendingRetirement(t *testing.T) {
	runNativeRecaptureCampaign(t, true)
}
func runNativeRecaptureCampaign(t *testing.T, pending bool) {
	bomber := campaignDesign("Landing support", B17Bomber.Name, "bomber")
	carrier := campaignDesign("Landing carrier", LargeFreighter.Name, "collector")
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Player: proposalTestPlayer(), Designs: Designs(DesignBattleBeam, DesignLongRangeScout, DesignSpaceStation, bomber, carrier), Fleets: []ScenarioFleet{
			{Name: "Force", Design: DesignBattleBeam.Name, Quantity: 30, At: "Home"},
			{Name: "Scout", Design: DesignLongRangeScout.Name, At: "Home"},
			{Name: "Bomber", Design: bomber.Name, Quantity: 3, At: "Home"},
			{Name: "Carrier", Design: carrier.Name, At: "Home"},
		}},
		{Designs: Designs(DesignSpaceStation)},
	}, Planets: []ScenarioPlanet{
		{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name, Scanner: true, Cargo: Cargo{Colonists: 10000, Ironium: 5000, Boranium: 5000, Germanium: 5000}},
		{Name: "Lost colony", Owner: 2, Starbase: DesignSpaceStation.Name, Position: Vector{30, 0}, Cargo: Cargo{Colonists: 1600}},
	}})
	p, target := u.Player(1), u.Planet("Lost colony")
	target.Hab = u.Planet("Home").Hab
	target.BaseHab = target.Hab
	intel := *target
	intel.ReportAge = 12
	p.PlanetIntels = []*Planet{u.Planet("Home"), &intel}
	p.Automation.PriorityPlanets = map[int]bool{target.Num: true}
	p.Automation.FleetRoles = map[int]string{u.Fleet("Force").Num: "patrol", u.Fleet("Scout").Num: "scout", u.Fleet("Bomber").Num: "bomber", u.Fleet("Carrier").Num: "freighter"}
	p.Automation.DisableMineralSupply = true
	b := u.Fleet("Bomber")
	if pending {
		p.DesignMaintenance = map[int]int{b.Tokens[0].DesignNum: RetireDesign}
		saved := bomberOrders{Orders: FleetOrders{Waypoints: []Waypoint{{Position: b.Position}, {Position: target.Position, MapObjectTarget: target.ToTarget(), WarpSpeed: 5}}}}
		raw, _ := json.Marshal(saved)
		b.SetTag(bomberTag, string(raw))
	}
	u.Game.Rules.ResetSeed(4231)
	refreshed, cleared, bombed := false, false, false
	for n := 0; n < 14 && target.PlayerNum != p.Num; n++ {
		prior := target.Cargo.Colonists
		u.GenerateTurn()
		refreshed = refreshed || p.GetPlanetIntel(target.Num).ReportAge == 0
		cleared = cleared || !target.Spec.HasStarbase
		bombed = bombed || target.PlayerNum != p.Num && target.Cargo.Colonists < prior
		if pending && target.PlayerNum != p.Num {
			require.False(t, b.InDesignMaintenance(), "a pending retirement must not interrupt ongoing bombardment")
			require.Equal(t, target.Num, bomberGoal(b), "arrival must retain the mission identity")
		}
		t.Logf("RECAPTURE_NATIVE year=%d owner=%d base=%v population=%d reportAge=%d", u.Game.Year, target.PlayerNum, target.Spec.HasStarbase, target.GetPopulation(), p.GetPlanetIntel(target.Num).ReportAge)
	}
	require.True(t, refreshed)
	require.True(t, cleared)
	require.True(t, bombed)
	require.Equal(t, p.Num, target.PlayerNum, "fresh reconnaissance must lead through base destruction and bombing to a native landing")
	require.Positive(t, target.Cargo.Colonists)
	if pending {
		u.GenerateTurn()
		require.True(t, b.InDesignMaintenance() || b.Delete, "maintenance resumes after the target is captured")
	}
}
