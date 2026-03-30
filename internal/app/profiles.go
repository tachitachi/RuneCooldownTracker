package app

import (
	"fmt"
	"strings"

	"github.com/tachitachi/RuneCooldownTracker/internal/detection"
)

// slotKey encodes/decodes a SlotKey to/from the "col:row" string used in
// profile JSON.
func encodeSlotKey(k detection.SlotKey) string {
	return fmt.Sprintf("%d:%d", k.Col, k.Row)
}

func decodeSlotKey(s string) (detection.SlotKey, bool) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return detection.SlotKey{}, false
	}
	var col, row int
	if _, err := fmt.Sscan(parts[0], &col); err != nil {
		return detection.SlotKey{}, false
	}
	if _, err := fmt.Sscan(parts[1], &row); err != nil {
		return detection.SlotKey{}, false
	}
	return detection.SlotKey{Col: col, Row: row}, true
}

// currentSlotRefsAsMap returns the detector's current slot refs as a
// "col:row" → abilityName map suitable for storing in a ProfileConfig.
func (a *App) currentSlotRefsAsMap() map[string]string {
	if a.detector == nil {
		return nil
	}
	refs := a.detector.GetSlotRefs()
	if len(refs) == 0 {
		return nil
	}
	out := make(map[string]string, len(refs))
	for k, name := range refs {
		if name != "unknown" && name != "" {
			out[encodeSlotKey(k)] = name
		}
	}
	return out
}

// saveCurrentToActiveProfile updates the active profile's slot refs to match
// the detector's current state. Does nothing if there is no active profile.
func (a *App) saveCurrentToActiveProfile() {
	if a.activeProfile == "" {
		return
	}
	slotRefs := a.currentSlotRefsAsMap()
	for i := range a.profiles {
		if a.profiles[i].Name == a.activeProfile {
			a.profiles[i].SlotRefs = slotRefs
			return
		}
	}
}

// GetProfiles returns all profile names in order.
func (a *App) GetProfiles() []string {
	names := make([]string, len(a.profiles))
	for i, p := range a.profiles {
		names[i] = p.Name
	}
	return names
}

// GetActiveProfile returns the name of the currently active profile.
func (a *App) GetActiveProfile() string {
	return a.activeProfile
}

// CreateProfile creates a new profile from the current slot assignments and
// makes it the active profile. Returns an error message, or "" on success.
func (a *App) CreateProfile(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Profile name cannot be empty."
	}
	for _, p := range a.profiles {
		if p.Name == name {
			return fmt.Sprintf("A profile named %q already exists.", name)
		}
	}
	a.profiles = append(a.profiles, ProfileConfig{
		Name:     name,
		SlotRefs: a.currentSlotRefsAsMap(),
	})
	a.activeProfile = name
	a.saveConfig()
	if a.app != nil {
		a.app.Event.Emit("profile:changed", map[string]any{"active": a.activeProfile})
	}
	return ""
}

// DeleteProfile removes the named profile. If it was the active profile,
// activeProfile is cleared. Returns an error message, or "" on success.
func (a *App) DeleteProfile(name string) string {
	for i, p := range a.profiles {
		if p.Name == name {
			a.profiles = append(a.profiles[:i], a.profiles[i+1:]...)
			if a.activeProfile == name {
				a.activeProfile = ""
			}
			a.saveConfig()
			if a.app != nil {
				a.app.Event.Emit("profile:changed", map[string]any{"active": a.activeProfile})
			}
			return ""
		}
	}
	return fmt.Sprintf("Profile %q not found.", name)
}

// LoadProfile loads slot refs from the named profile, makes it the active
// profile, and persists the change. Returns an error message, or "" on success.
func (a *App) LoadProfile(name string) string {
	for _, p := range a.profiles {
		if p.Name == name {
			a.activeProfile = name
			if a.detector != nil {
				refs := make(map[detection.SlotKey]string, len(p.SlotRefs))
				for encoded, abilityName := range p.SlotRefs {
					if key, ok := decodeSlotKey(encoded); ok {
						refs[key] = abilityName
					}
				}
				a.detector.ApplySlotRefs(refs, a.refImages)
			}
			a.saveConfig()
			if a.app != nil {
				a.app.Event.Emit("profile:changed", map[string]any{"active": a.activeProfile})
				a.app.Event.Emit("combat:timeout", map[string]any{"seconds": a.GetCombatTimeout()})
			}
			return ""
		}
	}
	return fmt.Sprintf("Profile %q not found.", name)
}
