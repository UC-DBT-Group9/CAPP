package kv

// Since returns the usage accumulated between an earlier snapshot (before) and
// this one. KVServer.GetUsage reports running totals since the system started,
// so experiments take a snapshot before and after each query and use Since to
// get that query's own cost:
//
//	before, _ := coord.GetUsage()
//	// ... run the query ...
//	after, _ := coord.GetUsage()
//	cost := after.Since(before).Cost(DefaultRates())
//
// StoredBytes is a current level rather than a running total, so it keeps the
// later value. Storage-at-rest cost over one query is tiny and the same for
// every strategy, so it does not affect which strategy wins.
func (u SystemUsage) Since(before SystemUsage) SystemUsage {
	var d SystemUsage
	d.StoredBytes = u.StoredBytes
	d.StorageByteHours = u.StorageByteHours - before.StorageByteHours
	d.Requests = u.Requests - before.Requests
	d.GetBytesReturned = u.GetBytesReturned - before.GetBytesReturned
	d.SelectBytesScanned = u.SelectBytesScanned - before.SelectBytesScanned
	d.SelectBytesReturned = u.SelectBytesReturned - before.SelectBytesReturned
	d.OutboundBytes = u.OutboundBytes - before.OutboundBytes
	d.ComputeTime = u.ComputeTime - before.ComputeTime
	return d
}

// StrategyCost pairs a strategy with what one query cost when run that way.
type StrategyCost struct {
	Strategy Strategy
	Cost     UsageCost
}

// Oracle returns the cheaper of always-pushdown and never-pushdown for one
// query, decided in hindsight from both measured runs. It is the best any
// planner choosing between the two could do, so CAPP's total cost is compared
// against the oracle's. Ties go to pull, which avoids S3 Select charges.
func Oracle(push, pull UsageCost) StrategyCost {
	if push.Total < pull.Total {
		return StrategyCost{Strategy: StrategyPushdown, Cost: push}
	}
	return StrategyCost{Strategy: StrategyPull, Cost: pull}
}
