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
