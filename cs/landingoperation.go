package cs

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
)

const landingOperationTag = "automatic-landing"
const landingMemberTag = "automatic-landing-member"

// The operation lives on its transport leader, not on an invisible global
// reservation. Ships, people, fuel and escorts must physically reach the rally.
type landingOperation struct {
	Target, Owner, Need, Scout, Started int
	ProgressYear                        int
	BaseRallyBy                         int
	BaseRallyRetryYear                  int
	Progress                            string
	Rally                               Vector
	Phase                               string
	ScanYear                            int
	Base                                bool
	EscortDesigns                       map[int]bool
}

func readLandingOperation(f *Fleet) *landingOperation {
	if f == nil {
		return nil
	}
	var op landingOperation
	if json.Unmarshal([]byte(f.GetTag(landingOperationTag)), &op) != nil {
		return nil
	}
	return &op
}
func saveLandingOperation(f *Fleet, op *landingOperation) {
	b, _ := json.Marshal(op)
	f.SetTag(landingOperationTag, string(b))
}
func landingManaged(f *Fleet) bool {
	return f != nil && (f.GetTag(landingMemberTag) != "" || f.GetTag(landingOperationTag) != "")
}
func landingScout(p *Player, f *Fleet) bool {
	if !landingManaged(f) {
		return false
	}
	role := p.Automation.FleetRoles[f.Num]
	recon := f.GetTag(landingBaseReconTag)
	return role == "scout" || role == "patrol" && recon != "" && recon == f.GetTag(landingMemberTag)
}

// Saved solo invasions must join preparation after upgrading. Keep their cargo;
// a new operation can recruit it without unloading or inventing passengers.
func (t *turnGenerator) migrateLegacyLandings() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			if landingManaged(f) || p.Automation.FleetRoles[f.Num] != "freighter" {
				continue
			}
			if j := logisticsWork(f); j != nil && (j.Kind == "invade" || j.Kind == "invade-load" || j.Kind == "landing-reserve") {
				if f.CargoMass() == 0 {
					// An empty legacy pickup has no delivery to finish. Keeping it
					// reserved also makes a pending refit wait forever for that job.
					delete(f.Tags, logisticsTag)
					delete(f.Tags, "maintenance-pending")
					t.releaseEscorts(p, f)
					holdAutomaticFleet(p, f, "Empty legacy landing released for automatic duties")
					continue
				}
				if j.Kind == "landing-reserve" {
					continue
				}
				j.Kind = "landing-reserve"
				j.Status = "Awaiting a coordinated landing force and scout"
				saveLogisticsWork(f, j)
				holdAutomaticFleet(p, f, j.Status)
			}
		}
	}
}

func landingBaseRequested(p *Player, num int, fleets []*Fleet) bool {
	for _, f := range fleets {
		if op := readLandingOperation(f); op != nil && !f.Delete && landingTransport(p, f) && op.Target == num && op.Phase == "base" {
			return true
		}
	}
	return false
}

const landingGroundReportTag = "automatic-landing-ground-report"

func (t *turnGenerator) landingRequirement(p *Player, target *Planet) int {
	rules := &t.game.Rules
	people, coverage := target.Cargo.Colonists, target.Spec.DefenseCoverage
	var owner, year, survivors int
	var observedCoverage, defenseMultiplier float64
	_, err := fmt.Sscanf(target.GetTag(landingGroundReportTag), "%d:%d:%d:%f:%f", &owner, &year, &survivors, &observedCoverage, &defenseMultiplier)
	fought := err == nil && owner == target.PlayerNum
	if fought && (target.ReportAge == ReportAgeUnexplored || t.game.Year-target.ReportAge <= year) {
		people, coverage = survivors, observedCoverage
	}
	survival := max(.1, 1-coverage*rules.InvasionDefenseCoverageFactor)
	multiplier := reportedInvasionDefense(rules)
	if fought && defenseMultiplier > 0 {
		multiplier = min(multiplier, defenseMultiplier)
	}
	losses := float64(people) * multiplier / max(.1, p.Race.Spec.InvasionAttackBonus) / survival
	need := max(1, int(math.Ceil(losses*1.10))+1)
	probe := max(2500, p.Automation.transportPolicy().AttritionMinimumPeople/100)
	if !fought && need > probe && usefulAttritionLanding(rules, p, target, probe) {
		return probe
	}
	return need
}

func (t *turnGenerator) landingMembers(p *Player, leader *Fleet) []*Fleet {
	group := []*Fleet{}
	for _, f := range t.game.getFleets(p.Num) {
		if !f.Delete && (f == leader || f.GetTag(landingMemberTag) == strconv.Itoa(leader.Num)) {
			group = append(group, f)
		}
	}
	slices.SortFunc(group, func(a, b *Fleet) int { return a.Num - b.Num })
	return group
}

func landingAvailable(p *Player, f *Fleet) bool {
	if landingTransport(p, f) && (f.Spec.Radiating || f.Cargo.Colonists > landingArmyLimit) {
		return false
	}
	if job := logisticsWork(f); landingTransport(p, f) && job != nil && job.Kind != "invade" && job.Kind != "invade-load" && job.Kind != "landing-reserve" && job.Kind != "salvage" {
		return false // an assigned civilian delivery is not an idle transport
	}
	// An empty collector is working, not idle. Let a known, still valid debris
	// pickup finish before assigning its hold to an army. Stale jobs must not
	// reserve ships forever, and a reused salvage number is a different target.
	if j := logisticsWork(f); !p.Automation.DisableSalvage && j != nil && j.Kind == "salvage" {
		if salvage := p.GetSalvageIntel(j.TargetNum); salvage != nil && !salvage.Delete && salvage.Position == j.Position && salvage.Cargo.ToMineral().Total() > 0 {
			return false
		}
	}
	return !f.Delete && !f.Starbase && !landingManaged(f) && !f.InDesignMaintenance() && !f.InAutomaticSafety() && !fuelNetworkReserved(f) && readTankerJob(f) == nil && f.GetTag(tankerAwaitTag) == "" && !p.FleetInActiveAssembly(f) && f.GetTag("maintenance-pending") == "" && f.GetTag(containmentTag) == "" && f.GetTag(anticipatedSalvageTag) == "" && f.GetTag(escortTag) == ""
}

type landingPickup struct {
	fleet  *Fleet
	source *Planet
	load   int
	score  float64
}

func (t *turnGenerator) landingPickups(p *Player, target *Planet, rally Vector, reserved map[int]int, required int) []landingPickup {
	// Rank cheap geometric/population candidates first. Running the full fuel
	// graph for every fleet/source pair made a large galaxy's first turn take
	// minutes before any ship could be assigned.
	candidates := []landingPickup{}
	for _, f := range t.game.getFleets(p.Num) {
		if !landingTransport(p, f) || !landingAvailable(p, f) || !automaticPassengersAllowed(p, f) || f.carriesInfrastructure() || f.Cargo.ToMineral().Total() > 0 {
			continue
		}
		job := logisticsWork(f)
		if f.Cargo.Colonists > 0 {
			if colonizerPassengerCarrier(p, f) || job != nil && (job.Kind == "invade" || job.Kind == "landing-reserve") {
				candidates = append(candidates, landingPickup{fleet: f, score: f.Position.DistanceTo(rally) - 2000000})
			}
			continue
		}
		load := min(f.availableColonistSpace(), landingArmyLimit)
		if load <= 0 {
			continue
		}
		reach := f.StrategicSpeed(automaticMaxWarp(p, f)) * 12
		for _, source := range t.game.getPlanets(p.Num) {
			if source.Cargo.Colonists-sourcePopulationReserve(p, source)-reserved[source.Num] < load {
				continue
			}
			pickup, delivery := f.Position.DistanceTo(source.Position), source.Position.DistanceTo(rally)
			if !source.Spec.HasStargate && (pickup > reach || delivery > reach) {
				continue
			}
			score := pickup + delivery
			if source.Spec.MaxPopulation > 0 && source.Cargo.Colonists >= populationTarget(source, 95) {
				score -= 1000000
			}
			candidates = append(candidates, landingPickup{f, source, load, score})
		}
	}
	slices.SortFunc(candidates, func(a, b landingPickup) int {
		if a.score < b.score {
			return -1
		}
		if a.score > b.score {
			return 1
		}
		if a.fleet.Num != b.fleet.Num {
			return a.fleet.Num - b.fleet.Num
		}
		if a.source != nil && b.source != nil {
			return a.source.Num - b.source.Num
		}
		return 0
	})
	used := map[int]bool{}
	attempts := map[int]int{}
	stock := map[int]int{}
	for num, amount := range reserved {
		stock[num] = amount
	}
	choices := []landingPickup{}
	total, tried := 0, 0
	for _, c := range candidates {
		if used[c.fleet.Num] || attempts[c.fleet.Num] >= 4 || total+c.load+c.fleet.Cargo.Colonists > landingArmyLimit {
			continue
		}
		if c.source != nil && c.source.Cargo.Colonists-sourcePopulationReserve(p, c.source)-stock[c.source.Num] < c.load {
			continue
		}
		tried++
		if tried > 256 {
			break
		}
		attempts[c.fleet.Num]++
		probe := *c.fleet
		if c.source != nil {
			probe.Position, probe.Fuel = c.source.Position, c.fleet.Spec.FuelCapacity
			probe.Cargo.Colonists += c.load
		}
		out := planAutomaticRoute(&t.game.Rules, p, []*Fleet{&probe}, rally, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: 12})
		if out == nil || out.WaitForFuel {
			continue
		}
		if c.source != nil && c.fleet.Position != c.source.Position {
			pickup := planAutomaticRoute(&t.game.Rules, p, []*Fleet{c.fleet}, c.source.Position, c.source.ToTarget(), automaticRoutePolicy{MaxTurns: 12})
			if pickup == nil || pickup.WaitForFuel {
				continue
			}
		}
		choices = append(choices, c)
		used[c.fleet.Num] = true
		total += c.load + c.fleet.Cargo.Colonists
		if c.source != nil {
			stock[c.source.Num] += c.load
		}
		if total >= required {
			break
		}
	}
	return choices
}

func (t *turnGenerator) startLandingOperation(p *Player, handled map[int]bool) *Fleet {
	if p.Automation.DisablePassengerTransport || p.Race.Spec.LivesOnStarbases {
		return nil
	}
	// Empty oversized legacy groups must not reserve many civilian hulls for a small army.
	for _, f := range slices.Clone(t.game.getFleets(p.Num)) {
		if landingAvailable(p, f) && landingTransport(p, f) && f.CargoMass() == 0 && f.Spec.CargoCapacity > landingArmyLimit && f.Spec.TotalShips > 1 {
			t.disperseLandingTransports(p, f, false)
		}
	}
	var anchor *Fleet
	for _, f := range t.game.getFleets(p.Num) {
		if !landingAvailable(p, f) {
			continue
		}
		legacyLoaded := logisticsWork(f) != nil && logisticsWork(f).Kind == "landing-reserve" && f.Cargo.Colonists > 0
		if landingTransport(p, f) && automaticPassengersAllowed(p, f) && (f.CargoMass() == 0 || legacyLoaded || colonizerPassengerCarrier(p, f)) && f.Spec.CargoCapacity > 0 && (anchor == nil || f.Num < anchor.Num) {
			anchor = f
		}
	}
	if anchor == nil {
		return nil
	}
	targets := []*Planet{}
	for _, target := range p.PlanetIntels {
		if invasionCandidate(p, target) && !handled[target.Num] && !t.landingTargetClaimed(p, target.Num) {
			if !t.landingPrepared(p, target) {
				target.SetTag(landingPreparationTag, "true")
				continue
			}
			retry, _ := strconv.Atoi(target.GetTag(landingRetryTag))
			if retry > t.game.Year {
				continue
			}
			targets = append(targets, target)
		}
	}
	slices.SortFunc(targets, func(a, b *Planet) int {
		if rank := expansionPriority(p, b) - expansionPriority(p, a); rank != 0 {
			return rank
		}
		da, db := anchor.Position.DistanceTo(a.Position), anchor.Position.DistanceTo(b.Position)
		if da < db {
			return -1
		}
		if da > db {
			return 1
		}
		return a.Num - b.Num
	})
	for i, target := range targets {
		if i >= 8 {
			break
		}
		// Start inside a conservative single-year jump; verify every actual
		// participant and the physically merged fleet again before departure.
		direction := anchor.Position.Subtract(target.Position).Normalized()
		if direction.LengthSquared() == 0 {
			direction = VectorFloat64{X: 1}
		}
		radius := max(1.0, min(35.0, anchor.StrategicSpeed(automaticCruiseWarp(p, anchor))-2))
		rally := target.Position.ToFloat64().Add(direction.Scale(radius)).ToInt(true)
		if rally == target.Position || hostileOrbitalBase(p, rally) {
			continue
		}
		scout := t.pickLandingScout(p, rally)
		reserved := t.landingReservations(p)
		selected := []landingPickup{}
		used := map[int]bool{}
		need, total := t.landingArmyNeed(p, target), 0
		for _, c := range t.landingPickups(p, target, rally, reserved, need) {
			if used[c.fleet.Num] {
				continue
			}
			if c.source != nil && c.source.Cargo.Colonists-sourcePopulationReserve(p, c.source)-reserved[c.source.Num] < c.load {
				continue
			}
			if c.source != nil {
				reserved[c.source.Num] += c.load
			}
			selected = append(selected, c)
			used[c.fleet.Num] = true
			total += c.load + c.fleet.Cargo.Colonists
			if total >= need {
				break
			}
		}
		if len(selected) == 0 {
			continue
		}
		leader := selected[0].fleet
		op := &landingOperation{Target: target.Num, Owner: target.PlayerNum, Need: need, Scout: 0, Rally: rally, Phase: "gather", Started: t.game.Year, EscortDesigns: map[int]bool{}}
		for _, c := range selected {
			t.releaseEscorts(p, c.fleet)
			c.fleet.SetTag(landingMemberTag, strconv.Itoa(leader.Num))
			if c.source != nil {
				saveLogisticsWork(c.fleet, &logisticsJob{Kind: "landing-load", TargetNum: c.source.Num, DeliveryNum: target.Num, Passengers: c.load, Position: c.source.Position, Status: "Loading a coordinated landing army"})
			} else {
				saveLogisticsWork(c.fleet, &logisticsJob{Kind: "landing-rally", DeliveryNum: target.Num, Position: rally, Status: "Joining the landing army with existing passengers"})
			}
		}
		if scout != nil {
			op.Scout = scout.Num
			scout.SetTag(landingMemberTag, strconv.Itoa(leader.Num))
			delete(scout.Tags, scoutReconTag)
			delete(scout.Tags, priorityReconTag)
		}
		t.recruitLandingProtection(p, leader, op)
		saveLandingOperation(leader, op)
		return leader
	}
	return nil
}

func (t *turnGenerator) pickLandingScout(p *Player, rally Vector) *Fleet {
	candidates := []*Fleet{}
	for _, f := range t.game.getFleets(p.Num) {
		if landingAvailable(p, f) && p.Automation.FleetRoles[f.Num] == "scout" && scoutReconEligible(p, f) && f.GetTag(preyWatchTag) == "" {
			candidates = append(candidates, f)
		}
	}
	slices.SortFunc(candidates, func(a, b *Fleet) int {
		da, db := a.Position.DistanceTo(rally), b.Position.DistanceTo(rally)
		if da < db {
			return -1
		}
		if da > db {
			return 1
		}
		return a.Num - b.Num
	})
	for _, scout := range candidates {
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{scout}, rally, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: 12})
		if route != nil && !route.WaitForFuel {
			return scout
		}
	}
	return nil
}

func (t *turnGenerator) routeLanding(p *Player, f *Fleet, point Vector, target MapObjectTarget, threat bool, maxTurns int) bool {
	if f.Position == point {
		holdAutomaticFleet(p, f, "Landing force assembled; awaiting coordinated departure")
		return true
	}
	route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point, target, automaticRoutePolicy{AllowThreat: threat, Fast: true, MaxTurns: maxTurns})
	if route == nil || route.WaitForFuel {
		holdAutomaticFleet(p, f, "Landing operation awaiting a safe route with sufficient fuel")
		requestAutomaticRescue(p, f)
		return false
	}
	applyExpeditionRoute(p, f, route)
	return true
}

func (t *turnGenerator) mergeLandingGroup(p *Player, leader *Fleet, group []*Fleet) bool {
	if len(group) < 2 {
		return true
	}
	for _, f := range group {
		if landingTransport(p, leader) && (!landingTransport(p, f) || f.Spec.Radiating) {
			return false
		}
		if f.Position != leader.Position || f.InDesignMaintenance() || fuelNetworkReserved(f) {
			return false
		}
	}
	colonyRoles := map[int]string{}
	_ = json.Unmarshal([]byte(leader.GetTag("landing-colonizer-roles")), &colonyRoles)
	for _, f := range group {
		if role := p.Automation.FleetRoles[f.Num]; colonizationMode(role) != "" {
			for _, token := range f.Tokens {
				if token.design != nil && token.design.Spec.Colonizer {
					colonyRoles[token.DesignNum] = role
				}
			}
		}
	}
	if _, err := NewOrderer().Merge(&t.game.Rules, p, group); err != nil {
		return false
	}
	if len(colonyRoles) > 0 {
		data, _ := json.Marshal(colonyRoles)
		leader.SetTag("landing-colonizer-roles", string(data))
	}
	for _, f := range group[1:] {
		t.game.deleteFleet(f)
		delete(p.Automation.FleetRoles, f.Num)
		delete(p.Automation.RebuildFleets, f.Num)
	}
	return true
}

// Only actual/predicted arrival at the landing site matters. A course crossing
// halfway through the year is not a battle in the native movement engine.
func landingOrbitalReports(p *Player, point Vector) (present, incoming []*Fleet) {
	for _, e := range p.FleetIntels {
		if e == nil || e.Delete || e.Starbase || e.PlayerNum == p.Num || p.IsFriend(e.PlayerNum) || e.ReportAge > 1 || !(patrolReportedCombatant(p, e) || patrolReportedBomber(p, e)) {
			continue
		}
		if e.Position == point {
			present = append(present, e)
			continue
		}
		if e.WarpSpeed <= 0 || e.Heading.LengthSquared() == 0 {
			continue
		}
		delta := point.Subtract(e.Position).ToFloat64()
		heading := e.Heading.Normalized()
		along := delta.X*heading.X + delta.Y*heading.Y
		cross := math.Abs(delta.X*heading.Y - delta.Y*heading.X)
		if along > 0 && along <= e.observedStrategicSpeed()*float64(e.ReportAge+1)+3 && cross <= 3 {
			incoming = append(incoming, e)
		}
	}
	return
}

func (t *turnGenerator) landingOrbitClear(p *Player, leader *Fleet, op *landingOperation) bool {
	target := p.GetPlanetIntel(op.Target)
	if target == nil || leader.Spec.Radiating {
		return false
	}
	return t.landingOrbitSafeWith(p, leader, op, target, t.landingArrivingCover(p, leader, target))
}

func (t *turnGenerator) landingArrivingCover(p *Player, leader *Fleet, target *Planet) []*Fleet {
	cover := []*Fleet{}
	for _, f := range t.game.getFleets(p.Num) {
		role := p.Automation.FleetRoles[f.Num]
		if f == leader || f.Delete || !f.Spec.HasWeapons || role != "patrol" && role != "escort" ||
			f.InAutomaticSafety() || f.InDesignMaintenance() {
			continue
		}
		if f.Position == target.Position && (len(f.Waypoints) <= 1 || f.Waypoints[1].Position == target.Position) {
			cover = append(cover, f)
			continue
		}
		if f.GetTag(landingMemberTag) != strconv.Itoa(leader.Num) || f.GetTag("automatic-landing-screen") == "" || len(f.Waypoints) != 2 {
			continue
		}
		wp := f.Waypoints[1]
		if wp.Position == target.Position && (wp.WarpSpeed == StargateWarpSpeed || wp.WarpSpeed > 0 && f.Position.DistanceTo(wp.Position) <= f.StrategicSpeed(wp.WarpSpeed)) {
			cover = append(cover, f)
		}
	}
	return cover
}

func (t *turnGenerator) landingOrbitSafeWith(p *Player, leader *Fleet, op *landingOperation, target *Planet, cover []*Fleet) bool {
	reports, known := t.landingEncounterReports(p, op, target)
	if leader.Spec.Radiating || !known {
		return false
	}
	if len(reports) == 0 {
		return true
	}
	if len(cover) == 0 {
		return false
	}
	evaluator := t.defensiveEvaluator(p)
	if !evaluator.assessFormations(p, cover, reports, target.Position, 1, false, true).Ready {
		return false
	}
	result := evaluator.assessFormations(p, append([]*Fleet{leader}, cover...), reports, target.Position, 1, false, true)
	if result.Ready {
		for _, guard := range cover {
			applyPatrolBattlePlan(p, guard, result.Tactic)
		}
	}
	return result.Ready
}

func (t *turnGenerator) landingEncounterReports(p *Player, op *landingOperation, target *Planet) ([]*Fleet, bool) {
	if op.ScanYear == 0 || t.game.Year-op.ScanYear > 1 {
		return nil, false
	}
	present, incoming := landingOrbitalReports(p, target.Position)
	reports := append(present, incoming...)
	if target.Spec.HasStarbase {
		base := reportedRecaptureBase(p, target, true)
		if base == nil {
			return nil, false
		}
		reports = append(reports, base)
	} else if op.Base {
		return nil, false
	}
	return reports, true
}

func (t *turnGenerator) landingTargetClaimed(p *Player, num int) bool {
	for _, f := range t.game.getFleets(p.Num) {
		if op := readLandingOperation(f); !f.Delete && op != nil && op.Target == num {
			return true
		}
	}
	return false
}
func (t *turnGenerator) landingReservations(p *Player) map[int]int {
	_, reserved := t.passengerReservations(p, 0)
	return reserved
}
func (t *turnGenerator) planLandingOperations() {
	t.releaseBlockedLandings()
	t.planLandingOperationsForPriority(true)
	t.planLandingOperationsForPriority(false)
}

func (t *turnGenerator) planLandingOperationsForPriority(landingsFirst bool) {
	if t.ordersOnly {
		return
	}
	for _, p := range t.automaticPlayers() {
		if p.Automation.transportPolicy().LandingsFirst != landingsFirst {
			continue
		}
		for _, f := range t.game.getFleets(p.Num) {
			n, _ := strconv.Atoi(f.GetTag(landingMemberTag))
			owner := t.game.getFleet(p.Num, n)
			if n != 0 && (owner == nil || owner.Delete || readLandingOperation(owner) == nil) {
				delete(f.Tags, landingMemberTag)
				delete(f.Tags, landingBaseReconTag)
				delete(f.Tags, landingRaidTag)
				delete(f.Tags, "automatic-landing-screen")
			}
		}
		leaders := slices.Clone(t.game.getFleets(p.Num))
		handled := map[int]bool{}
		baseDistance := map[int]float64{}
		territorialBases := map[int]bool{}
		colonies := t.game.getPlanets(p.Num)
		for _, f := range leaders {
			if op := readLandingOperation(f); op != nil && op.Phase == "base" {
				if target := p.GetPlanetIntel(op.Target); target != nil {
					distance := math.Inf(1)
					for _, colony := range colonies {
						distance = min(distance, colony.Position.DistanceTo(target.Position))
					}
					baseDistance[f.Num] = distance
					territorialBases[f.Num] = patrolOwnTerritory(p, target.Position, target.Num)
				}
			}
		}
		slices.SortFunc(leaders, func(a, b *Fleet) int {
			if territorialBases[a.Num] != territorialBases[b.Num] {
				if territorialBases[a.Num] {
					return -1
				}
				return 1
			}
			da, aBase := baseDistance[a.Num]
			db, bBase := baseDistance[b.Num]
			if aBase != bBase {
				if aBase {
					return -1
				}
				return 1
			}
			if da < db {
				return -1
			}
			if da > db {
				return 1
			}
			return a.Num - b.Num
		})
		for _, leader := range leaders {
			op := readLandingOperation(leader)
			if op == nil || leader.Delete {
				continue
			}
			handled[op.Target] = true
			target := p.GetPlanetIntel(op.Target)
			if !landingTransport(p, leader) || p.Automation.DisablePassengerTransport || target == nil || target.PlayerNum != op.Owner || p.IsFriend(target.PlayerNum) || op.Phase == "landed" || op.Phase == "cancelled" || t.game.Year-op.Started > 30 {
				t.finishLandingOperation(p, leader, op)
				continue
			}
			if leader.InAutomaticSafety() || leader.InDesignMaintenance() {
				continue
			}
			if !t.separateLegacyLandingEscort(p, leader, op) {
				continue
			}
			if op.Phase != "assault" {
				need := t.landingArmyNeed(p, target)
				if op.Phase == "scan" && op.Need > 0 {
					need = min(op.Need, need)
				}
				op.Need = need
			}
			t.reinforceLandingOperation(p, leader, op, target)
			t.recruitLandingProtection(p, leader, op)
			t.advanceLandingOperation(p, leader, op, target)
			if op.Phase != "cancelled" {
				saveLandingOperation(leader, op)
			}
		}
		// Complete existing armies before assigning scarce free holds to new
		// targets. Otherwise each turn creates another undersized landing.
		// Loading still starts immediately, before a scout or patrol is free.
		for range 3 {
			leader := t.startLandingOperation(p, handled)
			if leader == nil {
				break
			}
			op := readLandingOperation(leader)
			t.advanceLandingOperation(p, leader, op, p.GetPlanetIntel(op.Target))
			if op.Phase != "cancelled" {
				saveLandingOperation(leader, op)
			}
		}
	}
}

func (t *turnGenerator) reinforceLandingOperation(p *Player, leader *Fleet, op *landingOperation, target *Planet) {
	if op.Phase == "assault" {
		return
	}
	total := 0
	for _, f := range t.landingMembers(p, leader) {
		total += f.Cargo.Colonists
		if j := logisticsWork(f); j != nil && j.Kind == "landing-load" {
			total += j.Passengers
		}
	}
	if total >= op.Need {
		return
	}
	reserved := t.landingReservations(p)
	used := map[int]bool{}
	for _, c := range t.landingPickups(p, target, op.Rally, reserved, op.Need-total) {
		if used[c.fleet.Num] || total+c.load+c.fleet.Cargo.Colonists > landingArmyLimit || c.source != nil && c.source.Cargo.Colonists-sourcePopulationReserve(p, c.source)-reserved[c.source.Num] < c.load {
			continue
		}
		c.fleet.SetTag(landingMemberTag, strconv.Itoa(leader.Num))
		used[c.fleet.Num] = true
		if c.source != nil {
			reserved[c.source.Num] += c.load
			saveLogisticsWork(c.fleet, &logisticsJob{Kind: "landing-load", TargetNum: c.source.Num, DeliveryNum: op.Target, Passengers: c.load, Position: c.source.Position, Status: "Loading landing reinforcements"})
		} else {
			saveLogisticsWork(c.fleet, &logisticsJob{Kind: "landing-rally", DeliveryNum: op.Target, Position: op.Rally})
		}
		total += c.load + c.fleet.Cargo.Colonists
		if total >= op.Need {
			break
		}
	}
}

func (t *turnGenerator) recruitLandingProtection(p *Player, leader *Fleet, op *landingOperation) {
	if op.Phase == "assault" || op.Phase == "base" || len(op.EscortDesigns) > 0 {
		return
	}
	owner := strconv.Itoa(leader.Num)
	candidates := []landingBaseGuard{}
	for _, f := range t.game.getFleets(p.Num) {
		role := p.Automation.FleetRoles[f.Num]
		member := f.GetTag(landingMemberTag) == owner
		if role != "patrol" && role != "escort" || !f.Spec.HasWeapons || f.Delete || f.InAutomaticSafety() || f.InDesignMaintenance() || defencePatrolCommitted(p, f) || fuelNetworkManaged(f) {
			continue
		}
		if member && (f.GetTag(landingRaidTag) != "" || f.GetTag("automatic-landing-screen") != "") {
			return
		}
		if !member && (!landingAvailable(p, f) || !patrolAvailable(p, f) || readPatrolJob(f).Leader != 0 || patrolNeededForColony(p, f)) {
			continue
		}
		if f.Position.DistanceTo(op.Rally) > 2*f.StrategicSpeed(automaticMaxWarp(p, f))+3 {
			continue
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, op.Rally, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: 2, Fast: true})
		if route == nil || route.WaitForFuel {
			continue
		}
		candidates = append(candidates, landingBaseGuard{fleet: f, route: route})
	}
	slices.SortFunc(candidates, compareLandingGuards)
	keep := map[int]bool{}
	ships := 0
	for _, c := range candidates {
		f := c.fleet
		keep[f.Num] = true
		f.SetTag(landingMemberTag, owner)
		delete(f.Tags, patrolTag)
		applyExpeditionRoute(p, f, c.route)
		ships += f.Spec.TotalShips
		if ships >= p.Automation.transportPolicy().LandingEscortShips {
			break
		}
	}
	t.releaseLandingBaseSurplus(p, leader, keep)
}

// A pickup can become inaccessible after it was reserved (for example, a bomber
// arrives at the source). Empty transports may use another surplus colony; the
// reservation and both funded legs must change together, without borrowing the
// same people twice or assuming fuel at a colony without a dock.
func (t *turnGenerator) retargetLandingPickup(p *Player, f *Fleet, op *landingOperation, job *logisticsJob) bool {
	if f.CargoMass() != 0 || fuelNetworkManaged(f) {
		return false
	}
	load := min(f.availableColonistSpace(), landingArmyLimit)
	if load <= 0 {
		return false
	}
	reserved := t.landingReservations(p)
	candidates := []landingPickup{}
	for _, source := range t.game.getPlanets(p.Num) {
		if source.Num == job.TargetNum || source.Cargo.Colonists-sourcePopulationReserve(p, source)-reserved[source.Num] < load {
			continue
		}
		score := f.Position.DistanceTo(source.Position) + source.Position.DistanceTo(op.Rally)
		if source.Spec.MaxPopulation > 0 && source.Cargo.Colonists >= populationTarget(source, 95) {
			score -= 1000000
		}
		candidates = append(candidates, landingPickup{fleet: f, source: source, load: load, score: score})
	}
	slices.SortFunc(candidates, func(a, b landingPickup) int {
		if a.score < b.score {
			return -1
		}
		if a.score > b.score {
			return 1
		}
		return a.source.Num - b.source.Num
	})
	for _, c := range candidates[:min(4, len(candidates))] {
		pickup := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, c.source.Position, c.source.ToTarget(), automaticRoutePolicy{MaxTurns: 12})
		if pickup == nil || pickup.WaitForFuel || len(pickup.ArrivalFuel) == 0 {
			continue
		}
		probe := *f
		probe.Position, probe.Fuel = c.source.Position, pickup.ArrivalFuel[0]
		if probe.CanFuel(p, c.source) {
			probe.Fuel = probe.Spec.FuelCapacity
		}
		probe.Cargo.Colonists += load
		out := planAutomaticRoute(&t.game.Rules, p, []*Fleet{&probe}, op.Rally, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: 12})
		if out == nil || out.WaitForFuel {
			continue
		}
		job.TargetNum, job.Position, job.Passengers = c.source.Num, c.source.Position, load
		job.Status = "Retargeting blocked landing pickup to an accessible surplus colony"
		saveLogisticsWork(f, job)
		delete(f.Tags, fuelRequestTag) // the old failed voyage no longer needs rescue
		applyExpeditionRoute(p, f, pickup)
		return true
	}
	return false
}

func (t *turnGenerator) advanceLandingOperation(p *Player, leader *Fleet, op *landingOperation, target *Planet) {
	// A fresh ordinary scan or an allied orbital patrol can confirm that the
	// base is gone. Clear the remembered warning, then require the scout again.
	if op.Phase == "base" && !target.Spec.HasStarbase && (target.ReportAge <= 1 || op.ScanYear > 0 && op.ScanYear >= t.game.Year-1) {
		t.releaseLandingBaseRecon(p, op)
		t.releaseLandingBaseSurplus(p, leader, t.landingBaseProtectors(p, leader, op))
		op.Base = false
		op.BaseRallyBy = 0
		op.BaseRallyRetryYear = 0
		op.Phase = "scan"
		op.ScanYear = 0
	}
	members := t.landingMembers(p, leader)
	localTroops := 0
	for _, f := range members {
		if f.Position == op.Rally && landingTransport(p, f) && !f.Spec.Radiating && !f.InAutomaticSafety() && !f.InDesignMaintenance() {
			localTroops += f.Cargo.Colonists
		}
	}
	scout := t.game.getFleet(p.Num, op.Scout)
	if scout == nil || scout.Delete || !landingScout(p, scout) {
		op.Scout = 0
		scout = t.pickLandingScout(p, op.Rally)
		if scout != nil {
			op.Scout = scout.Num
			scout.SetTag(landingMemberTag, strconv.Itoa(leader.Num))
		}
	}
	ready := leader.Position == op.Rally
	transports := []*Fleet{leader}
	guards := []*Fleet{}
	for _, f := range members {
		role := p.Automation.FleetRoles[f.Num]
		if (role == "patrol" || role == "escort") && defencePatrolCommitted(p, f) {
			delete(f.Tags, landingMemberTag)
			delete(f.Tags, landingBaseReconTag)
			delete(f.Tags, landingRaidTag)
			delete(f.Tags, "automatic-landing-screen")
			continue
		}
		if f.InAutomaticSafety() || f.InDesignMaintenance() {
			continue
		}
		if landingTransport(p, f) {
			surplusAtRally := localTroops >= op.Need && (f.Position != op.Rally || f.Cargo.Colonists == 0)
			armyAlreadyLoaded := (op.Base || target.Spec.HasStarbase) && leader.Cargo.Colonists >= op.Need && !leader.Spec.Radiating
			if op.Phase != "assault" && f != leader && (surplusAtRally || armyAlreadyLoaded) {
				delete(f.Tags, landingMemberTag)
				delete(f.Tags, logisticsTag)
				holdAutomaticFleet(p, f, "Landing army assembled; returning to passenger logistics")
				continue
			}
			if f.Spec.Radiating {
				holdAutomaticFleet(p, f, "Landing held: transport has a radiating engine")
				ready = false
				continue
			}
			job := logisticsWork(f)
			if job != nil && job.Kind == "landing-load" {
				source := t.game.getPlanet(job.TargetNum)
				if source == nil || !source.OwnedBy(p.Num) {
					if t.retargetLandingPickup(p, f, op, job) {
						ready = false
						continue
					}
					if f == leader {
						t.finishLandingOperation(p, leader, op)
						op.Phase = "cancelled"
						return
					}
					delete(f.Tags, landingMemberTag)
					delete(f.Tags, logisticsTag)
					holdAutomaticFleet(p, f, "Landing pickup lost; returning to passenger logistics")
					continue
				}
				if f.Position != source.Position {
					if !t.routeLanding(p, f, source.Position, source.ToTarget(), false, 0) {
						t.retargetLandingPickup(p, f, op, job)
					}
					ready = false
					continue
				}
				load := min(f.availableColonistSpace(), job.Passengers, landingArmyLimit)
				if source.Cargo.Colonists-sourcePopulationReserve(p, source) < load || t.armedEnemyAt(p, source.Position) {
					if !t.retargetLandingPickup(p, f, op, job) {
						holdAutomaticFleet(p, f, "Landing army awaits a full safe load at source")
					}
					ready = false
					continue
				}
				probe := *f
				probe.Cargo.Colonists += load
				route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{&probe}, op.Rally, MapObjectTarget{}, automaticRoutePolicy{})
				if route == nil || route.WaitForFuel {
					if !t.retargetLandingPickup(p, f, op, job) {
						holdAutomaticFleet(p, f, "Landing pickup awaits fuel for its loaded voyage")
					}
					ready = false
					continue
				}
				source.Cargo.Colonists -= load
				f.Cargo.Colonists += load
				job.Kind = "landing-rally"
				job.Position = op.Rally
				saveLogisticsWork(f, job)
			}
			if op.Phase == "assault" {
				continue
			}
			if f.Position != op.Rally {
				t.routeLanding(p, f, op.Rally, MapObjectTarget{}, false, 0)
				continue
			}
			holdAutomaticFleet(p, f, "Loaded transport at landing rendezvous")
			if f != leader {
				transports = append(transports, f)
			}
		} else if (role == "patrol" || role == "escort") && !landingScout(p, f) {
			if f.GetTag(landingRaidTag) != "" {
				restorePatrolBattlePlan(p, f)
				if f.Position == op.Rally {
					delete(f.Tags, landingRaidTag)
				}
			}
			if f.GetTag("automatic-landing-screen") != "" {
				t.routeLanding(p, f, target.Position, target.ToTarget(), true, 1)
				if f.Position != target.Position {
					guards = append(guards, f)
				}
				continue
			}
			guards = append(guards, f)
			if f.Position != op.Rally {
				t.routeLanding(p, f, op.Rally, MapObjectTarget{}, false, 0)
			} else {
				holdAutomaticFleet(p, f, "Guarding landing rendezvous")
			}
		}
	}
	if op.Phase == "assault" {
		if leader.Position != target.Position {
			t.routeLanding(p, leader, target.Position, target.ToTarget(), true, 1)
		}
		return
	}
	if leader.Position == op.Rally {
		t.mergeLandingGroup(p, leader, transports)
	}
	if scout != nil && !scout.Delete && op.Scout != 0 {
		if op.Phase == "scan" {
			t.routeLanding(p, scout, target.Position, target.ToTarget(), true, 0)
		} else if op.Phase == "base" && scout.GetTag(landingBaseReconTag) == "" {
			// One transmitted warning is enough; do not feed replacement scouts
			// into a known base while patrols prepare its destruction.
			t.routeLanding(p, scout, op.Rally, MapObjectTarget{}, false, 0)
		} else if op.Phase == "base" {
			// Keep the fallback contact's outbound route until its report arrives.
		} else if scout.Position != op.Rally {
			t.routeLanding(p, scout, op.Rally, MapObjectTarget{}, false, 0)
			ready = false
		} else {
			holdAutomaticFleet(p, scout, "Scout assembled with the landing force")
		}
	} else {
		ready = false
	}
	protection := 0
	for _, f := range members {
		if (p.Automation.FleetRoles[f.Num] == "patrol" || p.Automation.FleetRoles[f.Num] == "escort") && !landingScout(p, f) && !f.InAutomaticSafety() && !defencePatrolCommitted(p, f) && (f.Position == op.Rally || f.GetTag("automatic-landing-screen") != "" && f.Position == target.Position) {
			protection += f.Spec.TotalShips
		}
	}
	if op.Phase == "gather" && protection < p.Automation.transportPolicy().LandingEscortShips {
		ready = false
	}
	// Once the troops and core escort are in position, a known orbital base can
	// be assessed immediately. An unknown design still requires a scout.
	if (op.Phase != "gather" || leader.Position == op.Rally && leader.Cargo.Colonists >= op.Need && protection >= p.Automation.transportPolicy().LandingEscortShips) && (op.Base || target.Spec.HasStarbase) {
		op.Phase = "base"
		if ready && protection >= p.Automation.transportPolicy().LandingEscortShips && leader.Cargo.Colonists >= op.Need && t.tryLandingArrival(p, leader, op, target, members) {
			return
		}
		local := []*Fleet{}
		for _, guard := range guards {
			if guard.Position == op.Rally {
				local = append(local, guard)
			}
		}
		t.prepareLandingBaseAssault(p, leader, op, target, local)
		return
	}
	if !ready || leader.Cargo.Colonists < op.Need && (leader.CargoMass() != leader.Spec.CargoCapacity || !usefulAttritionLanding(&t.game.Rules, p, target, leader.Cargo.Colonists)) {
		if op.Phase == "gather" && leader.Cargo.Colonists < op.Need {
			t.landingEscortRaid(p, leader, op)
		}
		return
	}
	if op.Phase == "gather" {
		op.Phase = "scan"
		if scout != nil {
			t.routeLanding(p, scout, target.Position, target.ToTarget(), true, 1)
		}
		return
	}
	if op.Phase == "base" {
		op.Phase = "scan"
		op.ScanYear = 0
		return
	}
	if protection < p.Automation.transportPolicy().LandingEscortShips && len(op.EscortDesigns) == 0 {
		return
	}
	t.tryLandingArrival(p, leader, op, target, members)
}

func (t *turnGenerator) tryLandingArrival(p *Player, leader *Fleet, op *landingOperation, target *Planet, members []*Fleet) bool {
	if !t.landingPrepared(p, target) || leader.Cargo.Colonists > landingArmyLimit {
		return false
	}
	// Each ship keeps its own engines, passengers and route. Evaluate the
	// arrival as distinct fleets, including escorts already on the orbit.
	transportRoute := planAutomaticRoute(&t.game.Rules, p, []*Fleet{leader}, target.Position, target.ToTarget(), automaticRoutePolicy{AllowThreat: true, Fast: true, MaxTurns: 1})
	if transportRoute == nil || transportRoute.WaitForFuel || transportRoute.NetworkRequest != nil || transportRoute.Turns > 1 {
		return false
	}
	cover := []*Fleet{}
	routes := map[*Fleet]*expeditionRoute{}
	for _, guard := range members {
		if guard == leader || !guard.Spec.HasWeapons || guard.Cargo.Colonists > 0 || guard.InAutomaticSafety() || guard.InDesignMaintenance() {
			continue
		}
		role := p.Automation.FleetRoles[guard.Num]
		if role != "patrol" && role != "escort" || defencePatrolCommitted(p, guard) {
			continue
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{guard}, target.Position, target.ToTarget(), automaticRoutePolicy{AllowThreat: true, Fast: true, MaxTurns: 1})
		if route == nil || route.WaitForFuel || route.NetworkRequest != nil || route.Turns > 1 {
			continue
		}
		routes[guard] = route
		cover = append(cover, guard)
	}
	if !t.landingOrbitSafeWith(p, leader, op, target, cover) {
		// A safe escort can clear the orbit first when adding the loaded
		// transport makes the encounter unsafe.
		reports, known := t.landingEncounterReports(p, op, target)
		if known && len(reports) > 0 && len(cover) > 0 && t.defensiveEvaluator(p).assessFormations(p, cover, reports, target.Position, 1, false, true).Ready {
			for _, guard := range cover {
				guard.SetTag("automatic-landing-screen", "true")
				applyExpeditionRoute(p, guard, routes[guard])
			}
		}
		holdAutomaticFleet(p, leader, "Landing held: scout report, hostile orbit or incoming warships")
		return false
	}
	for _, guard := range cover {
		guard.SetTag("automatic-landing-screen", "true")
		applyExpeditionRoute(p, guard, routes[guard])
	}
	op.Phase = "assault"
	saveLogisticsWork(leader, &logisticsJob{Kind: "landing-assault", TargetNum: target.Num, DeliveryNum: target.Num, Position: target.Position, Status: "Coordinated landing with separate patrol and transport fleets"})
	applyExpeditionRoute(p, leader, transportRoute)
	return true
}

// Contact is recorded before battle, so a scout can transmit a base warning
// even when it is destroyed there. No remote hidden planet/fleet state is read.
func (t *turnGenerator) scanLandingContacts() {
	for _, p := range t.automaticPlayers() {
		for _, leader := range t.game.getFleets(p.Num) {
			op := readLandingOperation(leader)
			if op == nil || leader.Delete {
				continue
			}
			scout := t.game.getFleet(p.Num, op.Scout)
			target := t.game.getPlanet(op.Target)
			if target == nil {
				continue
			}
			if op.Phase == "base" {
				for _, guard := range t.game.getFleets(p.Num) {
					if !guard.Delete && guard.Position == target.Position && guard.GetTag(recaptureAssaultTag) == strconv.Itoa(op.Target) {
						op.Base = target.Spec.HasStarbase
						op.ScanYear = t.game.Year
						if intel := p.GetPlanetIntel(op.Target); intel != nil {
							intel.Spec.HasStarbase = op.Base
						}
						saveLandingOperation(leader, op)
						break
					}
				}
			}
			if scout == nil || scout.Delete || !landingScout(p, scout) || scout.Position != target.Position {
				continue
			}
			if p.discoverer == nil {
				p.discoverer = newDiscoverer(t.log, p)
			}
			p.discoverer.discoverPlanet(&t.game.Rules, target, false, false)
			p.discoverer.discoverPlanetStarbase(target)
			if target.Starbase != nil {
				for _, tok := range target.Starbase.Tokens {
					if tok.design != nil {
						p.discoverer.discoverDesign(tok.design, true)
					}
				}
			}
			for _, f := range t.game.Fleets {
				if !f.Delete && f.Position == target.Position && f.PlayerNum != p.Num {
					p.discoverer.discoverFleet(f, true)
					for _, tok := range f.Tokens {
						if tok.design != nil {
							p.discoverer.discoverDesign(tok.design, true)
						}
					}
				}
			}
			op.ScanYear = t.game.Year
			op.Base = target.Spec.HasStarbase
			if scout.GetTag(landingBaseReconTag) != "" {
				delete(scout.Tags, landingBaseReconTag)
				delete(scout.Tags, landingMemberTag)
				op.Scout = 0
			}
			saveLandingOperation(leader, op)
		}
	}
}

// Called after native orbital combat. Ground defenses affect native casualties
// but do not veto the prepared drop. A surviving hostile warship/base does.
func (t *turnGenerator) resolveLandingDrops() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			op := readLandingOperation(f)
			if op == nil || op.Phase != "assault" || f.Delete || f.Spec.Radiating || f.Cargo.Colonists <= 0 {
				continue
			}
			target := t.game.getPlanet(op.Target)
			if target == nil || f.Position != target.Position || target.Spec.HasStarbase || t.landingHostileAt(p, f.Position) {
				continue
			}
			if target.PlayerNum == p.Num {
				target.Cargo.Colonists += f.Cargo.Colonists
				f.Cargo.Colonists = 0
				op.Phase = "landed"
				saveLandingOperation(f, op)
				continue
			}
			if !target.Owned() || p.IsFriend(target.PlayerNum) {
				continue
			}
			defender := t.game.getPlayer(target.PlayerNum)
			if defender.Race.Spec.LivesOnStarbases {
				continue
			}
			t.colonistDrops.addInvasion(invasion{planet: target, defender: defender, attacker: p, attackers: f.Cargo.Colonists * 100, fleets: []*Fleet{f}})
			f.Cargo.Colonists = 0
			op.Phase = "landed"
			saveLandingOperation(f, op)
		}
	}
}

func (t *turnGenerator) finishLandingOperation(p *Player, leader *Fleet, op *landingOperation) {
	if !t.ordersOnly && (op.Phase == "landed" || op.Phase == "cancelled" && (leader.GetTag("landing-colonizer-roles") != "" || colonizationMode(p.Automation.FleetRoles[leader.Num]) != "")) {
		defer t.disperseEmptyLandingTransports(p, leader)
	}
	for _, f := range t.landingMembers(p, leader) {
		delete(f.Tags, landingMemberTag)
		delete(f.Tags, landingBaseReconTag)
		delete(f.Tags, landingRaidTag)
		delete(f.Tags, landingOperationTag)
		delete(f.Tags, "automatic-landing-screen")
		restorePatrolBattlePlan(p, f)
		delete(f.Tags, logisticsTag)
		if isAutomaticFleet(p, f) {
			holdAutomaticFleet(p, f, "Landing operation finished; resuming automatic duties")
		}
	}
	// Return surviving warships to patrol duty, keeping all remaining cargo on
	// the transports and dividing real fuel through the native split operation.
	source, dest := []ShipToken{}, []ShipToken{}
	for _, tok := range leader.Tokens {
		a, b := tok, tok
		if op.EscortDesigns[tok.DesignNum] {
			a.Quantity = 0
		} else {
			b.Quantity = 0
		}
		source = append(source, a)
		dest = append(dest, b)
	}
	ships := 0
	for _, tok := range dest {
		ships += tok.Quantity
	}
	if ships == 0 {
		return
	}
	remaining := 0
	for _, tok := range source {
		remaining += tok.Quantity
	}
	if remaining == 0 {
		p.Automation.FleetRoles[leader.Num] = "patrol"
		return
	}
	probe := *leader
	probe.Tokens = nil
	for _, tok := range dest {
		if tok.Quantity > 0 {
			probe.Tokens = append(probe.Tokens, tok)
		}
	}
	capacity := ComputeFleetSpec(&t.game.Rules, p, &probe).FuelCapacity
	fuel := min(capacity, leader.Fuel*capacity/max(1, leader.Spec.FuelCapacity))
	_, escort, err := NewOrderer().SplitFleet(&t.game.Rules, p, t.game.getFleets(p.Num), SplitFleetRequest{Source: leader, SourceTokens: source, DestTokens: dest, DestBaseName: "Landing patrol", TransferAmount: CargoTransferRequest{Fuel: -fuel}})
	if err != nil {
		leader.SetTag(fuelStatusTag, fmt.Sprintf("Landing escort separation pending: %v", err))
		return
	}
	t.game.Fleets = append(t.game.Fleets, escort)
	_ = t.game.Universe.addFleet(escort)
	p.Automation.FleetRoles[escort.Num] = "patrol"
}
