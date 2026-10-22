package assets

import (
	_ "embed"
)

// CloudflareGreenPNG contiene el icono con bolita verde (activo).
//go:embed cloudflare_green.png
var CloudflareGreenPNG []byte

// CloudflareRedPNG contiene el icono con bolita roja (pausado).
//go:embed cloudflare_red.png
var CloudflareRedPNG []byte

// CloudflareICO contiene el binario del icono ICO para Windows.
//go:embed cloudflare.ico
var CloudflareICO []byte

// CloudflarePNG contiene la imagen PNG para macOS y Linux.
//go:embed cloudflare.png
var CloudflarePNG []byte

// CloudflareSVG contiene el vector oficial de Cloudflare.
//go:embed cloudflare.svg
var CloudflareSVG []byte
