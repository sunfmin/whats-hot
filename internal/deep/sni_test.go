package deep

import (
	"encoding/binary"
	"testing"
)

// buildClientHello constructs a minimal TLS ClientHello record carrying one SNI host_name
// entry, so the parser is tested against bytes shaped exactly like the wire format.
func buildClientHello(sni string) []byte {
	// server_name extension body
	var ext []byte
	name := []byte(sni)
	entry := append([]byte{0x00}, u16(len(name))...) // name_type=host_name, name_len
	entry = append(entry, name...)
	list := append(u16(len(entry)), entry...) // server_name_list_len + entry
	ext = append(u16(0x0000), u16(len(list))...)
	ext = append(ext, list...)

	var hs []byte
	hs = append(hs, 0x03, 0x03)             // client_version TLS1.2
	hs = append(hs, make([]byte, 32)...)    // random
	hs = append(hs, 0x00)                   // session_id len 0
	hs = append(hs, u16(2)...)              // cipher_suites len
	hs = append(hs, 0x00, 0x2f)             // one cipher suite
	hs = append(hs, 0x01, 0x00)             // compression: len 1, null
	hs = append(hs, u16(len(ext))...)       // extensions length
	hs = append(hs, ext...)

	body := append([]byte{0x01}, u24(len(hs))...) // handshake type + length
	body = append(body, hs...)

	rec := append([]byte{0x16, 0x03, 0x01}, u16(len(body))...) // record: handshake, version, len
	rec = append(rec, body...)
	return rec
}

func u16(n int) []byte { b := make([]byte, 2); binary.BigEndian.PutUint16(b, uint16(n)); return b }
func u24(n int) []byte { return []byte{byte(n >> 16), byte(n >> 8), byte(n)} }

func TestParseClientHelloSNI(t *testing.T) {
	host, ok := parseClientHelloSNI(buildClientHello("dl.google.com"))
	if !ok || host != "dl.google.com" {
		t.Fatalf("got (%q,%v), want (dl.google.com,true)", host, ok)
	}
}

func TestParseClientHelloSNI_Rejects(t *testing.T) {
	cases := map[string][]byte{
		"empty":            {},
		"not handshake":    {0x17, 0x03, 0x03, 0x00, 0x01, 0x00},
		"truncated":        buildClientHello("example.com")[:20],
		"not clienthello":  append([]byte{0x16, 0x03, 0x01, 0x00, 0x04}, 0x02, 0x00, 0x00, 0x00),
	}
	for name, rec := range cases {
		if h, ok := parseClientHelloSNI(rec); ok {
			t.Errorf("%s: expected no SNI, got %q", name, h)
		}
	}
}
