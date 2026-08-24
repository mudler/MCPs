package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// toolNames lists the tools a configuration exposes.
func toolNames(cfg Config) []string {
	srv := newServer(cfg, &stubClient{})
	names := []string{}
	for _, spec := range srv.toolSpecs() {
		names = append(names, spec.name)
	}
	return names
}

var _ = Describe("toolSpecs", func() {
	It("should expose all eight tools by default, each carrying the prefix", func() {
		Expect(toolNames(Config{ToolPrefix: "openhab_"})).To(ConsistOf(
			"openhab_list_items", "openhab_get_item", "openhab_send_command",
			"openhab_update_item_state", "openhab_list_things", "openhab_get_thing_status",
			"openhab_list_rules", "openhab_run_rule_now",
		))
	})

	It("should drop the acting tools in read-only mode", func() {
		names := toolNames(Config{ToolPrefix: "openhab_", ReadOnly: true})
		Expect(names).To(ConsistOf(
			"openhab_list_items", "openhab_get_item", "openhab_list_things",
			"openhab_get_thing_status", "openhab_list_rules",
		))
		Expect(names).ToNot(ContainElement("openhab_send_command"))
		Expect(names).ToNot(ContainElement("openhab_update_item_state"))
		Expect(names).ToNot(ContainElement("openhab_run_rule_now"))
	})

	It("should expose bare names when the prefix is empty", func() {
		Expect(toolNames(Config{})).To(ContainElement("get_item"))
	})

	It("should describe every tool it exposes", func() {
		srv := newServer(Config{ToolPrefix: "openhab_"}, &stubClient{})
		for _, spec := range srv.toolSpecs() {
			Expect(spec.description).ToNot(BeEmpty(), spec.name)
		}
	})
})
