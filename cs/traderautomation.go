package cs

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
)

const traderConvoyTag = "automatic-trader"

type traderConvoyJob struct {
	Offering bool    `json:"offering,omitempty"`
	Trader   int     `json:"trader"`
	Source   int     `json:"source,omitempty"`
	Rally    int     `json:"rally"`
	Load     Mineral `json:"load,omitzero"`
	Status   string  `json:"status"`
}

func readTraderConvoy(f *Fleet) *traderConvoyJob {
	var job traderConvoyJob
	if json.Unmarshal([]byte(f.GetTag(traderConvoyTag)), &job) != nil {
		return nil
	}
	return &job
}
func saveTraderConvoy(f *Fleet, job traderConvoyJob) {
	raw, _ := json.Marshal(job)
	f.SetTag(traderConvoyTag, string(raw))
}
func traderCarrierAvailable(p *Player, f *Fleet) bool {
	if f.Delete || f.Starbase || p.Automation.FleetRoles[f.Num] != "freighter" || f.InDesignMaintenance() || f.InAutomaticSafety() || p.FleetInActiveAssembly(f) || fuelNetworkManaged(f) || f.GetTag("maintenance-pending") != "" || f.GetTag(tankerAwaitTag) != "" || readTankerJob(f) != nil || f.GetTag(anticipatedSalvageTag) != "" || f.CargoMass() != 0 || f.Spec.CargoCapacity <= 0 {
		return false
	}
	job := logisticsWork(f)
	return f.Idle() || job != nil && strings.HasPrefix(job.Status, "Waiting")
}

// Only public trader reports are used. Existing free automatic freighters are
// reserved as a finite one-shot offering, including their actual mineral loads.
// The native meeting rule absorbs the whole fleet, not merely its cargo.
func (t *turnGenerator) planTraderAutomation() {
	for _, p := range t.automaticPlayers() {
		traders := map[int]*MysteryTrader{}
		for _, mt := range p.MysteryTraderIntels {
			if mt != nil && !mt.Delete && mt.ReportAge == 0 && !mt.Spec.Rewarded && !mt.PlayersRewarded[p.Num] && p.Automation.PriorityTraders[mt.Num] {
				traders[mt.Num] = mt
			}
		}
		for _, f := range t.game.getFleets(p.Num) {
			if job := readTraderConvoy(f); job != nil && (traders[job.Trader] == nil || p.Automation.FleetRoles[f.Num] != "freighter" || f.InDesignMaintenance()) {
				delete(f.Tags, traderConvoyTag)
				if !f.InDesignMaintenance() && !f.InAutomaticSafety() {
					holdAutomaticFleet(p, f, "Trade reservation released; resuming ordinary transport")
				}
			}
		}
		ids := make([]int, 0, len(traders))
		for id := range traders {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			mt := traders[id]
			members := []*Fleet{}
			for _, f := range t.game.getFleets(p.Num) {
				if j := readTraderConvoy(f); !f.Delete && j != nil && j.Trader == id {
					members = append(members, f)
				}
			}
			if len(members) == 0 {
				members = t.reserveTraderConvoy(p, mt)
			}
			if len(members) == 0 {
				continue
			}
			rally := t.game.getPlanet(readTraderConvoy(members[0]).Rally)
			if rally == nil || !rally.OwnedBy(p.Num) || !rally.Spec.HasStarbase {
				for _, f := range members {
					delete(f.Tags, traderConvoyTag)
				}
				continue
			}
			for _, f := range members {
				if f.Delete || f.InAutomaticSafety() {
					continue
				}
				job := readTraderConvoy(f)
				if job.Offering {
					t.interceptTrader(p, f, mt)
					continue
				}
				if job.Source != 0 {
					source := t.game.getPlanet(job.Source)
					if source == nil || !source.OwnedBy(p.Num) {
						delete(f.Tags, traderConvoyTag)
						continue
					}
					if f.Position != source.Position {
						t.routeTraderConvoy(p, f, source.Position, source.ToTarget(), "Collecting reserved minerals for trader")
						continue
					}
					if !t.ordersOnly {
						spare := source.Cargo.ToMineral().ToCost().Subtract(t.productionMinerals(p, source, false).ToCost()).MinZero().ToMineral()
						for _, kind := range []ResourceType{Ironium, Boranium, Germanium} {
							amount := max(0, min(job.Load.GetAmount(kind), spare.GetAmount(kind), f.Spec.CargoCapacity-f.CargoMass()))
							source.Cargo = source.Cargo.SetAmount(kind, source.Cargo.GetAmount(kind)-amount)
							f.Cargo = f.Cargo.SetAmount(kind, f.Cargo.GetAmount(kind)+amount)
							job.Load.Set(kind, job.Load.GetAmount(kind)-amount)
						}
						if job.Load.Total() == 0 {
							job.Source = 0
						}
						job.Status = "Loading reserved trader minerals; committed production remains protected"
						saveTraderConvoy(f, *job)
					}
					if job.Source != 0 {
						continue
					}
				}
				if f.Position != rally.Position {
					t.routeTraderConvoy(p, f, rally.Position, rally.ToTarget(), "Assembling mineral offering at "+rally.Name)
				}
			}
			if t.ordersOnly {
				continue
			}
			ready := []*Fleet{}
			cargo := 0
			waiting := false
			for _, f := range members {
				if f.Delete {
					continue
				}
				job := readTraderConvoy(f)
				if job == nil {
					continue
				}
				if job.Offering {
					waiting = true
					continue
				}
				if job.Source != 0 || f.Position != rally.Position {
					waiting = true
					continue
				}
				ready = append(ready, f)
				cargo += f.Cargo.ToMineral().Total()
			}
			if waiting || len(ready) == 0 || cargo < mt.RequestedBoon {
				continue
			}
			leader := ready[0]
			for _, f := range ready[1:] {
				if _, err := NewOrderer().Merge(&t.game.Rules, p, []*Fleet{leader, f}); err == nil {
					t.game.deleteFleet(f)
				}
			}
			if leader.Cargo.ToMineral().Total() < mt.RequestedBoon {
				continue
			}
			job := readTraderConvoy(leader)
			job.Offering = true
			saveTraderConvoy(leader, *job)
			t.interceptTrader(p, leader, mt)
		}
	}
}

func (t *turnGenerator) routeTraderConvoy(p *Player, f *Fleet, pos Vector, target MapObjectTarget, status string) bool {
	route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, pos, target, automaticRoutePolicy{MaxTurns: 20, MinWarp: automaticCruiseWarp(p, f)})
	job := readTraderConvoy(f)
	if job == nil {
		return false
	}
	if route == nil || route.WaitForFuel {
		job.Status = "Trade convoy needs a funded route or local fuel; retrying next turn"
		saveTraderConvoy(f, *job)
		t.routeEmergencyReturn(p, f)
		return false
	}
	applyExpeditionRoute(p, f, route)
	job.Status = status
	saveTraderConvoy(f, *job)
	return true
}

func (t *turnGenerator) reserveTraderConvoy(p *Player, mt *MysteryTrader) []*Fleet {
	owned := slices.Clone(t.game.getPlanets(p.Num))
	sort.SliceStable(owned, func(i, j int) bool { return owned[i].Num < owned[j].Num })
	var rally *Planet
	best := math.Inf(1)
	for _, base := range owned {
		if !base.Spec.HasStarbase {
			continue
		}
		score := segmentDistance(base.Position, mt.Position, mt.Destination) + base.Position.DistanceTo(mt.Position)/10
		if score < best {
			rally = base
			best = score
		}
	}
	if rally == nil {
		return nil
	}
	available := []*Fleet{}
	capacity := 0
	for _, f := range t.game.getFleets(p.Num) {
		if readTraderConvoy(f) == nil && traderCarrierAvailable(p, f) {
			available = append(available, f)
		}
	}
	sort.SliceStable(available, func(i, j int) bool {
		a, b := available[i], available[j]
		if a.Spec.CargoCapacity != b.Spec.CargoCapacity {
			return a.Spec.CargoCapacity > b.Spec.CargoCapacity
		}
		return a.Num < b.Num
	})
	for _, f := range available {
		capacity += f.Spec.CargoCapacity
	}
	if capacity < mt.RequestedBoon {
		return nil
	}
	spares := map[int]Mineral{}
	stock := 0
	for _, source := range owned {
		spare := source.Cargo.ToMineral().ToCost().Subtract(t.productionMinerals(p, source, false).ToCost()).MinZero().ToMineral()
		spares[source.Num] = spare
		stock += spare.Total()
	}
	// Existing convoys already reserve their pickup stock against other traders.
	for _, f := range t.game.getFleets(p.Num) {
		if j := readTraderConvoy(f); !f.Delete && j != nil && j.Source != 0 {
			spares[j.Source] = spares[j.Source].ToCost().Subtract(j.Load.ToCost()).MinZero().ToMineral()
			stock -= j.Load.Total()
		}
	}
	if stock < mt.RequestedBoon {
		return nil
	}
	jobs := map[*Fleet]traderConvoyJob{}
	remaining := mt.RequestedBoon
	for _, f := range available {
		if remaining <= 0 {
			break
		}
		sort.SliceStable(owned, func(i, j int) bool {
			return f.Position.DistanceTo(owned[i].Position)+owned[i].Position.DistanceTo(rally.Position) < f.Position.DistanceTo(owned[j].Position)+owned[j].Position.DistanceTo(rally.Position)
		})
		for _, source := range owned {
			spare := spares[source.Num]
			want := min(remaining, f.Spec.CargoCapacity, spare.Total())
			if want <= 0 {
				continue
			}
			probe := *f
			probe.Cargo = Cargo{}
			load := Mineral{}
			room := want
			for _, kind := range []ResourceType{Ironium, Boranium, Germanium} {
				n := min(room, spare.GetAmount(kind))
				load.Set(kind, n)
				probe.Cargo = probe.Cargo.SetAmount(kind, n)
				room -= n
			}
			if planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, source.Position, source.ToTarget(), automaticRoutePolicy{MaxTurns: 20}) == nil {
				continue
			}
			probe.Position = source.Position
			probe.Fuel = f.Spec.FuelCapacity
			if planAutomaticRoute(&t.game.Rules, p, []*Fleet{&probe}, rally.Position, rally.ToTarget(), automaticRoutePolicy{MaxTurns: 20}) == nil {
				continue
			}
			jobs[f] = traderConvoyJob{Trader: mt.Num, Source: source.Num, Rally: rally.Num, Load: load, Status: "Reserved mineral offering; the trader will absorb this fleet"}
			spares[source.Num] = spare.ToCost().Subtract(load.ToCost()).MinZero().ToMineral()
			remaining -= want
			break
		}
	}
	if remaining > 0 {
		return nil
	} // Do not strand a partial offering or borrow manual ships.
	members := []*Fleet{}
	for _, f := range available {
		if j, ok := jobs[f]; ok {
			t.releaseEscorts(p, f)
			delete(f.Tags, logisticsTag)
			saveTraderConvoy(f, j)
			members = append(members, f)
		}
	}
	return members
}

func (t *turnGenerator) interceptTrader(p *Player, f *Fleet, mt *MysteryTrader) {
	// Re-evaluate the public course every year, allowing a convoy to wait ahead
	// instead of chasing warp 9 from behind. Native movement updates the trader
	// target on the final one-year leg; course changes are retried next turn.
	point := *mt
	for years := 1; years <= 30; years++ {
		point.move()
		if point.Position == point.Destination {
			break
		} // A trader may leave the map at this endpoint before fleets move.
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, point.Position, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: years, Fast: true, MinWarp: automaticCruiseWarp(p, f)})
		if route == nil || route.WaitForFuel {
			if point.Position == point.Destination {
				break
			}
			continue
		}
		applyExpeditionRoute(p, f, route)
		job := readTraderConvoy(f)
		job.Status = fmt.Sprintf("Offering %d/%d kT; staging for trader in %d years (whole fleet is absorbed)", f.Cargo.ToMineral().Total(), mt.RequestedBoon, years)
		if years == 1 {
			if len(f.Waypoints) == 1 {
				f.Waypoints[0].MapObjectTarget = mt.ToTarget()
			} else {
				last := len(f.Waypoints) - 1
				if f.Waypoints[last].WarpSpeed == StargateWarpSpeed {
					f.Waypoints = append(f.Waypoints, NewMysteryTraderWaypoint(mt, 1))
					f.Waypoints[len(f.Waypoints)-1].Position = point.Position
				} else {
					f.Waypoints[last].MapObjectTarget = mt.ToTarget()
				}
			}
		}
		saveTraderConvoy(f, *job)
		return
	}
	job := readTraderConvoy(f)
	job.Status = "Loaded trade convoy cannot yet intercept the reported course; retaining minerals and retrying"
	saveTraderConvoy(f, *job)
}

// Trader staging has a complete route of its own. Do not collapse its
// waypoints or replace an ahead-of-course rendezvous with ordinary logistics.
func fundedTraderWaypoints(rules *Rules, p *Player, f *Fleet) bool {
	if readTraderConvoy(f) == nil || len(f.Waypoints) < 2 {
		return false
	}
	copy := *f
	for _, wp := range f.Waypoints[1:] {
		if wp.WarpSpeed == StargateWarpSpeed {
			if !copy.CanJump(p, copy.Position.DistanceTo(wp.Position), p.GetPlanetIntel(copy.OrbitingPlanetNum), p.GetPlanetIntel(wp.TargetNum)) {
				return false
			}
			copy.Fuel = copy.Spec.FuelCapacity
		} else {
			reserve := automaticReturnFuel(rules, p, &copy, wp.Position, expeditionStations(p, &copy), false)
			if !automaticLegSafe(rules, p, copy.Position, wp.Position, wp.WarpSpeed, false, copy.StrategicSpeed(wp.WarpSpeed)) {
				return false
			}
			_, left, _, ok := automaticLegFuel(p, &copy, wp.WarpSpeed, copy.Position.DistanceTo(wp.Position), copy.Fuel, reserve)
			if !ok {
				return false
			}
			copy.Fuel = left
		}
		copy.Position = wp.Position
		if wp.TargetType == MapObjectTypePlanet {
			copy.OrbitingPlanetNum = wp.TargetNum
			if base := p.GetPlanetIntel(wp.TargetNum); base != nil && base.Spec.HasStarbase {
				copy.Fuel = copy.Spec.FuelCapacity
			}
		}
	}
	return true
}
