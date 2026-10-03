// Package peoplenest splits employee wages across stores proportional to
// worked minutes while preserving per-employee wages and per-store targets.
package peoplenest

// WorkRecord is one employee's worked minutes at one store.
type WorkRecord struct {
	StoreID string
	Minutes int64
}

// Employee carries a stable ID, the confirmed wage (integer cents), and the
// worked-minutes records for this period.
type Employee struct {
	ID    string
	Wage  int64
	Works []WorkRecord
}

// Store carries a stable ID and the settlement target (integer cents).
type Store struct {
	ID     string
	Target int64
}

// Solve splits each employee's wage across the stores they worked at.
//
// Every worked pair gets either floor or ceil of its theoretical amount
// (wage * minutes / total minutes); pairs without work records get nothing.
// Per-employee sums equal the wage and per-store sums equal the target.
// Among all feasible splits it minimizes the total absolute deviation from
// the theoretical amounts. The returned matrix maps
// employeeID -> storeID -> cents and contains exactly the worked pairs.
// The second return value is false when no feasible split exists.
func Solve(employees []Employee, stores []Store) (map[string]map[string]int64, bool) {
	storeIndex := make(map[string]int, len(stores))
	for i, s := range stores {
		storeIndex[s.ID] = i
	}

	// base[e][r] is the floor of the theoretical amount for record r of
	// employee e; rem[e][r] is the scaled remainder used for rounding costs.
	base := make([][]int64, len(employees))
	rem := make([][]int64, len(employees))
	denom := make([]int64, len(employees))
	empExtra := make([]int64, len(employees))
	storeExtra := make([]int64, len(stores))

	for e, emp := range employees {
		var totalMin int64
		for _, w := range emp.Works {
			totalMin += w.Minutes
		}
		base[e] = make([]int64, len(emp.Works))
		rem[e] = make([]int64, len(emp.Works))
		denom[e] = totalMin
		var floorSum int64
		for r, w := range emp.Works {
			prod := emp.Wage * w.Minutes
			base[e][r] = prod / totalMin
			rem[e][r] = prod % totalMin
			floorSum += base[e][r]
			storeExtra[storeIndex[w.StoreID]] -= base[e][r]
		}
		empExtra[e] = emp.Wage - floorSum
	}
	for i, s := range stores {
		storeExtra[i] += s.Target
		if storeExtra[i] < 0 {
			return nil, false
		}
	}

	// The extra cents employees must place beyond the floors must match the
	// extra cents stores still need; equivalently total wages must equal
	// total targets.
	var empExtraSum, storeExtraSum int64
	for _, v := range empExtra {
		empExtraSum += v
	}
	for _, v := range storeExtra {
		storeExtraSum += v
	}
	if empExtraSum != storeExtraSum {
		return nil, false
	}

	// Nodes: 0 = source, 1..E employees, E+1..E+S stores, E+S+1 = sink.
	numEmp, numStore := len(employees), len(stores)
	source, sink := 0, 1+numEmp+numStore
	flow := newMinCostFlow(sink + 1)

	var required int64
	for e := range employees {
		flow.addEdge(source, 1+e, empExtra[e], 0, -1)
		required += empExtra[e]
	}
	type midEdge struct {
		node, edgeIdx int
		emp, record   int
	}
	var mids []midEdge
	for e, emp := range employees {
		for r, w := range emp.Works {
			sNode := 1 + numEmp + storeIndex[w.StoreID]
			// Rounding up changes the scaled deviation by denom - 2*rem.
			cost := denom[e] - 2*rem[e][r]
			idx := flow.addEdge(1+e, sNode, 1, cost, -1)
			mids = append(mids, midEdge{node: 1 + e, edgeIdx: idx, emp: e, record: r})
		}
	}
	for i := range stores {
		flow.addEdge(1+numEmp+i, sink, storeExtra[i], 0, -1)
	}

	var sent int64
	for sent < required {
		amount, _, ok := flow.augment(source, sink)
		if !ok {
			return nil, false
		}
		sent += amount
	}

	matrix := make(map[string]map[string]int64, numEmp)
	for e, emp := range employees {
		row := make(map[string]int64, len(emp.Works))
		for r, w := range emp.Works {
			row[w.StoreID] = base[e][r]
		}
		matrix[emp.ID] = row
	}
	for _, m := range mids {
		if flow.graph[m.node][m.edgeIdx].cap == 0 { // the unit of flow was used
			emp := employees[m.emp]
			matrix[emp.ID][emp.Works[m.record].StoreID]++
		}
	}
	return matrix, true
}
