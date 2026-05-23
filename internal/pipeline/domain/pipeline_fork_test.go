package domain

import "testing"

func TestPipeline_Fork(t *testing.T) {
	parent := NewPipeline("pipe-1", "proj-A", "Parent", "alice", 1, 1)
	parent.Region = "bj"
	parent.Config = PipelineConfig{Language: "go", Framework: "gin", MaxAgents: 3}

	child := parent.Fork("pipe-2", "Child Fork", "bob")

	if child.ID != "pipe-2" {
		t.Errorf("child ID = %q, want pipe-2", child.ID)
	}
	if child.ProjectID != parent.ProjectID {
		t.Errorf("child ProjectID = %q, want %q", child.ProjectID, parent.ProjectID)
	}
	if child.Level != "L2" {
		t.Errorf("child Level = %q, want L2 (parent is L1)", child.Level)
	}
	if *child.ParentPipelineID != "pipe-1" {
		t.Errorf("child ParentPipelineID = %q, want pipe-1", *child.ParentPipelineID)
	}
	if child.Region != "bj" {
		t.Errorf("child Region = %q, want bj", child.Region)
	}
	if child.Config.Language != "go" {
		t.Errorf("child Config not inherited: %+v", child.Config)
	}
	if child.Status != "pending" {
		t.Errorf("child Status = %q, want pending", child.Status)
	}
	if len(child.Stages) == 0 {
		t.Fatal("child has no stages")
	}
	if child.CurrentStage == "" {
		t.Error("child CurrentStage is empty — will fail DB CHECK constraint")
	}
}

func TestPipeline_IsSubPipeline(t *testing.T) {
	parent := NewPipeline("pipe-1", "proj-A", "Parent", "alice", 1, 1)
	if parent.IsSubPipeline() {
		t.Error("root pipeline should not be sub-pipeline")
	}

	child := parent.Fork("pipe-2", "Child", "bob")
	if !child.IsSubPipeline() {
		t.Error("forked pipeline should be sub-pipeline")
	}
}

func TestPipeline_Fork_L1ParentYieldsL2Child(t *testing.T) {
	parent := &Pipeline{ID: "p1", ProjectID: "proj-A", Level: "L1"}
	child := parent.Fork("p2", "Child", "alice")
	if child.Level != "L2" {
		t.Errorf("L1 parent → child level = %q, want L2", child.Level)
	}
}

func TestPipeline_Fork_L2ParentYieldsL3Child(t *testing.T) {
	parent := &Pipeline{ID: "p1", ProjectID: "proj-A", Level: "L2"}
	child := parent.Fork("p2", "Child", "alice")
	if child.Level != "L3" {
		t.Errorf("L2 parent → child level = %q, want L3", child.Level)
	}
}

func TestPipeline_Fork_L3ParentYieldsL3Child(t *testing.T) {
	parent := &Pipeline{ID: "p1", ProjectID: "proj-A", Level: "L3"}
	child := parent.Fork("p2", "Child", "alice")
	if child.Level != "L3" {
		t.Errorf("L3 parent → child level = %q, want L3 (max)", child.Level)
	}
}
