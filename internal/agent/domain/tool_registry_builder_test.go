package domain

import (
	"context"
	"strings"
	"testing"
)

func TestIsAbs_PathVariants(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "unix absolute", path: "/tmp/a.txt", want: true},
		{name: "windows absolute backslash", path: `C:\\tmp\\a.txt`, want: true},
		{name: "windows absolute slash", path: "C:/tmp/a.txt", want: true},
		{name: "windows drive relative", path: "C:tmp\\a.txt", want: false},
		{name: "windows UNC", path: `\\\\server\\share\\a.txt`, want: true},
		{name: "relative", path: "tmp/a.txt", want: false},
		{name: "empty", path: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAbs(tt.path); got != tt.want {
				t.Fatalf("isAbs(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestDefaultToolRegistryWithWorkDir_FileOpsSmoke(t *testing.T) {
	workDir := t.TempDir()
	reg := DefaultToolRegistryWithWorkDir(workDir)
	ctx := context.Background()

	writeOut, err := reg["write_file"].Executor(ctx, map[string]interface{}{
		"path":    "hello.py",
		"content": "print('hello')\n",
	})
	if err != nil || !strings.Contains(writeOut, "wrote") {
		t.Fatalf("write_file failed, out=%q err=%v", writeOut, err)
	}

	readOut, err := reg["read_file"].Executor(ctx, map[string]interface{}{
		"path": "hello.py",
	})
	if err != nil || !strings.Contains(readOut, "print('hello')") {
		t.Fatalf("read_file failed, out=%q err=%v", readOut, err)
	}

	editOut, err := reg["edit_file"].Executor(ctx, map[string]interface{}{
		"path":    "hello.py",
		"old_str": "hello",
		"new_str": "openforge",
	})
	if err != nil || !strings.Contains(editOut, "edited") {
		t.Fatalf("edit_file failed, out=%q err=%v", editOut, err)
	}

	readOut, err = reg["read_file"].Executor(ctx, map[string]interface{}{
		"path": "hello.py",
	})
	if err != nil || !strings.Contains(readOut, "openforge") {
		t.Fatalf("read_file after edit failed, out=%q err=%v", readOut, err)
	}

	lsOut, err := reg["ls"].Executor(ctx, map[string]interface{}{
		"path": ".",
	})
	if err != nil || !strings.Contains(strings.ToLower(lsOut), "hello.py") {
		t.Fatalf("ls failed, out=%q err=%v", lsOut, err)
	}
}
