package peoplenest

// mcmfEdge is one directed edge in the min-cost max-flow graph.
type mcmfEdge struct {
	to, rev    int
	cap, cost  int64
	flowOrigin int // index of the work record this edge carries, -1 otherwise
}

// minCostFlow is a small successive-shortest-path solver. Edge costs may be
// negative, so each augmentation uses SPFA instead of Dijkstra.
type minCostFlow struct {
	graph [][]mcmfEdge
}

func newMinCostFlow(nodes int) *minCostFlow {
	return &minCostFlow{graph: make([][]mcmfEdge, nodes)}
}

// addEdge appends a forward edge with the given capacity and cost plus a
// zero-capacity residual edge, and returns the forward edge's position in
// graph[from].
func (m *minCostFlow) addEdge(from, to int, cap, cost int64, origin int) int {
	fwd := mcmfEdge{to: to, rev: len(m.graph[to]), cap: cap, cost: cost, flowOrigin: origin}
	rev := mcmfEdge{to: from, rev: len(m.graph[from]), cap: 0, cost: -cost, flowOrigin: -1}
	m.graph[from] = append(m.graph[from], fwd)
	m.graph[to] = append(m.graph[to], rev)
	return len(m.graph[from]) - 1
}

// augment finds a shortest source->sink path by cost and pushes as much flow
// as the bottleneck allows. It returns the pushed amount and the path cost
// per unit, or ok=false when no path remains.
func (m *minCostFlow) augment(source, sink int) (amount, unitCost int64, ok bool) {
	n := len(m.graph)
	const inf = int64(1) << 62
	dist := make([]int64, n)
	inQueue := make([]bool, n)
	prevNode := make([]int, n)
	prevEdge := make([]int, n)
	for i := range dist {
		dist[i] = inf
	}
	dist[source] = 0
	queue := []int{source}
	inQueue[source] = true
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		inQueue[u] = false
		for ei, e := range m.graph[u] {
			if e.cap <= 0 {
				continue
			}
			if nd := dist[u] + e.cost; nd < dist[e.to] {
				dist[e.to] = nd
				prevNode[e.to] = u
				prevEdge[e.to] = ei
				if !inQueue[e.to] {
					inQueue[e.to] = true
					queue = append(queue, e.to)
				}
			}
		}
	}
	if dist[sink] == inf {
		return 0, 0, false
	}
	amount = inf
	for v := sink; v != source; v = prevNode[v] {
		if c := m.graph[prevNode[v]][prevEdge[v]].cap; c < amount {
			amount = c
		}
	}
	for v := sink; v != source; v = prevNode[v] {
		u, ei := prevNode[v], prevEdge[v]
		m.graph[u][ei].cap -= amount
		rev := m.graph[u][ei].rev
		m.graph[v][rev].cap += amount
	}
	return amount, dist[sink], true
}
