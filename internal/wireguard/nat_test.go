package wireguard

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestNATTranslator(t *testing.T) {
	trans, err := NewNATTranslator("192.168.1.0/24", "10.200.1.0/24")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	// Crear paquete IPv4 sintético (20 bytes de cabecera IPv4)
	pkt := make([]byte, 28) // 20 IP + 8 UDP
	pkt[0] = 0x45           // IPv4, IHL=5 (20 bytes)
	pkt[9] = 17             // UDP
	// Src: 10.0.0.5, Dst: 10.200.1.77
	copy(pkt[12:16], net.ParseIP("10.0.0.5").To4())
	copy(pkt[16:20], net.ParseIP("10.200.1.77").To4())
	// UDP header
	binary.BigEndian.PutUint16(pkt[20:22], 12345) // Src Port
	binary.BigEndian.PutUint16(pkt[22:24], 80)    // Dst Port
	binary.BigEndian.PutUint16(pkt[24:26], 8)     // UDP length
	binary.BigEndian.PutUint16(pkt[26:28], 0x1234)// UDP checksum

	updateIPChecksum(pkt)
	origCsum := binary.BigEndian.Uint16(pkt[10:12])

	// Outbound: Dst (10.200.1.77) debe convertirse en 192.168.1.77
	trans.TranslateOutbound(pkt)

	dstIP := net.IP(pkt[16:20]).String()
	if dstIP != "192.168.1.77" {
		t.Errorf("expected Dst IP 192.168.1.77, got %s", dstIP)
	}

	newCsum := binary.BigEndian.Uint16(pkt[10:12])
	if newCsum == origCsum {
		t.Errorf("expected IP checksum to change after translation")
	}

	// Inbound: Un paquete con Src 192.168.1.77 debe convertirse en 10.200.1.77
	inPkt := make([]byte, 28)
	inPkt[0] = 0x45
	inPkt[9] = 17
	copy(inPkt[12:16], net.ParseIP("192.168.1.77").To4())
	copy(inPkt[16:20], net.ParseIP("10.0.0.5").To4())
	binary.BigEndian.PutUint16(inPkt[24:26], 8)

	trans.TranslateInbound(inPkt)

	srcIP := net.IP(inPkt[12:16]).String()
	if srcIP != "10.200.1.77" {
		t.Errorf("expected Src IP 10.200.1.77, got %s", srcIP)
	}
}
