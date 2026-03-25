package skill

import (
	"pipeline/internal/tool"

	filesystemscripts "pipeline/skills/filesystem/scripts"
	shellscripts "pipeline/skills/shell/scripts"
	websearchscripts "pipeline/skills/web-search/scripts"
)

// Providers maps skill folder names to functions that return their tools.
// When you add a new skill:
//  1. Create skills/<name>/SKILL.md and scripts/tools.go
//  2. Import the scripts package above
//  3. Add an entry here
var Providers = map[string]func() []tool.Tool{
	"filesystem": filesystemscripts.Tools,
	"shell":      shellscripts.Tools,
	"web-search": websearchscripts.Tools,
}
