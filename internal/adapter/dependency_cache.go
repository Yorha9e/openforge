package adapter

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

type DependencySpec struct {
	ProjectID    string
	Runtime      string
	LockfileHash string
}

type DependencyCache struct {
	rootDir string
}

func NewDependencyCache(rootDir string) *DependencyCache {
	return &DependencyCache{rootDir: rootDir}
}

func (dc *DependencyCache) Layer(spec DependencySpec) string {
	h := sha256.New()
	h.Write([]byte(spec.ProjectID + ":" + spec.Runtime + ":" + spec.LockfileHash))
	sum := hex.EncodeToString(h.Sum(nil))
	return filepath.Join(dc.rootDir, sum)
}

func (dc *DependencyCache) Warm(spec DependencySpec) (string, error) {
	dir := dc.Layer(spec)
	err := os.MkdirAll(dir, 0755)
	return dir, err
}
