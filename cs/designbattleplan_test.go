//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDesignStrategiesUseEquipmentAndRespectExplicitOrders(t *testing.T) {
	r := NewRulesWithSeed(4)
	p := testPlayer().WithNum(1)
	var beam, torpedo *TechHullComponent
	for _, component := range r.techs.HullComponents {
		if component.Category == TechCategoryBeamWeapon && !component.DamageShieldsOnly {
			beam = &component
		}
		if component.Category == TechCategoryTorpedo {
			torpedo = &component
		}
	}
	require.NotNil(t, beam)
	require.NotNil(t, torpedo)
	for _, tc := range []struct {
		name   string
		weapon *TechHullComponent
		tactic BattleTactic
	}{
		{"unarmed", nil, BattleTacticDisengage}, {"beam", beam, BattleTacticMaximizeDamage}, {"torpedo", torpedo, BattleTacticMaximizeRange},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &ShipDesign{}
			if tc.weapon != nil {
				d.Spec.HasWeapons = true
				d.Spec.WeaponSlots = []ShipDesignSlot{{HullComponent: tc.weapon.Name, Quantity: 1}}
			}
			got := designBattlePlan(&r, p, d, p.BattlePlans[0])
			require.Equal(t, tc.tactic, got.Tactic)
			custom := BattlePlan{Num: 17, Tactic: BattleTacticMinimizeDamageToSelf, PrimaryTarget: BattleTargetFreighters}
			p.BattlePlans = append(p.BattlePlans, custom)
			d.Spec.DefaultBattlePlan = custom.Num + 1
			require.Equal(t, custom, designBattlePlan(&r, p, d, p.BattlePlans[0]))
			fleet := BattlePlan{Num: 23, Tactic: BattleTacticDisengage}
			require.Equal(t, fleet, designBattlePlan(&r, p, d, fleet))
		})
	}
}

func TestSniperDoesNotCloseWhenAlreadyAtMaximumFiringRange(t *testing.T) {
	source := testBattleToken(1, Vector{2, 4}, BattleTacticMaximizeRange, 2)
	enemy := testBattleToken(2, Vector{6, 4}, BattleTacticMaximizeDamage, 1)
	source.weaponSlots = []*battleWeaponSlot{{weaponType: battleWeaponTypeTorpedo, power: 100, slotQuantity: 1, weaponRange: 4, accuracy: 1}}
	enemy.weaponSlots = []*battleWeaponSlot{{weaponType: battleWeaponTypeBeam, power: 100, slotQuantity: 1, weaponRange: 1, beamBonus: 1}}
	source.movesLeft = 1
	enemy.movesLeft = 0
	r := NewRulesWithSeed(4)
	b := battle{rules: &r, tokens: []*battleToken{source, enemy}}
	moves := b.getBestMoves(source)
	require.NotEmpty(t, moves)
	for _, pos := range moves {
		require.Equal(t, 4, pos.chebyshevDistance(enemy.Position), "a Sniper should hold firing range instead of following the center tie-break")
	}
	source.Position = Vector{0, 4}
	radius, closest := b.movementSearch(source, BattleTargetAny)
	require.Equal(t, 1, radius)
	require.Same(t, enemy, closest, "approach when no target can be fired upon")
}

func TestRangeThreeTorpedoesAreNotTreatedAsShieldOnlyBeams(t *testing.T) {
	source := testBattleToken(1, Vector{}, BattleTacticMaximizeRange, 2)
	enemy := testBattleToken(2, Vector{3, 0}, BattleTacticMaximizeDamage, 1)
	source.movesLeft = 1
	source.weaponSlots = []*battleWeaponSlot{{weaponType: battleWeaponTypeTorpedo, power: 10, slotQuantity: 1, weaponRange: 3, accuracy: 1}}
	b := battle{tokens: []*battleToken{source, enemy}}
	_, closest := b.movementSearch(source, BattleTargetAny)
	require.Nil(t, closest)
}

func TestDefaultStrategySurvivesSpecRecompute(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	d := p.Designs[0]
	d.Spec.DefaultBattlePlan = 4
	spec, err := ComputeShipDesignSpec(&u.Game.Rules, p.TechLevels, p.Race.Spec, d)
	require.NoError(t, err)
	require.Equal(t, 4, spec.DefaultBattlePlan)
}
