package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"golang.org/x/crypto/pbkdf2"
	_ "modernc.org/sqlite"
)

var ErrMasterPassword = errors.New("thunderbird profile is protected by a master password; credential decryption is not supported")

type Credentials struct {
	byHostUser map[string]string // "host\x00user" -> password
}

func credKey(host, user string) string { return host + "\x00" + user }

func (c *Credentials) Lookup(host, user string) (string, bool) {
	if c == nil {
		return "", false
	}
	pw, ok := c.byHostUser[credKey(host, user)]
	return pw, ok
}

// ---- ASN.1 shapes ----

type algorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}
type encryptedItem struct { // outer: { AlgorithmIdentifier, OCTET STRING }
	Algo   algorithmIdentifier
	Cipher []byte
}
type pbeParam3DES struct { // legacy params: { salt, iterations }
	EntrySalt  []byte
	Iterations int
}
type pbkdf2Params struct {
	EntrySalt  []byte
	Iterations int
	KeyLength  int                 `asn1:"optional"`
	Prf        algorithmIdentifier `asn1:"optional"`
}
type pbes2Params struct {
	KeyDerivation algorithmIdentifier // PBKDF2 + params
	Encryption    algorithmIdentifier // aes256-CBC + IV OCTET STRING
}
type loginASN1 struct { // logins.json entry: { keyID, {des-ede3-cbc, iv}, cipher }
	KeyID  []byte
	Algo   algorithmIdentifier
	Cipher []byte
}

var (
	oidPBES2      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	oid3DESLegacy = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 5, 1, 3}
)

func pkcs7Unpad(b []byte, blockSize int) ([]byte, error) {
	if len(b) == 0 || len(b)%blockSize != 0 {
		return nil, fmt.Errorf("invalid padded length %d", len(b))
	}
	n := int(b[len(b)-1])
	if n == 0 || n > blockSize || n > len(b) {
		return nil, fmt.Errorf("invalid padding byte %d", n)
	}
	for _, c := range b[len(b)-n:] {
		if int(c) != n {
			return nil, errors.New("invalid padding")
		}
	}
	return b[:len(b)-n], nil
}

func deriveKeyPBES2(globalSalt, entrySalt []byte, iterations int) []byte {
	h := sha1.New()
	h.Write(globalSalt) // empty master password => nothing else appended
	pw := h.Sum(nil)    // 20 bytes
	return pbkdf2.Key(pw, entrySalt, iterations, 32, sha256.New)
}

func aesCBCDecrypt(key, iv, ct []byte) ([]byte, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ct)%blk.BlockSize() != 0 {
		return nil, errors.New("aes: ciphertext not a multiple of block size")
	}
	out := make([]byte, len(ct))
	cipher.NewCBCDecrypter(blk, iv).CryptBlocks(out, ct)
	return out, nil
}

func des3CBCDecrypt(key, iv, ct []byte) ([]byte, error) {
	blk, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ct)%blk.BlockSize() != 0 {
		return nil, errors.New("3des: ciphertext not a multiple of block size")
	}
	out := make([]byte, len(ct))
	cipher.NewCBCDecrypter(blk, iv).CryptBlocks(out, ct)
	return out, nil
}

// decryptPBEItem unwraps an outer {AlgorithmIdentifier, cipher} and returns the
// decrypted plaintext (still padded) using the recovered-or-derived key path.
func decryptPBEItem(globalSalt, raw []byte) ([]byte, error) {
	var item encryptedItem
	if _, err := asn1.Unmarshal(raw, &item); err != nil {
		return nil, err
	}
	switch {
	case item.Algo.Algorithm.Equal(oidPBES2):
		var p pbes2Params
		if _, err := asn1.Unmarshal(item.Algo.Parameters.FullBytes, &p); err != nil {
			return nil, err
		}
		var kdf pbkdf2Params
		if _, err := asn1.Unmarshal(p.KeyDerivation.Parameters.FullBytes, &kdf); err != nil {
			return nil, err
		}
		var iv14 []byte
		if _, err := asn1.Unmarshal(p.Encryption.Parameters.FullBytes, &iv14); err != nil {
			return nil, err
		}
		key := deriveKeyPBES2(globalSalt, kdf.EntrySalt, kdf.Iterations)
		iv := append([]byte{0x04, 0x0e}, iv14...) // NSS prefixes DER tag+len to make 16 bytes
		return aesCBCDecrypt(key, iv, item.Cipher)
	case item.Algo.Algorithm.Equal(oid3DESLegacy):
		var pp pbeParam3DES
		if _, err := asn1.Unmarshal(item.Algo.Parameters.FullBytes, &pp); err != nil {
			return nil, err
		}
		key, iv := legacy3DESKeyIV(globalSalt, pp.EntrySalt)
		return des3CBCDecrypt(key, iv, item.Cipher)
	default:
		return nil, fmt.Errorf("unsupported PBE algorithm %v", item.Algo.Algorithm)
	}
}

// legacy3DESKeyIV implements the PKCS#12 SHA1/HMAC derivation (firepwd decrypt3DES).
func legacy3DESKeyIV(globalSalt, entrySalt []byte) (key, iv []byte) {
	hp := sha1.Sum(append([]byte{}, globalSalt...))
	pes := make([]byte, 20)
	copy(pes, entrySalt)
	chpArr := sha1.Sum(append(append([]byte{}, hp[:]...), entrySalt...))
	chp := chpArr[:]
	k1 := hmacSHA1(chp, append(append([]byte{}, pes...), entrySalt...))
	tk := hmacSHA1(chp, pes)
	k2 := hmacSHA1(chp, append(append([]byte{}, tk...), entrySalt...))
	k := append(append([]byte{}, k1...), k2...) // 40 bytes
	return k[:24], k[len(k)-8:]
}

// recoverMasterKey reads key4.db, verifies password-check, and returns the 24-byte
// 3DES master key from nssPrivate.a11.
func recoverMasterKey(key4Path string) ([]byte, error) {
	db, err := sql.Open("sqlite", "file:"+key4Path+"?mode=ro&immutable=1")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var globalSalt, item2 []byte
	err = db.QueryRow(`SELECT item1, item2 FROM metaData WHERE id = 'password'`).Scan(&globalSalt, &item2)
	if err != nil {
		return nil, fmt.Errorf("reading metaData: %w", err)
	}
	check, err := decryptPBEItem(globalSalt, item2)
	if err == nil {
		check, err = pkcs7Unpad(check, blockSizeFor(item2))
	}
	if err != nil || !bytes.Equal(check, []byte("password-check")) {
		return nil, ErrMasterPassword
	}

	var a11 []byte
	if err := db.QueryRow(`SELECT a11 FROM nssPrivate WHERE a11 IS NOT NULL LIMIT 1`).Scan(&a11); err != nil {
		return nil, fmt.Errorf("reading nssPrivate: %w", err)
	}
	dec, err := decryptPBEItem(globalSalt, a11)
	if err != nil {
		return nil, err
	}
	// The master key blob is an ASN.1 { OID, OCTET STRING key } after unpad; but
	// in practice NSS stores the 24-byte 3DES key as the last 24 bytes. Unpad then
	// take the trailing 24 bytes.
	dec, err = pkcs7Unpad(dec, blockSizeFor(a11))
	if err != nil {
		return nil, err
	}
	if len(dec) < 24 {
		return nil, fmt.Errorf("master key too short: %d", len(dec))
	}
	return dec[len(dec)-24:], nil
}

func blockSizeFor(raw []byte) int {
	var item encryptedItem
	if _, err := asn1.Unmarshal(raw, &item); err == nil && item.Algo.Algorithm.Equal(oidPBES2) {
		return 16
	}
	return 8
}

// LoadCredentials decrypts logins.json into a host/user -> password map.
func LoadCredentials(profileDir string) (*Credentials, error) {
	masterKey, err := recoverMasterKey(filepath.Join(profileDir, "key4.db"))
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(profileDir, "logins.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return &Credentials{byHostUser: map[string]string{}}, nil
		}
		return nil, err
	}
	var file struct {
		Logins []struct {
			Hostname          string `json:"hostname"`
			EncryptedUsername string `json:"encryptedUsername"`
			EncryptedPassword string `json:"encryptedPassword"`
		} `json:"logins"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	creds := &Credentials{byHostUser: map[string]string{}}
	for _, l := range file.Logins {
		user, err1 := decryptLogin(masterKey, l.EncryptedUsername)
		pass, err2 := decryptLogin(masterKey, l.EncryptedPassword)
		if err1 != nil || err2 != nil {
			continue
		}
		host := hostOnly(l.Hostname) // "imap://imap.example.com" -> "imap.example.com"
		creds.byHostUser[credKey(host, user)] = pass
	}
	return creds, nil
}

func decryptLogin(masterKey []byte, b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	var l loginASN1
	if _, err := asn1.Unmarshal(raw, &l); err != nil {
		return "", err
	}
	var iv []byte
	if _, err := asn1.Unmarshal(l.Algo.Parameters.FullBytes, &iv); err != nil {
		return "", err
	}
	pt, err := des3CBCDecrypt(masterKey, iv, l.Cipher)
	if err != nil {
		return "", err
	}
	pt, err = pkcs7Unpad(pt, 8)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

func hostOnly(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

func hmacSHA1(key, msg []byte) []byte {
	m := hmac.New(sha1.New, key)
	m.Write(msg)
	return m.Sum(nil)
}
