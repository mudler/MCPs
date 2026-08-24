# Running the samba integration specs

The unit specs need nothing: `go test ./samba/`. They drive the tool handlers
against a temporary local directory and skip the four integration specs.

The integration specs talk to a real SMB server. **Point them at a throwaway
server only** — they create, move and delete files, so never aim them at a NAS
you care about.

Start a disposable Samba container:

```bash
mkdir -p /tmp/sambaroot && chmod 777 /tmp/sambaroot
docker run -d --name mcps-samba-test -p 13445:445 \
  -v /tmp/sambaroot:/share \
  dperson/samba -p \
  -u "smbtest;smbtest" \
  -s "testshare;/share;yes;no;no;smbtest"
```

Run the specs against it:

```bash
SMB_TEST_ADDRESS=127.0.0.1:13445 \
SMB_TEST_SHARE=testshare \
SMB_TEST_USER=smbtest \
SMB_TEST_PASSWORD=smbtest \
go test ./samba/ -v
```

Tear it down:

```bash
docker rm -f mcps-samba-test && rm -rf /tmp/sambaroot
```

Each integration spec works inside a scratch directory named after the Ginkgo
random seed and removes it afterwards, so repeated runs do not pile up.
