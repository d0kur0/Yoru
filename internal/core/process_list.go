package core

import "sort"

// ProcessInfo deliberately excludes command lines, which can contain credentials.
type ProcessInfo struct {
	PID  int    `json:"pid"`
	Name string `json:"name"`
	Path string `json:"path"`
}

func sortProcesses(items []ProcessInfo) []ProcessInfo {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].PID < items[j].PID
		}
		return items[i].Name < items[j].Name
	})
	return items
}
