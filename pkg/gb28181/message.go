package gb28181

import (
	"github.com/emiago/sipgo/sip"
)

func (ep *SIPEndpoint) onMessage(req *sip.Request, tx sip.ServerTransaction) {
	deviceID := deviceIDFromFrom(req)
	remote := req.Source()
	body := req.Body()
	cmd := ""
	if len(body) > 0 {
		cmd = ParseMessageCmdType(body)
	}

	ep.Log.Info().
		Str("method", string(req.Method)).
		Str("device", deviceID).
		Str("remote", remote).
		Str("cmd", cmd).
		Int("body_len", len(body)).
		Msg("[gb28181] sip: message")

	if len(body) == 0 {
		_ = tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
		return
	}

	switch cmd {
	case "Keepalive":
		id := ParseKeepaliveDeviceID(body)
		if id == "" {
			id = deviceID
		}
		if id == "" {
			_ = tx.Respond(sip.NewResponseFromRequest(req, 400, "Bad Request", nil))
			ep.Log.Warn().Str("remote", remote).Msg("[gb28181] keepalive: reject 400 (empty device id)")
			return
		}
		ok := true
		if ep.Handler != nil {
			ok = ep.Handler.OnKeepalive(id, remote)
		}
		if !ok {
			// 未登记 / 已拒绝：拒绝保活，促使设备重新 REGISTER
			_ = tx.Respond(sip.NewResponseFromRequest(req, 404, "Not Found", nil))
			ep.Log.Warn().
				Str("device", id).
				Str("remote", remote).
				Msg("[gb28181] keepalive: reject 404 (unknown or blocked)")
			return
		}

		// 成功应答
		if ep.GW != nil && ep.GW.Registry != nil {
			ep.GW.Registry.TouchKeepalive(id)
		}
		_ = tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
		ep.Log.Info().Str("device", id).Str("remote", remote).Msg("[gb28181] keepalive")

	case "Catalog":
		_ = tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
		resp, err := ParseCatalogResponse(body)
		if err != nil {
			ep.Log.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] catalog: parse failed")
			return
		}
		id := resp.DeviceID
		if id == "" {
			id = deviceID
		}
		chs := ChannelsFromCatalog(id, resp.DeviceList.Items)
		if ep.GW != nil && ep.GW.Registry != nil {
			ep.GW.Registry.UpsertChannels(id, chs)
		}
		if ep.Handler != nil {
			ep.Handler.OnCatalog(id, chs)
		}
		ep.Log.Info().
			Str("device", id).
			Int("channels", len(chs)).
			Int("sum_num", resp.SumNum).
			Msg("[gb28181] catalog: synced")

	case "RecordInfo":
		_ = tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
		resp, err := ParseRecordInfoResponse(body)
		if err != nil {
			ep.Log.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] recordinfo: parse failed")
			return
		}
		ep.onRecordInfoResponse(resp)
		ep.Log.Info().
			Str("device", deviceID).
			Str("sn", resp.SN).
			Int("sum_num", resp.SumNum).
			Int("items", len(resp.List.Items)).
			Msg("[gb28181] recordinfo: packet")

	default:
		_ = tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
		ep.Log.Info().Str("cmd", cmd).Str("device", deviceID).Msg("[gb28181] sip: manscdp")
	}
}
