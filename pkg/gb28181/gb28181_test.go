package gb28181

import (
	"testing"
)

func TestPortPool(t *testing.T) {
	p, err := NewPortPool("1000-1002")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := p.Acquire()
	b, _ := p.Acquire()
	c, _ := p.Acquire()
	if a == b || b == c {
		t.Fatalf("ports not unique: %d %d %d", a, b, c)
	}
	if _, err := p.Acquire(); err == nil {
		t.Fatal("expected exhausted")
	}
	p.Release(a)
	d, err := p.Acquire()
	if err != nil || d != a {
		t.Fatalf("reuse got %d err %v", d, err)
	}
}

func TestBuildPTZCmd(t *testing.T) {
	s, err := BuildPTZCmd(PTZLeft, 32)
	if err != nil || len(s) != 16 {
		t.Fatalf("got %q err %v", s, err)
	}
	_, err = BuildPTZCmd("nope", 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseCatalog(t *testing.T) {
	body := []byte(`<?xml version="1.0"?>
<Response>
<CmdType>Catalog</CmdType>
<SN>1</SN>
<DeviceID>34020000001110000001</DeviceID>
<SumNum>1</SumNum>
<DeviceList Num="1">
<Item>
<DeviceID>34020000001320000001</DeviceID>
<Name>cam1</Name>
<Status>ON</Status>
</Item>
</DeviceList>
</Response>`)
	if ParseMessageCmdType(body) != "Catalog" {
		t.Fatal(ParseMessageCmdType(body))
	}
	resp, err := ParseCatalogResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	chs := ChannelsFromCatalog(resp.DeviceID, resp.DeviceList.Items)
	if len(chs) != 1 || chs[0].ID != "34020000001320000001" {
		t.Fatalf("%+v", chs)
	}
}

func TestParseCatalogNotify(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="GB2312"?>
<Notify>
<CmdType>Catalog</CmdType>
<SN>2</SN>
<DeviceID>34020000001310000002</DeviceID>
<SumNum>2</SumNum>
<DeviceList Num="2">
<Item>
<DeviceID>34020000001310000002</DeviceID>
<Name>IPC</Name>
<Status>ON</Status>
</Item>
<Item>
<DeviceID>34020000001310000111</DeviceID>
<Name>Channel1</Name>
<Status>ON</Status>
</Item>
</DeviceList>
</Notify>`)
	resp, err := ParseCatalogResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.DeviceID != "34020000001310000002" {
		t.Fatalf("device %q", resp.DeviceID)
	}
	chs := ChannelsFromCatalog(resp.DeviceID, resp.DeviceList.Items)
	if len(chs) != 1 || chs[0].ID != "34020000001310000111" {
		t.Fatalf("want media channel only, got %+v", chs)
	}
}

func TestChannelsFromCatalogKeepsDeviceWhenAlone(t *testing.T) {
	items := []CatalogItem{{DeviceID: "d1", Name: "self"}}
	chs := ChannelsFromCatalog("d1", items)
	if len(chs) != 1 || chs[0].ID != "d1" {
		t.Fatalf("%+v", chs)
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	r.UpsertDevice(&Device{ID: "d1", Online: true})
	r.UpsertChannels("d1", []*Channel{{ID: "c1", Name: "cam"}})
	_, ch, ok := r.Channel("d1", "c1")
	if !ok || ch.Name != "cam" {
		t.Fatal(ok, ch)
	}
}

func TestInviteSDP(t *testing.T) {
	s := InviteSDP("192.168.1.1", 10000, "0000000001", TransportUDP, StreamSub)
	if !containsAll(s,
		"m=video 10000 RTP/AVP 96",
		"a=rtpmap:96 PS/90000",
		"y=0000000001",
		"a=stream:1",
		"a=stream:SUB",
		"a=streamnumber:1",
		"a=streamprofile:1",
		"a=streamMode:sub",
	) {
		t.Fatal(s)
	}
	sMain := InviteSDP("192.168.1.1", 10000, "0000000001", TransportUDP, StreamMain)
	if !containsAll(sMain,
		"a=stream:0",
		"a=stream:MAIN",
		"a=streamnumber:0",
		"a=streamprofile:0",
		"a=streamMode:main",
	) {
		t.Fatal(sMain)
	}
	s2 := InviteSDP("192.168.1.1", 10001, "0000000002", TransportTCPPassive, "")
	if !containsAll(s2, "TCP/RTP/AVP 96", "a=setup:passive", "a=streamprofile:1") {
		t.Fatal(s2)
	}
}

func TestRecipientURIPrefersRemoteAddr(t *testing.T) {
	dev := &Device{
		ID:         "34020000001310000002",
		RemoteAddr: "192.168.1.24:5060",
		ContactURI: "sip:34020000001310000002@192.168.1.100:5060",
	}
	uri, err := recipientURI(dev)
	if err != nil {
		t.Fatal(err)
	}
	if uri.Host != "192.168.1.24" || uri.Port != 5060 {
		t.Fatalf("want remote 192.168.1.24:5060, got %s:%d", uri.Host, uri.Port)
	}
	if uri.User != "34020000001310000002" {
		t.Fatalf("user=%q", uri.User)
	}

	// Remote missing → fall back to Contact
	dev2 := &Device{
		ID:         "34020000001310000002",
		ContactURI: "sip:34020000001310000002@10.0.0.8:5060",
	}
	uri2, err := recipientURI(dev2)
	if err != nil {
		t.Fatal(err)
	}
	if uri2.Host != "10.0.0.8" || uri2.Port != 5060 {
		t.Fatalf("want contact host, got %s:%d", uri2.Host, uri2.Port)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !containsStr(s, p) {
			return false
		}
	}
	return true
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
