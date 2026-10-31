package cs

import "math"

// Evaluate simultaneous motion from reports, not hidden enemy destinations.
func reportedEscapeSeparation(player *Player, fleet *Fleet, position Vector) (closest, ending, starting float64) {
	closest, ending, starting = math.Inf(1), math.Inf(1), math.Inf(1)
	ours := position.Subtract(fleet.Position)
	for _, enemy := range player.FleetIntels {
		if enemy == nil || enemy.Delete || enemy.ReportAge != 0 || enemy.PlayerNum == player.Num || player.IsFriend(enemy.PlayerNum) || !patrolReportedCombatant(player, enemy) {
			continue
		}
		ev := enemy.Heading.Scale(enemy.observedStrategicSpeed())
		dx, dy := float64(enemy.Position.X-fleet.Position.X), float64(enemy.Position.Y-fleet.Position.Y)
		starting = min(starting, math.Hypot(dx, dy))
		vx, vy := ev.X-float64(ours.X), ev.Y-float64(ours.Y)
		at := 0.0
		den := vx*vx + vy*vy
		if den > 0 {
			at = max(0, min(1, -(dx*vx+dy*vy)/den))
		}
		closest = min(closest, math.Hypot(dx+vx*at, dy+vy*at))
		ending = min(ending, math.Hypot(dx+vx, dy+vy))
	}
	return closest, ending, starting
}

func (t *turnGenerator) planOpenSpaceEscape(player *Player, fleet *Fleet) *expeditionRoute {
	best := -math.MaxFloat64
	var chosen *expeditionRoute
	for speed := automaticMaxWarp(player, fleet); speed >= automaticCruiseWarp(player, fleet); speed-- {
		for direction := 0; direction < 16; direction++ {
			angle := float64(direction) * math.Pi / 8
			for _, fraction := range []float64{1, 0.75, 0.5} {
				distance := fleet.StrategicSpeed(speed) * fraction
				position := fleet.Position.Add(Vector{int(math.Round(distance * math.Cos(angle))), int(math.Round(distance * math.Sin(angle)))})
				if t.game.Area.X > 0 && (position.X < 0 || position.Y < 0 || position.X > t.game.Area.X || position.Y > t.game.Area.Y) {
					continue
				}
				if position == fleet.Position || fleet.Position.DistanceTo(position) > fleet.StrategicSpeed(speed) || !automaticLegSafe(&t.game.Rules, player, fleet.Position, position, speed, true, fleet.StrategicSpeed(speed)) {
					continue
				}
				closest, ending, starting := reportedEscapeSeparation(player, fleet, position)
				if closest <= 3 && (starting > 3 || closest < starting-0.01) || ending <= max(3, starting+1) {
					continue
				}
				reserve := automaticReturnFuel(&t.game.Rules, player, fleet, position, expeditionStations(player, fleet), true)
				_, left, burned, enough := automaticLegFuel(player, fleet, speed, fleet.Position.DistanceTo(position), fleet.Fuel, reserve)
				if !enough {
					continue
				}
				score := closest*10 + ending - float64(burned)/float64(max(1, fleet.Spec.FuelCapacity))
				if score > best {
					best = score
					target := MapObjectTarget{}
					for _, dock := range expeditionStations(player, fleet) {
						if dock.Position == position {
							target = dock.MapObject.ToTarget()
							break
						}
					}
					chosen = &expeditionRoute{Waypoints: []Waypoint{{Position: fleet.Position}, {Position: position, MapObjectTarget: target, WarpSpeed: speed}}, Turns: 1, Fuel: burned, ArrivalFuel: []int{left}}
				}
			}
		}
	}
	return chosen
}
