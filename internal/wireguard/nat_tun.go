package wireguard

import "golang.zx2c4.com/wireguard/tun"

// NATDevice envuelve una interfaz tun.Device para aplicar traducción 1:1 de subred en memoria.
type NATDevice struct {
	tun.Device
	nat *NATTranslator
}

// WrapNATDevice decora el dispositivo TUN con el traductor de paquetes si está activo.
func WrapNATDevice(dev tun.Device, nat *NATTranslator) tun.Device {
	if nat == nil {
		return dev
	}
	return &NATDevice{
		Device: dev,
		nat:    nat,
	}
}

func (d *NATDevice) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	n, err := d.Device.Read(bufs, sizes, offset)
	if err != nil || d.nat == nil {
		return n, err
	}
	for i := 0; i < n; i++ {
		if offset+sizes[i] <= len(bufs[i]) {
			pkt := bufs[i][offset : offset+sizes[i]]
			d.nat.TranslateOutbound(pkt)
		}
	}
	return n, nil
}

func (d *NATDevice) Write(bufs [][]byte, offset int) (int, error) {
	if d.nat != nil {
		for i := range bufs {
			if offset < len(bufs[i]) {
				pkt := bufs[i][offset:]
				d.nat.TranslateInbound(pkt)
			}
		}
	}
	return d.Device.Write(bufs, offset)
}
