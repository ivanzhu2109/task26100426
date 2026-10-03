package peoplenest

import (
	"container/heap"
	"math/big"
)

// mcfEdge is a directed edge in the residual graph. Costs are exact
// rationals: rounding deviations are fractions of an employee's total
// minutes and do not share a common denominator in general.
type mcfEdge struct {
	to, rev int
	cap     int64
	cost    *big.Rat
}

// minCostFlow is a successive-shortest-path solver with Johnson potentials.
type minCostFlow struct {
	graph [][]mcfEdge
}

func newMinCostFlow(numNodes int) *minCostFlow {
	return &minCostFlow{graph: make([][]mcfEdge, numNodes)}
}

// addEdge adds a forward edge with the given capacity and per-unit cost.
func (m *minCostFlow) addEdge(from, to int, cap int64, cost *big.Rat) {
	fwd := mcfEdge{to: to, rev: len(m.graph[to]), cap: cap, cost: cost}
	rev := mcfEdge{to: from, rev: len(m.graph[from]), cap: 0, cost: new(big.Rat).Neg(cost)}
	m.graph[from] = append(m.graph[from], fwd)
	m.graph[to] = append(m.graph[to], rev)
}

// flow pushes amount units from source to sink at minimum total cost.
// It returns the achieved flow, its cost, and whether amount was reached.
func (m *minCostFlow) flow(source, sink int, amount int64) (int64, *big.Rat, bool) {
	n := len(m.graph)
	potential := make([]*big.Rat, n)
	dist := make([]*big.Rat, n)
	for i := 0; i < n; i++ {
		potential[i] = new(big.Rat)
	}
	// Bellman-Ford for initial potentials (edge costs may be negative).
	reachable := make([]bool, n)
	dist[source] = new(big.Rat)
	reachable[source] = true
	for iter := 0; iter < n-1; iter++ {
		updated := false
		for v := 0; v < n; v++ {
			if !reachable[v] {
				continue
			}
			for _, e := range m.graph[v] {
				if e.cap <= 0 {
					continue
				}
				cand := new(big.Rat).Add(dist[v], e.cost)
				if !reachable[e.to] || cand.Cmp(dist[e.to]) < 0 {
					if dist[e.to] == nil {
						dist[e.to] = new(big.Rat)
					}
					dist[e.to].Set(cand)
					reachable[e.to] = true
					updated = true
				}
			}
		}
		if !updated {
			break
		}
	}
	for v := 0; v < n; v++ {
		if reachable[v] {
			potential[v].Set(dist[v])
		}
	}

	totalCost := new(big.Rat)
	var totalFlow int64
	prevV := make([]int, n)
	prevE := make([]int, n)
	for totalFlow < amount {
		d := dijkstra(m.graph, source, potential, prevV, prevE)
		if d[sink] == nil {
			return totalFlow, totalCost, false
		}
		for v := 0; v < n; v++ {
			if d[v] != nil {
				potential[v].Add(potential[v], d[v])
			}
		}
		add := amount - totalFlow
		for v := sink; v != source; v = prevV[v] {
			if c := m.graph[prevV[v]][prevE[v]].cap; c < add {
				add = c
			}
		}
		for v := sink; v != source; v = prevV[v] {
			e := &m.graph[prevV[v]][prevE[v]]
			e.cap -= add
			m.graph[v][e.rev].cap += add
			unit := new(big.Rat).Set(e.cost)
			unit.Mul(unit, new(big.Rat).SetInt64(add))
			totalCost.Add(totalCost, unit)
		}
		totalFlow += add
	}
	return totalFlow, totalCost, true
}

type pqItem struct {
	node int
	dist *big.Rat
}

type priorityQueue []pqItem

func (q priorityQueue) Len() int           { return len(q) }
func (q priorityQueue) Less(i, j int) bool { return q[i].dist.Cmp(q[j].dist) < 0 }
func (q priorityQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *priorityQueue) Push(x any)        { *q = append(*q, x.(pqItem)) }
func (q *priorityQueue) Pop() any {
	old := *q
	item := old[len(old)-1]
	*q = old[:len(old)-1]
	return item
}

// dijkstra computes shortest reduced-cost distances from source. A nil entry
// in the result means the node is unreachable.
func dijkstra(graph [][]mcfEdge, source int, potential []*big.Rat, prevV, prevE []int) []*big.Rat {
	n := len(graph)
	dist := make([]*big.Rat, n)
	for i := range prevV {
		prevV[i] = -1
	}
	dist[source] = new(big.Rat)
	pq := &priorityQueue{{node: source, dist: new(big.Rat)}}
	heap.Init(pq)
	for pq.Len() > 0 {
		item := heap.Pop(pq).(pqItem)
		if dist[item.node] != nil && item.dist.Cmp(dist[item.node]) > 0 {
			continue
		}
		for ei, e := range graph[item.node] {
			if e.cap <= 0 {
				continue
			}
			nd := new(big.Rat).Add(item.dist, e.cost)
			nd.Add(nd, potential[item.node])
			nd.Sub(nd, potential[e.to])
			if dist[e.to] == nil || nd.Cmp(dist[e.to]) < 0 {
				dist[e.to] = nd
				prevV[e.to] = item.node
				prevE[e.to] = ei
				heap.Push(pq, pqItem{node: e.to, dist: nd})
			}
		}
	}
	return dist
}
