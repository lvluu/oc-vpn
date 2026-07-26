package worktree

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lvluu/oc-vpn/internal/config"
)

type Entry struct {
	ProjectDir string `json:"project_dir"`
	Profile    string `json:"profile"`
	CreatedAt  string `json:"created_at"`
}

type Inventory struct {
	Worktrees []Entry `json:"worktrees"`
}

func Path() string {
	return filepath.Join(config.Dir(), "worktrees.json")
}

func Load() (*Inventory, error) {
	p := Path()
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Inventory{}, nil
		}
		return nil, fmt.Errorf("reading worktrees: %w", err)
	}
	var inv Inventory
	if err := json.Unmarshal(data, &inv); err != nil {
		return nil, fmt.Errorf("parsing worktrees: %w", err)
	}
	return &inv, nil
}

func (inv *Inventory) Save() error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func (inv *Inventory) Add(projectDir, profile string) error {
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return err
	}
	for _, e := range inv.Worktrees {
		if e.ProjectDir == abs {
			return fmt.Errorf("worktree already exists for %s (profile: %s)", abs, e.Profile)
		}
	}
	inv.Worktrees = append(inv.Worktrees, Entry{
		ProjectDir: abs,
		Profile:    profile,
		CreatedAt:  time.Now().Format(time.RFC3339),
	})
	return nil
}

func (inv *Inventory) Remove(projectDir string) bool {
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return false
	}
	for i, e := range inv.Worktrees {
		if e.ProjectDir == abs {
			inv.Worktrees = append(inv.Worktrees[:i], inv.Worktrees[i+1:]...)
			return true
		}
	}
	return false
}

func (inv *Inventory) Lookup(projectDir string) (Entry, bool) {
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return Entry{}, false
	}
	for _, e := range inv.Worktrees {
		if e.ProjectDir == abs {
			return e, true
		}
	}
	return Entry{}, false
}
