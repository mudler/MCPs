package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSamba(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Samba Suite")
}
