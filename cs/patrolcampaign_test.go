//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// All campaign objects are synthetic. Never load a personal save here.
func campaignDesign(name, hull, role string) ShipDesign {
	h := StaticTechStore.GetHull(hull)
	d := ShipDesign{Name: name, Hull: h.Name}
	for i, slot := range h.Slots {
		component := ""
		switch {
		case slot.Type&HullSlotTypeEngine != 0:
			component = TransStar10.Name
			if role == "bomber" || role == "weak" {
				component = LongHump6.Name
			}
			if role == "scout" {
				component = FuelMizer.Name
			}
		case slot.Type&HullSlotTypeWeapon != 0 && (role == "patrol" || role == "enemy" || role == "weak"):
			component = "Anti-Matter Pulverizer"
			if role == "enemy" {
				component = DeltaTorpedo.Name
			}
			if role == "weak" {
				component = XRayLaser.Name
			}
		case slot.Type&HullSlotTypeBomb != 0 && role == "bomber":
			component = BlackCatBomb.Name
		case slot.Type&HullSlotTypeScanner != 0 && role == "scout":
			component = PeerlessScanner.Name
		case slot.Type&HullSlotTypeShield != 0:
			component = GorillaDelagator.Name
		case slot.Type&HullSlotTypeArmor != 0:
			component = Neutronium.Name
		case slot.Type&HullSlotTypeMechanical != 0:
			component = Overthruster.Name
		case slot.Type&HullSlotTypeElectrical != 0:
			component = BattleComputer.Name
		}
		if component != "" {
			d.Slots = append(d.Slots, ShipDesignSlot{HullSlotIndex: i + 1, HullComponent: component, Quantity: slot.Capacity})
		}
	}
	if err := d.Validate(&rules, proposalTestPlayer()); err != nil {
		panic(err)
	}
	return d
}

func campaignUniverse(t *testing.T, seed int64) *testUniverse {
	gun := campaignDesign("Campaign cruiser", Cruiser.Name, "patrol")
	light := campaignDesign("Campaign destroyer", Destroyer.Name, "patrol")
	scout := campaignDesign("Campaign observer", Scout.Name, "scout")
	collector := campaignDesign("Campaign collector", LargeFreighter.Name, "collector")
	dock := ShipDesign{Name: "Campaign supply station", Hull: SpaceStation.Name}
	foe := campaignDesign("Enemy torpedo cruiser", Cruiser.Name, "enemy")
	weak := campaignDesign("Enemy weak destroyer", Destroyer.Name, "weak")
	bomber := campaignDesign("Enemy bomber", MiniBomber.Name, "bomber")
	ownFleets := []ScenarioFleet{}
	for i := 0; i < 6; i++ {
		ownFleets = append(ownFleets, ScenarioFleet{Name: fmt.Sprintf("Guard-%d", i), Design: gun.Name, Position: Vector{i * 12, 20}})
	}
	for i := 0; i < 4; i++ {
		ownFleets = append(ownFleets, ScenarioFleet{Name: fmt.Sprintf("Light-%d", i), Design: light.Name, Position: Vector{30 + i*12, -20}})
	}
	for i := 0; i < 8; i++ {
		ownFleets = append(ownFleets, ScenarioFleet{Name: fmt.Sprintf("Observer-%d", i), Design: scout.Name, Position: Vector{20 + i*18, 35}})
	}
	for i := 0; i < 3; i++ {
		ownFleets = append(ownFleets, ScenarioFleet{Name: fmt.Sprintf("Collector-%d", i), Design: collector.Name, At: "Home"})
	}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Player: proposalTestPlayer(), Designs: Designs(gun, light, scout, collector, dock), Fleets: ownFleets},
		{Player: proposalTestPlayer(), Designs: Designs(foe, weak, bomber), Fleets: []ScenarioFleet{
			{Name: "Bombarding", Design: bomber.Name, Quantity: 2, At: "Front"},
			{Name: "Bomber escort", Design: weak.Name, Quantity: 2, At: "Front"},
			{Name: "Near raider", Design: weak.Name, Position: Vector{35, -35}},
			{Name: "Incoming wing A", Design: foe.Name, Quantity: 3, Position: Vector{220, 20}, Waypoints: []ScenarioWaypoint{{Position: Vector{220, 20}}, {Position: Vector{180, 0}, Warp: 4}, {To: "Front", Warp: 4}}},
			{Name: "Incoming wing B", Design: foe.Name, Quantity: 3, Position: Vector{220, -20}, Waypoints: []ScenarioWaypoint{{Position: Vector{220, -20}}, {Position: Vector{180, 0}, Warp: 4}, {To: "Front", Warp: 4}}},
			{Name: "Slow weak flank", Design: weak.Name, Quantity: 2, Position: Vector{160, -75}, Waypoints: []ScenarioWaypoint{{Position: Vector{160, -75}}, {To: "Home", Warp: 1}}},
		}},
	}, Planets: []ScenarioPlanet{
		{Name: "Home", Owner: 1, Starbase: dock.Name, Scanner: true, Cargo: Cargo{Colonists: 5000, Ironium: 10000, Boranium: 10000, Germanium: 10000}},
		{Name: "Front", Owner: 1, Position: Vector{120, 0}, Scanner: true, Cargo: Cargo{Colonists: 2000}},
		{Name: "Enemy port", Owner: 2, Position: Vector{400, 0}, Scanner: true, Cargo: Cargo{Colonists: 2000}},
	}})
	p := u.Player(1)
	p.Race.Spec.DiscoverDesignOnScan = false
	p.Automation.FleetRoles = map[int]string{}
	p.Automation.DisablePassengerTransport = true
	p.Automation.DisableMineralSupply = true
	for _, f := range u.Game.getFleets(1) {
		role := "patrol"
		if len(f.Name) >= 8 && f.Name[:8] == "Observer" {
			role = "scout"
		}
		if len(f.Name) >= 9 && f.Name[:9] == "Collector" {
			role = "freighter"
		}
		p.Automation.FleetRoles[f.Num] = role
	}
	u.Game.Rules.ResetSeed(seed)
	u.RunE((*turnGenerator).scan)
	return u
}

func campaignSnapshot(t *testing.T, u *testUniverse, step int) {
	p := u.Player(1)
	kills, ownLost, bombings, bombedColonists := 0, 0, 0, 0
	for _, battle := range p.BattleRecords {
		kills += battle.Stats.ShipsDestroyedByPlayer[2]
		ownLost += battle.Stats.ShipsDestroyedByPlayer[1]
	}
	for _, message := range p.Messages {
		if message.Type == PlayerMessagePlanetBombed {
			bombings++
			if message.Spec.Bombing != nil {
				bombedColonists += message.Spec.Bombing.ColonistsKilled
			}
		}
	}
	unknown, visible, enemyShips := 0, 0, 0
	for _, f := range p.FleetIntels {
		if f.PlayerNum == 2 && !f.Delete && f.ReportAge == 0 {
			visible++
			enemyShips += reportedFleetShips(f)
			if unknownFleetEquipment(p, f) {
				unknown++
			}
		}
	}
	salvage, loaded := 0, 0
	for _, s := range p.SalvageIntels {
		if !s.Delete {
			salvage += s.Cargo.ToMineral().Total()
		}
	}
	for _, f := range u.Game.getFleets(1) {
		if !f.Delete && p.Automation.FleetRoles[f.Num] == "freighter" {
			loaded += f.Cargo.ToMineral().Total()
		}
	}
	t.Logf("CAMPAIGN step=%d year=%d visible=%d unknown=%d enemy_ships=%d battles=%d enemy_killed=%d own_lost=%d bombings=%d bombed_colonists=%d front_population=%d visible_debris=%d collector_cargo=%d", step, u.Game.Year, visible, unknown, enemyShips, len(p.BattleRecords), kills, ownLost, bombings, bombedColonists, u.Planet("Front").Cargo.Colonists, salvage, loaded)
	for _, f := range u.Game.getFleets(1) {
		if f.Delete {
			continue
		}
		job := readPatrolJob(f)
		if p.Automation.FleetRoles[f.Num] == "freighter" {
			raw, _ := json.Marshal(map[string]any{"step": step, "name": f.Name, "position": f.Position, "cargo": f.Cargo, "fuel": f.Fuel, "job": logisticsWork(f)})
			t.Logf("CAMPAIGN_COLLECTOR %s", raw)
		}
		if job.Leader != 0 || readScoutRecon(f) != nil || logisticsWork(f) != nil || f.GetTag(anticipatedSalvageTag) != "" {
			raw, _ := json.Marshal(map[string]any{"step": step, "name": f.Name, "num": f.Num, "role": p.Automation.FleetRoles[f.Num], "position": f.Position, "ships": f.Spec.TotalShips, "fuel": f.Fuel, "damage": fleetDamagePercent(f), "patrol": job, "scout": readScoutRecon(f), "logistics": logisticsWork(f), "anticipation": f.GetTag(anticipatedSalvageTag), "waypoints": f.Waypoints})
			t.Logf("CAMPAIGN_ORDER %s", raw)
		}
	}
	for _, battle := range p.BattleRecords {
		collectors, retreated, ownScoutLost, awayMoves := 0, 0, 0, 0
		enemyStart := VectorFloat64{}
		enemyStacks := 0
		for _, token := range battle.Tokens {
			if token.PlayerNum != 1 {
				enemyStart = enemyStart.Add(token.Position.ToFloat64())
				enemyStacks++
			}
		}
		if enemyStacks > 0 {
			enemyStart = enemyStart.Scale(1 / float64(enemyStacks))
		}
		collectorTokens := map[int]bool{}
		ownTokens := map[int]BattleRecordToken{}
		for _, token := range battle.Tokens {
			if token.PlayerNum == 1 {
				ownTokens[token.Num] = token
				d := p.GetDesign(token.DesignNum)
				if d != nil && d.Hull == LargeFreighter.Name {
					collectors++
					collectorTokens[token.Num] = true
					require.Equal(t, BattleTacticDisengage, token.Tactic)
				}
			}
		}
		last := map[int]*ShipToken{}
		for _, round := range battle.ActionsPerRound {
			for _, action := range round {
				if action.Type == TokenActionMove && collectorTokens[action.TokenNum] && action.To.ToFloat64().Subtract(enemyStart).LengthSquared() > action.From.ToFloat64().Subtract(enemyStart).LengthSquared() {
					awayMoves++
				}
				if action.Type == TokenActionRanAway && collectorTokens[action.TokenNum] {
					retreated++
				}
				if action.Target != nil {
					last[action.TargetNum] = action.Target
				}
			}
		}
		peakDamage := 0.
		for num, token := range last {
			if record, ok := ownTokens[num]; ok && token.Quantity > 0 {
				d := p.GetDesign(record.DesignNum)
				if d != nil {
					peakDamage = max(peakDamage, token.Damage/float64(max(1, d.Spec.Armor)))
				}
			}
		}
		for _, token := range battle.DestroyedTokens {
			if token.PlayerNum == 1 {
				d := p.GetDesign(token.DesignNum)
				if d != nil && d.Hull == Scout.Name {
					ownScoutLost += token.Quantity
				}
			}
		}
		t.Logf("CAMPAIGN_BATTLE step=%d position=%v participants=%v destroyed=%v collectors=%d collector_retreats=%d collector_away_moves=%d scouts_lost=%d peak_survivor_damage=%.4f", step, battle.Position, battle.Stats.NumShipsByPlayer, battle.Stats.ShipsDestroyedByPlayer, collectors, retreated, awayMoves, ownScoutLost, peakDamage)
	}
}

// Isolated counterfactual: can the real pickup phase load debris where an
// anticipatory collector is already present, without inventing cargo or scans?
func campaignPickupProbe(t *testing.T, u *testUniverse, step int) {
	p := u.Player(1)
	for _, f := range u.Game.getFleets(1) {
		if f.Delete || p.Automation.FleetRoles[f.Num] != "freighter" || f.CargoMass() != 0 || f.InAutomaticSafety() {
			continue
		}
		for _, report := range p.SalvageIntels {
			if report.Delete || report.Position != f.Position || report.Cargo.ToMineral().Total() == 0 {
				continue
			}
			removeDeleted(u.Game.Universe)
			raw, err := json.Marshal(u.Game)
			require.NoError(t, err)
			var copy FullGame
			require.NoError(t, json.Unmarshal(raw, &copy))
			copy.TechStore = &StaticTechStore
			copy.Rules.SetTechStore(&StaticTechStore)
			copy.Rules.ResetSeed(1)
			copy.Universe.setLogger(testLogger)
			require.NoError(t, copy.computeSpecs())
			collector := copy.getFleet(1, f.Num)
			salvage := copy.getSalvage(report.Num)
			if collector == nil || salvage == nil {
				continue
			}
			amount := salvage.Cargo.ToMineral().Total()
			work := &logisticsJob{Kind: "salvage", TargetNum: report.Num, Position: f.Position}
			saveLogisticsWork(collector, work)
			generator := newTurnGenerator(&copy)
			generator.resolveLogisticsAutomation()
			picked := collector.Cargo.ToMineral().Total()
			t.Logf("CAMPAIGN_PICKUP_PROBE step=%d collector=%s available=%d native_pickup=%d position=%v", step, f.Name, amount, picked, f.Position)
			require.Equal(t, min(amount, collector.Spec.CargoCapacity), picked)
			return
		}
	}
}

func TestPatrolCampaignNativeTwelveTurns(t *testing.T) {
	for _, seed := range []int64{17, 89} {
		for _, mode := range []string{"native", "reload", "reload-split"} {
			reloadState, splitEnemy := mode != "native", mode == "reload-split"
			t.Run(fmt.Sprintf("seed%d-%s", seed, mode), func(t *testing.T) {
				u := campaignUniverse(t, seed)
				if splitEnemy {
					// In the stress variant both wings start co-located so their
					// native merge and subsequent split happen before improved
					// patrols can intercept the formerly separate wings.
					for _, name := range []string{"Incoming wing A", "Incoming wing B"} {
						f := u.Fleet(name)
						f.Position = Vector{220, 0}
						f.Waypoints[0].Position = f.Position
					}
					scan := newPlayerScanner(u.Game.Universe, u.Game.Players, &u.Game.Rules, u.Player(1))
					require.NoError(t, scan.scan())
				}
				require.Greater(t, len(u.Player(1).FleetIntels), 0, "native scanners must supply initial reports")
				initial, _ := json.Marshal(u.Game)
				t.Logf("CAMPAIGN_START seed=%d own_fleets=%d enemy_fleets=%d snapshot_bytes=%d", seed, len(u.Game.getFleets(1)), len(u.Game.getFleets(2)), len(initial))
				if seed == 17 && mode == "native" {
					manifest, _ := json.Marshal(map[string]any{"ownDesigns": u.Player(1).Designs, "enemyDesigns": u.Player(2).Designs, "fleets": u.Game.Fleets, "planets": u.Game.Planets})
					t.Logf("CAMPAIGN_MANIFEST %s", manifest)
				}
				campaignSnapshot(t, u, 0)
				totalBattles, splits, deliveries, totalEnemyKilled, ownWarLost := 0, 0, 0, 0, 0
				for step := 1; step <= 12; step++ {
					// The opponent uses native merge when its two actual wings meet.
					wings := []*Fleet{}
					for _, f := range u.Game.getFleets(2) {
						if !f.Delete && (f.Name == "Incoming wing A" || f.Name == "Incoming wing B") {
							wings = append(wings, f)
						}
					}
					if len(wings) == 2 && wings[0].Position == wings[1].Position {
						_, err := NewOrderer().Merge(&u.Game.Rules, u.Player(2), wings)
						require.NoError(t, err)
						u.Game.deleteFleet(wings[1])
						t.Logf("CAMPAIGN_ENEMY_MERGE step=%d ships=%d position=%v", step, wings[0].Spec.TotalShips, wings[0].Position)
					}
					if splitEnemy && step == 2 {
						for _, f := range slices.Clone(u.Game.getFleets(2)) {
							if !f.Delete && f.Spec.TotalShips >= 6 {
								children, err := NewOrderer().SplitAll(&u.Game.Rules, u.Player(2), u.Game.getFleets(2), f)
								require.NoError(t, err)
								for _, child := range children {
									u.Game.Fleets = append(u.Game.Fleets, child)
									require.NoError(t, u.Game.Universe.addFleet(child))
								}
								for i, part := range append([]*Fleet{f}, children...) {
									part.Waypoints = []Waypoint{NewPositionWaypoint(part.Position, 0), NewPositionWaypoint(Vector{100, (i - 2) * 35}, 3)}
								}
								splits++
								t.Logf("CAMPAIGN_ENEMY_SPLIT step=%d children=%d", step, len(children))
								break
							}
						}
						for _, scout := range u.Game.getFleets(1) {
							if job := readScoutRecon(scout); !scout.Delete && job != nil {
								reported := u.Player(1).GetFleetIntel(job.Enemy, job.Target)
								actual := u.Game.getFleet(job.Enemy, job.Target)
								known := false
								if actual != nil {
									known = !unknownFleetEquipment(u.Player(1), actual)
								}
								t.Logf("CAMPAIGN_RECON_LIFECYCLE step=%d scout=%s target=%d reported_target=%t actual_target=%t actual_equipment_already_known=%t", step, scout.Name, job.Target, reported != nil, actual != nil, known)
							}
						}
					}
					beforeCargo := map[int]Cargo{}
					beforePositions := map[int]Vector{}
					for _, f := range u.Game.getFleets(1) {
						if !f.Delete && u.Player(1).Automation.FleetRoles[f.Num] == "freighter" {
							beforeCargo[f.Num] = f.Cargo
							beforePositions[f.Num] = f.Position
						}
					}
					// Recreate the generator each year, as the actual server does.
					u.Game.Rules.ResetSeed(seed + int64(u.Game.Year)*997)
					current := newTurnGenerator(u.Game)
					current.log = testLogger
					u.turn = &current
					u.GenerateTurn()
					for _, f := range u.Game.getFleets(1) {
						if !f.Delete && beforeCargo[f.Num].ToMineral().Total() > f.Cargo.ToMineral().Total() {
							for _, planet := range u.Game.getPlanets(1) {
								if planet.Position == f.Position {
									amount := beforeCargo[f.Num].ToMineral().Total() - f.Cargo.ToMineral().Total()
									deliveries += amount
									t.Logf("CAMPAIGN_DELIVERY step=%d collector=%s amount=%d origin=%v planet=%s", step, f.Name, amount, beforePositions[f.Num], planet.Name)
								}
							}
						}
					}
					totalBattles += len(u.Player(1).BattleRecords)
					for _, battle := range u.Player(1).BattleRecords {
						totalEnemyKilled += battle.Stats.ShipsDestroyedByPlayer[2]
						for _, token := range battle.DestroyedTokens {
							if token.PlayerNum == 1 {
								d := u.Player(1).GetDesign(token.DesignNum)
								if d != nil && d.Hull != Scout.Name {
									ownWarLost += token.Quantity
								}
							}
						}
					}
					campaignSnapshot(t, u, step)
					if splitEnemy && step == 6 {
						campaignPickupProbe(t, u, step)
					}
					actualShips, observedShips := 0, 0
					for _, f := range u.Game.getFleets(2) {
						if !f.Delete {
							actualShips += f.Spec.TotalShips
						}
					}
					for _, f := range u.Player(1).FleetIntels {
						if f.PlayerNum == 2 && !f.Delete && f.ReportAge == 0 {
							observedShips += reportedFleetShips(f)
						}
					}
					require.Equal(t, actualShips, observedShips, "territorial sensor coverage must actually keep every surviving enemy visible")
					for _, f := range u.Game.Fleets {
						require.GreaterOrEqual(t, f.Fuel, 0)
						for _, token := range f.Tokens {
							require.GreaterOrEqual(t, token.Quantity, 0)
						}
					}
					if reloadState {
						// Persistence removes deleted objects before writing, just
						// as the server does; Delete itself is not serialized.
						removeDeleted(u.Game.Universe)
						raw, err := json.Marshal(u.Game)
						require.NoError(t, err)
						var restored FullGame
						require.NoError(t, json.Unmarshal(raw, &restored))
						restored.TechStore = &StaticTechStore
						restored.Universe.setLogger(testLogger)
						restored.Rules.SetTechStore(&StaticTechStore)
						restored.Rules.ResetSeed(seed + int64(restored.Year)*997)
						require.NoError(t, restored.computeSpecs())
						u.Game = &restored
					}
				}
				require.Greater(t, totalBattles, 0, "campaign must exercise actual combat, not just planning")
				if splitEnemy {
					require.Equal(t, 1, splits)
				}
				require.Positive(t, deliveries, "real collected cargo must return to an owned planet")
				require.Equal(t, 13, totalEnemyKilled)
				require.Zero(t, ownWarLost, "only contact reconnaissance scouts may be lost in this scenario")
				remaining := 0
				for _, f := range u.Game.getFleets(2) {
					if !f.Delete && f.Spec.TotalShips > 0 {
						remaining += f.Spec.TotalShips
					}
				}
				require.Zero(t, remaining)
				t.Logf("CAMPAIGN_END seed=%d mode=%s battles=%d splits=%d delivered=%d enemy_remaining_ships=%d", seed, mode, totalBattles, splits, deliveries, remaining)
				// Full campaign facts are reported; unmet doctrine is evidence, not a
				// manufactured success assertion or a reason to modify a personal save.
				require.NotEmpty(t, slices.Clone(u.Player(1).PlanetIntels))
			})
		}
	}
}
