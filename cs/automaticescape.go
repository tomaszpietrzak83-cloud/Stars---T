package cs

import "math"

// Evaluate the end of movement from public heading/speed. Do not invent an
// enemy pursuit toward every possible escape point or reject mid-turn passing.
func reportedEscapeSeparation(player *Player, fleet *Fleet, position Vector) float64 {
	ending := math.Inf(1)
	for _, enemy := range player.FleetIntels {
		if enemy == nil || enemy.Delete || enemy.ReportAge != 0 || enemy.PlayerNum == player.Num || player.IsFriend(enemy.PlayerNum) || !patrolReportedCombatant(player, enemy) {
			continue
		}
		ending = min(ending, position.DistanceTo(patrolInterceptPosition(enemy)))
	}
	return ending
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
				ending := reportedEscapeSeparation(player, fleet, position)
				if ending <= 3 {
					continue
				}
				reserve := automaticReturnFuel(&t.game.Rules, player, fleet, position, expeditionStations(player, fleet), true)
				_, left, burned, enough := automaticLegFuel(player, fleet, speed, fleet.Position.DistanceTo(position), fleet.Fuel, reserve)
				if !enough {
					continue
				}
				score := ending - float64(burned)/float64(max(1, fleet.Spec.FuelCapacity))
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
