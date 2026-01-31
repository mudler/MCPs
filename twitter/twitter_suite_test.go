package main

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTwitter(t *testing.T) {
	if os.Getenv("TWITTER_ACCEPTANCE") != "true" {
		t.Skip("Twitter acceptance tests are disabled. Set TWITTER_ACCEPTANCE=true and Twitter env credentials to run.")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Twitter acceptance")
}
