package wireguard

import "testing"

func TestExtractPeerFromConfigZeroPort(t *testing.T) {
	var peer cfPeerConfig
	peer.PublicKey = cfPeerPubKey
	peer.Endpoint.V4 = "162.159.193.8:0"
	peer.Endpoint.Host = "engage.cloudflareclient.com:2408"
	peer.Endpoint.Ports = []int{2408, 500, 1701, 4500}

	pub, ep := extractPeerFromConfig([]cfPeerConfig{peer}, "100.96.0.1")
	if pub != cfPeerPubKey {
		t.Fatalf("unexpected pubKey: %s", pub)
	}
	if ep != "162.159.193.8:2408" {
		t.Fatalf("expected 162.159.193.8:2408, got %s", ep)
	}
}

func TestExtractPeerFromConfigZeroTrustFallback(t *testing.T) {
	_, ep := extractPeerFromConfig(nil, "100.96.0.1")
	if ep != cfZeroTrustDefaultPeer {
		t.Fatalf("expected Zero Trust fallback %s, got %s", cfZeroTrustDefaultPeer, ep)
	}

	_, consumerEp := extractPeerFromConfig(nil, "172.16.0.2")
	if consumerEp != cfDefaultPeer {
		t.Fatalf("expected consumer fallback %s, got %s", cfDefaultPeer, consumerEp)
	}
}

func TestNeedsZeroTrustPeerBackfill(t *testing.T) {
	if !needsZeroTrustPeerBackfill("", "100.96.0.1") {
		t.Fatal("empty endpoint should require backfill")
	}
	if !needsZeroTrustPeerBackfill("engage.cloudflareclient.com:2408", "100.96.0.1") {
		t.Fatal("consumer domain endpoint on ZT address should require backfill")
	}
	if !needsZeroTrustPeerBackfill("162.159.192.1:2408", "100.96.0.1") {
		t.Fatal("consumer IPv4 endpoint on ZT address should require backfill")
	}
	if needsZeroTrustPeerBackfill("162.159.193.8:2408", "100.96.0.1") {
		t.Fatal("valid Zero Trust Gateway endpoint should not require backfill")
	}
}
