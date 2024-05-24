package gptscript

// Options represents options for the gptscript tool or file.
type Options struct {
	Input         string `json:"input"`
	DisableCache  bool   `json:"disableCache"`
	CacheDir      string `json:"cacheDir"`
	Quiet         bool   `json:"quiet"`
	SubTool       string `json:"subTool"`
	Workspace     string `json:"workspace"`
	ChatState     string `json:"chatState"`
	IncludeEvents bool   `json:"includeEvents"`
}
