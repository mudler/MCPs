package main

import "context"

// stubClient is an openHABClient whose answers the specs set directly.
type stubClient struct {
	items       []Item
	item        Item
	things      []Thing
	thingStatus ThingStatus
	rules       []Rule

	err error

	// Recorded calls.
	commanded  [][2]string
	stated     [][2]string
	ranRules   []string
	ruleTag    string
	wantedMeta bool
}

func (s *stubClient) Items(context.Context) ([]Item, error) {
	return s.items, s.err
}

func (s *stubClient) Item(_ context.Context, _ string, withMetadata bool) (Item, error) {
	s.wantedMeta = withMetadata
	return s.item, s.err
}

func (s *stubClient) SendCommand(_ context.Context, name, command string) error {
	s.commanded = append(s.commanded, [2]string{name, command})
	return s.err
}

func (s *stubClient) UpdateState(_ context.Context, name, state string) error {
	s.stated = append(s.stated, [2]string{name, state})
	return s.err
}

func (s *stubClient) Things(context.Context) ([]Thing, error) {
	return s.things, s.err
}

func (s *stubClient) ThingStatus(context.Context, string) (ThingStatus, error) {
	return s.thingStatus, s.err
}

func (s *stubClient) Rules(_ context.Context, tag string) ([]Rule, error) {
	s.ruleTag = tag
	return s.rules, s.err
}

func (s *stubClient) RunRule(_ context.Context, uid string) error {
	s.ranRules = append(s.ranRules, uid)
	return s.err
}
