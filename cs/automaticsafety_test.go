//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAutomaticRepairReturnsWholeFleetAndResumes(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	f.Position = Vector{60, 0}
	f.OrbitingPlanetNum = 0
	original := Waypoint{Position: u.Planet("Far").Position, MapObjectTarget: u.Planet("Far").MapObject.ToTarget(), Task: WaypointTaskColonize, WarpSpeed: 5}
	f.Waypoints = []Waypoint{{Position: f.Position}, original}
	f.Tokens[0].Damage = float64(f.Tokens[0].design.Spec.Armor) / 2
	f.Tokens[0].QuantityDamaged = 1
	require.GreaterOrEqual(t, fleetDamagePercent(f), 50)
	u.Run((*turnGenerator).planAutomaticSafety)
	require.True(t, f.InAutomaticSafety())
	require.Equal(t, u.Planet("Home").Position, f.Waypoints[len(f.Waypoints)-1].Position)
	require.Len(t, f.Tokens, 1)
	require.Equal(t, 1, f.Tokens[0].Quantity)
	f.Tokens[0].Damage = 0
	f.Tokens[0].QuantityDamaged = 0
	f.Position = u.Planet("Home").Position
	u.Run((*turnGenerator).planAutomaticSafety)
	require.False(t, f.InAutomaticSafety())
	require.Equal(t, original.Position, f.Waypoints[1].Position)
	p.Automation.DisableAutomaticRepair = true
	f.Tokens[0].Damage = 50
	f.Tokens[0].QuantityDamaged = 1
	u.Run((*turnGenerator).planAutomaticSafety)
	require.False(t, f.InAutomaticSafety())
}
func TestInterceptionUsesFreshVisibleCourseAndSimultaneousApproach(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	enemy := &Fleet{MapObject: MapObject{PlayerNum: 2, Position: Vector{20, 0}, ReportAge: 0}, WarpSpeed: 5, Heading: VectorFloat64{-1, 0}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HasWeapons: true, PowerRating: 100}}}
	p.FleetIntels = []*Fleet{enemy}
	require.Positive(t, interceptionThreat(p, f))
	enemy.Heading = VectorFloat64{1, 0}
	require.Zero(t, interceptionThreat(p, f))
	enemy.Heading = VectorFloat64{-1, 0}
	enemy.ReportAge = 1
	require.Zero(t, interceptionThreat(p, f))
}

func TestKnownDesignReportsTriggerEvasionWithoutPrivateFleetSpecs(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	f.Position = Vector{20, 0}
	f.OrbitingPlanetNum = 0
	original := NewPositionWaypoint(Vector{0, 0}, 5)
	f.Waypoints = []Waypoint{{Position: f.Position}, original}
	p.Automation.FleetRoles = map[int]string{f.Num: "freighter"}
	enemy := &Fleet{MapObject: MapObject{Num: 9, PlayerNum: 2, Position: Vector{10, 0}}, WarpSpeed: 5, Heading: VectorFloat64{X: 1}, Tokens: []ShipToken{{DesignNum: 8, Quantity: 1}}}
	p.FleetIntels = []*Fleet{enemy}
	p.ShipDesignIntels = []*ShipDesign{{Num: 8, PlayerNum: 2, Hull: Destroyer.Name, Slots: []ShipDesignSlot{{Quantity: 1}}, Spec: ShipDesignSpec{PowerRating: 100}}}
	u.Run((*turnGenerator).refreshAutomaticFuelIntel)
	require.Greater(t, knownRouteThreat(p, f.Position, f.Position), 0)
	require.Greater(t, interceptionThreat(p, f), 0)
	u.Run((*turnGenerator).planAutomaticSafety)
	require.True(t, f.InAutomaticSafety())
	require.Len(t, f.Waypoints, 2)
	require.NotEqual(t, u.Planet("Home").Position, f.Waypoints[1].Position, "do not run through the pursuer on the way to a dock")
	require.Contains(t, f.GetTag(fuelStatusTag), "open space")
	u.Run((*turnGenerator).fleetMove)
	p.FleetIntels = nil
	u.Run((*turnGenerator).planAutomaticSafety)
	require.False(t, f.InAutomaticSafety())
	require.Equal(t, original.Position, f.Waypoints[1].Position)
	require.Equal(t, "freighter", p.Automation.FleetRoles[f.Num])
}

func TestStrongEscortDoesNotFleeFromAWeakerInterceptor(t *testing.T) {
	u := logisticsUniverse(t)
	p, freighter, escort := u.Player(1), u.Fleet("Transport"), u.Fleet("Escort")
	p.Automation.FleetRoles[escort.Num] = "escort"
	escort.SetTag(escortTag, fmt.Sprint(freighter.Num))
	enemy := &Fleet{MapObject: MapObject{Num: 9, PlayerNum: 2, Position: Vector{20, 0}}, WarpSpeed: 5, Heading: VectorFloat64{X: -1}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HasWeapons: true, PowerRating: 1}}}
	p.FleetIntels = []*Fleet{enemy}
	u.Run((*turnGenerator).planAutomaticSafety)
	require.False(t, freighter.InAutomaticSafety())
	require.False(t, escort.InAutomaticSafety())
}
func TestBomberWaitsForEscortAndTravelsWithIt(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignTeamster, DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Bomber", Design: DesignTeamster.Name, At: "Home", Waypoints: []ScenarioWaypoint{{To: "Enemy", Warp: 5}}}, {Name: "Escort", Design: DesignDestroyerDelta.Name, At: "Home"}}}, {}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Enemy", Owner: 2, Position: Vector{30, 0}}}})
	p, b, e := u.Player(1), u.Fleet("Bomber"), u.Fleet("Escort")
	p.PlanetIntels = []*Planet{u.Planet("Home"), u.Planet("Enemy")}
	b.Spec.Bomber = true
	b.Waypoints = []Waypoint{{Position: b.Position}, {Position: u.Planet("Enemy").Position, MapObjectTarget: u.Planet("Enemy").MapObject.ToTarget(), WarpSpeed: 5}}
	p.Automation.FleetRoles = map[int]string{b.Num: "bomber"}
	u.Run((*turnGenerator).planBomberEscorts)
	require.Len(t, b.Waypoints, 1)
	require.NotEmpty(t, b.GetTag(bomberTag))
	p.Automation.FleetRoles[e.Num] = "escort"
	u.Run((*turnGenerator).planBomberEscorts)
	require.Len(t, b.Waypoints, 2, b.GetTag(bomberTag), e.Spec.PowerRating, b.Spec.Mass)
	require.Equal(t, b.Waypoints[1].WarpSpeed, e.Waypoints[1].WarpSpeed)
	require.Equal(t, b.Waypoints[1].Position, e.Waypoints[1].Position)
}
func TestAutomaticColonizerFillsHoldAndUnloadsMinerals(t *testing.T) {
	u := colonizerUniverse(t)
	f := u.Fleet("Colony Ship")
	f.Cargo.Ironium = 3
	f.Cargo.Colonists = 5
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, f.Spec.CargoCapacity, f.Cargo.Colonists)
	require.Zero(t, f.Cargo.Ironium)
	require.Equal(t, 3, u.Planet("Home").Cargo.Ironium)
}
func TestSplitAllInheritsRolesWithoutDuplicatingJobs(t *testing.T) {
	for _, role := range []string{"colonizer:auto", "scout", "escort"} {
		t.Run(role, func(t *testing.T) {
			u := colonizerUniverse(t)
			p, f := u.Player(1), u.Fleet("Colony Ship")
			if role == "escort" {
				f.Tokens[0].design.Spec.HasWeapons = true
			}
			f.Tokens[0].Quantity = 3
			f.Spec = ComputeFleetSpec(&u.Game.Rules, p, f)
			f.Fuel = f.Spec.FuelCapacity
			p.Automation.FleetRoles[f.Num] = role
			saveScoutMission(f, &scoutMission{Origin: Vector{10, 10}, Radius: 20, Target: 3})
			children, err := NewOrderer().SplitAll(&u.Game.Rules, p, u.Game.Fleets, f)
			require.NoError(t, err)
			require.Len(t, children, 2)
			for _, child := range children {
				require.Equal(t, role, p.Automation.FleetRoles[child.Num])
				require.Empty(t, child.GetTag(colonizationTag))
				require.Empty(t, child.GetTag(escortTag))
				if role == "scout" {
					require.Zero(t, readScoutMission(child).Target)
					require.Equal(t, Vector{10, 10}, readScoutMission(child).Origin)
				}
			}
		})
	}
}
