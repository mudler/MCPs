package main

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// sampleItems is a small house: two switches, a number, a group.
func sampleItems() []Item {
	return []Item{
		{Name: "AlarmTrigger", Type: "Switch", State: "OFF", Label: "Sirena", Tags: []string{"Switchable"}},
		{Name: "AutomaticGate", Type: "Switch", State: "OFF", Label: "Apertura cancello"},
		{Name: "Ozone", Type: "Number", State: "12.4", Label: "Ozono", GroupNames: []string{"Sensors"}},
		{Name: "gMobiles", Type: "Group", State: "NULL", Label: "Telefoni"},
	}
}

var _ = Describe("item read tools", func() {
	var (
		stub *stubClient
		srv  *server
		ctx  context.Context
	)

	BeforeEach(func() {
		stub = &stubClient{items: sampleItems()}
		srv = newServer(Config{}, stub)
		ctx = context.Background()
	})

	Describe("list_items", func() {
		It("should return every item as a compact row", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Items).To(HaveLen(4))
			Expect(out.Total).To(Equal(4))
			Expect(out.Items[0].Name).To(Equal("AlarmTrigger"))
			Expect(out.Items[0].State).To(Equal("OFF"))
			Expect(out.Items[0].Label).To(Equal("Sirena"))
		})

		It("should filter by type", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{FilterType: "switch"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Items).To(HaveLen(2))
			Expect(out.Total).To(Equal(2))
		})

		It("should filter by name as a case-insensitive substring", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{FilterName: "auto"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Items).To(HaveLen(1))
			Expect(out.Items[0].Name).To(Equal("AutomaticGate"))
		})

		It("should filter by label", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{FilterLabel: "sirena"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Items).To(HaveLen(1))
			Expect(out.Items[0].Name).To(Equal("AlarmTrigger"))
		})

		It("should filter by tag", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{FilterTag: "Switchable"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Items).To(HaveLen(1))
			Expect(out.Items[0].Name).To(Equal("AlarmTrigger"))
		})

		It("should combine filters", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{FilterType: "Switch", FilterName: "gate"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Items).To(HaveLen(1))
			Expect(out.Items[0].Name).To(Equal("AutomaticGate"))
		})

		It("should paginate and report the unpaginated total", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{Page: 2, PageSize: 3})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Items).To(HaveLen(1))
			Expect(out.Total).To(Equal(4))
			Expect(out.Page).To(Equal(2))
		})

		It("should return an empty page past the end rather than an error", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{Page: 99, PageSize: 50})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Items).To(BeEmpty())
			Expect(out.Total).To(Equal(4))
		})

		It("should cap the page size", func() {
			stub.items = make([]Item, 500)
			for i := range stub.items {
				stub.items[i] = Item{Name: fmt.Sprintf("Item%d", i), Type: "Switch", State: "OFF"}
			}
			_, out, err := srv.listItems(ctx, nil, listItemsInput{PageSize: 10000})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Items).To(HaveLen(maxPageSize))
		})

		It("should reject a negative page", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{Page: -1})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("page"))
		})

		It("should report a client failure in the payload", func() {
			stub.err = errors.New("connection refused")
			_, out, err := srv.listItems(ctx, nil, listItemsInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("connection refused"))
		})
	})

	Describe("get_item", func() {
		BeforeEach(func() {
			stub.item = Item{
				Name: "AlarmTrigger", Type: "Switch", State: "OFF", Label: "Sirena",
				Tags: []string{"Switchable"},
			}
		})

		It("should return the item", func() {
			_, out, err := srv.getItem(ctx, nil, getItemInput{Name: "AlarmTrigger"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Item.Name).To(Equal("AlarmTrigger"))
			Expect(out.Item.Tags).To(ConsistOf("Switchable"))
		})

		It("should require a name", func() {
			_, out, err := srv.getItem(ctx, nil, getItemInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("name is required"))
		})

		It("should ask for metadata only when requested", func() {
			_, _, err := srv.getItem(ctx, nil, getItemInput{Name: "AlarmTrigger"})
			Expect(err).ToNot(HaveOccurred())
			Expect(stub.wantedMeta).To(BeFalse())

			_, _, err = srv.getItem(ctx, nil, getItemInput{Name: "AlarmTrigger", WithMetadata: true})
			Expect(err).ToNot(HaveOccurred())
			Expect(stub.wantedMeta).To(BeTrue())
		})

		It("should say plainly when the item does not exist", func() {
			stub.err = fmt.Errorf("/rest/items/Nope: %w", errNotFound)
			_, out, err := srv.getItem(ctx, nil, getItemInput{Name: "Nope"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring(`no item named "Nope"`))
		})
	})
})
