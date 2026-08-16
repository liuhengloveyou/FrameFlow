package gb28181

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/icholy/digest"
)

func (ep *SIPEndpoint) onRegister(req *sip.Request, tx sip.ServerTransaction) {
	remote := req.Source()
	transport := strings.ToLower(req.Transport())
	hasAuth := req.GetHeader("Authorization") != nil

	deviceID := deviceIDFromFrom(req)
	ep.Log.Info().
		Str("device", deviceID).
		Str("remote", remote).
		Str("transport", transport).
		Bool("has_auth", hasAuth).
		Msg("[gb28181] register: received")

	if deviceID == "" {
		ep.Log.Warn().
			Str("remote", remote).
			Msg("[gb28181] register: reject 400 (empty From user / device id)")
		_ = tx.Respond(sip.NewResponseFromRequest(req, 400, "Bad Request", nil))
		return
	}

	expires := 3600
	if h := req.GetHeader("Expires"); h != nil {
		if v, err := strconv.Atoi(h.Value()); err == nil {
			expires = v
		}
	}
	ep.Log.Debug().
		Str("device", deviceID).
		Int("expires", expires).
		Msg("[gb28181] register: parsed expires")

	if expires == 0 {
		ep.GW.Registry.SetOffline(deviceID)
		ep.Handler.OnUnregistered(deviceID)
		_ = tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
		ep.Log.Info().
			Str("device", deviceID).
			Str("remote", remote).
			Msg("[gb28181] register: unregister ok")
		return
	}

	auth, err := ep.Handler.LookupDevice(deviceID)
	if err != nil {
		ep.Log.Error().Err(err).Str("device", deviceID).Msg("[gb28181] register: lookup failed")
		_ = tx.Respond(sip.NewResponseFromRequest(req, 500, "Server Error", nil))
		return
	}

	if !auth.Known {
		auth, err = ep.Handler.OnDiscover(deviceID, remote)
		if err != nil {
			ep.Log.Error().Err(err).Str("device", deviceID).Msg("[gb28181] register: discover/persist failed")
			_ = tx.Respond(sip.NewResponseFromRequest(req, 500, "Server Error", nil))
			return
		}
		ep.Log.Info().
			Str("device", deviceID).
			Str("status", auth.Status).
			Msg("[gb28181] register: discovered (awaiting admin confirm)")
	}

	if !AllowsRegister(auth.Status) {
		ep.Log.Warn().
			Str("device", deviceID).
			Str("status", auth.Status).
			Str("remote", remote).
			Msg("[gb28181] register: reject 403 (status not allowed)")
		_ = tx.Respond(sip.NewResponseFromRequest(req, 403, "Forbidden", nil))
		return
	}

	// Platform password empty => skip Digest entirely (device password ignored).
	password := ep.GW.Config.Password
	if password != "" && auth.Password != "" {
		password = auth.Password
	}

	ep.Log.Info().
		Str("device", deviceID).
		Str("status", auth.Status).
		Bool("auth_required", password != "").
		Msg("[gb28181] register: lookup")

	if password != "" {
		if !ep.checkDigest(req, tx, deviceID, password) {
			return
		}
	} else {
		ep.Log.Debug().
			Str("device", deviceID).
			Msg("[gb28181] register: skip digest (platform password empty)")
	}

	contact := ""
	if c := req.Contact(); c != nil {
		contact = c.Address.String()
	}

	now := time.Now()
	dev := &Device{
		ID:            deviceID,
		Transport:     transport,
		RemoteAddr:    remote,
		ContactURI:    contact,
		Status:        auth.Status,
		Online:        true,
		Registered:    now,
		Expires:       time.Duration(expires) * time.Second,
		LastKeepalive: now,
	}
	ep.GW.Registry.MergeOnline(dev)
	ep.Handler.OnRegistered(dev)

	res := sip.NewResponseFromRequest(req, 200, "OK", nil)
	res.AppendHeader(sip.NewHeader("Expires", strconv.Itoa(expires)))
	_ = tx.Respond(res)
	ep.Log.Info().
		Str("device", deviceID).
		Str("status", auth.Status).
		Str("remote", remote).
		Str("contact", contact).
		Int("expires", expires).
		Str("transport", transport).
		Msg("[gb28181] register: ok")

	if ep.AutoCatalogAfterRegister {
		go func() {
			time.Sleep(300 * time.Millisecond)
			ep.Log.Info().Str("device", deviceID).Msg("[gb28181] register: send catalog query")
			if err := ep.SendCatalog(deviceID); err != nil {
				ep.Log.Warn().Err(err).Str("device", deviceID).Msg("[gb28181] register: catalog query failed")
			}
		}()
	}
}

// checkDigest returns false if the response was already sent (challenge or reject).
func (ep *SIPEndpoint) checkDigest(req *sip.Request, tx sip.ServerTransaction, deviceID, password string) bool {
	auth := req.GetHeader("Authorization")
	if auth == nil {
		chal := digest.Challenge{
			Realm:     ep.GW.Config.Realm,
			Nonce:     fmt.Sprintf("%d", time.Now().UnixNano()),
			Algorithm: "MD5",
		}
		ep.nonceMu.Lock()
		ep.nonces[deviceID] = chal
		ep.nonceMu.Unlock()
		res := sip.NewResponseFromRequest(req, 401, "Unauthorized", nil)
		res.AppendHeader(sip.NewHeader("WWW-Authenticate", chal.String()))
		_ = tx.Respond(res)
		ep.Log.Info().
			Str("device", deviceID).
			Str("realm", ep.GW.Config.Realm).
			Msg("[gb28181] register: challenge 401 (digest)")
		return false
	}

	cred, err := digest.ParseCredentials(auth.Value())
	if err != nil {
		ep.Log.Warn().Err(err).
			Str("device", deviceID).
			Msg("[gb28181] register: reject 401 (bad Authorization)")
		_ = tx.Respond(sip.NewResponseFromRequest(req, 401, "Unauthorized", nil))
		return false
	}

	ep.nonceMu.Lock()
	chal, ok := ep.nonces[deviceID]
	ep.nonceMu.Unlock()
	if !ok {
		chal = digest.Challenge{Realm: ep.GW.Config.Realm, Nonce: cred.Nonce, Algorithm: "MD5"}
		ep.Log.Debug().
			Str("device", deviceID).
			Msg("[gb28181] register: nonce miss, reuse client nonce")
	}

	dig, err := digest.Digest(&chal, digest.Options{
		Method:   "REGISTER",
		URI:      cred.URI,
		Username: cred.Username,
		Password: password,
	})
	if err != nil || cred.Response != dig.Response {
		ep.Log.Warn().
			Str("device", deviceID).
			Str("username", cred.Username).
			Msg("[gb28181] register: reject 403 (digest mismatch)")
		_ = tx.Respond(sip.NewResponseFromRequest(req, 403, "Forbidden", nil))
		return false
	}

	ep.Log.Debug().
		Str("device", deviceID).
		Str("username", cred.Username).
		Msg("[gb28181] register: digest ok")
	return true
}
