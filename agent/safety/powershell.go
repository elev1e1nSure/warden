package safety

import (
	"regexp"
	"strings"
)

var (
	deleteAliases = map[string]bool{
		"rm": true, "del": true, "erase": true, "rmdir": true,
		"rd": true, "ri": true, "remove-item": true,
	}
	killAliases = map[string]bool{
		"kill": true, "spps": true, "stop-process": true,
	}
	invokeAliases = map[string]bool{
		"iex": true, "invoke-expression": true,
	}
	blockedCmdlets = map[string]bool{
		"remove-item": true, "ri": true, "rmdir": true,
		"del": true, "erase": true, "rd": true,
		"stop-process": true, "spps": true, "kill": true, "taskkill": true,
		"format": true, "mkfs": true, "diskpart": true,
		"set-service": true, "sc": true, "sc.exe": true,
		"new-service": true, "remove-service": true,
		"reg": true, "reg.exe": true,
		"set-itemproperty": true, "remove-itemproperty": true, "new-itemproperty": true,
		"invoke-expression": true, "iex": true, "invoke-command": true,
		"clear-content": true, "clc": true, "set-executionpolicy": true,
		"netsh": true, "bcdedit": true, "cipher": true,
	}
	confirmCmdlets = map[string]bool{
		"set-content": true, "add-content": true, "out-file": true,
		"copy-item": true, "cp": true, "cpi": true,
		"move-item": true, "mv": true, "mi": true,
		"rename-item": true, "rni": true, "ren": true,
		"start-process": true, "saps": true, "start": true,
		"winget": true, "npm": true, "pnpm": true, "pip": true,
		"go": true, "git": true, "node": true, "python": true, "py": true,
	}
	safeCmdlets = map[string]bool{
		"get-childitem": true, "gci": true, "ls": true, "dir": true,
		"get-content": true, "gc": true, "cat": true, "type": true,
		"test-path": true, "resolve-path": true,
		"get-process": true, "gps": true, "ps": true,
		"get-service": true, "gsv": true,
		"get-item": true, "gi": true,
		"where-object": true, "?": true,
		"foreach-object": true, "%": true,
		"select-object": true, "sort-object": true, "measure-object": true,
		"write-output": true, "write-host": true, "write-verbose": true, "write-warning": true,
		"out-string": true, "out-null": true,
		"findstr": true, "grep": true, "rg": true, "fd": true,
	}
	packageManagers = map[string]bool{
		"winget": true, "npm": true, "pnpm": true, "pip": true,
		"uv": true, "gem": true, "cargo": true,
	}
)

var (
	encodedRe        = regexp.MustCompile(`(?i)-[eE]nc(?:oded)?[cC]ommand\b|/[eE]:\b|-enc\b|-[eE][cCnN]\b`)
	remotePipeRe     = regexp.MustCompile(`(?i)(iwr|irm|invoke-webrequest|invoke-restmethod|curl\.exe|wget\.exe)\s+.*\|\s*iex`)
	chainRe          = regexp.MustCompile("[;&\r\n]")
	contRe           = regexp.MustCompile("`\\s+")
	tokenSplitRe     = regexp.MustCompile("[\\s|;&`|]+")
	destructiveCmdRe = regexp.MustCompile(`(?i)\b(rd|rmdir|del|erase|deltree)\b.*/[fFsSqQ]\b`)
	diskFormatRe     = regexp.MustCompile(`(?i)\b(format\s+[a-z]:|mkfs|diskpart|cipher\s+/w)\b`)
	subexprRe        = regexp.MustCompile(`\$\(([^)]+)\)`)
	dynamicCmdRe     = regexp.MustCompile(`(?i)&\s*\([^)]*\+[^)]*\)`)
	shutdownRe       = regexp.MustCompile(`(?i)\bshutdown\b.*\s/[srph]\b`)
	destructiveGitRe = regexp.MustCompile(`(?i)git\s+(reset\s+--hard|clean\s+-fd|push\s+--force|push\s+-f|branch\s+-D)`)
	registryRe       = regexp.MustCompile(`(?i)\b(reg\s+(add|delete|edit)|set-itemproperty|remove-itemproperty|new-itemproperty|netsh\s+advfirewall)\b`)
	nestedCmdRe      = regexp.MustCompile(`(?i)(?:cmd\.exe|cmd)\s+/[cCkK]\s+(?:['"])?(.+?)(?:['"])?$|(?:pwsh|powershell)\s+(?:-[cC]ommand|-c)\s+['"]?(.+?)['"]?$|(?:bash|sh)\s+-c\s+['"]?(.+?)['"]?$`)
	recurseFlagRe    = regexp.MustCompile(`(?i)^-(r(?:ecurse)?|rf?|fr?)\b`)
	forceFlagRe      = regexp.MustCompile(`(?i)^-(f(?:orce)?|rf?|fr?)\b`)
	rfFlagRe         = regexp.MustCompile(`(?i)^-rf?\b`)
	frFlagRe         = regexp.MustCompile(`(?i)^-fr?\b`)
)

func normalize(command string) string {
	text := strings.ReplaceAll(command, "`\r\n", " ")
	text = strings.ReplaceAll(text, "`\n", " ")
	text = strings.ReplaceAll(text, "`\r", " ")
	text = contRe.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func tokens(command string) []string {
	parts := tokenSplitRe.Split(command, -1)
	var result []string
	for _, t := range parts {
		t = strings.Trim(t, "\t\r\n'\"")
		if t != "" {
			result = append(result, strings.ToLower(t))
		}
	}
	return result
}

func hasAny(tokens []string, candidates map[string]bool) bool {
	for _, t := range tokens {
		if candidates[t] {
			return true
		}
	}
	return false
}

func hasAnyIn(tokens []string, maps ...map[string]bool) bool {
	for _, t := range tokens {
		for _, m := range maps {
			if m[t] {
				return true
			}
		}
	}
	return false
}

func classify(command string) (risk string, reason string, details []string) {
	norm := normalize(command)
	toks := tokens(norm)

	if encodedRe.MatchString(norm) {
		return "blocked", "encoded command execution", []string{"uses -EncodedCommand or similar"}
	}

	if remotePipeRe.MatchString(norm) {
		return "blocked", "remote script execution via iex", []string{"downloads remote content and executes it"}
	}

	if matches := nestedCmdRe.FindStringSubmatch(norm); matches != nil {
		for _, g := range matches[1:] {
			if g != "" {
				return classify(g)
			}
		}
	}

	if destructiveCmdRe.MatchString(norm) {
		return "blocked", "destructive cmd.exe command", []string{"uses cmd-style delete with force/recurse flags"}
	}

	if diskFormatRe.MatchString(norm) {
		return "blocked", "disk destruction command", []string{"can erase drives or volumes"}
	}

	for _, match := range subexprRe.FindAllStringSubmatch(norm, -1) {
		subToks := tokens(match[1])
		if hasAnyIn(subToks, blockedCmdlets, deleteAliases, killAliases, invokeAliases) {
			return "blocked", "blocked command inside subexpression", []string{"$(...) contains restricted cmdlet"}
		}
	}

	if dynamicCmdRe.MatchString(norm) {
		return "blocked", "dynamic command construction", []string{"& with string concatenation can bypass safety filters"}
	}

	if shutdownRe.MatchString(norm) {
		return "blocked", "system power command", []string{"shuts down, restarts, or powers off the machine"}
	}

	if destructiveGitRe.MatchString(norm) {
		return "blocked", "destructive git command", []string{"matched destructive git operation"}
	}

	if registryRe.MatchString(norm) {
		return "blocked", "system/registry modification", []string{"changes system configuration"}
	}

	if chainRe.MatchString(norm) {
		return "confirm", "chained command", []string{"contains command chains (;/&)"}
	}

	exe := ""
	rest := []string{}
	if len(toks) > 0 {
		exe = toks[0]
		rest = toks[1:]
	}

	if deleteAliases[exe] {
		hasRecurse, hasForce := false, false
		for _, t := range toks {
			if recurseFlagRe.MatchString(t) {
				hasRecurse = true
			}
			if forceFlagRe.MatchString(t) {
				hasForce = true
			}
		}
		for _, t := range toks {
			if rfFlagRe.MatchString(t) || frFlagRe.MatchString(t) {
				hasRecurse = true
				hasForce = true
				break
			}
		}
		if hasRecurse && hasForce {
			return "blocked", "recursive forced deletion", []string{"uses -Recurse and -Force on delete"}
		}
		return "confirm", "file deletion", []string{"deletes files or directories"}
	}

	if killAliases[exe] {
		return "confirm", "process termination", []string{"stops a running process"}
	}

	for _, t := range toks {
		if t == "taskkill" {
			return "confirm", "process termination", []string{"stops a running process"}
		}
	}

	if invokeAliases[exe] {
		return "blocked", "code evaluation", []string{"Invoke-Expression can execute arbitrary code"}
	}

	if blockedCmdlets[exe] || hasAny(toks, blockedCmdlets) {
		return "blocked", "restricted system command", []string{"command '" + exe + "' is blocked in leashed mode"}
	}

	if packageManagers[exe] {
		return "confirm", "package installation or modification", []string{"uses " + exe}
	}

	if exe == "git" {
		sub := ""
		if len(rest) > 0 {
			sub = rest[0]
		}
		switch sub {
		case "status", "diff", "log", "show", "branch", "tag",
			"config", "remote", "stash", "ls-files":
			return "safe", "read-only git command", []string{"git " + sub}
		}
		return "confirm", "git command", []string{"git may change repository state"}
	}

	if exe == "go" {
		sub := ""
		if len(rest) > 0 {
			sub = rest[0]
		}
		switch sub {
		case "test", "fmt", "vet", "env", "version", "mod", "list", "doc":
			return "safe", "read-only go command", []string{"go " + sub}
		}
		return "confirm", "go command", []string{"go may change project state"}
	}

	if exe == "python" || exe == "py" {
		if hasAny(toks, map[string]bool{"-m": true}) && hasAny(toks, map[string]bool{"py_compile": true}) {
			return "safe", "read-only python check", []string{"py_compile"}
		}
		return "confirm", "python execution", []string{"python may execute arbitrary code"}
	}

	if confirmCmdlets[exe] || hasAny(toks, confirmCmdlets) {
		return "confirm", "file or system modification", []string{"command '" + exe + "' changes state"}
	}

	if safeCmdlets[exe] || hasAny(toks, safeCmdlets) {
		return "safe", "read-only command", []string{}
	}

	return "confirm", "unknown command", []string{"no safety policy defined for this command"}
}
