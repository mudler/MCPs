package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCua(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CUA Suite")
}
