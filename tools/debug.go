package tools

import (
	"fmt"
	"strings"
)

var DebugTrace = &ToolDef{
	Name:        "debug_trace",
	Description: "Control execution trace logging. Tracks tool calls, arguments, results, and timing. Actions: on (enable), off (disable), dump (show trace), clear (reset trace)",
	Args: []ToolArg{
		{Name: "action", Description: "Action: 'on', 'off', 'dump', or 'clear'", Required: true},
	},
	ExecuteWithContext: func(args map[string]string, senderID string) string {
		action := strings.TrimSpace(args["action"])
		if action == "" {
			return "Error: action is required (on/off/dump/clear)"
		}

		// Get session from global map
		session := GetDebugSession(senderID)
		if session == nil {
			return "Error: no active session"
		}

		action = strings.ToLower(action)
		switch action {
		case "on":
			session.SetDebugMode(true)
			return "Debug trace enabled. Tool calls will be recorded."

		case "off":
			session.SetDebugMode(false)
			return "Debug trace disabled."

		case "dump":
			return session.DumpTrace()

		case "clear":
			session.ClearTrace()
			return "Trace log cleared."

		default:
			return fmt.Sprintf("Error: unknown action %q (use: on, off, dump, clear)", action)
		}
	},
}

// GetDebugSession retrieves the agent session for debug operations
// This will be wired in core/register.go
var GetDebugSession func(senderID string) DebugSessionInterface

type DebugSessionInterface interface {
	SetDebugMode(enabled bool)
	DumpTrace() string
	ClearTrace()
}
