package main

// Item is one openHAB item as the REST API returns it.
type Item struct {
	Name       string         `json:"name" jsonschema:"the item name, the identifier every other tool addresses it by"`
	Type       string         `json:"type" jsonschema:"item type, for example Switch, Number, String, Dimmer or Group"`
	State      string         `json:"state" jsonschema:"current state as openHAB reports it, for example ON, OFF, 12.4 or NULL when it has never been set"`
	Label      string         `json:"label" jsonschema:"human-readable label, often in the user's own language"`
	Category   string         `json:"category" jsonschema:"icon category the openHAB UI draws the item with, for example light or heating"`
	Tags       []string       `json:"tags" jsonschema:"semantic tags on the item, for example Switchable, Lighting or Point"`
	GroupNames []string       `json:"groupNames" jsonschema:"names of the groups this item belongs to"`
	Metadata   map[string]any `json:"metadata,omitempty" jsonschema:"metadata namespaces, present only when they were asked for"`
}

// Thing is one openHAB thing. The channels array is deliberately absent: it
// dominates the payload and no tool reports it. bridgeUID and properties are
// absent for the same reason.
type Thing struct {
	UID          string      `json:"UID" jsonschema:"the thing UID, for example astro:sun:home"`
	ThingTypeUID string      `json:"thingTypeUID" jsonschema:"the binding and thing type, for example zwave:device"`
	Label        string      `json:"label" jsonschema:"human-readable label"`
	StatusInfo   ThingStatus `json:"statusInfo" jsonschema:"the status openHAB reports for the thing"`
}

// ThingStatus is the status block openHAB reports for a thing.
type ThingStatus struct {
	Status       string `json:"status" jsonschema:"ONLINE, OFFLINE, UNINITIALIZED, INITIALIZING or REMOVED"`
	StatusDetail string `json:"statusDetail" jsonschema:"why it is in that status, for example NONE, COMMUNICATION_ERROR or HANDLER_MISSING_ERROR"`
	Description  string `json:"description" jsonschema:"what the binding says about the status, when it says anything"`
}

// Rule is one openHAB rule. Triggers, conditions and actions are omitted: the
// read-and-control surface reports rules and runs them, it does not edit them,
// and the editable flag is omitted for the same reason.
type Rule struct {
	UID         string     `json:"uid" jsonschema:"the rule UID, the identifier the rule is run by"`
	Name        string     `json:"name" jsonschema:"human-readable rule name"`
	Description string     `json:"description" jsonschema:"what the rule does, when the author wrote it down"`
	Tags        []string   `json:"tags" jsonschema:"tags on the rule, which the rule listing can filter by"`
	Status      RuleStatus `json:"status" jsonschema:"the status openHAB reports for the rule"`
}

// RuleStatus is the status block openHAB reports for a rule.
type RuleStatus struct {
	Status       string `json:"status" jsonschema:"IDLE, RUNNING or UNINITIALIZED"`
	StatusDetail string `json:"statusDetail" jsonschema:"why it is in that status, for example NONE or HANDLER_INITIALIZING_ERROR"`
}
