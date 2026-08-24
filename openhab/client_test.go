package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("restClient", func() {
	var ctx context.Context

	BeforeEach(func() { ctx = context.Background() })

	It("should send the bearer token", func() {
		var seen string
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			seen = r.Header.Get("Authorization")
			w.Write([]byte(itemsJSON))
		})
		defer server.Close()

		_, err := client.Items(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(seen).To(Equal("Bearer test-token"))
	})

	It("should fall back to basic auth when no token is set", func() {
		var user, pass string
		var ok bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok = r.BasicAuth()
			w.Write([]byte(itemsJSON))
		}))
		defer server.Close()

		client, err := newRESTClient(Config{
			BaseURL: server.URL, Username: "admin", Password: "hunter2", Timeout: time.Second,
		})
		Expect(err).ToNot(HaveOccurred())

		_, err = client.Items(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(user).To(Equal("admin"))
		Expect(pass).To(Equal("hunter2"))
	})

	It("should decode items", func() {
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.URL.Path).To(Equal("/rest/items"))
			w.Write([]byte(itemsJSON))
		})
		defer server.Close()

		items, err := client.Items(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(items).To(HaveLen(2))
		Expect(items[0].Name).To(Equal("AlarmTrigger"))
		Expect(items[0].State).To(Equal("OFF"))
		Expect(items[1].GroupNames).To(ConsistOf("Sensors"))
	})

	It("should request metadata only when asked", func() {
		var query string
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			query = r.URL.RawQuery
			w.Write([]byte(`{"name":"AlarmTrigger","type":"Switch","state":"OFF"}`))
		})
		defer server.Close()

		_, err := client.Item(ctx, "AlarmTrigger", false)
		Expect(err).ToNot(HaveOccurred())
		Expect(query).To(BeEmpty())

		_, err = client.Item(ctx, "AlarmTrigger", true)
		Expect(err).ToNot(HaveOccurred())
		Expect(query).To(Equal("metadata=.%2A"))
	})

	It("should escape item names in the path", func() {
		var path string
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			path = r.URL.EscapedPath()
			w.Write([]byte(`{"name":"a b","type":"Switch","state":"OFF"}`))
		})
		defer server.Close()

		_, err := client.Item(ctx, "a b", false)
		Expect(err).ToNot(HaveOccurred())
		Expect(path).To(Equal("/rest/items/a%20b"))
	})

	It("should report a 404 as errNotFound", func() {
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		defer server.Close()

		_, err := client.Item(ctx, "Nope", false)
		Expect(errors.Is(err, errNotFound)).To(BeTrue())
	})

	It("should carry the status and body on other failures", func() {
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("boom"))
		})
		defer server.Close()

		_, err := client.Items(ctx)
		Expect(err).To(MatchError(ContainSubstring("500")))
		Expect(err).To(MatchError(ContainSubstring("boom")))
	})

	It("should post a command as text/plain", func() {
		var method, contentType, body string
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			method = r.Method
			contentType = r.Header.Get("Content-Type")
			raw, _ := io.ReadAll(r.Body)
			body = string(raw)
			w.WriteHeader(http.StatusOK)
		})
		defer server.Close()

		Expect(client.SendCommand(ctx, "AlarmTrigger", "ON")).To(Succeed())
		Expect(method).To(Equal(http.MethodPost))
		Expect(contentType).To(Equal("text/plain"))
		Expect(body).To(Equal("ON"))
	})

	It("should put a state update on the state sub-resource", func() {
		var method, path, body string
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			method = r.Method
			path = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			body = string(raw)
			w.WriteHeader(http.StatusAccepted)
		})
		defer server.Close()

		Expect(client.UpdateState(ctx, "Ozone", "13.1")).To(Succeed())
		Expect(method).To(Equal(http.MethodPut))
		Expect(path).To(Equal("/rest/items/Ozone/state"))
		Expect(body).To(Equal("13.1"))
	})

	It("should decode things and ignore their channels", func() {
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(thingsJSON))
		})
		defer server.Close()

		things, err := client.Things(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(things).To(HaveLen(1))
		Expect(things[0].UID).To(Equal("astro:sun:home"))
		Expect(things[0].StatusInfo.Status).To(Equal("ONLINE"))
	})

	It("should fetch a thing status", func() {
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.URL.Path).To(Equal("/rest/things/astro:sun:home/status"))
			w.Write([]byte(`{"status":"ONLINE","statusDetail":"NONE","description":""}`))
		})
		defer server.Close()

		status, err := client.ThingStatus(ctx, "astro:sun:home")
		Expect(err).ToNot(HaveOccurred())
		Expect(status.Status).To(Equal("ONLINE"))
	})

	It("should pass a rule tag filter to openHAB", func() {
		var query string
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			query = r.URL.RawQuery
			w.Write([]byte(rulesJSON))
		})
		defer server.Close()

		rules, err := client.Rules(ctx, "night")
		Expect(err).ToNot(HaveOccurred())
		Expect(query).To(Equal("tags=night"))
		Expect(rules).To(HaveLen(1))
		Expect(rules[0].UID).To(Equal("nightmode"))
	})

	It("should run a rule", func() {
		var method, path string
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			method = r.Method
			path = r.URL.Path
			w.WriteHeader(http.StatusOK)
		})
		defer server.Close()

		Expect(client.RunRule(ctx, "nightmode")).To(Succeed())
		Expect(method).To(Equal(http.MethodPost))
		Expect(path).To(Equal("/rest/rules/nightmode/runnow"))
	})

	It("should refuse an untrusted certificate by default", func() {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(itemsJSON))
		}))
		defer server.Close()

		client, err := newRESTClient(Config{BaseURL: server.URL, Token: "t", Timeout: time.Second})
		Expect(err).ToNot(HaveOccurred())

		_, err = client.Items(ctx)
		Expect(err).To(HaveOccurred())
	})

	It("should accept an untrusted certificate when verification is skipped", func() {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(itemsJSON))
		}))
		defer server.Close()

		client, err := newRESTClient(Config{
			BaseURL: server.URL, Token: "t", Timeout: time.Second, InsecureSkipVerify: true,
		})
		Expect(err).ToNot(HaveOccurred())

		items, err := client.Items(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(items).To(HaveLen(2))
	})

	It("should accept a certificate signed by the configured CA", func() {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(itemsJSON))
		}))
		defer server.Close()

		path := filepath.Join(GinkgoT().TempDir(), "ca.pem")
		Expect(os.WriteFile(path, encodeCertPEM(server.Certificate().Raw), 0o644)).To(Succeed())

		client, err := newRESTClient(Config{
			BaseURL: server.URL, Token: "t", Timeout: time.Second, CACertPath: path,
		})
		Expect(err).ToNot(HaveOccurred())

		items, err := client.Items(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(items).To(HaveLen(2))
	})
})
