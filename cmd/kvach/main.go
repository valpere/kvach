// Command kvach is an AI coding agent.
//
// Run without arguments to open the interactive TUI.
// Run `kvach --help` for all available commands.
package main

import (
	// Blank-import all tool packages so their init() functions register
	// themselves with tool.DefaultRegistry before the agent starts.
	_ "github.com/valpere/kvach/internal/tool/bash"
	_ "github.com/valpere/kvach/internal/tool/edit"
	_ "github.com/valpere/kvach/internal/tool/glob"
	_ "github.com/valpere/kvach/internal/tool/grep"
	_ "github.com/valpere/kvach/internal/tool/ls"
	_ "github.com/valpere/kvach/internal/tool/multipatch"
	_ "github.com/valpere/kvach/internal/tool/question"
	_ "github.com/valpere/kvach/internal/tool/read"
	_ "github.com/valpere/kvach/internal/tool/skill"
	_ "github.com/valpere/kvach/internal/tool/task"
	_ "github.com/valpere/kvach/internal/tool/todo"
	_ "github.com/valpere/kvach/internal/tool/webfetch"
	_ "github.com/valpere/kvach/internal/tool/websearch"
	_ "github.com/valpere/kvach/internal/tool/write"

	"github.com/valpere/kvach/internal/cli"
)

func main() {
	cli.Execute()
}
