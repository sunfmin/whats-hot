package deep

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Resolve captures TLS ClientHello packets for roughly `seconds` and returns a map of
// remote server IP -> SNI hostname. It shells out to tcpdump under sudo (using
// SUDO_ASKPASS if set), so it requires privilege. Only handshakes that happen during the
// window are seen — connections already established are not re-observed. Errors are
// returned but a partial map is still usable.
func Resolve(ctx context.Context, seconds int) (map[string]string, error) {
	if seconds < 1 {
		seconds = 1
	}
	capCtx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()

	iface := defaultInterface()
	args := []string{"-i", iface, "-s", "0", "-U", "-n", "-w", "-", "tcp", "port", "443"}
	var cmd *exec.Cmd
	if os.Getenv("SUDO_ASKPASS") != "" {
		cmd = exec.CommandContext(capCtx, "sudo", append([]string{"-A", "tcpdump"}, args...)...)
	} else {
		cmd = exec.CommandContext(capCtx, "sudo", append([]string{"tcpdump"}, args...)...)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start tcpdump (needs sudo): %w", err)
	}
	hosts := parsePcapSNI(stdout)
	_ = cmd.Wait()
	return hosts, nil
}

// parsePcapSNI reads a pcap stream (as written by `tcpdump -w -`) and extracts SNI
// hostnames keyed by the destination (server) IP of each ClientHello.
func parsePcapSNI(r io.Reader) map[string]string {
	hosts := map[string]string{}
	gh := make([]byte, 24)
	if _, err := io.ReadFull(r, gh); err != nil {
		return hosts
	}
	var bo binary.ByteOrder
	switch binary.LittleEndian.Uint32(gh[:4]) {
	case 0xa1b2c3d4:
		bo = binary.LittleEndian
	case 0xd4c3b2a1:
		bo = binary.BigEndian
	default:
		return hosts
	}
	linktype := bo.Uint32(gh[20:24])

	hdr := make([]byte, 16)
	for {
		if _, err := io.ReadFull(r, hdr); err != nil {
			return hosts
		}
		inclLen := bo.Uint32(hdr[8:12])
		if inclLen == 0 || inclLen > 1<<20 {
			return hosts
		}
		pkt := make([]byte, inclLen)
		if _, err := io.ReadFull(r, pkt); err != nil {
			return hosts
		}
		dstIP, payload, ok := tcpPayload(linktype, pkt)
		if !ok || len(payload) == 0 {
			continue
		}
		if host, ok := parseClientHelloSNI(payload); ok && host != "" {
			hosts[dstIP] = host
		}
	}
}

// tcpPayload strips the link, IPv4, and TCP layers, returning the destination IP and the
// TCP payload. IPv6 and non-TCP packets are skipped.
func tcpPayload(linktype uint32, pkt []byte) (string, []byte, bool) {
	var ip []byte
	switch linktype {
	case 1: // EN10MB
		if len(pkt) < 14 || pkt[12] != 0x08 || pkt[13] != 0x00 {
			return "", nil, false
		}
		ip = pkt[14:]
	case 0, 108: // NULL / LOOP: 4-byte address-family header
		if len(pkt) < 4 {
			return "", nil, false
		}
		ip = pkt[4:]
	case 12, 14, 101: // RAW
		ip = pkt
	default:
		return "", nil, false
	}
	if len(ip) < 20 || ip[0]>>4 != 4 || ip[9] != 6 { // IPv4, TCP
		return "", nil, false
	}
	ihl := int(ip[0]&0x0f) * 4
	if len(ip) < ihl+20 {
		return "", nil, false
	}
	dst := net.IP(ip[16:20]).String()
	tcp := ip[ihl:]
	dataOff := int(tcp[12]>>4) * 4
	if len(tcp) < dataOff {
		return "", nil, false
	}
	return dst, tcp[dataOff:], true
}

func defaultInterface() string {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "interface:") {
				return strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
			}
		}
	}
	return "en0"
}
