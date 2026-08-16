package gb28181

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

// SendCatalog sends a Catalog query MESSAGE to deviceID.
func (ep *SIPEndpoint) SendCatalog(deviceID string) error {
	return ep.SendMessage(deviceID, "Application/MANSCDP+xml", BuildCatalogQuery(deviceID))
}

// SendMessage sends a SIP MESSAGE to an online device.
func (ep *SIPEndpoint) SendMessage(deviceID, contentType, body string) error {
	if ep.cli == nil {
		return fferrors.ErrGbSipNotStarted
	}
	dev, ok := ep.GW.Registry.Get(deviceID)
	if !ok || !dev.Online {
		return fferrors.ErrGbDeviceOffline
	}
	recipient, err := recipientURI(dev)
	if err != nil {
		return err
	}
	req := sip.NewRequest(sip.MESSAGE, recipient)
	req.AppendHeader(sip.NewHeader("Content-Type", contentType))
	req.SetBody([]byte(body))
	from := sip.FromHeader{
		Address: sip.Uri{User: ep.GW.Config.Serial, Host: ep.GW.Config.Realm},
		Params:  sip.NewParams(),
	}
	from.Params.Add("tag", sip.GenerateTagN(8))
	to := sip.ToHeader{Address: sip.Uri{User: deviceID, Host: ep.GW.Config.Realm}}
	req.AppendHeader(&from)
	req.AppendHeader(&to)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := ep.cli.TransactionRequest(ctx, req)
	if err != nil {
		return err
	}
	defer tx.Terminate()
	select {
	case <-tx.Done():
		return tx.Err()
	case res := <-tx.Responses():
		if res.StatusCode >= 300 {
			return fmt.Errorf("MESSAGE status %d", res.StatusCode)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// InviteLive starts a live play INVITE dialog.
// streamProfile: "main" | "sub" (empty defaults to sub).
func (ep *SIPEndpoint) InviteLive(deviceID, channelID, transport string, mediaPort uint16, ssrc, streamProfile string) (string, func() error, []byte, error) {
	if ep.cli == nil {
		return "", nil, nil, fferrors.ErrGbSipNotStarted
	}
	dev, ok := ep.GW.Registry.Get(deviceID)
	if !ok || !dev.Online {
		return "", nil, nil, fferrors.ErrGbDeviceOffline
	}
	if !AllowsInvite(dev.Status) {
		return "", nil, nil, fferrors.ErrGbInviteNotAllowed
	}
	recipient, err := recipientURI(dev)
	if err != nil {
		return "", nil, nil, err
	}
	sipPort, _ := strconv.Atoi(listenPort(ep.GW.Config.Listen))
	dua := &sipgo.DialogUA{
		Client: ep.cli,
		ContactHDR: sip.ContactHeader{
			Address: sip.Uri{
				User: ep.GW.Config.Serial,
				Host: ep.GW.Config.SipIP,
				Port: sipPort,
			},
		},
	}

	mediaIP := ep.GW.Config.MediaIP
	if mediaIP == "" {
		mediaIP = ep.GW.Config.SipIP
	}
	profile := NormalizeStreamProfile(streamProfile)
	tr := NormalizeMediaTransport(transport)
	if tr == TransportAuto {
		tr = NormalizeMediaTransport(ep.GW.Config.Transport)
		if tr == TransportAuto {
			tr = TransportUDP
		}
	}
	sdp := InviteSDP(mediaIP, mediaPort, ssrc, tr, profile)
	fromHdr := &sip.FromHeader{
		Address: sip.Uri{User: ep.GW.Config.Serial, Host: ep.GW.Config.Realm},
		Params:  sip.NewParams(),
	}
	fromHdr.Params.Add("tag", sip.GenerateTagN(8))
	headers := []sip.Header{
		sip.NewHeader("Content-Type", "Application/SDP"),
		sip.NewHeader("Subject", SubjectHeader(channelID, ssrc, deviceID, ep.GW.Config.Serial)),
		fromHdr,
		&sip.ToHeader{Address: sip.Uri{User: channelID, Host: ep.GW.Config.Realm}},
	}

	ep.Log.Info().
		Str("device", deviceID).
		Str("channel", channelID).
		Str("transport", tr).
		Str("stream", profile).
		Strs("sdp_stream_attrs", StreamSDPAttrs(profile)).
		Str("media_ip", mediaIP).
		Uint16("media_port", mediaPort).
		Str("ssrc", ssrc).
		Str("remote", recipient.String()).
		Int("sdp_len", len(sdp)).
		Msg("[gb28181] invite: request")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sess, err := dua.Invite(ctx, recipient, []byte(sdp), headers...)
	if err != nil {
		ep.Log.Error().Err(err).Str("device", deviceID).Str("channel", channelID).Msg("[gb28181] invite: send failed")
		return "", nil, nil, err
	}
	if err := sess.WaitAnswer(ctx, sipgo.AnswerOptions{}); err != nil {
		ep.Log.Error().Err(err).Str("device", deviceID).Str("channel", channelID).Msg("[gb28181] invite: wait answer failed")
		_ = sess.Close()
		if strings.Contains(err.Error(), "486") {
			return "", nil, nil, fferrors.ErrGbDeviceBusy
		}
		return "", nil, nil, err
	}
	var answer []byte
	if sess.InviteResponse != nil {
		answer = append([]byte(nil), sess.InviteResponse.Body()...)
	}
	if err := sess.Ack(ctx); err != nil {
		ep.Log.Error().Err(err).Str("device", deviceID).Str("channel", channelID).Msg("[gb28181] invite: ack failed")
		_ = sess.Close()
		return "", nil, nil, err
	}

	callID := ""
	if h := sess.InviteRequest.CallID(); h != nil {
		callID = h.Value()
	}

	byeFn := func() error {
		bctx, bcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer bcancel()
		err := sess.Bye(bctx)
		_ = sess.Close()
		return err
	}

	ep.Log.Info().
		Str("device", deviceID).
		Str("channel", channelID).
		Str("callid", callID).
		Str("stream", profile).
		Str("media_ip", mediaIP).
		Uint16("media_port", mediaPort).
		Str("transport", tr).
		Int("answer_sdp_len", len(answer)).
		Msg("[gb28181] invite ok")
	return callID, byeFn, answer, nil
}

// InvitePlayback starts a playback INVITE dialog for [start, end].
func (ep *SIPEndpoint) InvitePlayback(deviceID, channelID, transport string, mediaPort uint16, ssrc string, start, end time.Time) (string, func() error, []byte, error) {
	if ep.cli == nil {
		return "", nil, nil, fferrors.ErrGbSipNotStarted
	}
	dev, ok := ep.GW.Registry.Get(deviceID)
	if !ok || !dev.Online {
		return "", nil, nil, fferrors.ErrGbDeviceOffline
	}
	if !AllowsInvite(dev.Status) {
		return "", nil, nil, fferrors.ErrGbInviteNotAllowed
	}
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return "", nil, nil, fferrors.ErrGbPlaybackRangeInvalid
	}
	recipient, err := recipientURI(dev)
	if err != nil {
		return "", nil, nil, err
	}
	sipPort, _ := strconv.Atoi(listenPort(ep.GW.Config.Listen))
	dua := &sipgo.DialogUA{
		Client: ep.cli,
		ContactHDR: sip.ContactHeader{
			Address: sip.Uri{
				User: ep.GW.Config.Serial,
				Host: ep.GW.Config.SipIP,
				Port: sipPort,
			},
		},
	}

	mediaIP := ep.GW.Config.MediaIP
	if mediaIP == "" {
		mediaIP = ep.GW.Config.SipIP
	}
	tr := NormalizeMediaTransport(transport)
	if tr == TransportAuto {
		tr = NormalizeMediaTransport(ep.GW.Config.Transport)
		if tr == TransportAuto {
			tr = TransportUDP
		}
	}
	sdp := InvitePlaybackSDP(mediaIP, mediaPort, ssrc, tr, channelID, start, end)
	fromHdr := &sip.FromHeader{
		Address: sip.Uri{User: ep.GW.Config.Serial, Host: ep.GW.Config.Realm},
		Params:  sip.NewParams(),
	}
	fromHdr.Params.Add("tag", sip.GenerateTagN(8))
	headers := []sip.Header{
		sip.NewHeader("Content-Type", "Application/SDP"),
		sip.NewHeader("Subject", SubjectHeader(channelID, ssrc, deviceID, ep.GW.Config.Serial)),
		fromHdr,
		&sip.ToHeader{Address: sip.Uri{User: channelID, Host: ep.GW.Config.Realm}},
	}

	ep.Log.Info().
		Str("device", deviceID).
		Str("channel", channelID).
		Str("transport", tr).
		Time("start", start).
		Time("end", end).
		Str("media_ip", mediaIP).
		Uint16("media_port", mediaPort).
		Str("ssrc", ssrc).
		Msg("[gb28181] playback invite: request")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sess, err := dua.Invite(ctx, recipient, []byte(sdp), headers...)
	if err != nil {
		ep.Log.Error().Err(err).Str("device", deviceID).Str("channel", channelID).Msg("[gb28181] playback invite: send failed")
		return "", nil, nil, err
	}
	if err := sess.WaitAnswer(ctx, sipgo.AnswerOptions{}); err != nil {
		ep.Log.Error().Err(err).Str("device", deviceID).Str("channel", channelID).Msg("[gb28181] playback invite: wait answer failed")
		_ = sess.Close()
		if strings.Contains(err.Error(), "486") {
			return "", nil, nil, fferrors.ErrGbDeviceBusy
		}
		return "", nil, nil, err
	}
	var answer []byte
	if sess.InviteResponse != nil {
		answer = append([]byte(nil), sess.InviteResponse.Body()...)
	}
	if err := sess.Ack(ctx); err != nil {
		ep.Log.Error().Err(err).Str("device", deviceID).Str("channel", channelID).Msg("[gb28181] playback invite: ack failed")
		_ = sess.Close()
		return "", nil, nil, err
	}

	callID := ""
	if h := sess.InviteRequest.CallID(); h != nil {
		callID = h.Value()
	}
	byeFn := func() error {
		bctx, bcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer bcancel()
		err := sess.Bye(bctx)
		_ = sess.Close()
		return err
	}

	ep.Log.Info().
		Str("device", deviceID).
		Str("channel", channelID).
		Str("callid", callID).
		Str("transport", tr).
		Int("answer_sdp_len", len(answer)).
		Msg("[gb28181] playback invite ok")
	return callID, byeFn, answer, nil
}

// QueryRecordInfo sends RecordInfo and waits for aggregated responses (timeout 60s).
func (ep *SIPEndpoint) QueryRecordInfo(ctx context.Context, deviceID, channelID string, start, end time.Time) ([]RecordItem, error) {
	if ep.cli == nil {
		return nil, fferrors.ErrGbSipNotStarted
	}
	dev, ok := ep.GW.Registry.Get(deviceID)
	if !ok || !dev.Online {
		return nil, fferrors.ErrGbDeviceOffline
	}
	snVal, body := BuildRecordInfoQuery(channelID, start, end)
	wait := &recordWait{ch: make(chan struct{})}
	ep.recordMu.Lock()
	if ep.recordWait == nil {
		ep.recordWait = map[string]*recordWait{}
	}
	ep.recordWait[snVal] = wait
	ep.recordMu.Unlock()

	defer func() {
		ep.recordMu.Lock()
		delete(ep.recordWait, snVal)
		ep.recordMu.Unlock()
	}()

	if err := ep.SendMessage(deviceID, "Application/MANSCDP+xml", body); err != nil {
		return nil, err
	}

	timeout := 60 * time.Second
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			ep.recordMu.Lock()
			items := append([]RecordItem(nil), wait.items...)
			ep.recordMu.Unlock()
			if len(items) > 0 {
				return items, nil
			}
			return nil, fferrors.ErrGbDeviceTimeout
		case <-wait.ch:
			ep.recordMu.Lock()
			items := append([]RecordItem(nil), wait.items...)
			done := wait.done
			ep.recordMu.Unlock()
			if done {
				return items, nil
			}
		}
	}
}

func (ep *SIPEndpoint) onRecordInfoResponse(resp *RecordInfoResponse) {
	if resp == nil || resp.SN == "" {
		return
	}
	ep.recordMu.Lock()
	defer ep.recordMu.Unlock()
	w := ep.recordWait[resp.SN]
	if w == nil {
		return
	}
	if resp.SumNum > 0 {
		w.sumNum = resp.SumNum
	}
	w.items = append(w.items, resp.List.Items...)
	if w.sumNum > 0 && len(w.items) >= w.sumNum {
		if !w.done {
			w.done = true
			close(w.ch)
		}
		return
	}
	// SumNum==0 means no records
	if w.sumNum == 0 && len(resp.List.Items) == 0 {
		if !w.done {
			w.done = true
			close(w.ch)
		}
	}
}

// recipientURI 构造发往设备的 Request-URI。
// 优先用 REGISTER/Keepalive 的包源地址 RemoteAddr，而不是 Contact：
// 很多摄像机 Contact 填内网/映射前地址（如 192.168.1.100），实际 SIP 却从另一地址到达平台
// （如 192.168.1.24）；按 Contact 回发会导致 no route to host / catalog 超时。
func recipientURI(dev *Device) (sip.Uri, error) {
	user := strings.TrimSpace(dev.ID)
	if host, portStr, err := net.SplitHostPort(strings.TrimSpace(dev.RemoteAddr)); err == nil && host != "" {
		port, _ := strconv.Atoi(portStr)
		if user == "" {
			user = contactUser(dev.ContactURI)
		}
		return sip.Uri{User: user, Host: host, Port: port}, nil
	}
	if uri, ok := parseContactURI(dev.ContactURI); ok {
		if uri.User == "" {
			uri.User = user
		}
		return uri, nil
	}
	return sip.Uri{}, fmt.Errorf("bad remote addr %q (contact=%q)", dev.RemoteAddr, dev.ContactURI)
}

func parseContactURI(contact string) (sip.Uri, bool) {
	contact = strings.TrimSpace(contact)
	if contact == "" {
		return sip.Uri{}, false
	}
	uri := sip.Uri{}
	if err := sip.ParseUri(contact, &uri); err == nil {
		return uri, true
	}
	s := strings.TrimPrefix(contact, "<")
	s = strings.TrimSuffix(s, ">")
	if err := sip.ParseUri(s, &uri); err == nil {
		return uri, true
	}
	return sip.Uri{}, false
}

func contactUser(contact string) string {
	if uri, ok := parseContactURI(contact); ok {
		return uri.User
	}
	return ""
}
