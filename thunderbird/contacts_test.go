package main

import (
	"database/sql"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	_ "modernc.org/sqlite"
)

func seedDB(seedFile string) string {
	tmp, _ := os.CreateTemp("", "seed-*.sqlite")
	tmp.Close()
	seed, err := os.ReadFile(seedFile)
	Expect(err).NotTo(HaveOccurred())
	db, err := sql.Open("sqlite", "file:"+tmp.Name())
	Expect(err).NotTo(HaveOccurred())
	_, err = db.Exec(string(seed))
	Expect(err).NotTo(HaveOccurred())
	db.Close()
	return tmp.Name()
}

var _ = Describe("Contacts.Search", func() {
	It("pivots properties into contacts and matches by name or email", func() {
		c, err := openContactsAt(seedDB("testdata/abook_seed.sql"))
		Expect(err).NotTo(HaveOccurred())
		defer c.Close()
		res, err := c.Search("bob", 10)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(HaveLen(1))
		Expect(res[0].Name).To(Equal("Bob Jones"))
		Expect(res[0].Email).To(Equal("bob@example.com"))
	})
})
