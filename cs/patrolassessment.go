package cs

import (
	"crypto/sha256"
	"encoding/json"
	"math"
	"slices"
)

type patrolAssessment struct {
	Ready                           bool
	Tactic                          BattleTactic
	Loss, Damage, Score, PeakDamage float64
	RepairTurns                     int
	LostShips                       int
	Reason                          string
}

// Per-player/per-orders-phase, deterministic work budget. Native samples have
// independent seeds and cannot consume the real turn's RNG.
type patrolEvaluator struct {
	rules               *Rules
	cache               map[[32]byte]patrolAssessment
	Work, Battles, Hits int
	Limit               int
	Deferred            map[string]bool
	DefencePlanned      bool
	PlanningDefence     bool
	DefenceMembers      map[int]bool
	DefenceChecks       *patrolEvaluator
}

func (t *turnGenerator) startPatrolEvaluation() {
	t.recaptureEvaluation = map[int]*patrolEvaluator{}
	t.civilianWarnings = map[civilianWarningKey][]*Fleet{}
	t.bombardmentEvaluation = map[int]*patrolEvaluator{}
	for _, p := range t.automaticPlayers() {
		p.patrolEvaluator = &patrolEvaluator{rules: &t.game.Rules, cache: map[[32]byte]patrolAssessment{}, Limit: 4096}
		p.patrolQueue = makePatrolMissionQueue(p, t.game.Year-1)
		t.bombardmentEvaluation[p.Num] = &patrolEvaluator{rules: &t.game.Rules, cache: map[[32]byte]patrolAssessment{}, Limit: 2048}
	}
}

// Direct participants are not merely hypothetical reinforcements. Otherwise
// include only a public converging course arriving within this battle window.
func patrolEncounterReports(p *Player, target *Fleet, point Vector, horizon float64) []*Fleet {
	result := []*Fleet{target}
	tv := target.Heading.Normalized().Scale(target.observedStrategicSpeed())
	for _, other := range p.FleetIntels {
		if other == nil || other.Delete || other.Starbase || other.ReportAge != 0 || other.PlayerNum == p.Num || p.IsFriend(other.PlayerNum) || other.PlayerNum == target.PlayerNum && other.Num == target.Num {
			continue
		}
		direct := other.Position.DistanceTo(point) <= 3 || other.Position.DistanceTo(target.Position) <= 3
		ov := other.Heading.Normalized().Scale(other.observedStrategicSpeed())
		delta := target.Position.Subtract(other.Position).ToFloat64()
		relative := ov.Subtract(tv)
		// Relative motion also catches a moving transport approaching a
		// stationary hostile formation and stopping there to join it.
		closing := delta.X*relative.X+delta.Y*relative.Y > 0
		arrives := false
		if closing && relative.LengthSquared() > 0 {
			at := (delta.X*relative.X + delta.Y*relative.Y) / relative.LengthSquared()
			arrives = at >= 0 && at <= horizon && delta.Subtract(relative.Scale(at)).Length() <= 3
		}
		// A native bounded pursuit can finish at a different reported encounter site.
		predicted := other.Position.ToFloat64().Add(ov.Scale(horizon))
		if direct || arrives || predicted.DistanceTo(point.ToFloat64()) <= 3 {
			result = append(result, other)
		}
	}
	return result
}

func (e *patrolEvaluator) assess(p *Player, own []*Fleet, target *Fleet, point Vector, horizon float64) patrolAssessment {
	return e.assessEncounter(p, own, patrolEncounterReports(p, target, point, horizon), point, horizon)
}

func (e *patrolEvaluator) assessEncounter(p *Player, own []*Fleet, reports []*Fleet, point Vector, horizon float64) patrolAssessment {
	return e.assessFormations(p, own, reports, point, horizon, true)
}

func (e *patrolEvaluator) assessFormations(p *Player, own []*Fleet, reports []*Fleet, point Vector, horizon float64, merge bool) patrolAssessment {
	for _, planet := range p.PlanetIntels {
		if planet != nil && planet.PlayerNum != p.Num && planet.Owned() && !p.IsFriend(planet.PlayerNum) && planet.Spec.HasStarbase && point.DistanceTo(planet.Position) <= 3 {
			included := slices.ContainsFunc(reports, func(f *Fleet) bool {
				return f != nil && f.Starbase && f.PlayerNum == planet.PlayerNum && f.Position == planet.Position && !unknownFleetEquipment(p, f)
			})
			if !included {
				return patrolAssessment{Reason: "Reported hostile starbase requires a dedicated assault"}
			}
		}
	}
	count, unknown := 0, false
	for _, report := range reports {
		count += reportedFleetShips(report)
		unknown = unknown || unknownFleetEquipment(p, report)
	}
	if unknown {
		if count >= 8 {
			return patrolAssessment{Reason: "Scout equipment reconnaissance required"}
		}
		// Preserve the user's small-unknown-contact rule. No invented hidden model
		// is presented as a native forecast of that unknown equipment.
		strength := 0
		for _, f := range own {
			strength += patrolPower(f)
		}
		threat := 0
		for _, report := range reports {
			threat += patrolReportedPower(p, report)
		}
		return patrolAssessment{Ready: strength >= threat*3/2+1, Reason: "Small unknown contact: bounded reported-strength assessment"}
	}
	type formation struct {
		Owner     int
		Tokens    []ShipToken
		Cargo     Cargo
		Plan      BattlePlan
		Automatic bool
		Designs   []*ShipDesign
	}
	inputs := []formation{}
	for _, f := range own {
		auto := automaticPatrolBattlePlan(p, f)
		plan := patrolPlanForFleet(p, f)
		if auto || isManagedPatrolPlan(f) {
			plan = patrolPlanForFleet(p, &Fleet{})
		}
		input := formation{Owner: p.Num, Tokens: f.Tokens, Cargo: f.Cargo, Plan: plan, Automatic: auto}
		for _, token := range f.Tokens {
			input.Designs = append(input.Designs, p.GetDesign(token.DesignNum))
		}
		inputs = append(inputs, input)
	}
	for _, report := range reports {
		input := formation{Owner: report.PlayerNum, Tokens: report.Tokens}
		for _, token := range report.Tokens {
			input.Designs = append(input.Designs, p.GetShipDesignIntel(report.PlayerNum, token.DesignNum))
		}
		inputs = append(inputs, input)
	}
	stocks := Cargo{}
	var orbit *Planet
	for _, planet := range p.PlanetIntels {
		if planet != nil && planet.OwnedBy(p.Num) {
			stocks = stocks.Add(planet.Cargo)
		}
		if planet != nil && planet.Position == point {
			orbit = planet
		}
	}
	raw, _ := json.Marshal([]any{inputs, point, horizon, merge, p.Race, p.TechLevels, p.BattlePlans, stocks.ToMineral(), orbit, e.rules.BattleRules, e.rules.RepairRates})
	key := sha256.Sum256(raw)
	if result, ok := e.cache[key]; ok {
		e.Hits++
		return result
	}
	modeled := own
	if merge {
		merged := patrolMergedFormation(e.rules, p, own)
		if merged == nil {
			return patrolAssessment{Reason: "Player battle plans cannot be combined by automation"}
		}
		modeled = []*Fleet{merged}
	}
	model, err := newPatrolBattleCopy(e.rules, p, modeled, reports)
	if err != nil {
		return patrolAssessment{Reason: "Combat report incomplete; reconnaissance or replan required"}
	}
	for _, planet := range p.PlanetIntels {
		if planet == nil || planet.Position != point {
			continue
		}
		model.repairRate = e.rules.RepairRates[RepairRateOrbiting]
		if planet.OwnedBy(p.Num) {
			model.repairRate = e.rules.RepairRates[RepairRateOrbitingOwnPlanet]
			if design := p.GetDesign(planet.Spec.StarbaseDesignNum); planet.Spec.HasStarbase && design != nil {
				model.repairRate += design.Spec.RepairBonus
			}
		}
		break
	}
	tactics := []BattleTactic{"", BattleTacticMaximizeDamage, BattleTacticMaximizeRange, BattleTacticMinimizeDamageToSelf}
	automatic := false
	for _, value := range model.automatic {
		automatic = automatic || value
	}
	if !automatic {
		tactics = tactics[:1]
	}
	enemyPlans := []BattleTactic{"", BattleTacticMaximizeDamage, BattleTacticMaximizeRange, BattleTacticDisengage, BattleTacticMinimizeDamageToSelf, BattleTacticDisengageIfChallenged}
	armed := false
	work := 1
	for _, f := range model.fleets {
		for _, token := range f.Tokens {
			if f.PlayerNum != p.Num {
				armed = armed || token.design.Spec.HasWeapons
			}
			work += len(token.design.Spec.WeaponSlots) * token.Quantity / 64
		}
	}
	if !armed {
		enemyPlans = []BattleTactic{BattleTacticDisengage}
	}
	sample := func(tactic BattleTactic, seeds int) patrolAssessment {
		result := patrolAssessment{Ready: true, Tactic: tactic}
		for _, ep := range enemyPlans {
			loss, damage, repair := 0.0, 0.0, 0
			for seed := 0; seed < seeds; seed++ {
				if e.Work+work > e.Limit {
					return patrolAssessment{Reason: "Native forecast budget exhausted; concentration retained"}
				}
				e.Work += work
				e.Battles++
				outcome := model.run(tactic, ep, int64(17011+seed*997))
				if !outcome.safe(model.ownShips, model.ownValue, model.ownMass) || !merge && outcome.CivilianLost > 0 {
					return patrolAssessment{Reason: "Native forecast: ineffective mission, unsafe tonnage exchange, civilian losses or own count/value/mass loss exceeds 5%"}
				}
				result.LostShips = max(result.LostShips, outcome.LostShips)
				loss += outcome.LostValue
				damage += outcome.DamageValue
				repair = max(repair, outcome.RepairTurns)
				result.PeakDamage = max(result.PeakDamage, outcome.MaxDamage)
			}
			result.Loss = max(result.Loss, loss/float64(seeds))
			result.Damage = max(result.Damage, damage/float64(seeds))
			result.RepairTurns = max(result.RepairTurns, repair)
		}
		result.Ready = true // Every sampled outcome passed both 5% loss limits.
		result.Score = result.Loss + result.Damage + float64(result.RepairTurns)*model.ownValue*.02
		return result
	}
	screened := []patrolAssessment{}
	for _, tactic := range tactics {
		result := sample(tactic, 2)
		if result.Ready {
			screened = append(screened, result)
		}
	}
	slices.SortStableFunc(screened, func(a, b patrolAssessment) int {
		if a.Score < b.Score {
			return -1
		}
		if a.Score > b.Score {
			return 1
		}
		return 0
	})
	best := patrolAssessment{Reason: "No tested composition and battle plan confirms superiority"}
	best.Score = math.Inf(1)
	for _, candidate := range screened {
		seeds := 16
		if candidate.PeakDamage > .1 {
			seeds = 32
		}
		if candidate.Loss > 0 || candidate.PeakDamage > .5 {
			seeds = 64
		}
		result := sample(candidate.Tactic, seeds)
		if result.Ready {
			best = result
			break
		}
	}
	if e.Work < e.Limit {
		e.cache[key] = best
	}
	return best
}
