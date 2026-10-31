package cs

// A capacity-based fleet split can put passengers in a mineral-only extension
// when the designs have differently rounded bonuses. Exchange minerals for
// passengers between the two results without losing either kind of cargo.
func rebalanceEEEColonists(a, b *Fleet) {
	if a.Spec.MineralCargoBonus == 0 && b.Spec.MineralCargoBonus == 0 {
		return
	}
	for _, pair := range [][2]*Fleet{{a, b}, {b, a}} {
		from, to := pair[0], pair[1]
		excess := max(0, from.Cargo.Colonists-(from.Spec.CargoCapacity-from.Spec.MineralCargoBonus))
		move := min(excess, max(0, to.Spec.CargoCapacity-to.Spec.MineralCargoBonus-to.Cargo.Colonists))
		swap := max(0, move-to.availableCargoSpace())
		for _, mineral := range MineralTypes {
			amount := min(swap, to.Cargo.GetAmount(mineral))
			to.Cargo = to.Cargo.SetAmount(mineral, to.Cargo.GetAmount(mineral)-amount)
			from.Cargo = from.Cargo.AddAmount(mineral, amount)
			swap -= amount
		}
		from.Cargo.Colonists -= move
		to.Cargo.Colonists += move
	}
}
