//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func recaptureBaseUniverse(t *testing.T) *testUniverse {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignBattleBeam, DesignLongRangeScout, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Force", Design: DesignBattleBeam.Name, Quantity: 30, At: "Home"}, {Name: "Scout", Design: DesignLongRangeScout.Name, At: "Home"}, {Name: "Reserve", Design: DesignBattleBeam.Name, Quantity: 30, Position: Vector{1000, 0}}}},
		{Designs: Designs(DesignSpaceStation, DesignBattleBeam), Fleets: []ScenarioFleet{{Name: "Guards", Design: DesignBattleBeam.Name, Quantity: 1000, Position: Vector{400, 0}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Target", Owner: 2, Starbase: DesignSpaceStation.Name, Position: Vector{30, 0}, Cargo: Cargo{Colonists: 1500}}}})
	p := u.Player(1)
	target := *u.Planet("Target")
	target.ReportAge = 10
	p.PlanetIntels = []*Planet{u.Planet("Home"), &target}
	p.Automation.PriorityPlanets = map[int]bool{target.Num: true}
	p.Automation.FleetRoles = map[int]string{u.Fleet("Scout").Num: "scout", u.Fleet("Force").Num: "patrol"}
	u.Fleet("Scout").SetTag(priorityReconTag, strconv.Itoa(target.Num))
	return u
}

func TestRecaptureAssaultRecruitsAfterNewEnemyReinforcements(t *testing.T) {
	u := recaptureBaseUniverse(t)
	p, f, reserve, enemy, target := u.Player(1), u.Fleet("Force"), u.Fleet("Reserve"), u.Fleet("Guards"), u.Planet("Target")
	f.Tokens[0].Quantity = 5
	enemy.Tokens[0].Quantity = 15
	reserve.Tokens[0].Quantity = 60
	require.NoError(t, u.Game.computeSpecs())
	p.Automation.FleetRoles[reserve.Num] = "patrol"
	u.Fleet("Scout").Position = target.Position
	u.Run((*turnGenerator).scanColonyReconContacts, (*turnGenerator).planRecaptureAssaults)
	require.NotEmpty(t, f.GetTag(recaptureAssaultTag))
	enemy.Position = target.Position
	p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
	p.discoverer.discoverFleet(enemy, true)
	u.Run((*turnGenerator).planRecaptureAssaults)
	require.Empty(t, f.GetTag(recaptureAssaultTag), "an unsafe force must release its exclusive target claim")
	reserve.Position = f.Position
	reserve.Waypoints = []Waypoint{{Position: reserve.Position}}
	u.Run((*turnGenerator).planRecaptureAssaults)
	ready := false
	for _, member := range u.Game.getFleets(p.Num) {
		if !member.Delete && member.GetTag(recaptureAssaultTag) == strconv.Itoa(target.Num) {
			ready = true
			require.Greater(t, member.Spec.TotalShips, 1)
			require.Greater(t, len(member.Waypoints), 1)
		}
	}
	require.True(t, ready, "physically available support should permit a new safe native assessment")
}

func TestRecaptureContactTransmitsOnlyPhysicalScannerEvidence(t *testing.T) {
	u := recaptureBaseUniverse(t)
	p, f, target := u.Player(1), u.Fleet("Scout"), u.Planet("Target")
	f.Position = Vector{-1000, 0}
	f.Waypoints = []Waypoint{{Position: f.Position}, {Position: target.Position, WarpSpeed: 5}}
	u.Run((*turnGenerator).scanColonyReconContacts)
	require.Equal(t, 10, p.GetPlanetIntel(target.Num).ReportAge)
	require.Empty(t, p.GetShipDesignIntel(2, target.Spec.StarbaseDesignNum).Slots)
	f.Position = target.Position
	u.Run((*turnGenerator).scanColonyReconContacts)
	require.Zero(t, p.GetPlanetIntel(target.Num).ReportAge)
	require.NotEmpty(t, p.GetShipDesignIntel(2, target.Spec.StarbaseDesignNum).Slots)
	require.NotNil(t, reportedRecaptureBase(p, p.GetPlanetIntel(target.Num)))
}

func TestRecaptureBaseRequiresFreshCompleteIntelAndWholeSafeEncounter(t *testing.T) {
	u := recaptureBaseUniverse(t)
	p, f, scout, target := u.Player(1), u.Fleet("Force"), u.Fleet("Scout"), u.Planet("Target")
	require.Nil(t, reportedRecaptureBase(p, p.GetPlanetIntel(target.Num)))
	scout.Position = target.Position
	u.Run((*turnGenerator).scanColonyReconContacts)
	report := p.GetPlanetIntel(target.Num)
	base := reportedRecaptureBase(p, report)
	require.NotNil(t, base)
	u.Run(func(tg *turnGenerator) {
		result := tg.assessRecapture(p, []*Fleet{f}, base)
		require.True(t, result.Ready, result.Reason)
		enemy := u.Fleet("Guards")
		enemy.Position = target.Position
		p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
		p.discoverer.discoverFleet(enemy, true)
		result = tg.assessRecapture(p, []*Fleet{f}, base)
		require.False(t, result.Ready, "the base cannot hide a superior defending fleet")
		p.FleetIntels = nil
		report.ReportAge = 3
		require.Nil(t, reportedRecaptureBase(p, report))
		report.ReportAge = 0
		p.GetShipDesignIntel(2, target.Spec.StarbaseDesignNum).Slots = nil
		require.Nil(t, reportedRecaptureBase(p, report))
	})
}

func TestRecaptureAssaultSurvivesOrdinaryPatrolPlannerAndRespectsManualControl(t *testing.T) {
	u := recaptureBaseUniverse(t)
	p, f, scout, target := u.Player(1), u.Fleet("Force"), u.Fleet("Scout"), u.Planet("Target")
	scout.Position = target.Position
	u.Run((*turnGenerator).scanColonyReconContacts, (*turnGenerator).startPatrolEvaluation, (*turnGenerator).planRecaptureAssaults, (*turnGenerator).planPatrolAutomation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture)
	require.Equal(t, strconv.Itoa(target.Num), f.GetTag(recaptureAssaultTag))
	require.Greater(t, len(f.Waypoints), 1)
	require.Equal(t, target.Position, f.Waypoints[1].Position)
	delete(p.Automation.FleetRoles, f.Num)
	f.Waypoints = []Waypoint{{Position: f.Position}}
	u.Run((*turnGenerator).planRecaptureAssaults)
	require.Empty(t, f.GetTag(recaptureAssaultTag))
	require.Len(t, f.Waypoints, 1)
}

func TestPriorityLandingHonorsReportedDefenseCoverageAndCurrentHabitability(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, target := u.Player(1), u.Planet("Enemy")
	p.Automation.PriorityPlanets = map[int]bool{target.Num: true}
	target.Spec.DefenseCoverage = .2
	target.Defenses = 0
	require.False(t, invasionColony(p, target))
	require.True(t, priorityNeedsBombing(target))
	target.Spec.DefenseCoverage = 0
	p.Automation.ServeTerraformingWorlds = true
	p.Race.Spec.Instaforming = false
	target.Hab = Hab{0, 0, 0}
	target.BaseHab = target.Hab
	require.LessOrEqual(t, p.Race.GetPlanetHabitability(target.Hab), 0)
	require.False(t, invasionCandidate(p, target))
}
