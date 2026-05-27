package domain

import (
	"strconv"
	"testing"
)

func TestHashRing_GetNodeIsStable(t *testing.T) {
	hr := NewHashRing()
	hr.AddNode("node1", 100)
	hr.AddNode("node2", 100)

	key1 := "project-abc"
	nodeA := hr.GetNode(key1)
	nodeB := hr.GetNode(key1)

	if nodeA == "" {
		t.Fatal("expected non-empty node")
	}
	if nodeA != nodeB {
		t.Fatalf("expected node to be stable, got %q first then %q", nodeA, nodeB)
	}
}

func TestHashRing_RemoveNodeRemapsOnlyRemovedNodeKeys(t *testing.T) {
	hr := NewHashRing()
	hr.AddNode("node1", 50)
	hr.AddNode("node2", 50)
	hr.AddNode("node3", 50)

	keys := make([]string, 100)
	for i := 0; i < 100; i++ {
		keys[i] = "key-" + strconv.Itoa(i)
	}

	assignmentsBefore := make(map[string]string)
	for _, k := range keys {
		assignmentsBefore[k] = hr.GetNode(k)
	}

	hr.RemoveNode("node2")

	for _, k := range keys {
		oldNode := assignmentsBefore[k]
		newNode := hr.GetNode(k)

		if oldNode == "node2" {
			if newNode == "node2" {
				t.Fatalf("key %s should have been remapped from removed node2", k)
			}
		} else {
			if oldNode != newNode {
				t.Fatalf("key %s was mapped to %s but remapped to %s despite node not being removed", k, oldNode, newNode)
			}
		}
	}
}

func TestHashRing_AddNodeTwiceDoesNotDuplicateVirtualNodes(t *testing.T) {
	hr := NewHashRing()
	hr.AddNode("node1", 10)
	hr.AddNode("node1", 10)

	nodes := hr.Nodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	
	hr.mu.RLock()
	ringLen := len(hr.ring)
	hr.mu.RUnlock()
	if ringLen != 10 {
		t.Fatalf("expected ring size 10, got %d", ringLen)
	}
}

func TestHashRing_EmptyRingReturnsEmptyNode(t *testing.T) {
	hr := NewHashRing()
	node := hr.GetNode("any-key")
	if node != "" {
		t.Fatalf("expected empty node from empty ring, got %q", node)
	}
}
