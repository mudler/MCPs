package main

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("thing and rule tools", func() {
	var (
		stub *stubClient
		srv  *server
		ctx  context.Context
	)

	BeforeEach(func() {
		stub = &stubClient{
			things: []Thing{
				{UID: "astro:sun:home", ThingTypeUID: "astro:sun", Label: "Dati Astro Sole",
					StatusInfo: ThingStatus{Status: "ONLINE", StatusDetail: "NONE"}},
				{UID: "zwave:device:controller:node12", ThingTypeUID: "zwave:device", Label: "Serranda Sala",
					StatusInfo: ThingStatus{Status: "OFFLINE", StatusDetail: "COMMUNICATION_ERROR",
						Description: "Node is not responding"}},
			},
			rules: []Rule{
				{UID: "nightmode", Name: "Night mode", Tags: []string{"night"},
					Status: RuleStatus{Status: "IDLE"}},
			},
			thingStatus: ThingStatus{Status: "ONLINE", StatusDetail: "NONE"},
		}
		srv = newServer(Config{}, stub)
		ctx = context.Background()
	})

	Describe("list_things", func() {
		It("should return a compact row per thing", func() {
			_, out, err := srv.listThings(ctx, nil, listThingsInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Things).To(HaveLen(2))
			Expect(out.Things[0].UID).To(Equal("astro:sun:home"))
			Expect(out.Things[0].Status).To(Equal("ONLINE"))
			Expect(out.Things[1].StatusDetail).To(Equal("COMMUNICATION_ERROR"))
		})

		It("should filter by uid", func() {
			_, out, err := srv.listThings(ctx, nil, listThingsInput{FilterUID: "zwave"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Things).To(HaveLen(1))
			Expect(out.Things[0].UID).To(ContainSubstring("zwave"))
		})

		It("should filter by label", func() {
			_, out, err := srv.listThings(ctx, nil, listThingsInput{FilterLabel: "serranda"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Things).To(HaveLen(1))
		})

		It("should paginate", func() {
			_, out, err := srv.listThings(ctx, nil, listThingsInput{Page: 2, PageSize: 1})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Things).To(HaveLen(1))
			Expect(out.Total).To(Equal(2))
		})

		It("should return an empty page for a page number too large to multiply out", func() {
			_, out, err := srv.listThings(ctx, nil, listThingsInput{Page: hugePage})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Things).To(BeEmpty())
			Expect(out.Total).To(Equal(2))
		})

		It("should report a client failure in the payload", func() {
			stub.err = errors.New("connection refused")
			_, out, err := srv.listThings(ctx, nil, listThingsInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("connection refused"))
		})
	})

	Describe("get_thing_status", func() {
		It("should return the status", func() {
			_, out, err := srv.getThingStatus(ctx, nil, getThingStatusInput{UID: "astro:sun:home"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Status.Status).To(Equal("ONLINE"))
		})

		It("should say plainly when the thing does not exist", func() {
			stub.err = fmt.Errorf("/rest/things/nope/status: %w", errNotFound)
			_, out, err := srv.getThingStatus(ctx, nil, getThingStatusInput{UID: "nope"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring(`no thing with UID "nope"`))
			Expect(out.Error).To(ContainSubstring("list_things"))
		})

		It("should report a client failure in the payload", func() {
			stub.err = errors.New("connection refused")
			_, out, err := srv.getThingStatus(ctx, nil, getThingStatusInput{UID: "astro:sun:home"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("connection refused"))
		})

		It("should require a uid", func() {
			_, out, err := srv.getThingStatus(ctx, nil, getThingStatusInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("uid is required"))
		})
	})

	Describe("list_rules", func() {
		It("should return a compact row per rule", func() {
			_, out, err := srv.listRules(ctx, nil, listRulesInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Rules).To(HaveLen(1))
			Expect(out.Rules[0].UID).To(Equal("nightmode"))
			Expect(out.Rules[0].Status).To(Equal("IDLE"))
		})

		It("should pass the tag filter to openHAB", func() {
			_, _, err := srv.listRules(ctx, nil, listRulesInput{FilterTag: "night"})
			Expect(err).ToNot(HaveOccurred())
			Expect(stub.ruleTag).To(Equal("night"))
		})

		It("should return an empty page for a page number too large to multiply out", func() {
			_, out, err := srv.listRules(ctx, nil, listRulesInput{Page: hugePage})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Rules).To(BeEmpty())
			Expect(out.Total).To(Equal(1))
		})
	})

	Describe("run_rule_now", func() {
		It("should run the rule", func() {
			_, out, err := srv.runRule(ctx, nil, runRuleInput{UID: "nightmode"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(stub.ranRules).To(ConsistOf("nightmode"))
		})

		It("should require a uid", func() {
			_, out, err := srv.runRule(ctx, nil, runRuleInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("uid is required"))
			Expect(stub.ranRules).To(BeEmpty())
		})
	})
})
