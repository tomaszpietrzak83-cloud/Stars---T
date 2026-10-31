package ai

import "github.com/sirgwain/craig-stars/cs"

func RaceForDifficulty(race cs.Race, difficulty cs.AIDifficulty) cs.Race {
	if difficulty != cs.AIDifficultyElite {
		return race
	}
	a, b := CheaterRaces[0], CheaterRaces[1]
	avg := func(v, x, y int) int { return (2*v + x + y + 2) / 4 }
	race.FactoryOutput = max(race.FactoryOutput, avg(race.FactoryOutput, a.FactoryOutput, b.FactoryOutput))
	race.FactoryCost = min(race.FactoryCost, avg(race.FactoryCost, a.FactoryCost, b.FactoryCost))
	race.NumFactories = max(race.NumFactories, avg(race.NumFactories, a.NumFactories, b.NumFactories))
	race.MineOutput = max(race.MineOutput, avg(race.MineOutput, a.MineOutput, b.MineOutput))
	race.MineCost = min(race.MineCost, avg(race.MineCost, a.MineCost, b.MineCost))
	race.NumMines = max(race.NumMines, avg(race.NumMines, a.NumMines, b.NumMines))
	race.PopEfficiency = min(race.PopEfficiency, avg(race.PopEfficiency, a.PopEfficiency, b.PopEfficiency))
	return race
}
