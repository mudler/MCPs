package main

import (
	"database/sql"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	_ "modernc.org/sqlite"
)

func buildGlodaFixture() *Gloda {
	tmp, err := os.CreateTemp("", "gloda-*.sqlite")
	Expect(err).NotTo(HaveOccurred())
	tmp.Close()
	seed, err := os.ReadFile("testdata/gloda_seed.sql")
	Expect(err).NotTo(HaveOccurred())
	db, err := sql.Open("sqlite", "file:"+tmp.Name())
	Expect(err).NotTo(HaveOccurred())
	_, err = db.Exec(string(seed))
	Expect(err).NotTo(HaveOccurred())
	db.Close()
	g, err := openGlodaAt(tmp.Name())
	Expect(err).NotTo(HaveOccurred())
	return g
}

var _ = Describe("Gloda.Search", func() {
	var g *Gloda
	BeforeEach(func() { g = buildGlodaFixture() })
	AfterEach(func() { g.Close() })

	It("finds by body text and excludes deleted and ghost rows", func() {
		res, err := g.Search(SearchQuery{Text: "report", Limit: 10})
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(HaveLen(1))
		Expect(res[0].Subject).To(Equal("Quarterly report"))
		Expect(res[0].Ref).To(Equal("imap://alice@example.com/INBOX#101"))
		Expect(res[0].Date.Year()).To(Equal(2023))
	})
	It("filters by folder", func() {
		res, err := g.Search(SearchQuery{FolderURI: "imap://alice@example.com/Archive", Limit: 10})
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(HaveLen(1))
		Expect(res[0].Subject).To(Equal("Archive note"))
	})
	It("filters by from", func() {
		res, err := g.Search(SearchQuery{From: "carol", Limit: 10})
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(HaveLen(1))
		Expect(res[0].Author).To(Equal("carol@example.com"))
	})
	It("orders recent newest-first and never returns deleted/ghost", func() {
		res, err := g.Recent("", 10)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(HaveLen(3))
		Expect(res[0].Subject).To(Equal("Lunch")) // 1700100000000000 newest of the valid rows
	})
})
