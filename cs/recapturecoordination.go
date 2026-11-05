package cs

import (
	"slices"
	"strconv"
)

func recaptureEquipmentMissing(p *Player, planet *Planet) bool {
	if planet == nil || !p.Automation.PriorityPlanets[planet.Num] || !planet.Spec.HasStarbase || !planet.Owned() || planet.PlayerNum == p.Num || p.IsFriend(planet.PlayerNum) {
		return false
	}
	d := p.GetShipDesignIntel(planet.PlayerNum, planet.Spec.StarbaseDesignNum)
	return d == nil || len(d.Slots) == 0
}

// Reserve a bounded part of the scout force for landing sites and marked colonies before routine
// equipment contacts consume every scout. All targets come from our reports;
// an old unowned report is precisely why a marked colony needs another look.
func (t *turnGenerator) planRecaptureReconnaissance(p *Player, fleets []*Fleet) map[int]bool {
	assigned := map[int]bool{}
	scouts := []*Fleet{}
	for _, f := range fleets {
		if j := readScoutRecon(f); j != nil && enemyApproachingColony(p, p.GetFleetIntel(j.Enemy, j.Target)) {
			continue
		}
		if p.Automation.FleetRoles[f.Num] == "scout" && !f.Delete && !f.InAutomaticSafety() && !f.InDesignMaintenance() && !fuelNetworkManaged(f) && f.GetTag(tankerAwaitTag) == "" && f.GetTag(preyWatchTag) == "" && !scoutHasRetirement(p, f) && scoutReconEligible(p, f) {
			scouts = append(scouts, f)
		}
	}
	if len(scouts) == 0 {
		return assigned
	}
	targets := []*Planet{}
	landingScans := map[int]bool{}
	for _, f := range fleets {
		if job := logisticsWork(f); job != nil && !f.Delete && !f.InDesignMaintenance() && p.Automation.FleetRoles[f.Num] == "freighter" && galleonLandingNeedsScan(f, job, p.GetPlanetIntel(job.DeliveryNum)) {
			landingScans[job.DeliveryNum] = true
		}
	}
	for _, i := range p.PlanetIntels {
		if i != nil && (p.Automation.PriorityPlanets[i.Num] || landingScans[i.Num]) && i.PlayerNum != p.Num && (!i.Owned() || !p.IsFriend(i.PlayerNum)) && (i.ReportAge > 2 || recaptureEquipmentMissing(p, i)) && !i.Unexplored() {
			targets = append(targets, i)
		}
	}
	rank := func(i *Planet) int {
		value := 0
		for _, f := range fleets {
			if f.Delete || f.InDesignMaintenance() || f.InAutomaticSafety() {
				continue
			}
			if j := logisticsWork(f); j != nil && j.DeliveryNum == i.Num && (j.Kind == "invade" || j.Kind == "invade-load") {
				value += 1000
			}
			if bomberGoal(f) == i.Num {
				value += 10
			}
		}
		return value
	}
	ranks := map[int]int{}
	for _, i := range targets {
		ranks[i.Num] = rank(i)
	}
	slices.SortFunc(targets, func(a, b *Planet) int {
		if ranks[a.Num] != ranks[b.Num] {
			return ranks[b.Num] - ranks[a.Num]
		}
		if a.ReportAge != b.ReportAge {
			return b.ReportAge - a.ReportAge
		}
		return a.Num - b.Num
	})
	quota := max(1, min(4, (len(scouts)+3)/4))
	for _, target := range targets {
		if len(assigned) >= quota {
			break
		}
		candidates := slices.Clone(scouts)
		slices.SortStableFunc(candidates, func(a, b *Fleet) int {
			da, db := a.Position.DistanceTo(target.Position), b.Position.DistanceTo(target.Position)
			if a.GetTag(priorityReconTag) == strconv.Itoa(target.Num) {
				da *= .8
			}
			if b.GetTag(priorityReconTag) == strconv.Itoa(target.Num) {
				db *= .8
			}
			if da < db {
				return -1
			}
			if da > db {
				return 1
			}
			return a.Num - b.Num
		})
		tries := 0
		for _, f := range candidates {
			if assigned[f.Num] {
				continue
			}
			tries++
			if tries > 6 {
				break
			}
			position := scoutColonyScanPoint(p, f, target)
			dest := target.ToTarget()
			if position != target.Position {
				dest = MapObjectTarget{}
			}
			route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, position, dest, automaticRoutePolicy{AllowThreat: true, MinWarp: automaticCruiseWarp(p, f), MaxTurns: 8})
			if route == nil || route.WaitForFuel {
				continue
			}
			delete(f.Tags, scoutReconTag)
			f.SetTag(priorityReconTag, strconv.Itoa(target.Num))
			f.SetTag(scoutTag, "Refreshing colony intelligence for landing or bombardment: "+target.Name)
			applyExpeditionRoute(p, f, route)
			assigned[f.Num] = true
			break
		}
	}
	return assigned
}

// Colony reconnaissance accepts the same deliberate contact risk as fleet
// reconnaissance, but only for an actual scout following its reported target.
func colonyReconActive(p *Player, f *Fleet) bool {
	if p.Automation.FleetRoles[f.Num] != "scout" || !scoutReconEligible(p, f) || f.Delete || f.InDesignMaintenance() {
		return false
	}
	num, _ := strconv.Atoi(f.GetTag(priorityReconTag))
	i := p.GetPlanetIntel(num)
	if i == nil || i.ReportAge <= 2 && !recaptureEquipmentMissing(p, i) || i.PlayerNum == p.Num || i.Owned() && p.IsFriend(i.PlayerNum) {
		return false
	}
	if f.Position.DistanceTo(i.Position) <= scoutPlanetScanRadius(f) {
		return true
	}
	return len(f.Waypoints) > 1 && f.Waypoints[len(f.Waypoints)-1].Position.DistanceTo(i.Position) <= scoutPlanetScanRadius(f)+2
}

// A scout transmits only what its real scanners observe on arrival, before
// orbital combat can destroy it. Use the normal visibility/discovery rules.
func (t *turnGenerator) scanColonyReconContacts() {
	for _, p := range t.game.Players {
		for _, f := range t.game.getFleets(p.Num) {
			if !colonyReconActive(p, f) {
				continue
			}
			num, _ := strconv.Atoi(f.GetTag(priorityReconTag))
			planet := t.game.getPlanet(num)
			if planet == nil {
				continue
			}
			if p.discoverer == nil {
				p.discoverer = newDiscoverer(t.log, p)
			}
			scan := newPlayerScanner(t.game.Universe, t.game.Players, &t.game.Rules, p)
			_, err := scan.scanPlanet(planet, scanner{Position: f.Position, Range: f.Spec.ScanRange, RangePen: f.Spec.ScanRangePen, CloakReductionFactor: f.Spec.ReduceCloaking})
			if err != nil {
				t.log.Warn("colony reconnaissance", "error", err)
			}
			if f.Position == planet.Position && planet.Starbase != nil {
				for _, token := range planet.Starbase.Tokens {
					if token.design != nil {
						p.discoverer.discoverDesign(token.design, true)
					}
				}
			}
			scan.discoverPlayers()
		}
	}
}
