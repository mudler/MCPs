package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parseToolFilter", func() {
	It("returns nil for empty or 'all'", func() {
		Expect(parseToolFilter("")).To(BeNil())
		Expect(parseToolFilter("all")).To(BeNil())
	})
	It("parses a comma list trimming spaces", func() {
		got := parseToolFilter("search_messages, get_message ,list_folders")
		Expect(got).To(Equal(map[string]bool{
			"search_messages": true, "get_message": true, "list_folders": true,
		}))
	})
})
