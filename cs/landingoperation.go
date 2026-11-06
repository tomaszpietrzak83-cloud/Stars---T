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
	return landingManaged(f) && p.Automation.FleetRoles[f.Num] == "scout"
}

// Saved solo invasions must join preparation after upgrading. Keep their cargo;
// a new operation can recruit it without unloading or inventing passengers.
func (t *turnGenerator) migrateLegacyLandings() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			if landingManaged(f) || p.Automation.FleetRoles[f.Num] != "freighter" {
				continue
			}
			if j := logisticsWork(f); j != nil && (j.Kind == "invade" || j.Kind == "invade-load") {
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
		if op := readLandingOperation(f); op != nil && !f.Delete && p.Automation.FleetRoles[f.Num] == "freighter" && op.Target == num && op.Phase == "base" {
			return true
		}
	}
	return false
}

func landingRequirement(rules *Rules, p *Player, target *Planet) int {
	// Population is used to provision the army, never as a reconnaissance veto.
	// Do not compound decades of stale intelligence into an impossible army.
	survival := max(.1, 1-target.Spec.DefenseCoverage*rules.InvasionDefenseCoverageFactor)
	losses := float64(target.Cargo.Colonists) * reportedInvasionDefense(rules) / max(.1, p.Race.Spec.InvasionAttackBonus) / survival
	return int(math.Ceil(losses*1.25)) + max(50, populationTarget(target, p.Automation.destinationPopulationPercent()))
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
		if p.Automation.FleetRoles[f.Num] != "freighter" || !landingAvailable(p, f) || !automaticPassengersAllowed(p, f) || f.carriesInfrastructure() || f.Cargo.ToMineral().Total() > 0 {
			continue
		}
		job := logisticsWork(f)
		if f.Cargo.Colonists > 0 {
			if job != nil && (job.Kind == "invade" || job.Kind == "landing-reserve") {
				candidates = append(candidates, landingPickup{fleet: f, score: f.Position.DistanceTo(rally) - 2000000})
			}
			continue
		}
		load := f.availableColonistSpace()
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
		if used[c.fleet.Num] || attempts[c.fleet.Num] >= 4 {
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

func (t *turnGenerator) startLandingOperation(p *Player) *Fleet {
	if p.Automation.DisablePassengerTransport || p.Race.Spec.LivesOnStarbases {
		return nil
	}
	var anchor *Fleet
	var scout *Fleet
	guards := []*Fleet{}
	for _, f := range t.game.getFleets(p.Num) {
		if !landingAvailable(p, f) {
			continue
		}
		role := p.Automation.FleetRoles[f.Num]
		legacyLoaded := logisticsWork(f) != nil && logisticsWork(f).Kind == "landing-reserve" && f.Cargo.Colonists > 0
		if role == "freighter" && automaticPassengersAllowed(p, f) && (f.CargoMass() == 0 || legacyLoaded) && f.Spec.CargoCapacity > 0 && (anchor == nil || f.Num < anchor.Num) {
			anchor = f
		}
		if role == "scout" && scoutReconEligible(p, f) && f.GetTag(preyWatchTag) == "" && (scout == nil || f.Num < scout.Num) {
			scout = f
		}
		if (role == "patrol" || role == "escort") && patrolAvailable(p, f) && readPatrolJob(f).Leader == 0 {
			guards = append(guards, f)
		}
	}
	if anchor == nil {
		return nil
	}
	targets := []*Planet{}
	for _, target := range p.PlanetIntels {
		if invasionCandidate(p, target) && !t.landingTargetClaimed(p, target.Num) {
			targets = append(targets, target)
		}
	}
	slices.SortFunc(targets, func(a, b *Planet) int {
		if p.Automation.PriorityPlanets[a.Num] != p.Automation.PriorityPlanets[b.Num] {
			if p.Automation.PriorityPlanets[a.Num] {
				return -1
			}
			return 1
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
		if scout != nil {
			scoutRoute := planAutomaticRoute(&t.game.Rules, p, []*Fleet{scout}, rally, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: 12})
			if scoutRoute == nil || scoutRoute.WaitForFuel {
				scout = nil
			}
		}
		cover := []*Fleet{}
		ships := 0
		slices.SortFunc(guards, func(a, b *Fleet) int {
			da, db := a.Position.DistanceTo(rally), b.Position.DistanceTo(rally)
			if da < db {
				return -1
			}
			if da > db {
				return 1
			}
			return a.Num - b.Num
		})
		for _, f := range guards {
			route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, rally, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: 12})
			if route == nil || route.WaitForFuel {
				continue
			}
			cover = append(cover, f)
			ships += f.Spec.TotalShips
			if ships >= 2 {
				break
			}
		}
		reserved := t.landingReservations(p)
		selected := []landingPickup{}
		used := map[int]bool{}
		need, total := landingRequirement(&t.game.Rules, p, target), 0
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
			cover = append(cover, scout)
		}
		for _, f := range cover {
			f.SetTag(landingMemberTag, strconv.Itoa(leader.Num))
			delete(f.Tags, patrolTag)
			delete(f.Tags, scoutReconTag)
			delete(f.Tags, priorityReconTag)
		}
		saveLandingOperation(leader, op)
		return leader
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
		holdAutomaticFleet(p, f, "Landing operation awaiting a funded route")
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
		if f.Position != leader.Position || f.InDesignMaintenance() || fuelNetworkReserved(f) {
			return false
		}
	}
	if _, err := NewOrderer().Merge(&t.game.Rules, p, group); err != nil {
		return false
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
		if e == nil || e.Delete || e.Starbase || e.PlayerNum == p.Num || p.IsFriend(e.PlayerNum) || e.ReportAge > 1 || !patrolReportedCombatant(p, e) {
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
	if target == nil || target.Spec.HasStarbase || op.Base || op.ScanYear == 0 || t.game.Year-op.ScanYear > 1 {
		return false
	}
	present, incoming := landingOrbitalReports(p, target.Position)
	// Incoming warships require patrols ALREADY on station, not a promise of
	// escorts arriving with vulnerable transports during the same year.
	cover := []*Fleet{}
	for _, f := range t.game.getFleets(p.Num) {
		if !f.Delete && f.Position == target.Position && f.Spec.HasWeapons && (p.Automation.FleetRoles[f.Num] == "patrol" || p.Automation.FleetRoles[f.Num] == "escort") && (len(f.Waypoints) <= 1 || f.Waypoints[1].Position == target.Position) {
			cover = append(cover, f)
		}
	}
	if len(incoming) > 0 && len(cover) == 0 {
		return false
	}
	if len(present)+len(incoming) == 0 {
		return true
	}
	group := append([]*Fleet{leader}, cover...)
	return t.defensiveEvaluator(p).assessFormations(p, group, append(present, incoming...), target.Position, 1, false).Ready
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
	reserved := map[int]int{}
	for _, f := range t.game.getFleets(p.Num) {
		if j := logisticsWork(f); !f.Delete && j != nil && j.Kind == "landing-load" {
			reserved[j.TargetNum] += j.Passengers
		}
	}
	return reserved
}
func (t *turnGenerator) planLandingOperations() {
	if t.ordersOnly {
		return
	}
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			n, _ := strconv.Atoi(f.GetTag(landingMemberTag))
			owner := t.game.getFleet(p.Num, n)
			if n != 0 && (owner == nil || owner.Delete || readLandingOperation(owner) == nil) {
				delete(f.Tags, landingMemberTag)
				delete(f.Tags, "automatic-landing-screen")
			}
		}
		// Cargo preparation does not wait for a free scout or patrol. Start
		// several independent armies while other operations are travelling.
		for range 3 {
			if t.startLandingOperation(p) == nil {
				break
			}
		}
		leaders := slices.Clone(t.game.getFleets(p.Num))
		slices.SortFunc(leaders, func(a, b *Fleet) int { return a.Num - b.Num })
		for _, leader := range leaders {
			op := readLandingOperation(leader)
			if op == nil || leader.Delete {
				continue
			}
			target := p.GetPlanetIntel(op.Target)
			if p.Automation.FleetRoles[leader.Num] != "freighter" || p.Automation.DisablePassengerTransport || target == nil || target.PlayerNum != op.Owner || p.IsFriend(target.PlayerNum) || op.Phase == "landed" || op.Phase == "cancelled" || t.game.Year-op.Started > 30 {
				t.finishLandingOperation(p, leader, op)
				continue
			}
			if leader.InAutomaticSafety() || leader.InDesignMaintenance() {
				continue
			}
			t.reinforceLandingOperation(p, leader, op, target)
			t.recruitLandingProtection(p, leader, op)
			t.advanceLandingOperation(p, leader, op, target)
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
		if used[c.fleet.Num] || c.source != nil && c.source.Cargo.Colonists-sourcePopulationReserve(p, c.source)-reserved[c.source.Num] < c.load {
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
	ships := 0
	for _, f := range t.landingMembers(p, leader) {
		if p.Automation.FleetRoles[f.Num] == "patrol" || p.Automation.FleetRoles[f.Num] == "escort" {
			ships += f.Spec.TotalShips
		}
	}
	if ships >= 2 {
		return
	}
	candidates := slices.Clone(t.game.getFleets(p.Num))
	slices.SortFunc(candidates, func(a, b *Fleet) int {
		da, db := a.Position.DistanceTo(op.Rally), b.Position.DistanceTo(op.Rally)
		if da < db {
			return -1
		}
		if da > db {
			return 1
		}
		return a.Num - b.Num
	})
	for _, f := range candidates {
		if !landingAvailable(p, f) || !patrolAvailable(p, f) || readPatrolJob(f).Leader != 0 {
			continue
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, op.Rally, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: 12})
		if route == nil || route.WaitForFuel {
			continue
		}
		f.SetTag(landingMemberTag, strconv.Itoa(leader.Num))
		delete(f.Tags, patrolTag)
		applyExpeditionRoute(p, f, route)
		ships += f.Spec.TotalShips
		if ships >= 2 {
			break
		}
	}
}

func (t *turnGenerator) advanceLandingOperation(p *Player, leader *Fleet, op *landingOperation, target *Planet) {
	// A fresh ordinary scan or an allied orbital patrol can confirm that the
	// base is gone. Clear the remembered warning, then require the scout again.
	if op.Phase == "base" && !target.Spec.HasStarbase && target.ReportAge <= 1 {
		op.Base = false
		op.Phase = "scan"
		op.ScanYear = 0
	}
	members := t.landingMembers(p, leader)
	scout := t.game.getFleet(p.Num, op.Scout)
	if scout == nil || scout.Delete || p.Automation.FleetRoles[scout.Num] != "scout" {
		op.Scout = 0
		for _, f := range t.game.getFleets(p.Num) {
			if landingAvailable(p, f) && p.Automation.FleetRoles[f.Num] == "scout" && scoutReconEligible(p, f) {
				scout = f
				op.Scout = f.Num
				f.SetTag(landingMemberTag, strconv.Itoa(leader.Num))
				break
			}
		}
	}
	ready := true
	transports := []*Fleet{leader}
	guards := []*Fleet{}
	for _, f := range members {
		role := p.Automation.FleetRoles[f.Num]
		if f.InAutomaticSafety() || f.InDesignMaintenance() {
			ready = false
			continue
		}
		if role == "freighter" {
			job := logisticsWork(f)
			if job != nil && job.Kind == "landing-load" {
				source := t.game.getPlanet(job.TargetNum)
				if source == nil || !source.OwnedBy(p.Num) {
					t.finishLandingOperation(p, leader, op)
					op.Phase = "cancelled"
					return
				}
				if f.Position != source.Position {
					t.routeLanding(p, f, source.Position, source.ToTarget(), false, 0)
					ready = false
					continue
				}
				load := f.availableColonistSpace()
				if source.Cargo.Colonists-sourcePopulationReserve(p, source) < load || t.armedEnemyAt(p, source.Position) {
					holdAutomaticFleet(p, f, "Landing army awaits a full safe load at source")
					ready = false
					continue
				}
				probe := *f
				probe.Cargo.Colonists += load
				route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{&probe}, op.Rally, MapObjectTarget{}, automaticRoutePolicy{})
				if route == nil || route.WaitForFuel {
					holdAutomaticFleet(p, f, "Landing pickup awaits fuel for its loaded voyage")
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
				ready = false
				continue
			}
			holdAutomaticFleet(p, f, "Loaded transport at landing rendezvous")
			if f != leader {
				transports = append(transports, f)
			}
		} else if role == "patrol" || role == "escort" {
			if f.GetTag("automatic-landing-screen") != "" {
				t.routeLanding(p, f, target.Position, target.ToTarget(), true, 1)
				if f.Position != target.Position {
					ready = false
				}
				continue
			}
			guards = append(guards, f)
			if f.Position != op.Rally {
				t.routeLanding(p, f, op.Rally, MapObjectTarget{}, false, 0)
				ready = false
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
		} else if op.Phase == "base" {
			// One transmitted warning is enough; do not feed replacement scouts
			// into a known base while patrols prepare its destruction.
			t.routeLanding(p, scout, op.Rally, MapObjectTarget{}, false, 0)
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
		if (p.Automation.FleetRoles[f.Num] == "patrol" || p.Automation.FleetRoles[f.Num] == "escort") && !f.InAutomaticSafety() && (f.Position == op.Rally || f.GetTag("automatic-landing-screen") != "" && f.Position == target.Position) {
			protection += f.Spec.TotalShips
		}
	}
	if op.Phase == "gather" && protection < 2 {
		ready = false
	}
	if op.Phase != "gather" && (op.Base || target.Spec.HasStarbase) {
		op.Phase = "base"
		local := []*Fleet{}
		for _, guard := range guards {
			if guard.Position == op.Rally {
				local = append(local, guard)
			}
		}
		t.prepareLandingBaseAssault(p, leader, op, target, local)
		return
	}
	if !ready || leader.Cargo.Colonists < op.Need {
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
	guardShips := 0
	for _, f := range guards {
		guardShips += f.Spec.TotalShips
	}
	for _, f := range members {
		if f.GetTag("automatic-landing-screen") != "" && f.Position == target.Position {
			guardShips += f.Spec.TotalShips
		}
	}
	if guardShips < 2 && len(op.EscortDesigns) == 0 {
		for _, f := range t.game.getFleets(p.Num) {
			if landingAvailable(p, f) && patrolAvailable(p, f) && readPatrolJob(f).Leader == 0 {
				f.SetTag(landingMemberTag, strconv.Itoa(leader.Num))
				delete(f.Tags, patrolTag)
				t.routeLanding(p, f, op.Rally, MapObjectTarget{}, false, 0)
				guardShips += f.Spec.TotalShips
				if guardShips >= 2 {
					break
				}
			}
		}
		return
	}
	group := append([]*Fleet{leader}, guards...)
	// Assess before mutation; only physically present forces are counted.
	probe := *leader
	for _, f := range guards {
		probe.Tokens = append(slices.Clone(probe.Tokens), f.Tokens...)
	}
	probe.Spec = ComputeFleetSpec(&t.game.Rules, p, &probe)
	if !t.landingOrbitClear(p, &probe, op) {
		present, incoming := landingOrbitalReports(p, target.Position)
		if len(incoming) > 0 && len(guards) > 0 && op.ScanYear > 0 && t.game.Year-op.ScanYear <= 1 && t.defensiveEvaluator(p).assessFormations(p, guards, append(present, incoming...), target.Position, 1, false).Ready {
			for _, guard := range guards {
				guard.SetTag("automatic-landing-screen", "true")
				t.routeLanding(p, guard, target.Position, target.ToTarget(), true, 1)
			}
		}
		holdAutomaticFleet(p, leader, "Landing held: scout report, hostile orbit or incoming warships")
		return
	}
	route := planAutomaticRoute(&t.game.Rules, p, group, target.Position, target.ToTarget(), automaticRoutePolicy{AllowThreat: true, Fast: true, MaxTurns: 1})
	if route == nil || route.WaitForFuel || route.Turns > 1 {
		return
	}
	for _, f := range guards {
		for _, tok := range f.Tokens {
			op.EscortDesigns[tok.DesignNum] = true
		}
	}
	if !t.mergeLandingGroup(p, leader, group) {
		return
	}
	op.Phase = "assault"
	saveLogisticsWork(leader, &logisticsJob{Kind: "landing-assault", TargetNum: target.Num, DeliveryNum: target.Num, Position: target.Position, Status: "Coordinated landing: transports and patrols merged"})
	t.routeLanding(p, leader, target.Position, target.ToTarget(), true, 1)
}

func (t *turnGenerator) prepareLandingBaseAssault(p *Player, leader *Fleet, op *landingOperation, target *Planet, guards []*Fleet) {
	// Make the request visible to the existing orbital combat planner, without
	// changing the user's priority flags. Reinforcements gather at our rally.
	saveLandingOperation(leader, op)
	base := reportedRecaptureBase(p, target, true)
	if base == nil {
		return
	}
	if len(guards) > 0 && t.assessRecapture(p, guards, base).Ready {
		attack := guards[0]
		if !t.mergeLandingGroup(p, attack, guards) {
			return
		}
		delete(attack.Tags, landingMemberTag)
		p.Automation.FleetRoles[attack.Num] = "patrol"
		attack.SetTag(recaptureAssaultTag, strconv.Itoa(target.Num))
		t.routeLanding(p, attack, target.Position, target.ToTarget(), true, 1)
		return
	}
	for _, f := range t.game.getFleets(p.Num) {
		if landingAvailable(p, f) && patrolAvailable(p, f) && readPatrolJob(f).Leader == 0 {
			route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, op.Rally, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: 8})
			if route == nil || route.WaitForFuel {
				continue
			}
			f.SetTag(landingMemberTag, strconv.Itoa(leader.Num))
			delete(f.Tags, patrolTag)
			applyExpeditionRoute(p, f, route)
		}
	}
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
			if op == nil || op.Phase != "assault" || f.Delete || f.Cargo.Colonists <= 0 {
				continue
			}
			target := t.game.getPlanet(op.Target)
			if target == nil || f.Position != target.Position || target.Spec.HasStarbase || t.armedEnemyAt(p, f.Position) {
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
	for _, f := range t.landingMembers(p, leader) {
		delete(f.Tags, landingMemberTag)
		delete(f.Tags, landingOperationTag)
		delete(f.Tags, "automatic-landing-screen")
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
