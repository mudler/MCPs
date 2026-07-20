package main

// App holds all runtime state for the Thunderbird MCP server.
// Fields are added as later tasks introduce data sources.
type App struct {
	Config    *Config
	ReadOnly  bool
	AllowSend bool
}

// app is the process-wide server state, populated in main().
var app *App
