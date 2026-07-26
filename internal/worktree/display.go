package worktree

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lvluu/oc-vpn/internal/profiles"
)

// ProfileGroup holds a profile and its linked worktree entries.
type ProfileGroup struct {
	Profile    string
	Projects   []string
	EntryCount int
}

// ByProfile groups worktree entries by profile and enriches with profile info.
func ByProfile() ([]ProfileGroup, error) {
	inv, err := Load()
	if err != nil {
		return nil, err
	}

	profileNames := profiles.List()
	groupMap := make(map[string]*ProfileGroup)
	for _, n := range profileNames {
		groupMap[n] = &ProfileGroup{Profile: n}
	}
	for _, e := range inv.Worktrees {
		g, ok := groupMap[e.Profile]
		if !ok {
			g = &ProfileGroup{Profile: e.Profile}
			groupMap[e.Profile] = g
		}
		g.Projects = append(g.Projects, filepath.Base(e.ProjectDir))
		g.EntryCount++
	}
	groups := make([]ProfileGroup, 0, len(profileNames))
	for _, n := range profileNames {
		if g, ok := groupMap[n]; ok {
			groups = append(groups, *g)
		} else {
			groups = append(groups, ProfileGroup{Profile: n})
		}
	}
	return groups, nil
}

// FormatOne returns a one-line summary for a single profile group.
func FormatOne(g ProfileGroup) string {
	if g.EntryCount == 0 {
		return fmt.Sprintf("%s (no worktrees)", g.Profile)
	}
	projects := strings.Join(g.Projects, ", ")
	return fmt.Sprintf("%s (%s [%d worktrees])", g.Profile, projects, g.EntryCount)
}

// FormatAll returns a multi-line summary of all profiles with their worktrees.
func FormatAll(groups []ProfileGroup) string {
	var b strings.Builder
	for _, g := range groups {
		b.WriteString("  " + FormatOne(g) + "\n")
	}
	return b.String()
}

// ProfileChoices returns display strings for the TUI picker.
func ProfileChoices(groups []ProfileGroup) []string {
	choices := make([]string, len(groups))
	for i, g := range groups {
		choices[i] = FormatOne(g)
	}
	return choices
}
