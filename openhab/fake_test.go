package main

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/gomega"
)

// newFakeOpenHAB starts an HTTP server running handler and returns a client
// pointed at it. The caller closes the server.
func newFakeOpenHAB(handler http.HandlerFunc) (*httptest.Server, *restClient) {
	server := httptest.NewServer(handler)
	client, err := newRESTClient(Config{
		BaseURL: server.URL,
		Token:   "test-token",
		Timeout: 5 * time.Second,
	})
	Expect(err).ToNot(HaveOccurred())
	return server, client
}

// itemsJSON is a two-item response in openHAB's wire format.
const itemsJSON = `[
  {"name":"AlarmTrigger","type":"Switch","state":"OFF","label":"Sirena","tags":["Switchable"],"groupNames":[]},
  {"name":"Ozone","type":"Number","state":"12.4","label":"Ozono","tags":[],"groupNames":["Sensors"]}
]`

// thingsJSON carries a channels array, which the client must drop.
const thingsJSON = `[
  {"UID":"astro:sun:home","thingTypeUID":"astro:sun","label":"Dati Astro Sole",
   "statusInfo":{"status":"ONLINE","statusDetail":"NONE","description":""},
   "channels":[{"uid":"astro:sun:home:rise#start","id":"rise#start"}],
   "properties":{}}
]`

// rulesJSON is a one-rule response.
const rulesJSON = `[
  {"uid":"nightmode","name":"Night mode","description":"Shutters down","tags":["night"],
   "status":{"status":"IDLE","statusDetail":"NONE"},"editable":true}
]`

// encodeCertPEM wraps a DER certificate in the PEM armour a CA bundle needs.
func encodeCertPEM(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
