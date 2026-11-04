package cs

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"slices"
)

// All private design pointers in enemy fleet reports are deliberately ignored.
// The model owns its designs and fleets; native battle receives fresh mutable
// fleet/token copies and a separately seeded Rules RNG for every sample.
type patrolBattleCopy struct {
	rules                Rules
	owner                int
	players              map[int]*Player
	fleets               []*Fleet
	automatic            map[int]bool
	ownValue             float64
	ownShips, enemyShips int
	weights              [3]float64
	repairRate           float64
}

type patrolBattleOutcome struct {
	Wiped, Unfinished                 bool
	MissionSucceeded                  bool
	LostValue, DamageValue, MaxDamage float64
	RepairTurns                       int
	OwnSurvivors, EnemySurvivors      int
	LostShips                         int
	DangerBefore, DangerAfter         float64
	BombersBefore, BombersAfter       int
}

// Combat and bombardment capability are mission threats. Escaping scouts and
// unarmed tankers are follow-up targets, not a veto on rescuing a colony.
func patrolTokenDanger(token ShipToken) float64 {
	if !token.design.Spec.HasWeapons {
		return 0
	}
	remaining := float64(token.Quantity)
	remaining -= float64(token.QuantityDamaged) * Clamp(token.Damage/float64(max(1, token.design.Spec.Armor)), 0.0, 1.0)
	return float64(max(1, shipCombatPower(token.design.Spec))) * remaining
}

func patrolTokenBomber(token ShipToken) bool {
	s := token.design.Spec
	return len(s.Bombs)+len(s.SmartBombs)+len(s.RetroBombs) > 0
}

func (o patrolBattleOutcome) safe(ownShips int, ownValue float64) bool {
	return !o.Wiped && o.MissionSucceeded && float64(o.LostShips) <= float64(ownShips)*.05+1e-9 && o.LostValue <= ownValue*.05+1e-9
}

func patrolDeepCopy[T any](value *T) (*T, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result T
	err = json.Unmarshal(raw, &result)
	return &result, err
}

func patrolPlanForFleet(p *Player, f *Fleet) BattlePlan {
	for _, plan := range p.BattlePlans {
		if plan.Num == f.BattlePlanNum {
			return plan
		}
	}
	return BattlePlan{Num: 0, Name: "Default", Tactic: BattleTacticMaximizeDamage, PrimaryTarget: BattleTargetArmedShips, SecondaryTarget: BattleTargetAny, AttackWho: BattleAttackWhoEnemiesAndNeutrals}
}

func (m *patrolBattleCopy) value(c Cost) float64 {
	return float64(c.Resources) + float64(c.Ironium)*m.weights[0] + float64(c.Boranium)*m.weights[1] + float64(c.Germanium)*m.weights[2]
}

func newPatrolBattleCopy(rules *Rules, p *Player, own, reports []*Fleet) (*patrolBattleCopy, error) {
	if rules == nil || p == nil || len(own) == 0 || len(reports) == 0 {
		return nil, fmt.Errorf("incomplete battle model")
	}
	m := &patrolBattleCopy{rules: *rules, owner: p.Num, players: map[int]*Player{}, automatic: map[int]bool{}}
	m.repairRate = rules.RepairRates[RepairRateStopped]
	stocks := [3]int{}
	for _, planet := range p.PlanetIntels {
		if planet != nil && planet.OwnedBy(p.Num) {
			stocks[0] += planet.Cargo.Ironium
			stocks[1] += planet.Cargo.Boranium
			stocks[2] += planet.Cargo.Germanium
		}
	}
	for i, stock := range stocks {
		m.weights[i] = 1 + 300/float64(300+max(0, stock))
	}
	owner := &Player{Num: p.Num, Race: p.Race, PlayerPlans: PlayerPlans{BattlePlans: slices.Clone(p.BattlePlans)}}
	owner.Race.Spec = ComputeRaceSpec(&owner.Race, &m.rules)
	m.players[p.Num] = owner
	for _, source := range own {
		if source == nil || source.Delete || source.PlayerNum != p.Num {
			return nil, fmt.Errorf("invalid owned formation")
		}
		fleet, err := patrolDeepCopy(source)
		if err != nil {
			return nil, err
		}
		fleet.Tokens = slices.Clone(source.Tokens)
		for i := range fleet.Tokens {
			token := &fleet.Tokens[i]
			design := p.GetDesign(token.DesignNum)
			if design == nil || token.Quantity <= 0 {
				return nil, fmt.Errorf("missing owned design")
			}
			copied, err := patrolDeepCopy(design)
			if err != nil {
				return nil, err
			}
			token.design = copied
			owner.Designs = append(owner.Designs, copied)
			m.ownShips += token.Quantity
			m.ownValue += m.value(copied.Spec.Cost) * float64(token.Quantity)
		}
		plan := patrolPlanForFleet(p, source)
		if isManagedPatrolPlan(source) {
			plan = patrolPlanForFleet(p, &Fleet{})
		}
		fleet.battlePlan = &plan
		// An explicit player plan stays fixed. Automatically managed plans are
		// recognized separately by the production selector.
		m.automatic[source.Num] = automaticPatrolBattlePlan(p, source)
		fleet.Spec = ComputeFleetSpec(&m.rules, owner, fleet)
		m.fleets = append(m.fleets, fleet)
	}
	neutral := Humanoids()
	neutral.Spec = ComputeRaceSpec(&neutral, &m.rules)
	maxRegeneration := 0.0
	for _, lrt := range m.rules.LRTSpecs {
		maxRegeneration += max(0, lrt.ShieldRegenerationRateOffset)
	}
	for _, report := range reports {
		if report == nil || report.Delete || report.PlayerNum == p.Num || p.IsFriend(report.PlayerNum) || report.ReportAge != 0 || unknownFleetEquipment(p, report) {
			return nil, fmt.Errorf("enemy equipment requires reconnaissance")
		}
		shadow := m.players[report.PlayerNum]
		if shadow == nil {
			shadow = &Player{Num: report.PlayerNum, Race: neutral}
			shadow.Race.Spec.ShieldRegenerationRate = maxRegeneration
			m.players[shadow.Num] = shadow
		}
		fleet := &Fleet{MapObject: report.MapObject, FleetOrders: FleetOrders{}, Starbase: report.Starbase}
		fleet.Tags = nil
		for _, observed := range report.Tokens {
			intel := p.GetShipDesignIntel(report.PlayerNum, observed.DesignNum)
			if intel == nil || len(intel.Slots) == 0 || intel.Spec.Armor <= 0 || intel.Spec.Mass <= 0 {
				return nil, fmt.Errorf("incomplete public combat specification")
			}
			design, err := patrolDeepCopy(intel)
			if err != nil {
				return nil, err
			}
			spec, err := ComputeShipDesignSpec(&m.rules, p.TechLevels, neutral.Spec, design)
			if err != nil {
				return nil, err
			}
			// Bonuses, armor and speed are observable after equipment discovery.
			// Rebuild component-derived slots/engine fields; never borrow the actual
			// hidden design or its race, technology, cargo, plan or fuel.
			spec.Armor = intel.Spec.Armor
			spec.Shields = intel.Spec.Shields
			spec.Mass = intel.Spec.Mass
			spec.Initiative = intel.Spec.Initiative
			spec.BeamBonus = intel.Spec.BeamBonus
			spec.BeamDefense = intel.Spec.BeamDefense
			spec.TorpedoBonus = intel.Spec.TorpedoBonus
			spec.TorpedoJamming = intel.Spec.TorpedoJamming
			spec.ReduceMovement = intel.Spec.ReduceMovement
			design.Spec = spec
			if intel.Spec.Movement > 0 {
				design.Spec.MovementBonus += float64(intel.Spec.Movement - design.getMovement(&m.rules, 0))
			}
			shadow.Designs = append(shadow.Designs, design)
			// Reported damage is public. The report's private token.design is ignored.
			fleet.Tokens = append(fleet.Tokens, ShipToken{DesignNum: observed.DesignNum, Quantity: observed.Quantity, Damage: observed.Damage, QuantityDamaged: observed.QuantityDamaged, design: design})
			m.enemyShips += observed.Quantity
		}
		fleet.Spec = ComputeFleetSpec(&m.rules, shadow, fleet)
		m.fleets = append(m.fleets, fleet)
	}
	maxNum := 0
	for num := range m.players {
		maxNum = max(maxNum, num)
	}
	for num, shadow := range m.players {
		shadow.Relations = make([]PlayerRelationship, maxNum)
		for other := range m.players {
			relation := PlayerRelationFriend
			if (num == p.Num) != (other == p.Num) {
				relation = PlayerRelationEnemy
			}
			shadow.Relations[other-1] = PlayerRelationship{Relation: relation}
		}
	}
	if m.ownShips == 0 || m.enemyShips == 0 {
		return nil, fmt.Errorf("empty battle formation")
	}
	return m, nil
}

func (m *patrolBattleCopy) run(tactic, enemyTactic BattleTactic, seed int64) patrolBattleOutcome {
	rules := m.rules
	rules.ResetSeed(seed)
	fleets := make([]*Fleet, 0, len(m.fleets))
	for _, source := range m.fleets {
		fleet := *source
		fleet.Tokens = slices.Clone(source.Tokens)
		plan := BattlePlan{Num: 999999, Name: "Forecast", Tactic: enemyTactic, PrimaryTarget: BattleTargetArmedShips, SecondaryTarget: BattleTargetAny, AttackWho: BattleAttackWhoEnemies}
		if enemyTactic == "" {
			plan = patrolPlanForFleet(m.players[source.PlayerNum], &Fleet{})
		}
		if source.PlayerNum == m.owner {
			plan = *source.battlePlan
			if m.automatic[source.Num] {
				plan = patrolPlanForFleet(m.players[m.owner], &Fleet{})
			}
			if m.automatic[source.Num] && tactic != "" {
				plan = BattlePlan{Num: 999999, Name: "Forecast", Tactic: tactic, PrimaryTarget: BattleTargetArmedShips, SecondaryTarget: BattleTargetAny, AttackWho: BattleAttackWhoEnemies}
			}
		}
		fleet.battlePlan = &plan
		fleets = append(fleets, &fleet)
	}
	battle := newBattler(slog.New(slog.NewTextHandler(io.Discard, nil)), &rules, 1, m.players, fleets, nil)
	if battle == nil {
		return patrolBattleOutcome{Unfinished: true, Wiped: true}
	}
	battle.runBattle()
	result := patrolBattleOutcome{}
	for _, fleet := range fleets {
		for _, token := range fleet.Tokens {
			if fleet.PlayerNum != m.owner {
				result.EnemySurvivors += token.Quantity
				result.DangerAfter += patrolTokenDanger(token)
				if patrolTokenBomber(token) {
					result.BombersAfter += token.Quantity
				}
				continue
			}
			result.OwnSurvivors += token.Quantity
			if token.QuantityDamaged > 0 && token.Quantity > 0 {
				fraction := Clamp(token.Damage/float64(max(1, token.design.Spec.Armor)), 0.0, 1.0)
				result.MaxDamage = max(result.MaxDamage, fraction)
				result.DamageValue += m.value(token.design.Spec.Cost) * float64(token.QuantityDamaged) * fraction
				// Readiness estimate uses native stopped-space repair and the actual
				// participating fleet's repair equipment. It is not a repair invoice.
				rate := m.repairRate*m.players[m.owner].Race.Spec.RepairFactor + fleet.Spec.RepairBonus
				if rate <= 0 {
					result.RepairTurns = max(result.RepairTurns, 100)
				} else {
					result.RepairTurns = max(result.RepairTurns, int(math.Ceil(fraction/rate)))
				}
			}
		}
	}
	for i, original := range m.fleets {
		if original.PlayerNum != m.owner {
			for _, token := range original.Tokens {
				result.DangerBefore += patrolTokenDanger(token)
				if patrolTokenBomber(token) {
					result.BombersBefore += token.Quantity
				}
			}
		}
		if original.PlayerNum == m.owner {
			for _, before := range original.Tokens {
				remaining := 0
				for _, after := range fleets[i].Tokens {
					if before.DesignNum == after.DesignNum {
						remaining += after.Quantity
					}
				}
				result.LostValue += float64(max(0, before.Quantity-remaining)) * m.value(before.design.Spec.Cost)
			}
		}
	}
	result.Wiped = result.OwnSurvivors == 0
	result.LostShips = max(0, m.ownShips-result.OwnSurvivors)
	result.Unfinished = result.EnemySurvivors > 0
	// At least half of the reported combat capability must be removed, and
	// actual bombers must be eliminated. Harmless exchanges are not victories.
	if result.DangerBefore > 0 || result.BombersBefore > 0 {
		result.MissionSucceeded = result.BombersAfter == 0 && (result.DangerBefore == 0 || result.DangerAfter <= result.DangerBefore*.5)
	} else {
		result.MissionSucceeded = result.EnemySurvivors < m.enemyShips
	}
	return result
}
