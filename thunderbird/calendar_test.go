package main

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Calendar.Events", func() {
	It("returns events in a range with location and converts microseconds", func() {
		c, err := openCalendarAt(seedDB("testdata/calendar_seed.sql"), map[string]string{"cal-uuid-1": "Home"})
		Expect(err).NotTo(HaveOccurred())
		defer c.Close()
		evs, err := c.Events(time.Unix(1699000000, 0), time.Unix(1701000000, 0), 10)
		Expect(err).NotTo(HaveOccurred())
		Expect(evs).To(HaveLen(2))
		Expect(evs[0].Title).To(Equal("Standup"))
		Expect(evs[0].Location).To(Equal("Room A"))
		Expect(evs[0].Calendar).To(Equal("Home"))
		Expect(evs[0].Start.Year()).To(Equal(2023))
	})
})
