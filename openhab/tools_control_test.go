package main

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("item control tools", func() {
	var (
		stub *stubClient
		srv  *server
		ctx  context.Context
	)

	BeforeEach(func() {
		stub = &stubClient{}
		srv = newServer(Config{}, stub)
		ctx = context.Background()
	})

	Describe("send_command", func() {
		It("should send the command to the named item", func() {
			_, out, err := srv.sendCommand(ctx, nil, sendCommandInput{Name: "AlarmTrigger", Command: "ON"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(stub.commanded).To(Equal([][2]string{{"AlarmTrigger", "ON"}}))
		})

		It("should require a name", func() {
			_, out, err := srv.sendCommand(ctx, nil, sendCommandInput{Command: "ON"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("name is required"))
			Expect(stub.commanded).To(BeEmpty())
		})

		It("should require a command", func() {
			_, out, err := srv.sendCommand(ctx, nil, sendCommandInput{Name: "AlarmTrigger"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("command is required"))
			Expect(stub.commanded).To(BeEmpty())
		})

		It("should say plainly when the item does not exist", func() {
			stub.err = fmt.Errorf("/rest/items/Nope: %w", errNotFound)
			_, out, err := srv.sendCommand(ctx, nil, sendCommandInput{Name: "Nope", Command: "ON"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring(`no item named "Nope"`))
		})

		It("should report a client failure in the payload", func() {
			stub.err = errors.New("connection refused")
			_, out, err := srv.sendCommand(ctx, nil, sendCommandInput{Name: "AlarmTrigger", Command: "ON"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("connection refused"))
		})
	})

	Describe("update_item_state", func() {
		It("should update the state of the named item", func() {
			_, out, err := srv.updateState(ctx, nil, updateStateInput{Name: "Ozone", State: "13.1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(stub.stated).To(Equal([][2]string{{"Ozone", "13.1"}}))
			Expect(stub.commanded).To(BeEmpty())
		})

		It("should require a state", func() {
			_, out, err := srv.updateState(ctx, nil, updateStateInput{Name: "Ozone"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("state is required"))
			Expect(stub.stated).To(BeEmpty())
		})
	})
})
