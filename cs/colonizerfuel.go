package cs

// Routine acceleration must not lock a fueled colony ship into a stationary
// tanker node. Fixed bases remain usable; mobile tanker jobs may still shorten
// the route by moving to an agreed rendezvous with the customer.
func (t *turnGenerator) releaseColonizerFuelDetours() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			if f.Delete || colonizationMode(p.Automation.FleetRoles[f.Num]) == "" {
				continue
			}
			plan := readFuelNetworkPlan(f)
			if plan == nil || plan.Leg < 0 || plan.Leg >= len(plan.Route.Legs) {
				continue
			}
			donations := false
			for _, leg := range plan.Route.Legs[plan.Leg:] {
				donations = donations || len(leg.Donations) > 0
			}
			if !donations {
				continue
			}
			target := MapObjectTarget{}
			if orders, ok := plan.Orders[f.Num]; ok && len(orders.Waypoints) > 1 {
				target = orders.Waypoints[len(orders.Waypoints)-1].MapObjectTarget
			}
			direct := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, plan.Mission, target, automaticRoutePolicy{OneWay: true, MaxTurns: 50})
			if direct == nil || direct.WaitForFuel {
				continue
			} // Preserve genuinely necessary multi-stop rescue.
			t.closeFuelNetwork(p, f, plan, true)
			f.SetTag(fuelStatusTag, "Fuel already covers the colony mission via fixed docks; tankers remain available for remote rescue and combat")
		}
	}
}
