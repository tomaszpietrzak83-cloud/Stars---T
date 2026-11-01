package cs

import (
	"cmp"
	"math"
	"slices"
)

// Emergency recovery may need to leave a threat circle sideways before turning
// home. These are navigation points, never refuelling services.
func emergencyDetourRoute(rules *Rules, p *Player, f *Fleet, dock *Planet) *expeditionRoute {
	defer scopedRouteThreatIndex(p)()
	_ = knownRouteThreat(p, f.Position, dock.Position) // Build the same fresh armed report set once.
	points := []Vector{f.Position, dock.Position}
	enemies := slices.Clone(p.routeThreatIndex.threats)
	slices.SortStableFunc(enemies, func(a, b indexedRouteThreat) int {
		return cmp.Compare(segmentDistance(a.position, f.Position, dock.Position), segmentDistance(b.position, f.Position, dock.Position))
	})
	margin := p.Automation.escortMarginAsFloat()
	count := 0
	for _, e := range enemies {
		if segmentDistance(e.position, f.Position, dock.Position) > 2*margin {
			continue
		}
		for i := 0; i < 16; i++ {
			angle := 2 * math.Pi * float64(i) / 16
			radius := margin/math.Cos(math.Pi/16) + 3
			v := Vector{X: e.position.X + int(math.Round(radius*math.Cos(angle))), Y: e.position.Y + int(math.Round(radius*math.Sin(angle)))}
			if !slices.Contains(points, v) {
				points = append(points, v)
			}
		}
		count++
		if count == 12 {
			break
		}
	}
	if len(points) == 2 {
		return nil
	}
	n := len(points)
	inf := int(^uint(0) >> 2)
	used, years, prev, warps := make([]int, n), make([]int, n), make([]int, n), make([]int, n)
	done := make([]bool, n)
	for i := range points {
		used[i] = inf
		years[i] = inf
		prev[i] = -1
	}
	used[0], years[0] = 0, 0
	for range n {
		at := -1
		for i := range points {
			if !done[i] && (at < 0 || used[i] < used[at] || used[i] == used[at] && years[i] < years[at]) {
				at = i
			}
		}
		if at < 0 || used[at] > f.Fuel || at == 1 {
			break
		}
		done[at] = true
		for next := 1; next < n; next++ {
			if next == at || done[next] {
				continue
			}
			distance := points[at].DistanceTo(points[next])
			for warp := 1; warp <= automaticMaxWarp(p, f); warp++ {
				_, _, burned, ok := automaticLegFuel(p, f, warp, distance, f.Fuel-used[at], 0)
				if !ok || used[at]+burned > f.Fuel {
					continue
				}
				time := years[at] + max(1, int(math.Ceil(distance/f.StrategicSpeed(warp))))
				if time > 10000 || used[at]+burned > used[next] || used[at]+burned == used[next] && time >= years[next] {
					continue
				}
				if !automaticLegSafe(rules, p, points[at], points[next], warp, false, f.StrategicSpeed(warp)) {
					continue
				}
				used[next], years[next], prev[next], warps[next] = used[at]+burned, time, at, warp
			}
		}
	}
	if prev[1] < 0 {
		return nil
	}
	var reverse []Waypoint
	for at := 1; at != 0; at = prev[at] {
		reverse = append(reverse, Waypoint{Position: points[at], WarpSpeed: warps[at]})
	}
	slices.Reverse(reverse)
	reverse[len(reverse)-1].MapObjectTarget = dock.ToTarget()
	route := &expeditionRoute{Waypoints: append([]Waypoint{{Position: f.Position}}, reverse...), Turns: years[1], Fuel: used[1]}
	return optimizeEmergencyDetour(rules, p, f, route)
}

// Once a geometrically safe corridor exists, spend the actual fuel across the
// whole corridor to minimize years, rather than taking every leg at free warp.
func optimizeEmergencyDetour(rules *Rules, p *Player, f *Fleet, route *expeditionRoute) *expeditionRoute {
	type label struct {
		fuel, years, burned int
		warps               []int
	}
	labels := []label{{fuel: f.Fuel}}
	from := f.Position
	for _, wp := range route.Waypoints[1:] {
		next := []label{}
		for _, old := range labels {
			for warp := 1; warp <= automaticMaxWarp(p, f); warp++ {
				_, left, burned, ok := automaticLegFuel(p, f, warp, from.DistanceTo(wp.Position), old.fuel, 0)
				if !ok || !automaticLegSafe(rules, p, from, wp.Position, warp, false, f.StrategicSpeed(warp)) {
					continue
				}
				v := label{fuel: left, years: old.years + max(1, int(math.Ceil(from.DistanceTo(wp.Position)/f.StrategicSpeed(warp)))), burned: old.burned + burned, warps: append(slices.Clone(old.warps), warp)}
				if v.years > 10000 {
					continue
				}
				dominated := false
				for _, other := range next {
					if other.years <= v.years && other.fuel >= v.fuel {
						dominated = true
						break
					}
				}
				if dominated {
					continue
				}
				next = slices.DeleteFunc(next, func(other label) bool { return v.years <= other.years && v.fuel >= other.fuel })
				next = append(next, v)
			}
		}
		if len(next) == 0 {
			return nil
		}
		labels = next
		from = wp.Position
	}
	best := labels[0]
	for _, v := range labels {
		if v.years < best.years || v.years == best.years && v.fuel > best.fuel {
			best = v
		}
	}
	for i, warp := range best.warps {
		route.Waypoints[i+1].WarpSpeed = warp
	}
	route.Turns, route.Fuel, route.ArrivalFuel = best.years, best.burned, []int{best.fuel}
	return route
}
