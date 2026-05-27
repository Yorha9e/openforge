package domain

import (
	"hash/crc32"
	"sort"
	"strconv"
	"sync"
)

// HashRing provides consistent hashing for Coordinator node sharding.
// Project IDs are hashed onto a ring of virtual nodes, ensuring minimal
// redistribution when nodes are added or removed.
//
// This is used by the Coordinator (Phase 8, Section 8.1) to assign each
// project's pipeline execution to a specific coordinator instance, allowing
// stateful caches and in-memory agent conversations to remain local.
type HashRing struct {
	mu       sync.RWMutex
	nodes    map[string]int    // nodeID -> virtual node count
	ring     []uint32          // sorted hash values (the ring)
	ringMap  map[uint32]string // hash -> nodeID
}

// NewHashRing creates an empty hash ring.
func NewHashRing() *HashRing {
	return &HashRing{
		nodes:   make(map[string]int),
		ringMap: make(map[uint32]string),
	}
}

// AddNode registers a coordinator node with the given number of virtual nodes.
// Virtual nodes improve distribution uniformity; 100-200 per physical node is
// a reasonable default for clusters with 3-10 nodes.
func (h *HashRing) AddNode(nodeID string, virtualNodes int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.nodes[nodeID]; exists {
		h.removeNodeLocked(nodeID)
	}

	h.nodes[nodeID] = virtualNodes
	for i := 0; i < virtualNodes; i++ {
		hash := crc32.ChecksumIEEE([]byte(nodeID + "-" + strconv.Itoa(i)))
		h.ring = append(h.ring, hash)
		h.ringMap[hash] = nodeID
	}
	sort.Slice(h.ring, func(i, j int) bool { return h.ring[i] < h.ring[j] })
}

// GetNode returns the coordinator node responsible for the given key
// (typically a project_id).  Returns an empty string if no nodes are
// registered.
func (h *HashRing) GetNode(key string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.ring) == 0 {
		return ""
	}

	hash := crc32.ChecksumIEEE([]byte(key))
	idx := sort.Search(len(h.ring), func(i int) bool { return h.ring[i] >= hash })
	if idx >= len(h.ring) {
		idx = 0 // wrap around
	}
	return h.ringMap[h.ring[idx]]
}

// RemoveNode removes a coordinator node and all its virtual nodes from the
// ring.  No-op if the node was never registered.
func (h *HashRing) RemoveNode(nodeID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.removeNodeLocked(nodeID)
}

func (h *HashRing) removeNodeLocked(nodeID string) {
	delete(h.nodes, nodeID)

	newRing := make([]uint32, 0, len(h.ring))
	for _, hash := range h.ring {
		if h.ringMap[hash] != nodeID {
			newRing = append(newRing, hash)
		} else {
			delete(h.ringMap, hash)
		}
	}
	h.ring = newRing
}

// Nodes returns a snapshot of all registered node IDs and their virtual node
// counts.
func (h *HashRing) Nodes() map[string]int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make(map[string]int, len(h.nodes))
	for k, v := range h.nodes {
		result[k] = v
	}
	return result
}

// RebalanceKeys simulates adding a node and reports which keys would move to
// it.  Useful for capacity planning and rebalance detection before a new
// coordinator is deployed.
func (h *HashRing) RebalanceKeys(newNodeID string, virtualNodes int, keys []string) map[string]string {
	// Snapshot current assignments.
	before := make(map[string]string, len(keys))
	for _, key := range keys {
		before[key] = h.GetNode(key)
	}

	// Temporarily add the new node.
	h.AddNode(newNodeID, virtualNodes)
	defer h.RemoveNode(newNodeID)

	// Detect moves.
	moved := make(map[string]string)
	for _, key := range keys {
		after := h.GetNode(key)
		if after != before[key] {
			moved[key] = after
		}
	}
	return moved
}
