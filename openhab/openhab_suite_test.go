package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOpenHAB(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OpenHAB Suite")
}
