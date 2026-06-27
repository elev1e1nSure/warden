package client

type StatusResult struct {
	Model      string `json:"model"`
	Mode       string `json:"mode"`
	CWD        string `json:"cwd"`
	TokenCount int    `json:"token_count"`
	TokenLimit int    `json:"token_limit"`
}

type CompactResult struct {
	Summary      string `json:"summary"`
	TokensBefore int    `json:"tokens_before"`
	TokensAfter  int    `json:"tokens_after"`
}

type MemoryState struct {
	Enabled   bool `json:"enabled"`
	Entries   int  `json:"entries"`
	Snapshots int  `json:"snapshots"`
	DBSize    int  `json:"db_size"`
}

type TokenMsg struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolMsg struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Args   string `json:"args"`
	Result string `json:"result"`
	Diff   string `json:"diff"`
}

type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Location    string `json:"location"`
}
