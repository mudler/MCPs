package main

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Permissions", func() {
	var originalEnv string

	BeforeEach(func() {
		originalEnv = os.Getenv("TODO_ADMIN_MODE")
		os.Unsetenv("TODO_ADMIN_MODE")
	})

	AfterEach(func() {
		if originalEnv != "" {
			os.Setenv("TODO_ADMIN_MODE", originalEnv)
		} else {
			os.Unsetenv("TODO_ADMIN_MODE")
		}
	})

	Context("isAdminMode", func() {
		It("should return true when TODO_ADMIN_MODE is set to true", func() {
			os.Setenv("TODO_ADMIN_MODE", "true")
			Expect(isAdminMode()).To(BeTrue())
		})

		It("should return true when TODO_ADMIN_MODE is set to TRUE", func() {
			os.Setenv("TODO_ADMIN_MODE", "TRUE")
			Expect(isAdminMode()).To(BeTrue())
		})

		It("should return true when TODO_ADMIN_MODE is set to True", func() {
			os.Setenv("TODO_ADMIN_MODE", "True")
			Expect(isAdminMode()).To(BeTrue())
		})

		It("should return false when TODO_ADMIN_MODE is not set", func() {
			Expect(isAdminMode()).To(BeFalse())
		})

		It("should return false when TODO_ADMIN_MODE is set to false", func() {
			os.Setenv("TODO_ADMIN_MODE", "false")
			Expect(isAdminMode()).To(BeFalse())
		})

		It("should return false when TODO_ADMIN_MODE is set to empty string", func() {
			os.Setenv("TODO_ADMIN_MODE", "")
			Expect(isAdminMode()).To(BeFalse())
		})
	})

})
