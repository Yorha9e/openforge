package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileTool(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(tmpFile, []byte("hello world"), 0644)

	tool := &ReadFileTool{}
	input, _ := json.Marshal(map[string]string{"path": tmpFile})
	output, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	var result map[string]string
	json.Unmarshal(output, &result)
	if result["content"] != "hello world" {
		t.Errorf("content = %q, want %q", result["content"], "hello world")
	}
}

func TestWriteFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "output.txt")

	tool := &WriteFileTool{}
	input, _ := json.Marshal(map[string]string{
		"path":    tmpFile,
		"content": "test content",
	})
	_, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("content = %q, want %q", string(content), "test content")
	}
}

func TestEditFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "edit.txt")
	os.WriteFile(tmpFile, []byte("hello world"), 0644)

	tool := &EditFileTool{}
	input, _ := json.Marshal(map[string]string{
		"path":    tmpFile,
		"old_str": "world",
		"new_str": "golang",
	})
	_, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	content, _ := os.ReadFile(tmpFile)
	if string(content) != "hello golang" {
		t.Errorf("content = %q, want %q", string(content), "hello golang")
	}
}

func TestListDirTool(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("b"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	tool := &ListDirTool{}
	input, _ := json.Marshal(map[string]string{"path": tmpDir})
	output, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(output, &result)
	count := int(result["count"].(float64))
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestGrepTool(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "grep.txt")
	os.WriteFile(tmpFile, []byte("line1\nline2\nline3\n"), 0644)

	tool := &GrepTool{}
	input, _ := json.Marshal(map[string]string{
		"pattern": "line2",
		"path":    tmpFile,
	})
	output, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(output, &result)
	count := int(result["count"].(float64))
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestGlobTool(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte("package a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.go"), []byte("package b"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte("text"), 0644)

	tool := &GlobTool{}
	input, _ := json.Marshal(map[string]string{
		"pattern": filepath.Join(tmpDir, "*.go"),
	})
	output, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(output, &result)
	count := int(result["count"].(float64))
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestSearchFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte("package a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.go"), []byte("package b"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "sub"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "sub", "c.go"), []byte("package c"), 0644)

	tool := &SearchFileTool{}
	input, _ := json.Marshal(map[string]interface{}{
		"pattern":   "*.go",
		"path":      tmpDir,
		"recursive": true,
	})
	output, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(output, &result)
	count := int(result["count"].(float64))
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestDeleteFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "delete.txt")
	os.WriteFile(tmpFile, []byte("to delete"), 0644)

	tool := &DeleteFileTool{}
	input, _ := json.Marshal(map[string]string{"path": tmpFile})
	_, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestReplaceInFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "replace.txt")
	os.WriteFile(tmpFile, []byte("hello world"), 0644)

	tool := &ReplaceInFileTool{}
	input, _ := json.Marshal(map[string]string{
		"path":    tmpFile,
		"old_str": "world",
		"new_str": "golang",
	})
	_, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	content, _ := os.ReadFile(tmpFile)
	if string(content) != "hello golang" {
		t.Errorf("content = %q, want %q", string(content), "hello golang")
	}
}
