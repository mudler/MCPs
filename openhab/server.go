package main

// server carries the configuration and the openHAB client shared by every tool
// handler.
type server struct {
	cfg    Config
	client openHABClient
}

func newServer(cfg Config, client openHABClient) *server {
	return &server{cfg: cfg, client: client}
}

// tool renders one of this server's own tool names as a caller sees it. Every
// message that points a model at a sibling tool goes through here, so it names
// a tool that actually exists under the configured prefix rather than the bare
// name this package happens to register it under.
func (s *server) tool(name string) string { return s.cfg.ToolPrefix + name }
