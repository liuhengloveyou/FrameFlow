package gb28181_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/liuhengloveyou/FrameFlow/pkg/gb28181"
	"github.com/rs/zerolog"
)

func TestSIPEndpointRegister(t *testing.T) {
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.LocalAddr().String()
	_ = ln.Close()

	gw := &gb28181.Gateway{
		Config: gb28181.Config{
			Listen:       addr,
			Serial:       "34020000002000000001",
			Realm:        "3402000000",
			SipIP:        "127.0.0.1",
			AllowUnknown: true,
		},
		Registry: gb28181.NewRegistry(),
	}
	srv := gb28181.NewSIPEndpoint(gw, gb28181.NopHandler{}, zerolog.Nop())
	srv.AutoCatalogAfterRegister = false
	if err := srv.Start("test-server"); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	time.Sleep(50 * time.Millisecond)

	cua, err := sipgo.NewUA(sipgo.WithUserAgent("test-cam"))
	if err != nil {
		t.Fatal(err)
	}
	defer cua.Close()
	cli, err := sipgo.NewClient(cua)
	if err != nil {
		t.Fatal(err)
	}

	host, portStr, _ := net.SplitHostPort(addr)
	port := 5060
	fmt.Sscanf(portStr, "%d", &port)
	recipient := sip.Uri{User: "34020000002000000001", Host: host, Port: port}
	req := sip.NewRequest(sip.REGISTER, recipient)
	from := &sip.FromHeader{
		Address: sip.Uri{User: "34020000001320000001", Host: "3402000000"},
		Params:  sip.NewParams(),
	}
	from.Params.Add("tag", sip.GenerateTagN(8))
	req.AppendHeader(from)
	req.AppendHeader(&sip.ToHeader{Address: sip.Uri{User: "34020000001320000001", Host: "3402000000"}})
	req.AppendHeader(sip.NewHeader("Expires", "3600"))
	req.AppendHeader(&sip.ContactHeader{Address: sip.Uri{User: "34020000001320000001", Host: "127.0.0.1", Port: 5061}})

	rctx, rcancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer rcancel()
	tx, err := cli.TransactionRequest(rctx, req)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Terminate()
	select {
	case res := <-tx.Responses():
		if res.StatusCode != 200 {
			t.Fatalf("register %d", res.StatusCode)
		}
	case <-rctx.Done():
		t.Fatal("register timeout")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if d, ok := gw.Registry.Get("34020000001320000001"); ok && d.Online {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("device not registered")
}
