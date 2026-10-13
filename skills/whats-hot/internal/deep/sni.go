// Package deep is the optional privileged path: it sniffs TLS ClientHello packets to
// recover the exact hostname an app asked for (the SNI), which reverse DNS of a bare CDN
// IP often can't. It needs sudo (packet capture); the no-sudo path is reverse DNS in
// package monitor. See docs/adr and CONTEXT.md (Remote Endpoint).
package deep

import "encoding/binary"

// parseClientHelloSNI extracts the server_name (SNI) from a TLS record whose first byte
// is the content type. It returns ("", false) unless the record is a ClientHello that
// carries a host_name entry. All lengths are bounds-checked; malformed input yields
// ("", false) rather than a panic.
func parseClientHelloSNI(rec []byte) (string, bool) {
	if len(rec) < 5 || rec[0] != 0x16 { // 0x16 = handshake
		return "", false
	}
	p := rec[5:]
	if len(p) < 4 || p[0] != 0x01 { // 0x01 = ClientHello
		return "", false
	}
	hlen := int(p[1])<<16 | int(p[2])<<8 | int(p[3])
	p = p[4:]
	if len(p) > hlen {
		p = p[:hlen]
	}
	// client_version(2) + random(32)
	if len(p) < 34 {
		return "", false
	}
	p = p[34:]
	// session_id
	p, ok := skip8(p)
	if !ok {
		return "", false
	}
	// cipher_suites
	p, ok = skip16(p)
	if !ok {
		return "", false
	}
	// compression_methods
	p, ok = skip8(p)
	if !ok {
		return "", false
	}
	// extensions
	if len(p) < 2 {
		return "", false
	}
	elen := int(binary.BigEndian.Uint16(p))
	p = p[2:]
	if len(p) > elen {
		p = p[:elen]
	}
	for len(p) >= 4 {
		etype := binary.BigEndian.Uint16(p)
		dlen := int(binary.BigEndian.Uint16(p[2:]))
		p = p[4:]
		if len(p) < dlen {
			return "", false
		}
		data := p[:dlen]
		p = p[dlen:]
		if etype != 0x0000 { // server_name extension
			continue
		}
		if len(data) < 2 { // server_name_list length
			return "", false
		}
		data = data[2:]
		for len(data) >= 3 {
			nameType := data[0]
			nlen := int(binary.BigEndian.Uint16(data[1:]))
			data = data[3:]
			if len(data) < nlen {
				return "", false
			}
			if nameType == 0 { // host_name
				return string(data[:nlen]), true
			}
			data = data[nlen:]
		}
	}
	return "", false
}

// skip8 drops a 1-byte-length-prefixed vector; skip16 drops a 2-byte-length-prefixed one.
func skip8(p []byte) ([]byte, bool) {
	if len(p) < 1 {
		return nil, false
	}
	n := int(p[0])
	if len(p) < 1+n {
		return nil, false
	}
	return p[1+n:], true
}

func skip16(p []byte) ([]byte, bool) {
	if len(p) < 2 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint16(p))
	if len(p) < 2+n {
		return nil, false
	}
	return p[2+n:], true
}
