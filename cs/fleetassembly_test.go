//go:build !wasi && !wasm

package cs

import (
	"maps"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func assemblyOrder(u *testUniverse, quantity int, build bool) FleetAssembly {
	return FleetAssembly{Name: "Task Force", RallyPlanetNum: u.Planet("Yard").Num, Designs: map[int]int{u.Player(1).Designs[0].Num: quantity}, SourceFleets: map[int]bool{u.Fleet("Fleet").Num: true}, BuildMissing: build, Revision: 1}
}
func queueAssemblyCount(u *testUniverse) int {
	n := 0
	for _, p := range u.Game.Planets {
		for _, item := range p.ProductionQueue {
			if item.Tags[assemblyTag] != "" {
				n += item.Quantity
			}
		}
	}
	return n
}
func TestFleetAssemblyTakesOnlyRequiredShipsAndPreservesDamageFuel(t *testing.T) {
	u := maintenanceUniverse(t)
	p, f := u.Player(1), u.Fleet("Fleet")
	f.Tokens[0].Quantity = 5
	f.Tokens[0].QuantityDamaged = 3
	f.Tokens[0].Damage = 7
	f.Spec = ComputeFleetSpec(&u.Game.Rules, p, f)
	f.Fuel = 120
	p.Automation.FleetAssemblies = map[int]FleetAssembly{1: assemblyOrder(u, 2, false)}
	u.Run((*turnGenerator).planFleetAssemblies)
	members := u.turn.assemblyMembers(p, 1)
	require.Len(t, members, 1)
	require.Equal(t, 2, members[0].Spec.TotalShips)
	require.Equal(t, 3, f.Spec.TotalShips)
	require.Empty(t, f.GetTag(assemblyTag))
	require.Equal(t, 120, f.Fuel+members[0].Fuel)
	damage := 0.0
	for _, fleet := range u.Game.Fleets {
		for _, token := range fleet.Tokens {
			damage += float64(token.QuantityDamaged) * token.Damage
		}
	}
	require.Equal(t, 21.0, damage)
	require.Zero(t, queueAssemblyCount(u))
}
func TestFleetAssemblyBuildsOnlyMissingAndDoesNotReplaceLaterLosses(t *testing.T) {
	u := maintenanceUniverse(t)
	p := u.Player(1)
	p.Automation.FleetAssemblies = map[int]FleetAssembly{1: assemblyOrder(u, 5, true)}
	require.NoError(t, u.turn.generateTurn())
	plan := p.Automation.FleetAssemblies[1]
	require.Equal(t, 1, plan.CompletedRevision)
	require.Equal(t, 1, plan.NotifiedRevision)
	leader := u.Game.getFleet(p.Num, plan.LeaderFleetNum)
	require.Equal(t, 5, leader.Spec.TotalShips)
	require.Zero(t, queueAssemblyCount(u))
	seventy, hundred := 0, 0
	for _, m := range p.Messages {
		if strings.Contains(m.Text, "70% of the ordered") {
			seventy++
		}
		if strings.Contains(m.Text, "100% of the ordered") {
			hundred++
		}
	}
	require.Equal(t, 1, seventy)
	require.Equal(t, 1, hundred)
	leader.Tokens[0].Quantity = 3
	leader.Spec = ComputeFleetSpec(&u.Game.Rules, p, leader)
	u.Run((*turnGenerator).planFleetAssemblies)
	require.Zero(t, queueAssemblyCount(u))
	require.Equal(t, 3, leader.Spec.TotalShips)
	plan = p.Automation.FleetAssemblies[1]
	plan.Revision++
	plan.Designs = map[int]int{p.Designs[0].Num: 6}
	p.Automation.FleetAssemblies[1] = plan
	require.NoError(t, u.turn.generateTurn())
	plan = p.Automation.FleetAssemblies[1]
	require.Equal(t, 2, plan.CompletedRevision)
	require.Equal(t, 6, u.Game.getFleet(p.Num, plan.LeaderFleetNum).Spec.TotalShips)
}
func TestFleetAssemblyQueueIsIdempotentAndCanBeReducedOrCancelled(t *testing.T) {
	u := maintenanceUniverse(t)
	p := u.Player(1)
	p.Automation.FleetAssemblies = map[int]FleetAssembly{1: assemblyOrder(u, 10, true)}
	u.Run((*turnGenerator).planFleetAssemblies)
	require.Equal(t, 8, queueAssemblyCount(u))
	u.Run((*turnGenerator).planFleetAssemblies)
	require.Equal(t, 8, queueAssemblyCount(u))
	plan := p.Automation.FleetAssemblies[1]
	plan.Revision++
	plan.Designs = map[int]int{p.Designs[0].Num: 4}
	p.Automation.FleetAssemblies[1] = plan
	u.Run((*turnGenerator).planFleetAssemblies)
	require.Equal(t, 2, queueAssemblyCount(u))
	delete(p.Automation.FleetAssemblies, 1)
	u.Run((*turnGenerator).planFleetAssemblies)
	require.Zero(t, queueAssemblyCount(u))
	require.Empty(t, u.Game.getFleet(p.Num, plan.LeaderFleetNum).GetTag(assemblyTag))
}
func TestFleetAssemblyReinforcementsFollowMovingLeaderAndNotifyOnce(t *testing.T) {
	scenario := maintenanceScenario()
	scenario.Players[0].Fleets[0].Quantity = 7
	scenario.Players[0].Fleets = append(scenario.Players[0].Fleets, ScenarioFleet{Name: "Reinforcements", Design: "Sword1", Quantity: 3, Position: Vector{80, 0}})
	u := newTestUniverse(t, scenario)
	p := u.Player(1)
	plan := assemblyOrder(u, 10, false)
	plan.SourceFleets[u.Fleet("Reinforcements").Num] = true
	p.Automation.FleetAssemblies = map[int]FleetAssembly{1: plan}
	u.Run((*turnGenerator).planFleetAssemblies)
	u.Run((*turnGenerator).resolveFleetAssemblies)
	require.Equal(t, 1, p.Automation.FleetAssemblies[1].NotifiedRevision)
	require.Zero(t, p.Automation.FleetAssemblies[1].CompletedRevision)
	leader := u.Game.getFleet(p.Num, p.Automation.FleetAssemblies[1].LeaderFleetNum)
	leader.Position = Vector{20, 0}
	u.Run((*turnGenerator).planFleetAssemblies)
	follower := u.Fleet("Reinforcements")
	require.Equal(t, leader.Position, follower.Waypoints[len(follower.Waypoints)-1].Position)
	follower.Position = leader.Position
	u.Run((*turnGenerator).resolveFleetAssemblies)
	u.Run((*turnGenerator).resolveFleetAssemblies)
	require.Equal(t, 1, p.Automation.FleetAssemblies[1].CompletedRevision)
	require.Equal(t, 10, leader.Spec.TotalShips)
	seventy, hundred := 0, 0
	for _, m := range p.Messages {
		if strings.Contains(m.Text, "70% of the ordered") {
			seventy++
		}
		if strings.Contains(m.Text, "100% of the ordered") {
			hundred++
		}
	}
	require.Equal(t, 1, seventy)
	require.Equal(t, 1, hundred)
}
func TestFleetAssemblyEditsReleaseExcessShips(t *testing.T) {
	u := maintenanceUniverse(t)
	p := u.Player(1)
	plan := assemblyOrder(u, 2, false)
	p.Automation.FleetAssemblies = map[int]FleetAssembly{1: plan}
	u.Run((*turnGenerator).planFleetAssemblies)
	u.Run((*turnGenerator).resolveFleetAssemblies)
	plan = p.Automation.FleetAssemblies[1]
	plan.Revision++
	plan.Designs = map[int]int{p.Designs[0].Num: 1}
	p.Automation.FleetAssemblies[1] = plan
	u.Run((*turnGenerator).planFleetAssemblies)
	u.Run((*turnGenerator).resolveFleetAssemblies)
	require.Len(t, u.turn.assemblyMembers(p, 1), 1)
	require.Equal(t, 1, u.turn.assemblyMembers(p, 1)[0].Spec.TotalShips)
	total := 0
	for _, f := range u.Game.Fleets {
		if !f.Delete {
			total += f.Spec.TotalShips
		}
	}
	require.Equal(t, 2, total)
}

func TestFleetAssemblyDistributesProductionAndPrioritizesBeforeRecurringOrders(t *testing.T) {
	scenario := maintenanceScenario()
	scenario.Planets = append(scenario.Planets, ScenarioPlanet{Name: "Second Yard", Owner: 1, Starbase: DesignSpaceStation.Name, Position: Vector{30, 0}, Cargo: Cargo{Colonists: 2500, Ironium: 10000, Boranium: 10000, Germanium: 10000}})
	u := newTestUniverse(t, scenario)
	p := u.Player(1)
	for _, planet := range u.Game.Planets {
		planet.Spec.ResourcesPerYearAvailable = 50
		planet.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeAutoFactories, Quantity: 250}}
	}
	p.Automation.FleetAssemblies = map[int]FleetAssembly{1: assemblyOrder(u, 100, true)}
	u.Run((*turnGenerator).planFleetAssemblies)
	require.Equal(t, 98, queueAssemblyCount(u))
	for _, planet := range u.Game.Planets {
		at := -1
		for i, item := range planet.ProductionQueue {
			if item.Tags[assemblyTag] == "1" {
				at = i
				break
			}
		}
		require.GreaterOrEqual(t, at, 0, "both yards must receive manufacturing work")
		for _, item := range planet.ProductionQueue[:at] {
			require.NotEmpty(t, item.GetTag(starbaseBoostTag), "only forecast-approved temporary investment may precede the assembly")
		}
	}
}
func TestFleetAssemblyRejectsForgedStateAndInvalidOrders(t *testing.T) {
	u := maintenanceUniverse(t)
	p := u.Player(1)
	plan := assemblyOrder(u, 2, true)
	validate := func(value FleetAssembly) error {
		return p.ValidateFleetAssemblies(map[int]FleetAssembly{1: value}, u.Game.Fleets, u.Game.Planets)
	}
	require.NoError(t, validate(plan))
	bad := plan
	bad.LeaderFleetNum = 999
	require.Error(t, validate(bad))
	bad = plan
	bad.SourceFleets = map[int]bool{999: true}
	require.Error(t, validate(bad))
	bad = plan
	bad.Designs = map[int]int{p.Designs[0].Num: -1}
	require.Error(t, validate(bad))
	bad = plan
	bad.RallyPlanetNum = 999
	require.Error(t, validate(bad))
	p.Automation.FleetAssemblies = map[int]FleetAssembly{1: plan}
	bad = plan
	bad.Designs = maps.Clone(plan.Designs)
	bad.Designs[p.Designs[0].Num]++
	require.Error(t, validate(bad))
	bad.Revision++
	require.NoError(t, validate(bad))
	forged := ProductionQueueItem{Type: QueueItemTypeShipToken, DesignNum: p.Designs[0].Num, Quantity: 1, Tags: Tags{assemblyTag: "1"}}
	require.Error(t, validateMaintenanceQueueEdits(nil, []ProductionQueueItem{forged}))
}
