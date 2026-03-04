package tools

import (
	"fmt"
)

// DeepWork enables extended execution mode for complex multi-step tasks
var DeepWork = &ToolDef{
	Name:        "deep_work",
	Description: "Enter deep work mode for complex multi-step tasks. Raises iteration limit to 50. Call this FIRST for tasks needing many steps (deploying, installing, browser workflows, etc.). Afterward, just work naturally - no need to report progress manually.",
	Args: []ToolArg{
		{Name: "plan", Description: "Brief plan of steps you will execute", Required: true},
		{Name: "max_steps", Description: "Estimated tool calls needed (default: 30, max: 50)", Required: false},
	},
	Sequential: true,
	ExecuteWithContext: func(args map[string]string, senderID string) string {
		plan := args["plan"]
		if plan == "" {
			return "Error: plan is required"
		}

		maxSteps := 30
		if ms := args["max_steps"]; ms != "" {
			fmt.Sscanf(ms, "%d", &maxSteps)
		}
		if maxSteps < 5 {
			maxSteps = 5
		}
		if maxSteps > 50 {
			maxSteps = 50
		}

		// Deep work activated - iteration limit raised, natural progress reporting enabled
		return fmt.Sprintf("🚀 Deep work mode activated\n\nPlan: %s\n\nMax steps: %d\n\nI'll work through this and report only major progress milestones naturally (e.g., 'I'm installing the CLI', 'CLI is ready', 'Deployment complete'). No spam, just important updates.", plan, maxSteps)
	},
}
