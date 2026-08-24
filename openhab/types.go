package main

// Item is one openHAB item as the REST API returns it.
type Item struct {
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	State      string         `json:"state"`
	Label      string         `json:"label"`
	Category   string         `json:"category"`
	Tags       []string       `json:"tags"`
	GroupNames []string       `json:"groupNames"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// Thing is one openHAB thing. The channels array is deliberately absent: it
// dominates the payload and no tool reports it.
type Thing struct {
	UID          string         `json:"UID"`
	ThingTypeUID string         `json:"thingTypeUID"`
	Label        string         `json:"label"`
	BridgeUID    string         `json:"bridgeUID"`
	StatusInfo   ThingStatus    `json:"statusInfo"`
	Properties   map[string]any `json:"properties"`
}

// ThingStatus is the status block openHAB reports for a thing.
type ThingStatus struct {
	Status       string `json:"status"`
	StatusDetail string `json:"statusDetail"`
	Description  string `json:"description"`
}

// Rule is one openHAB rule. Triggers, conditions and actions are omitted: the
// read-and-control surface reports rules and runs them, it does not edit them.
type Rule struct {
	UID         string     `json:"uid"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Tags        []string   `json:"tags"`
	Status      RuleStatus `json:"status"`
	Editable    bool       `json:"editable"`
}

// RuleStatus is the status block openHAB reports for a rule.
type RuleStatus struct {
	Status       string `json:"status"`
	StatusDetail string `json:"statusDetail"`
}
