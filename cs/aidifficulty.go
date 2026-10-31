package cs

// IsAdvancedAI enables the stronger economic and racial strategy goals.
func IsAdvancedAI(d AIDifficulty) bool {
	return d == AIDifficultyAdvanced || d == AIDifficultyHard || d == AIDifficultyElite || d == AIDifficultyCheater
}
