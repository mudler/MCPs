package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestThunderbird(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Thunderbird Suite")
}
