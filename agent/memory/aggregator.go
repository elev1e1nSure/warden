package memory

import (
	"sort"
	"time"
)

func Aggregate(store *MemoryStore, sessionID string) map[string]interface{} {
	entries, err := store.GetEntries(sessionID, "")
	if err != nil {
		entries = nil
	}

	result := map[string]interface{}{
		"user":        map[string]string{},
		"projects":    []map[string]interface{}{},
		"preferences": map[string]string{},
		"updated_at":  time.Now().UTC().Format(time.RFC3339),
	}

	user := make(map[string]string)
	prefs := make(map[string]string)
	projects := make(map[string]map[string]interface{})
	techStack := make(map[string]bool)

	for _, e := range entries {
		switch e.Category {
		case "user":
			user[e.Key] = e.Value
		case "preference":
			prefs[e.Key] = e.Value
		case "project":
			if _, ok := projects[e.Key]; !ok {
				projects[e.Key] = map[string]interface{}{"name": e.Value}
			} else {
				projects[e.Key]["name"] = e.Value
			}
		case "tech_stack":
			techStack[e.Value] = true
		}
	}

	result["user"] = user
	result["preferences"] = prefs

	projectList := make([]map[string]interface{}, 0, len(projects))
	for _, p := range projects {
		projectList = append(projectList, p)
	}

	var globalTech []string
	for t := range techStack {
		globalTech = append(globalTech, t)
	}
	sort.Strings(globalTech)

	if len(projectList) > 0 && len(globalTech) > 0 {
		projectList[len(projectList)-1]["tech_stack"] = globalTech
	} else if len(globalTech) > 0 {
		result["tech_stack"] = globalTech
	}

	result["projects"] = projectList
	return result
}

func Finalize(store *MemoryStore, sessionID string) error {
	snapshot := Aggregate(store, sessionID)
	prev, _ := store.GetLatestSnapshot()
	if prev != nil {
		snapshot = mergeSnapshots(prev, snapshot)
	}
	if err := store.SaveSnapshot(sessionID, snapshot); err != nil {
		return err
	}
	store.ClearEntries(sessionID)
	return nil
}

func mergeSnapshots(prev, curr map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range prev {
		result[k] = v
	}
	for k, v := range curr {
		result[k] = v
	}
	result["updated_at"] = curr["updated_at"]

	// Merge user
	prevUser := mapStringMap(prev, "user")
	currUser := mapStringMap(curr, "user")
	mergedUser := make(map[string]string)
	for k, v := range prevUser {
		mergedUser[k] = v
	}
	for k, v := range currUser {
		mergedUser[k] = v
	}
	result["user"] = mergedUser

	// Merge preferences
	prevPrefs := mapStringMap(prev, "preferences")
	currPrefs := mapStringMap(curr, "preferences")
	mergedPrefs := make(map[string]string)
	for k, v := range prevPrefs {
		mergedPrefs[k] = v
	}
	for k, v := range currPrefs {
		mergedPrefs[k] = v
	}
	result["preferences"] = mergedPrefs

	// Merge projects by name
	prevProjects := extractProjects(prev)
	currProjects := extractProjects(curr)
	mergedProjects := make(map[string]map[string]interface{})
	for _, p := range prevProjects {
		if name, ok := p["name"].(string); ok {
			mergedProjects[name] = p
		}
	}
	for _, p := range currProjects {
		if name, ok := p["name"].(string); ok {
			mergedProjects[name] = p
		}
	}
	projList := make([]map[string]interface{}, 0, len(mergedProjects))
	for _, p := range mergedProjects {
		projList = append(projList, p)
	}
	result["projects"] = projList

	// Merge tech_stack
	allTech := make(map[string]bool)
	for _, t := range stringSlice(prev["tech_stack"]) {
		allTech[t] = true
	}
	for _, t := range stringSlice(curr["tech_stack"]) {
		allTech[t] = true
	}
	for _, p := range prevProjects {
		if ts, ok := p["tech_stack"].([]interface{}); ok {
			for _, t := range stringSlice(ts) {
				allTech[t] = true
			}
		}
	}
	for _, p := range currProjects {
		if ts, ok := p["tech_stack"].([]interface{}); ok {
			for _, t := range stringSlice(ts) {
				allTech[t] = true
			}
		}
	}
	if len(allTech) > 0 {
		sorted := make([]string, 0, len(allTech))
		for t := range allTech {
			sorted = append(sorted, t)
		}
		sort.Strings(sorted)
		result["tech_stack"] = sorted
	} else {
		delete(result, "tech_stack")
	}

	return result
}

func mapStringMap(m map[string]interface{}, key string) map[string]string {
	out := make(map[string]string)
	if raw, ok := m[key]; ok {
		if sm, ok := raw.(map[string]interface{}); ok {
			for k, v := range sm {
				if vs, ok := v.(string); ok {
					out[k] = vs
				}
			}
		}
	}
	return out
}

func extractProjects(m map[string]interface{}) []map[string]interface{} {
	var out []map[string]interface{}
	if raw, ok := m["projects"]; ok {
		if list, ok := raw.([]interface{}); ok {
			for _, p := range list {
				if pm, ok := p.(map[string]interface{}); ok {
					out = append(out, pm)
				}
			}
		}
	}
	return out
}

func stringSlice(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		var out []string
		for _, s := range t {
			if str, ok := s.(string); ok {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}
