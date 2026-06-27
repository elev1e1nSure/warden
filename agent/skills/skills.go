package skills

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxSkillBytes = 64 * 1024

var nameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
var frontmatterRe = regexp.MustCompile(`\A---\s*\n(.*?)\n---\s*\n(.*)`)

type Skill struct {
	Name        string
	Description string
	Content     string
	Location    string
	Directory   string
}

func skillRoots() []string {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	return []string{
		filepath.Join(home, ".codex", "skills"),
		filepath.Join(home, ".agents", "skills"),
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".warden", "skills"),
		filepath.Join(cwd, "skills"),
		filepath.Join(cwd, ".claude", "skills"),
		filepath.Join(cwd, ".warden", "skills"),
	}
}

func parseFrontmatter(text string) (map[string]string, string) {
	m := frontmatterRe.FindStringSubmatch(text)
	if m == nil {
		return nil, text
	}

	meta := make(map[string]string)
	for _, line := range strings.Split(m[1], "\n") {
		line = strings.TrimRight(line, "\r ")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, ":") {
			continue
		}
		key, val, _ := strings.Cut(line, ":")
		key = strings.TrimSpace(strings.ToLower(key))
		val = strings.TrimSpace(strings.Trim(strings.Trim(val, `"'`), `"'`))
		if key != "" && val != "" {
			meta[key] = val
		}
	}

	body := strings.TrimLeft(m[2], "\n")
	return meta, body
}

func validateName(name string) bool {
	return name != "" && len(name) <= 64 && nameRe.MatchString(name)
}

func parseSkillFile(skillMd string) *Skill {
	data, err := os.ReadFile(skillMd)
	if err != nil {
		return nil
	}

	text := string(data)
	if len(data) > maxSkillBytes {
		data = data[:maxSkillBytes]
		text = string(data)
	}

	meta, body := parseFrontmatter(text)

	dirName := filepath.Base(filepath.Dir(skillMd))
	fmName := strings.TrimSpace(meta["name"])

	name := dirName
	if validateName(fmName) {
		name = fmName
	}

	if !validateName(name) {
		return nil
	}

	description := strings.TrimSpace(meta["description"])

	content := strings.TrimRight(body, "\r\n") + "\n"

	absPath, _ := filepath.Abs(skillMd)
	absDir, _ := filepath.Abs(filepath.Dir(skillMd))

	return &Skill{
		Name:        name,
		Description: description,
		Content:     content,
		Location:    absPath,
		Directory:   absDir,
	}
}

func DiscoverSkills() []Skill {
	byName := make(map[string]*Skill)

	for _, root := range skillRoots() {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}

		// Sort for deterministic order
		sort.Slice(entries, func(i, j int) bool {
			return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
		})

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillMd := filepath.Join(root, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillMd); os.IsNotExist(err) {
				continue
			}

			parsed := parseSkillFile(skillMd)
			if parsed == nil {
				continue
			}

			byName[parsed.Name] = parsed
		}
	}

	result := make([]Skill, 0, len(byName))
	for _, s := range byName {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func FindSkill(name string) *Skill {
	if !validateName(name) {
		return nil
	}
	for _, s := range DiscoverSkills() {
		if s.Name == name {
			return &s
		}
	}
	return nil
}

func ListSiblings(skill *Skill, limit int) []string {
	var out []string
	entries, err := os.ReadDir(skill.Directory)
	if err != nil {
		return out
	}

	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	for _, e := range entries {
		if e.Name() == "SKILL.md" {
			continue
		}
		if e.IsDir() {
			out = append(out, e.Name()+"/")
		} else {
			out = append(out, e.Name())
		}
		if len(out) >= limit {
			break
		}
	}
	return out
}

func FormatCatalog(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var lines []string
	lines = append(lines, "Skills provide specialized instructions and workflows for specific tasks.")
	lines = append(lines, "Use the skill tool to load a skill when a task matches its description.")
	lines = append(lines, "<available_skills>")
	for _, s := range skills {
		if s.Description == "" {
			continue
		}
		lines = append(lines, "  <skill>")
		lines = append(lines, "    <name>"+s.Name+"</name>")
		lines = append(lines, "    <description>"+s.Description+"</description>")
		lines = append(lines, "  </skill>")
	}
	lines = append(lines, "</available_skills>")
	return strings.Join(lines, "\n")
}

func WrapSkillContent(skill *Skill) string {
	files := ListSiblings(skill, 10)
	filesBlock := "(no extra files)"
	if len(files) > 0 {
		filesBlock = strings.Join(files, "\n")
	}

	var b strings.Builder
	b.WriteString(`<skill_content name="` + skill.Name + `">`)
	b.WriteString("\n")
	b.WriteString("# Skill: " + skill.Name + "\n")
	b.WriteString("\n")
	b.WriteString(strings.TrimRight(skill.Content, "\n") + "\n")
	b.WriteString("\n")
	b.WriteString("Base directory for this skill: " + skill.Directory + "\n")
	b.WriteString("Relative paths in this skill (e.g., scripts/, references/) are relative to this base directory.\n")
	b.WriteString("Note: file list is sampled.\n")
	b.WriteString("\n")
	b.WriteString("<skill_files>\n")
	b.WriteString(filesBlock + "\n")
	b.WriteString("</skill_files>\n")
	b.WriteString("</skill_content>")

	return b.String()
}
