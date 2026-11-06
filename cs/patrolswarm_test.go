//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatrolFivePercentCountsAndValueAreIndependent(t *testing.T) {
	good := patrolBattleOutcome{MissionSucceeded: true, OwnSurvivors: 19, LostShips: 1, LostValue: 50, DamageValue: 400}
	require.True(t, good.safe(20, 1000), "5%% inclusive; survivor damage is recorded separately")
	require.False(t, good.safe(19, 1000), "one loss in nineteen exceeds the count limit")
	good.LostValue = 51
	require.False(t, good.safe(20, 1000), "cheap hull counts cannot hide a valuable loss")
	good.LostValue = 0
	good.LostShips = 0
	good.MissionSucceeded = false
	require.False(t, good.safe(20, 1000), "a harmless skirmish is not mission success")
}

func TestPatrolNativeUsefulVictoryAllowsAuxiliarySurvivors(t *testing.T) {
	bomber := campaignDesign("Escorted bomber", B17Bomber.Name, "bomber")
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignJihadCruiser), Fleets: []ScenarioFleet{{Name: "Hunters", Design: DesignJihadCruiser.Name, Quantity: 8}}},
		{Designs: Designs(DesignDestroyerDelta, bomber, DesignLongRangeScout, DesignSmallFreighter), Fleets: []ScenarioFleet{{Name: "Reported group", Position: Vector{50, 0}, Tokens: []ScenarioShipToken{{Design: DesignDestroyerDelta.Name, Quantity: 10}, {Design: bomber.Name, Quantity: 4}, {Design: DesignLongRangeScout.Name, Quantity: 3}, {Design: DesignSmallFreighter.Name, Quantity: 5}}}}},
	}})
	p, f, e := u.Player(1), u.Fleet("Hunters"), u.Fleet("Reported group")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	// An explicit player plan destroys the dangerous combatants and bombers,
	// leaving unarmed scouts as later targets. The forecast must honor it.
	p.BattlePlans = append(p.BattlePlans, BattlePlan{Num: 77, Name: "Stop bombing", Tactic: BattleTacticMaximizeDamage, PrimaryTarget: BattleTargetArmedShips, SecondaryTarget: BattleTargetBombersFreighters, AttackWho: BattleAttackWhoEnemies})
	f.BattlePlanNum = 77
	for _, d := range u.Player(2).Designs {
		p.discoverer.discoverDesign(d, true)
	}
	p.discoverer.discoverFleet(e, true)
	u.Run((*turnGenerator).startPatrolEvaluation)
	report := p.GetFleetIntel(2, e.Num)
	result := p.patrolEvaluator.assess(p, []*Fleet{f}, report, report.Position, 1)
	require.True(t, result.Ready, result.Reason)
	model, err := newPatrolBattleCopy(&u.Game.Rules, p, []*Fleet{f}, []*Fleet{report})
	require.NoError(t, err)
	usefulPartial := 0
	for seed := int64(50000); seed < 50032; seed++ {
		outcome := model.run(result.Tactic, BattleTacticDisengage, seed)
		require.True(t, outcome.safe(model.ownShips, model.ownValue))
		require.Zero(t, outcome.BombersAfter)
		if outcome.Unfinished {
			usefulPartial++
		}
	}
	require.Positive(t, usefulPartial, "native survivors must exercise the previous Unfinished veto")
	t.Logf("USEFUL_VICTORY: %d/32 held-out trials retain auxiliaries while meeting mission and 5%% limits", usefulPartial)
}

func swarmUniverse(t *testing.T) *testUniverse {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignJihadCruiser, DesignSpaceStation), Fleets: []ScenarioFleet{
			{Name: "Old command", Design: DesignJihadCruiser.Name, Quantity: 2, Position: Vector{-250, 0}},
			{Name: "Local A", Design: DesignJihadCruiser.Name, Quantity: 4, Position: Vector{20, 0}},
			{Name: "Local B", Design: DesignJihadCruiser.Name, Quantity: 4, Position: Vector{20, 0}},
			{Name: "Late", Design: DesignJihadCruiser.Name, Quantity: 2, Position: Vector{-350, 0}},
		}},
		{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Threat", Design: DesignDestroyerDelta.Name, Quantity: 2, Position: Vector{50, 0}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Position: Vector{-100, 0}, Starbase: DesignSpaceStation.Name}}})
	p := u.Player(1)
	p.Automation.FleetRoles = map[int]string{}
	for _, f := range u.Game.getFleets(1) {
		p.Automation.FleetRoles[f.Num] = "patrol"
	}
	e := u.Fleet("Threat")
	p.discoverer.discoverDesign(e.Tokens[0].design, true)
	p.discoverer.discoverFleet(e, true)
	old := u.Fleet("Old command")
	for _, f := range u.Game.getFleets(1) {
		savePatrolJob(f, patrolJob{Leader: old.Num, Enemy: 2, Target: e.Num, GatheringSince: u.Game.Year - 3})
	}
	return u
}

func TestPatrolRefittingCommandTransfersWithoutChangingRefit(t *testing.T) {
	u := swarmUniverse(t)
	p := u.Player(1)
	old := u.Fleet("Old command")
	old.SetTag("maintenance-pending", "manual-refit")
	before, _ := json.Marshal(old.Waypoints)
	for i := 0; i < 3; i++ {
		u.Run(func(tg *turnGenerator) { tg.reconcilePatrolJobs(p) })
		require.Zero(t, readPatrolJob(old).Leader)
		a, b := readPatrolJob(u.Fleet("Local A")), readPatrolJob(u.Fleet("Local B"))
		require.NotZero(t, a.Leader)
		require.Equal(t, a.Leader, b.Leader)
		require.NotEqual(t, old.Num, a.Leader)
	}
	after, _ := json.Marshal(old.Waypoints)
	require.Equal(t, string(before), string(after))
	require.Equal(t, "manual-refit", old.GetTag("maintenance-pending"))
}

func TestPatrolLocalCriticalMassMergesAndLaunchesWithoutDistantCommand(t *testing.T) {
	u := swarmUniverse(t)
	p := u.Player(1)
	old := u.Fleet("Old command")
	a, b := u.Fleet("Local A"), u.Fleet("Local B")
	job := readPatrolJob(old)
	job.RendezvousBy, job.Rally = u.Game.Year-1, a.Position
	savePatrolJob(old, job)
	u.Run((*turnGenerator).startPatrolEvaluation)
	u.Run(func(tg *turnGenerator) { tg.advancePatrol(p, old) })
	require.True(t, a.Delete || b.Delete, "native merge must occur before departure")
	local := a
	if a.Delete {
		local = b
	}
	require.Equal(t, 8, local.Spec.TotalShips)
	require.Equal(t, local.Num, readPatrolJob(local).Leader)
	require.Greater(t, len(local.Waypoints), 1)
	require.Equal(t, u.Fleet("Threat").Position, local.Waypoints[len(local.Waypoints)-1].Position)
	require.False(t, readPatrolJob(local).Provisional)
	require.Zero(t, readPatrolJob(local).RendezvousBy, "a completed gathering must not expire later reinforcement rendezvous")
	require.Zero(t, readPatrolJob(local).GatheringSince)
	require.NotEqual(t, old.Num, readPatrolJob(u.Fleet("Late")).Leader)
	u.Run(func(tg *turnGenerator) { require.True(t, tg.patrolDepartureConfirmed(p, local)) })
}

func TestBomberFiveWithOneCoverEvadesReportedWarpSevenRaid(t *testing.T) {
	bomber := campaignDesign("Five bombers", B17Bomber.Name, "bomber")
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(bomber, DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Bombing pack", Position: Vector{100, 0}, Tokens: []ScenarioShipToken{{Design: bomber.Name, Quantity: 5}, {Design: DesignDestroyerDelta.Name, Quantity: 1}}}}},
		{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Raid", Design: DesignDestroyerDelta.Name, Quantity: 14, Position: Vector{160, -8}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Bombed", Owner: 2, Position: Vector{100, 0}, Cargo: Cargo{Colonists: 2000}}}})
	p, f, e := u.Player(1), u.Fleet("Bombing pack"), u.Fleet("Raid")
	p.Automation.FleetRoles = map[int]string{f.Num: "bomber"}
	p.discoverer.discoverDesign(e.Tokens[0].design, true)
	p.discoverer.discoverFleet(e, true)
	r := p.GetFleetIntel(2, e.Num)
	r.WarpSpeed = 7
	r.Heading = VectorFloat64{X: -.999, Y: .038}
	require.Positive(t, bomberChaseThreat(p, f))
	u.Run((*turnGenerator).startPatrolEvaluation, (*turnGenerator).planAutomaticSafety)
	require.True(t, f.InAutomaticSafety(), "one DD cannot certify safety against fourteen")
	require.Greater(t, len(f.Waypoints), 1)
	require.NotEqual(t, f.Position, f.Waypoints[1].Position)
	t.Logf("BOMBER_PROTECTION: 5 bombers + 1 DD evade 14 DD approaching at warp 7, next=%v", f.Waypoints[1].Position)
}

func TestPatrolReconReservationDoesNotGrowWithoutScan(t *testing.T) {
	u := swarmUniverse(t)
	p := u.Player(1)
	e := u.Fleet("Threat")
	e.Tokens[0].Quantity = 20
	e.Spec = ComputeFleetSpec(&u.Game.Rules, u.Player(2), e)
	p.discoverer.discoverFleet(e, true)
	p.GetShipDesignIntel(2, e.Tokens[0].DesignNum).Slots = nil
	u.Run((*turnGenerator).startPatrolEvaluation)
	u.Run(func(tg *turnGenerator) {
		limit := tg.reconPatrolReservation(p)
		for i := 0; i < 3; i++ {
			tg.planPatrolAutomation()
		}
		n := 0
		for _, f := range u.Game.getFleets(1) {
			if readPatrolJob(f).Leader != 0 {
				n += f.Spec.TotalShips
			}
		}
		require.LessOrEqual(t, n, limit+u.Fleet("Old command").Spec.TotalShips)
		t.Logf("RECON: bounded reservation %d, limit %d; repeated planning does not hoard all ready ships", n, limit)
	})
}

func TestBombardmentLocalCoverAndRemoteWarning(t *testing.T) {
	for _, mode := range []string{"local-reinforce", "local-no-early-withdrawal", "remote-withdrawal"} {
		t.Run(mode, func(t *testing.T) {
			bomber := campaignDesign("Expedition bombers", B17Bomber.Name, "bomber")
			for i := range bomber.Slots {
				if StaticTechStore.GetHullComponent(bomber.Slots[i].HullComponent).HullSlotType&HullSlotTypeEngine != 0 {
					bomber.Slots[i].HullComponent = RadiatingHydroRamScoop.Name
				}
			}
			position := Vector{100, 0}
			if mode == "remote-withdrawal" {
				position = Vector{300, 0}
			}
			own := []ScenarioFleet{{Name: "Bombers", Design: bomber.Name, Quantity: 5, Position: position}}
			if mode == "local-reinforce" {
				own = append(own, ScenarioFleet{Name: "Ready DD reserve", Design: DesignJihadCruiser.Name, Quantity: 8, Position: position.Add(Vector{-25, 0})})
			}
			u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
				{Designs: Designs(bomber, DesignJihadCruiser, DesignSpaceStation), Fleets: own},
				{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Incoming", Design: DesignDestroyerDelta.Name, Quantity: 2, Position: position.Add(Vector{150, 0})}}},
			}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Bombarded", Owner: 2, Position: position, Cargo: Cargo{Colonists: 2000}}}})
			p, b, e := u.Player(1), u.Fleet("Bombers"), u.Fleet("Incoming")
			p.Automation.FleetRoles = map[int]string{b.Num: "bomber"}
			p.discoverer.discoverDesign(e.Tokens[0].design, true)
			p.discoverer.discoverFleet(e, true)
			report := p.GetFleetIntel(2, e.Num)
			report.WarpSpeed = 2
			report.Heading = VectorFloat64{X: -1}
			if mode == "local-reinforce" {
				p.Automation.FleetRoles[u.Fleet("Ready DD reserve").Num] = "patrol"
			}
			require.Zero(t, bomberChaseThreat(p, b), "observed warp 2 alone used to miss the warning")
			require.NotEmpty(t, bomberWarningReports(p, b), "known engine maximum gives a turn of advance warning")
			u.Run((*turnGenerator).startPatrolEvaluation)
			if mode == "local-reinforce" {
				p.patrolEvaluator.Limit = 0
			}
			u.Run((*turnGenerator).planAutomaticSafety)
			switch mode {
			case "local-reinforce":
				cover := u.Fleet("Ready DD reserve")
				require.False(t, b.InAutomaticSafety())
				require.Equal(t, "escort", p.Automation.FleetRoles[cover.Num])
				require.NotEmpty(t, cover.GetTag(escortTag))
				require.Greater(t, len(cover.Waypoints), 1)
				require.Equal(t, b.Position, cover.Waypoints[len(cover.Waypoints)-1].Position)
			case "local-no-early-withdrawal":
				require.False(t, b.InAutomaticSafety(), "one-year early withdrawal applies only to remote expeditions")
			case "remote-withdrawal":
				require.True(t, b.InAutomaticSafety())
				require.Greater(t, len(b.Waypoints), 1)
				t.Logf("ESCAPE fuel=%d max=%d route=%v", b.Fuel, automaticMaxWarp(p, b), b.Waypoints)
				for speed := b.Waypoints[1].WarpSpeed + 1; speed <= automaticMaxWarp(p, b); speed++ {
					_, _, _, ok := automaticLegFuel(p, b, speed, b.Position.DistanceTo(b.Waypoints[1].Position), b.Fuel, 0)
					require.False(t, ok, "higher escape speed must be unfunded")
				}
			}
			t.Logf("BOMBARDMENT mode=%s warningMaxSpeed=%.0f safety=%v orders=%v", mode, reportedMaximumSpeed(p, report), b.InAutomaticSafety(), b.Waypoints)
		})
	}
}

func TestBomberEscapeRejectsAStationaryPackAtTheExit(t *testing.T) {
	u := swarmUniverse(t)
	p, f, e := u.Player(1), u.Fleet("Local A"), u.Fleet("Threat")
	p.Automation.FleetRoles[f.Num] = "bomber"
	u.Run((*turnGenerator).startPatrolEvaluation)
	eReport := p.GetFleetIntel(2, e.Num)
	eReport.Position = Vector{100, 0}
	eReport.WarpSpeed = 0
	eReport.Heading = VectorFloat64{}
	closest, ending, _ := reportedEscapeSeparation(p, f, Vector{80, 0})
	require.LessOrEqual(t, closest, 3.0)
	require.LessOrEqual(t, ending, 3.0, "an idle enemy can intercept an escape endpoint within known maximum engine reach")
}

func TestBombardingGuardDoesNotInventPursuitOfEscapingBombers(t *testing.T) {
	u := swarmUniverse(t)
	p, f, e := u.Player(1), u.Fleet("Local A"), u.Fleet("Threat")
	p.Automation.FleetRoles[f.Num] = "bomber"
	u.Run((*turnGenerator).startPatrolEvaluation)
	report := p.GetFleetIntel(2, e.Num)
	report.Position = Vector{100, 0}
	report.WarpSpeed = 0
	report.Heading = VectorFloat64{}
	// A fresh report of bombers over our colony identifies a committed siege.
	bomber := campaignDesign("Siege bomber", B17Bomber.Name, "bomber")
	bomber.PlayerNum = 2
	bomber.Num = 99
	p.ShipDesignIntels = append(p.ShipDesignIntels, &bomber)
	p.shipIntelIndex = nil
	report.Tokens = append(report.Tokens, ShipToken{DesignNum: 99, Quantity: 4})
	p.PlanetIntels = append(p.PlanetIntels, &Planet{MapObject: MapObject{PlayerNum: p.Num, Position: report.Position, Num: 88}})
	require.True(t, stationaryBombardmentGuard(p, report))
	require.Empty(t, bomberWarningReports(p, f))
	closest, ending, _ := reportedEscapeSeparation(p, f, Vector{80, 0})
	require.Greater(t, closest, 3.0)
	require.Greater(t, ending, 3.0, "escape may pass outside a committed stationary siege")
	closest, _, _ = reportedEscapeSeparation(p, f, report.Position)
	require.LessOrEqual(t, closest, 3.0, "the occupied orbit itself is still dangerous")
	report.WarpSpeed = 7
	report.Heading = VectorFloat64{X: -1}
	require.False(t, stationaryBombardmentGuard(p, report))
	require.NotEmpty(t, bomberWarningReports(p, f), "new movement immediately restores pursuit risk")
}

func TestGatheringCleanupKeepsDeadlineAndDoesNotDelayDefense(t *testing.T) {
	hunter := campaignDesign("Free hunter", Cruiser.Name, "patrol")
	for i := range hunter.Slots {
		if StaticTechStore.GetHullComponent(hunter.Slots[i].HullComponent).HullSlotType&HullSlotTypeEngine != 0 {
			hunter.Slots[i].HullComponent = FuelMizer.Name
		}
	}
	for _, urgent := range []bool{false, true} {
		u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
			{Player: proposalTestPlayer(), Designs: Designs(hunter, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Gatherer", Design: hunter.Name}}},
			{Designs: Designs(DesignDestroyerDelta, DesignLongRangeScout), Fleets: []ScenarioFleet{{Name: "Main force", Design: DesignDestroyerDelta.Name, Quantity: 20, Position: Vector{200, 0}}, {Name: "On the way", Design: DesignLongRangeScout.Name, Position: Vector{16, 0}}}},
		}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
		p, f, e := u.Player(1), u.Fleet("Gatherer"), u.Fleet("Main force")
		f.Fuel = 0
		p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
		for _, enemy := range u.Game.getFleets(2) {
			p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
			p.discoverer.discoverFleet(enemy, true)
		}
		if urgent {
			r := p.GetFleetIntel(2, e.Num)
			r.WarpSpeed = 10
			r.Heading = VectorFloat64{X: -1}
		}
		u.Run((*turnGenerator).startPatrolEvaluation)
		job := patrolJob{Leader: f.Num, Enemy: 2, Target: e.Num, Provisional: true, Rally: Vector{16, 0}, RendezvousBy: u.Game.Year + 1, GatheringSince: u.Game.Year - 1}
		savePatrolJob(f, job)
		f.Waypoints = []Waypoint{NewPositionWaypoint(f.Position, 0), NewPositionWaypoint(job.Rally, 4)}
		u.Run(func(tg *turnGenerator) { tg.patrolGatheringCleanup(p, f, p.GetFleetIntel(2, e.Num), &job, []*Fleet{f}) })
		if urgent {
			require.Zero(t, readPatrolJob(f).CleanupTarget)
			continue
		}
		require.NotZero(t, readPatrolJob(f).CleanupTarget)
		require.Equal(t, job.RendezvousBy, readPatrolJob(f).RendezvousBy)
		require.Equal(t, u.Fleet("On the way").Num, patrolTacticalEnemy(p, f).Num)
		require.True(t, patrolCanAttack(p, f))
		require.True(t, readPatrolJob(f).CleanupUsed)
		t.Logf("CLEANUP funded local kill: out=%v, main deadline=%d retained", f.Waypoints, job.RendezvousBy)
	}
}
