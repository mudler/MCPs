package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/pbkdf2"
)

var _ = Describe("pkcs7Unpad", func() {
	It("strips valid padding", func() {
		out, err := pkcs7Unpad([]byte("password-check\x02\x02"), 16)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(out)).To(Equal("password-check"))
	})
	It("rejects bad padding", func() {
		_, err := pkcs7Unpad([]byte{1, 2, 9}, 8)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("deriveKeyPBES2", func() {
	It("matches an independently-computed PBKDF2-HMAC-SHA256 vector", func() {
		globalSalt, _ := hex.DecodeString("00112233445566778899aabbccddeeff00112233")
		entrySalt, _ := hex.DecodeString("a1b2c3d4e5f6071829")
		key := deriveKeyPBES2(globalSalt, entrySalt, 10000)
		Expect(hex.EncodeToString(key)).To(Equal(
			"68f6312d48fc37689e7cfe1b73299f3b69cfab92ee4c272c9d3e2bea99b2276d"))
	})
})

var _ = Describe("LoadCredentials (generated fixture)", func() {
	It("decrypts the stored IMAP password", func() {
		dir := GinkgoT().TempDir()
		writeNSSFixture(dir, "") // empty master password
		creds, err := LoadCredentials(dir)
		Expect(err).NotTo(HaveOccurred())
		pw, ok := creds.Lookup("imap.example.com", "alice@example.com")
		Expect(ok).To(BeTrue())
		Expect(pw).To(Equal("s3cret-imap"))
	})

	It("fails with ErrMasterPassword when a master password protects the profile", func() {
		dir := GinkgoT().TempDir()
		writeNSSFixture(dir, "hunter2") // non-empty master password
		_, err := LoadCredentials(dir)
		Expect(err).To(MatchError(ErrMasterPassword))
	})
})

// ---- test-only crypto helpers (production only decrypts) ----

func pkcs7Pad(b []byte, blockSize int) []byte {
	n := blockSize - len(b)%blockSize
	pad := make([]byte, n)
	for i := range pad {
		pad[i] = byte(n)
	}
	return append(append([]byte{}, b...), pad...)
}

func aesCBCEncrypt(key, iv, pt []byte) []byte {
	blk, err := aes.NewCipher(key)
	Expect(err).NotTo(HaveOccurred())
	out := make([]byte, len(pt))
	cipher.NewCBCEncrypter(blk, iv).CryptBlocks(out, pt)
	return out
}

func des3CBCEncrypt(key, iv, pt []byte) []byte {
	blk, err := des.NewTripleDESCipher(key)
	Expect(err).NotTo(HaveOccurred())
	out := make([]byte, len(pt))
	cipher.NewCBCEncrypter(blk, iv).CryptBlocks(out, pt)
	return out
}

// ---- OIDs used only when building the fixture ----

var (
	oidPBKDF2     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 12}
	oidHmacSHA256 = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 9}
	oidAES256CBC  = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42}
	oidDES3CBC    = asn1.ObjectIdentifier{1, 2, 840, 113549, 3, 7}
)

// Fixed (deterministic) fixture material.
var (
	fxGlobalSalt = []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09,
		0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13,
	}
	fxEntrySalt = []byte{0xa1, 0xb2, 0xc3, 0xd4, 0xe5, 0xf6, 0x07, 0x18, 0x29}
	fxIters     = 10000
	fxIV14      = []byte{
		0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36,
		0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d,
	}
	fxMasterKey = []byte{
		0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
		0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00,
		0x13, 0x37, 0xbe, 0xef, 0xca, 0xfe, 0xba, 0xbe,
	}
	fxLoginIV = []byte{0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11}
	fxCKAID   = []byte{
		0xf8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
	}
)

// derivePBES2 mirrors deriveKeyPBES2 but allows a non-empty master password so
// the fixture can simulate a master-password-protected profile.
func derivePBES2(globalSalt, entrySalt []byte, iters int, password string) []byte {
	h := sha1.New()
	h.Write(globalSalt)
	h.Write([]byte(password))
	pw := h.Sum(nil)
	return pbkdf2.Key(pw, entrySalt, iters, 32, sha256.New)
}

// buildPBES2Item builds the outer encryptedItem DER (PBKDF2-HMAC-SHA256 +
// AES-256-CBC) using the same encoding/asn1 struct types secrets.go decodes.
func buildPBES2Item(aesKey, plaintext []byte) []byte {
	kdfDER, err := asn1.Marshal(pbkdf2Params{
		EntrySalt:  fxEntrySalt,
		Iterations: fxIters,
		KeyLength:  32,
		Prf: algorithmIdentifier{
			Algorithm:  oidHmacSHA256,
			Parameters: asn1.RawValue{FullBytes: []byte{0x05, 0x00}}, // NULL
		},
	})
	Expect(err).NotTo(HaveOccurred())

	ivDER, err := asn1.Marshal(fxIV14) // OCTET STRING
	Expect(err).NotTo(HaveOccurred())

	paramsDER, err := asn1.Marshal(pbes2Params{
		KeyDerivation: algorithmIdentifier{
			Algorithm:  oidPBKDF2,
			Parameters: asn1.RawValue{FullBytes: kdfDER},
		},
		Encryption: algorithmIdentifier{
			Algorithm:  oidAES256CBC,
			Parameters: asn1.RawValue{FullBytes: ivDER},
		},
	})
	Expect(err).NotTo(HaveOccurred())

	iv16 := append([]byte{0x04, 0x0e}, fxIV14...)
	ct := aesCBCEncrypt(aesKey, iv16, pkcs7Pad(plaintext, 16))

	itemDER, err := asn1.Marshal(encryptedItem{
		Algo: algorithmIdentifier{
			Algorithm:  oidPBES2,
			Parameters: asn1.RawValue{FullBytes: paramsDER},
		},
		Cipher: ct,
	})
	Expect(err).NotTo(HaveOccurred())
	return itemDER
}

// buildLogin builds one base64 logins.json entry (3DES-CBC with per-entry IV).
func buildLogin(masterKey []byte, plaintext string) string {
	ivDER, err := asn1.Marshal(fxLoginIV) // OCTET STRING
	Expect(err).NotTo(HaveOccurred())
	ct := des3CBCEncrypt(masterKey, fxLoginIV, pkcs7Pad([]byte(plaintext), 8))
	loginDER, err := asn1.Marshal(loginASN1{
		KeyID: fxCKAID,
		Algo: algorithmIdentifier{
			Algorithm:  oidDES3CBC,
			Parameters: asn1.RawValue{FullBytes: ivDER},
		},
		Cipher: ct,
	})
	Expect(err).NotTo(HaveOccurred())
	return base64.StdEncoding.EncodeToString(loginDER)
}

// writeNSSFixture generates a real, format-correct key4.db + logins.json in dir.
// An empty masterPassword yields a decryptable profile; a non-empty one encrypts
// the password-check with a key the empty-password path cannot reproduce, so
// LoadCredentials reports ErrMasterPassword.
func writeNSSFixture(dir, masterPassword string) {
	// password-check is encrypted with the (possibly master-password-derived) key.
	checkKey := derivePBES2(fxGlobalSalt, fxEntrySalt, fxIters, masterPassword)
	item2 := buildPBES2Item(checkKey, []byte("password-check"))

	// The wrapped master key is always encrypted with the empty-password key,
	// which is the only key recoverMasterKey derives once password-check passes.
	emptyKey := derivePBES2(fxGlobalSalt, fxEntrySalt, fxIters, "")
	a11 := buildPBES2Item(emptyKey, fxMasterKey)

	dbPath := filepath.Join(dir, "key4.db")
	db, err := sql.Open("sqlite", "file:"+dbPath)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`CREATE TABLE metaData (id TEXT PRIMARY KEY, item1 BLOB, item2 BLOB)`)
	Expect(err).NotTo(HaveOccurred())
	_, err = db.Exec(`INSERT INTO metaData (id, item1, item2) VALUES ('password', ?, ?)`, fxGlobalSalt, item2)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`CREATE TABLE nssPrivate (a11 BLOB, a102 BLOB)`)
	Expect(err).NotTo(HaveOccurred())
	_, err = db.Exec(`INSERT INTO nssPrivate (a11, a102) VALUES (?, ?)`, a11, fxCKAID)
	Expect(err).NotTo(HaveOccurred())

	Expect(db.Close()).To(Succeed())

	logins := map[string]any{
		"logins": []map[string]any{
			{
				"hostname":          "imap://imap.example.com",
				"encryptedUsername": buildLogin(fxMasterKey, "alice@example.com"),
				"encryptedPassword": buildLogin(fxMasterKey, "s3cret-imap"),
			},
		},
	}
	data, err := json.Marshal(logins)
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(filepath.Join(dir, "logins.json"), data, 0o600)).To(Succeed())
}
