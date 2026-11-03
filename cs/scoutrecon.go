package cs

import (
	"encoding/json"
	"fmt"
	"math"
)

const scoutReconTag = "scout-recon"

type scoutReconJob struct {
	Enemy, Target int
	Watching      bool   `json:"watching,omitempty"`
	Status        string `json:"status"`
}

func readScoutRecon(f *Fleet) *scoutReconJob {
	var j scoutReconJob
	if json.Unmarshal([]byte(f.GetTag(scoutReconTag)), &j) != nil {
		return nil
	}
	return &j
}

// Sacrificial contact reconnaissance belongs to actual scout hulls, never
// cargo ships which may carry a stale role from an older saved game.
func scoutReconEligible(p *Player, f *Fleet) bool {
	if f == nil || len(f.Tokens) == 0 || f.Spec.TotalShips != 1 || f.Cargo.Colonists > 0 {
		return false
	}
	for _, token := range f.Tokens {
		d := p.GetDesign(token.DesignNum)
		if d == nil {
			return false
		}
		hull := StaticTechStore.GetHull(d.Hull)
		if hull == nil || hull.Type != TechHullTypeScout {
			return false
		}
	}
	return true
}

func (t *turnGenerator) repairStaleScoutRoles() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			if f.Delete || p.Automation.FleetRoles[f.Num] != "scout" {
				continue
			}
			if f.Spec.HasWeapons && !f.Spec.Scanner {
				p.Automation.FleetRoles[f.Num] = "patrol"
				delete(f.Tags, scoutReconTag)
				delete(f.Tags, scoutMissionTag)
				delete(f.Tags, scoutTag)
				if work := readMaintenance(f); work != nil {
					work.OriginalRole = "patrol"
					writeMaintenance(f, work)
				}
				continue
			}
			freight := len(f.Tokens) > 0
			for _, token := range f.Tokens {
				d := p.GetDesign(token.DesignNum)
				if d == nil {
					freight = false
					break
				}
				h := StaticTechStore.GetHull(d.Hull)
				if h == nil || h.Type != TechHullTypeFreighter {
					freight = false
					break
				}
			}
			if !freight {
				continue
			}
			p.Automation.FleetRoles[f.Num] = "freighter"
			delete(f.Tags, scoutReconTag)
			delete(f.Tags, scoutMissionTag)
			delete(f.Tags, scoutTag)
			if w := readMaintenance(f); w != nil {
				w.OriginalRole = "freighter"
				writeMaintenance(f, w)
			}
			if !f.InDesignMaintenance() {
				holdAutomaticFleet(p, f, "Transport role restored; selecting a cargo mission")
			}
		}
	}
}

func scoutReconActive(p *Player, f *Fleet) bool {
	return p.Automation.FleetRoles[f.Num] == "scout" && scoutReconEligible(p, f) && readScoutRecon(f) != nil
}

// Only reported headings are used, never the enemy's private waypoints.
func enemyApproachingColony(p *Player, enemy *Fleet) bool {
	if enemy == nil || enemy.Delete || enemy.PlayerNum == p.Num || p.IsFriend(enemy.PlayerNum) || enemy.ReportAge < 0 || enemy.ReportAge > 2 || enemy.Heading.LengthSquared() == 0 {
		return false
	}
	heading := enemy.Heading.Normalized()
	speed := max(1, enemy.observedStrategicSpeed())
	for _, planet := range p.PlanetIntels {
		if planet == nil || !planet.OwnedBy(p.Num) {
			continue
		}
		delta := planet.Position.Subtract(enemy.Position).ToFloat64()
		along := delta.X*heading.X + delta.Y*heading.Y
		cross := math.Abs(delta.X*heading.Y - delta.Y*heading.X)
		if along >= 0 && along <= speed*10 && cross <= 20 {
			return true
		}
	}
	return false
}
func unknownFleetEquipment(p *Player, enemy *Fleet) bool {
	if len(enemy.Tokens) == 0 {
		return true
	}
	for _, token := range enemy.Tokens {
		design := p.GetShipDesignIntel(enemy.PlayerNum, token.DesignNum)
		if design == nil || len(design.Slots) == 0 {
			return true
		}
	}
	return false
}
func (t *turnGenerator) planScoutRecon(p *Player, f *Fleet) bool {
	if !t.ordersOnly && f.Spec.TotalShips > 1 && f.Cargo.Colonists == 0 {
		genuine := true
		for _, token := range f.Tokens {
			d := p.GetDesign(token.DesignNum)
			if d == nil {
				genuine = false
				break
			}
			h := StaticTechStore.GetHull(d.Hull)
			if h == nil || h.Type != TechHullTypeScout {
				genuine = false
				break
			}
		}
		if genuine {
			if children, err := NewOrderer().SplitAll(&t.game.Rules, p, t.game.getFleets(p.Num), f); err == nil {
				for _, child := range children {
					t.game.Fleets = append(t.game.Fleets, child)
					_ = t.game.Universe.addFleet(child)
					p.Automation.FleetRoles[child.Num] = "scout"
				}
			}
		}
	}
	if !scoutReconEligible(p, f) {
		if readScoutRecon(f) != nil {
			holdAutomaticFleet(p, f, "Cargo ships cannot perform combat reconnaissance")
		}
		delete(f.Tags, scoutReconTag)
		return false
	}
	var chosen *Fleet
	var best *expeditionRoute
	score := math.Inf(1)
	for _, enemy := range p.FleetIntels {
		if !enemyApproachingColony(p, enemy) || !reconCombatFormation(p, enemy) || !unknownFleetEquipment(p, enemy) && enemy.ReportAge == 0 {
			continue
		}
		claimed := false
		for _, other := range t.game.getFleets(p.Num) {
			job := readScoutRecon(other)
			if other != f && !other.Delete && job != nil && job.Enemy == enemy.PlayerNum && job.Target == enemy.Num {
				claimed = true
				break
			}
		}
		if claimed {
			continue
		}
		target := enemy.MapObject.ToTarget()
		position := enemy.Position
		// Equipment still requires contact unless this race can discover designs
		// on a remote scan. A scanner alone does not grant hidden components.
		if p.Race.Spec.DiscoverDesignOnScan || !unknownFleetEquipment(p, enemy) {
			position = scoutFleetReconPoint(p, f, enemy)
			target = MapObjectTarget{}
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, position, target, automaticRoutePolicy{AllowThreat: true, MaxTurns: 8, MinWarp: automaticCruiseWarp(p, f)})
		if route == nil || route.WaitForFuel {
			continue
		}
		value := float64(route.Turns)*1000 + f.Position.DistanceTo(enemy.Position)
		if value < score {
			chosen, best, score = enemy, route, value
		}
	}
	if chosen == nil {
		if readScoutRecon(f) != nil {
			holdAutomaticFleet(p, f, "Reconnaissance ended; resuming exploration or return")
		}
		delete(f.Tags, scoutReconTag)
		return false
	}
	job := scoutReconJob{Enemy: chosen.PlayerNum, Target: chosen.Num, Status: fmt.Sprintf("Priority reconnaissance of incoming enemy fleet %s; combat risk accepted", chosen.Name)}
	raw, _ := json.Marshal(job)
	f.SetTag(scoutReconTag, string(raw))
	applyExpeditionRoute(p, f, best)
	if !p.Race.Spec.DiscoverDesignOnScan && unknownFleetEquipment(p, chosen) && len(f.Waypoints) > 1 {
		f.Waypoints[len(f.Waypoints)-1].MapObjectTarget = chosen.ToTarget()
	}
	f.SetTag(scoutTag, job.Status)
	return true
}

// Contact reconnaissance is recorded before combat so a destroyed scout still
// transmits what it physically scanned. Hidden or out-of-range fleets are not read.
func (t *turnGenerator) scanScoutReconContacts() {
	for _, p := range t.game.Players {
		for _, f := range t.game.getFleets(p.Num) {
			job := readScoutRecon(f)
			if job == nil || f.Delete || p.Automation.FleetRoles[f.Num] != "scout" {
				continue
			}
			enemy := t.game.getFleet(job.Enemy, job.Target)
			if enemy == nil || enemy.Delete {
				continue
			}
			scanner := scanner{Position: f.Position, Range: f.Spec.ScanRange, RangePen: f.Spec.ScanRangePen, CloakReductionFactor: f.Spec.ReduceCloaking}
			scan := newPlayerScanner(t.game.Universe, t.game.Players, &t.game.Rules, p)
			if f.Position.DistanceTo(enemy.Position) > 1 && !scan.fleetInScannerRange(enemy, scanner) {
				continue
			}
			if p.discoverer == nil {
				p.discoverer = newDiscoverer(t.log, p)
			}
			for _, token := range enemy.Tokens {
				p.discoverer.discoverDesign(token.design, p.Race.Spec.DiscoverDesignOnScan || f.Position.DistanceTo(enemy.Position) <= 1)
			}
			p.discoverer.discoverFleet(enemy, false)
			if !job.Watching && !unknownFleetEquipment(p, p.GetFleetIntel(enemy.PlayerNum, enemy.Num)) {
				delete(f.Tags, scoutReconTag)
			}
		}
	}
}

// A lone ordinary scout is no reason to abandon exploration. A report with an
// unknown hull may still be a warship; known scout/freighter/miner hulls are not.
func reconCombatFormation(p *Player, enemy *Fleet) bool {
	if reportedScoutsOnly(p, enemy) {
		return false
	}
	if enemy.Spec.HasWeapons || enemy.Spec.Bomber {
		return true
	}
	for _, token := range enemy.Tokens {
		d := p.GetShipDesignIntel(enemy.PlayerNum, token.DesignNum)
		if d == nil {
			return enemy.Spec.ShipClass != ShipClassScout && enemy.Spec.ShipClass != ShipClassFreighter && enemy.Spec.ShipClass != ShipClassMiner
		}
		h := StaticTechStore.GetHull(d.Hull)
		if h != nil && h.Type != TechHullTypeScout && h.Class != ShipClassFreighter && h.Class != ShipClassMiner {
			return true
		}
	}
	return false
}
