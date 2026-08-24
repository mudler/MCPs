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
	It("should expose all eight tools by default", func() {
		Expect(toolNames(Config{})).To(ConsistOf(
			"list_items", "get_item", "send_command", "update_item_state",
			"list_things", "get_thing_status", "list_rules", "run_rule_now",
		))
	})

	It("should drop the acting tools in read-only mode", func() {
		names := toolNames(Config{ReadOnly: true})
		Expect(names).To(ConsistOf("list_items", "get_item", "list_things", "get_thing_status", "list_rules"))
		Expect(names).ToNot(ContainElement("send_command"))
		Expect(names).ToNot(ContainElement("update_item_state"))
		Expect(names).ToNot(ContainElement("run_rule_now"))
	})

	It("should describe every tool it exposes", func() {
		srv := newServer(Config{}, &stubClient{})
		for _, spec := range srv.toolSpecs() {
			Expect(spec.description).ToNot(BeEmpty(), spec.name)
		}
	})
})
