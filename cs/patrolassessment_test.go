//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPatrolEncounterUsesCourseAndEndpointNotThirtyLyCircle(t *testing.T) {
	p := &Player{Num: 1}
	target := &Fleet{MapObject: MapObject{PlayerNum: 2, Num: 1, Position: Vector{100, 0}}, Heading: VectorFloat64{X: 1}, WarpSpeed: 6}
	// This group is 36 ly away, directly at the public next contact position.
	port := &Fleet{MapObject: MapObject{PlayerNum: 2, Num: 2, Position: Vector{136, 0}}}
	nearby := &Fleet{MapObject: MapObject{PlayerNum: 2, Num: 3, Position: Vector{100, 20}}, Heading: VectorFloat64{Y: 1}, WarpSpeed: 6}
	farIncoming := &Fleet{MapObject: MapObject{PlayerNum: 2, Num: 4, Position: Vector{200, 0}}, Heading: VectorFloat64{X: -1}, WarpSpeed: 8}
	p.FleetIntels = []*Fleet{target, port, nearby, farIncoming}
	reports := patrolEncounterReports(p, target, Vector{136, 0}, 1)
	require.Contains(t, reports, target)
	require.Contains(t, reports, port)
	require.Contains(t, reports, farIncoming)
	require.NotContains(t, reports, nearby, "nearby receding traffic is not a reinforcement")
}

func TestPatrolRejectsFiveAgainstReportedSuperiorEndpoint(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Five", Design: DesignDestroyerDelta.Name, Quantity: 5}}},
		{Designs: Designs(DesignDestroyerDelta, DesignSmallFreighter), Fleets: []ScenarioFleet{{Name: "Bait", Design: DesignSmallFreighter.Name, Quantity: 3, Position: Vector{64, 0}}, {Name: "Main force", Design: DesignDestroyerDelta.Name, Quantity: 25, Position: Vector{100, 0}}}},
	}})
	p, f, bait, main := u.Player(1), u.Fleet("Five"), u.Fleet("Bait"), u.Fleet("Main force")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	for _, enemy := range []*Fleet{bait, main} {
		p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
		p.discoverer.discoverFleet(enemy, true)
	}
	report := p.GetFleetIntel(2, bait.Num)
	report.Heading, report.WarpSpeed = VectorFloat64{X: 1}, 6
	u.Run((*turnGenerator).startPatrolEvaluation)
	savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: 2, Target: bait.Num})
	require.False(t, patrolCanAttack(p, f))
	f.Waypoints = []Waypoint{{Position: f.Position}, {Position: main.Position, WarpSpeed: 10}}
	u.Run(func(tg *turnGenerator) { require.False(t, tg.patrolDepartureConfirmed(p, f)) })
	f.Waypoints[1].MapObjectTarget = bait.MapObject.ToTarget()
	u.Run(func(tg *turnGenerator) { require.False(t, tg.patrolPursuitSafe(f, main.Position, 100)) })
	// The same two checks must also stop a large uncertain group.
	p.GetShipDesignIntel(2, main.Tokens[0].DesignNum).Slots = nil
	u.Run(func(tg *turnGenerator) { require.False(t, tg.patrolDepartureConfirmed(p, f)) })
}

func TestPatrolGatheringPassesKnownUnarmedTrafficButNotEscorts(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Gathering", Design: DesignDestroyerDelta.Name}}},
		{Designs: Designs(DesignSmallFreighter, DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Traffic", Design: DesignSmallFreighter.Name, Position: Vector{16, 0}}, {Name: "Escort", Design: DesignDestroyerDelta.Name, Quantity: 20, Position: Vector{80, 0}}}},
	}})
	p, f, traffic, escort := u.Player(1), u.Fleet("Gathering"), u.Fleet("Traffic"), u.Fleet("Escort")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	p.discoverer.discoverDesign(traffic.Tokens[0].design, true)
	p.discoverer.discoverFleet(traffic, true)
	u.Run((*turnGenerator).startPatrolEvaluation)
	// No forecast is available: that cannot turn fully known noncombatants
	// into a reason to halt a funded gathering leg.
	p.patrolEvaluator.Limit = 0
	f.Waypoints = []Waypoint{{Position: f.Position}, {Position: Vector{32, 0}, WarpSpeed: 4}}
	savePatrolJob(f, patrolJob{Leader: f.Num, Provisional: true, Rally: Vector{32, 0}})
	u.Run(func(tg *turnGenerator) { require.True(t, tg.patrolDepartureConfirmed(p, f)) })
	// An attack/cleanup still requires its normal useful-victory forecast.
	job := readPatrolJob(f)
	job.CleanupTarget = traffic.Num
	savePatrolJob(f, job)
	u.Run(func(tg *turnGenerator) { require.False(t, tg.patrolDepartureConfirmed(p, f)) })
	job.CleanupTarget = 0
	savePatrolJob(f, job)
	design := p.GetShipDesignIntel(2, traffic.Tokens[0].DesignNum)
	slots := design.Slots
	design.Slots = nil
	trafficReport := p.GetFleetIntel(2, traffic.Num)
	require.False(t, harmlessPatrolTransit(&u.Game.Rules, p, trafficReport, traffic.Position))
	trafficReport.Tokens[0].Quantity = 8
	u.Run(func(tg *turnGenerator) { require.False(t, tg.patrolDepartureConfirmed(p, f)) })
	trafficReport.Tokens[0].Quantity = 1
	design.Slots = slots
	p.discoverer.discoverDesign(escort.Tokens[0].design, true)
	p.discoverer.discoverFleet(escort, true)
	report := p.GetFleetIntel(2, escort.Num)
	report.Heading, report.WarpSpeed = VectorFloat64{X: -1}, 8
	require.Contains(t, patrolEncounterReports(p, trafficReport, traffic.Position, 1), report)
	require.True(t, patrolReportedCombatant(p, report))
	u.Run(func(tg *turnGenerator) {
		require.False(t, tg.patrolDepartureConfirmed(p, f), "converging armed escort blocks transit")
	})
}

func TestCopied2459LossOrdersRejectFiveVsTwentyEight(t *testing.T) {
	path := os.Getenv("STARS_PATROL_LOSS_SNAPSHOT")
	if path == "" {
		t.Skip("optional read-only archived orders copy")
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var game FullGame
	require.NoError(t, json.Unmarshal(raw, &game))
	game.TechStore = &StaticTechStore
	game.Rules.SetTechStore(&StaticTechStore)
	game.Universe.setLogger(testLogger)
	require.NoError(t, game.computeSpecs())
	tg := newTurnGenerator(&game)
	tg.ordersPlayerNum = 1
	tg.startPatrolEvaluation()
	p, f := game.getPlayer(1), game.getFleet(1, 166)
	require.Equal(t, 5, f.Spec.TotalShips)
	enemy := p.GetFleetIntel(12, 25)
	require.NotNil(t, enemy)
	point := Vector{1694, 149}
	reports := patrolEncounterReports(p, enemy, point, 1)
	ships := 0
	for _, report := range reports {
		ships += reportedFleetShips(report)
	}
	require.Equal(t, 28, ships)
	require.False(t, patrolCanAttack(p, f), "the public relative course must reject the initial attack too")
	publicShips := 0
	for _, report := range patrolEncounterReports(p, enemy, patrolInterceptPosition(enemy), 1) {
		publicShips += reportedFleetShips(report)
	}
	require.Equal(t, 28, publicShips, "stationary formation approached by the transport belongs to the encounter")
	require.False(t, p.patrolEvaluator.assess(p, []*Fleet{f}, enemy, point, 1).Ready)
	f.Waypoints = []Waypoint{{Position: f.Position}, {Position: point, WarpSpeed: 8}}
	require.False(t, tg.patrolDepartureConfirmed(p, f))
	require.False(t, tg.patrolPursuitSafe(f, point, 64))
	require.Equal(t, 2459, game.Year)
	t.Logf("ARCHIVED_LOSS: own=%d enemy=%d rejected before departure and bounded pursuit; samples=%d", f.Spec.TotalShips, ships, p.patrolEvaluator.Battles)
	started := time.Now()
	tg.enablePlanningThreatIndex()
	tg.refreshScoutReconOrders()
	tg.planPatrolAutomation()
	tg.guardAutomaticFuel()
	tg.planAutomaticSafety()
	tg.recoverIdleAutomaticFleets()
	tg.enforceAutomaticDeparture()
	tg.disablePlanningThreatIndex()
	require.Equal(t, 2459, game.Year)
	require.LessOrEqual(t, p.patrolEvaluator.Work, p.patrolEvaluator.Limit)
	if f = game.getFleet(1, 166); f != nil && !f.Delete && f.Spec.TotalShips == 5 {
		require.True(t, len(f.Waypoints) < 2 || f.Waypoints[1].Position != point)
	}
	t.Logf("COPIED_LARGE_ORDERS: duration=%s work=%d battles=%d cache_hits=%d", time.Since(started), p.patrolEvaluator.Work, p.patrolEvaluator.Battles, p.patrolEvaluator.Hits)
}
