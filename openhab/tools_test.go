package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// bareToolNames lists every tool this server registers, without a prefix.
// A description that names a sibling tool must render it with the
// configured prefix, since that is the only name the caller can actually
// invoke.
var bareToolNames = []string{
	"list_items", "get_item", "send_command", "update_item_state",
	"list_things", "get_thing_status", "list_rules", "run_rule_now",
}

// listedTools drives a real MCP session against the registered tools and
// returns the tools/list payload exactly as a client receives it. Sweeping the
// wire payload rather than the toolSpec list covers every string the model
// reads: the tool descriptions, and the input and output schema field
// descriptions the SDK infers from the jsonschema struct tags.
func listedTools(cfg Config) string {
	srv := newServer(cfg, &stubClient{})
	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "openhab", Version: "spec"}, nil)
	for _, spec := range srv.toolSpecs() {
		spec.add(mcpServer)
	}

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := mcpServer.Connect(ctx, serverTransport, nil)
	Expect(err).ToNot(HaveOccurred())
	defer serverSession.Close()

	clientSession, err := mcp.NewClient(
		&mcp.Implementation{Name: "spec", Version: "spec"}, nil).Connect(ctx, clientTransport, nil)
	Expect(err).ToNot(HaveOccurred())
	defer clientSession.Close()

	result, err := clientSession.ListTools(ctx, nil)
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Tools).To(HaveLen(len(bareToolNames)))

	payload, err := json.Marshal(result.Tools)
	Expect(err).ToNot(HaveOccurred())
	return string(payload)
}

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

	It("should never reference a sibling tool by its bare, unprefixed name", func() {
		// The empty-prefix case is not exercised here: with no prefix the
		// bare names ARE the registered names, so the assertion below would
		// be vacuous for it.
		for _, prefix := range []string{"openhab_", "house_"} {
			payload := listedTools(Config{ToolPrefix: prefix})
			for _, name := range bareToolNames {
				// Strip every correctly prefixed occurrence first; whatever
				// bare name remains after that is a cross-reference a model
				// could not actually call under this configuration.
				stripped := strings.ReplaceAll(payload, prefix+name, "")
				Expect(stripped).ToNot(ContainSubstring(name),
					"prefix %q: the tools/list payload references %q without the configured prefix", prefix, name)
			}
		}
	})
})

var _ = Describe("error payloads", func() {
	var (
		stub *stubClient
		srv  *server
		ctx  context.Context
	)

	// A prefix other than the default, so an assertion cannot pass by
	// accident on a hardcoded "openhab_".
	BeforeEach(func() {
		stub = &stubClient{}
		srv = newServer(Config{ToolPrefix: "house_"}, stub)
		ctx = context.Background()
	})

	// namesTool asserts that message points at prefix+name and never at the
	// bare name, which is not registered under this configuration.
	namesTool := func(message, name string) {
		GinkgoHelper()
		Expect(message).To(ContainSubstring("house_" + name))
		Expect(strings.ReplaceAll(message, "house_"+name, "")).ToNot(ContainSubstring(name))
	}

	It("should point a missing item name at the prefixed listing tool", func() {
		_, out, _ := srv.getItem(ctx, nil, getItemInput{})
		namesTool(out.Error, "list_items")
	})

	It("should point an unknown item at the prefixed listing tool", func() {
		stub.err = fmt.Errorf("/rest/items/Nope: %w", errNotFound)
		_, out, _ := srv.getItem(ctx, nil, getItemInput{Name: "Nope"})
		namesTool(out.Error, "list_items")
	})

	It("should point a missing command target at the prefixed listing tool", func() {
		_, out, _ := srv.sendCommand(ctx, nil, sendCommandInput{Command: "ON"})
		namesTool(out.Error, "list_items")
	})

	It("should point an unknown command target at the prefixed listing tool", func() {
		stub.err = fmt.Errorf("/rest/items/Nope: %w", errNotFound)
		_, out, _ := srv.sendCommand(ctx, nil, sendCommandInput{Name: "Nope", Command: "ON"})
		namesTool(out.Error, "list_items")
	})

	It("should point a missing state target at the prefixed listing tool", func() {
		_, out, _ := srv.updateState(ctx, nil, updateStateInput{State: "ON"})
		namesTool(out.Error, "list_items")
	})

	It("should point a missing thing uid at the prefixed listing tool", func() {
		_, out, _ := srv.getThingStatus(ctx, nil, getThingStatusInput{})
		namesTool(out.Error, "list_things")
	})

	It("should point a missing rule uid at the prefixed listing tool", func() {
		_, out, _ := srv.runRule(ctx, nil, runRuleInput{})
		namesTool(out.Error, "list_rules")
	})

	It("should use bare names when no prefix is configured", func() {
		bare := newServer(Config{}, stub)
		_, out, _ := bare.getItem(ctx, nil, getItemInput{})
		Expect(out.Error).To(ContainSubstring("list_items"))
	})
})
