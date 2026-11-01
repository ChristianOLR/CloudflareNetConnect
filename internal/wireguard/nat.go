package wireguard

import (
	"encoding/binary"
	"net"
)

// NATTranslator realiza traducción 1:1 (Stateless Virtual NAT) para evitar colisiones de subred.
type NATTranslator struct {
	OrigNet *net.IPNet
	VirtNet *net.IPNet
}

// NewNATTranslator crea un traductor entre la subred real y la subred virtual asignada.
func NewNATTranslator(origCIDR, virtCIDR string) (*NATTranslator, error) {
	_, origNet, err := net.ParseCIDR(origCIDR)
	if err != nil {
		return nil, err
	}
	_, virtNet, err := net.ParseCIDR(virtCIDR)
	if err != nil {
		return nil, err
	}
	return &NATTranslator{
		OrigNet: origNet,
		VirtNet: virtNet,
	}, nil
}

// TranslateOutbound traduce paquetes salientes: DstIP (Virt) -> DstIP (Orig).
func (n *NATTranslator) TranslateOutbound(pkt []byte) {
	if n == nil || n.OrigNet == nil || n.VirtNet == nil || len(pkt) < 20 {
		return
	}
	if pkt[0]>>4 != 4 { // Solo IPv4
		return
	}

	dstIP := net.IP(pkt[16:20])
	if !n.VirtNet.Contains(dstIP) {
		return
	}

	oldIP := binary.BigEndian.Uint32(pkt[16:20])
	mask := binary.BigEndian.Uint32(n.OrigNet.Mask)
	origBase := binary.BigEndian.Uint32(n.OrigNet.IP.To4())
	hostBits := oldIP & ^mask
	newIP := origBase | hostBits

	binary.BigEndian.PutUint32(pkt[16:20], newIP)
	updateIPChecksum(pkt)
	updateL4Checksum(pkt, oldIP, newIP)
}

// TranslateInbound traduce paquetes entrantes: SrcIP (Orig) -> SrcIP (Virt).
func (n *NATTranslator) TranslateInbound(pkt []byte) {
	if n == nil || n.OrigNet == nil || n.VirtNet == nil || len(pkt) < 20 {
		return
	}
	if pkt[0]>>4 != 4 { // Solo IPv4
		return
	}

	srcIP := net.IP(pkt[12:16])
	if !n.OrigNet.Contains(srcIP) {
		return
	}

	oldIP := binary.BigEndian.Uint32(pkt[12:16])
	mask := binary.BigEndian.Uint32(n.VirtNet.Mask)
	virtBase := binary.BigEndian.Uint32(n.VirtNet.IP.To4())
	hostBits := oldIP & ^mask
	newIP := virtBase | hostBits

	binary.BigEndian.PutUint32(pkt[12:16], newIP)
	updateIPChecksum(pkt)
	updateL4Checksum(pkt, oldIP, newIP)
}

func updateIPChecksum(pkt []byte) {
	ihl := int(pkt[0]&0x0F) * 4
	if len(pkt) < ihl {
		return
	}
	pkt[10] = 0
	pkt[11] = 0

	var sum uint32
	for i := 0; i < ihl; i += 2 {
		sum += uint32(binary.BigEndian.Uint16(pkt[i : i+2]))
	}
	for sum > 0xFFFF {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	binary.BigEndian.PutUint16(pkt[10:12], ^uint16(sum))
}

func updateL4Checksum(pkt []byte, oldIP, newIP uint32) {
	ihl := int(pkt[0]&0x0F) * 4
	proto := pkt[9]
	if len(pkt) < ihl+4 {
		return
	}

	var csumOffset int
	switch proto {
	case 6: // TCP
		if len(pkt) < ihl+20 {
			return
		}
		csumOffset = ihl + 16
	case 17: // UDP
		if len(pkt) < ihl+8 {
			return
		}
		csumOffset = ihl + 6
		if binary.BigEndian.Uint16(pkt[csumOffset:csumOffset+2]) == 0 {
			return // UDP checksum 0 significa sin checksum
		}
	default:
		return
	}

	oldCsum := binary.BigEndian.Uint16(pkt[csumOffset : csumOffset+2])
	newCsum := updateRFC1624(oldCsum, oldIP, newIP)
	binary.BigEndian.PutUint16(pkt[csumOffset:csumOffset+2], newCsum)
}

// updateRFC1624 actualiza el checksum incrementalmente según RFC 1624 tras cambiar una IP de 32 bits.
func updateRFC1624(oldCsum uint16, oldIP, newIP uint32) uint16 {
	old1 := uint32(oldIP >> 16)
	old2 := uint32(oldIP & 0xFFFF)
	new1 := uint32(newIP >> 16)
	new2 := uint32(newIP & 0xFFFF)

	sum := uint32(^oldCsum)
	sum += ^old1 & 0xFFFF
	sum += new1
	sum += ^old2 & 0xFFFF
	sum += new2

	for sum > 0xFFFF {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	return ^uint16(sum)
}
