package cs

import "encoding/json"

type patrolReturnKey struct {
	design *ShipDesign
	state  string
}

// Only shared within one balanceDispersedPatrolFuel call, before any transfer.
// These are single-ship patrols of one owner at one position. Keep every
// serialized property except identity; include the actual design pointer too.
// Roles are checked before lookup. Nothing survives changes to reports/orders.
func dispersedPatrolReturnKey(f *Fleet) (patrolReturnKey, bool) {
	if f.Delete || len(f.Tokens) != 1 || f.Tokens[0].Quantity != 1 || f.Tokens[0].design == nil {
		return patrolReturnKey{}, false
	}
	copy := *f
	copy.GameDBObject = GameDBObject{}
	copy.Num, copy.Name, copy.BaseName = 0, "", ""
	data, err := json.Marshal(&copy)
	return patrolReturnKey{f.Tokens[0].design, string(data)}, err == nil
}
