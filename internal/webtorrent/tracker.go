package webtorrent

import (
	"context"
	"fmt"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/liuhengloveyou/FrameFlow/pkg/webtorrent"
)

var hashes map[string]map[string]*websocket.Conn

func tracker(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Warn().Err(err).Send()
		return
	}
	defer ws.CloseNow()

	ctx := context.Background()

	for {
		var msg webtorrent.Message
		if err = wsjson.Read(ctx, ws, &msg); err != nil {
			return
		}

		if msg.InfoHash == "" || msg.PeerId == "" {
			continue
		}

		if hashes == nil {
			hashes = map[string]map[string]*websocket.Conn{}
		}

		// new or old client with offers
		clients := hashes[msg.InfoHash]
		if clients == nil {
			clients = map[string]*websocket.Conn{
				msg.PeerId: ws,
			}
			hashes[msg.InfoHash] = clients
		} else {
			clients[msg.PeerId] = ws
		}

		switch {
		case msg.Offers != nil:
			// ask for ping
			raw := fmt.Sprintf(
				`{"action":"announce","interval":120,"info_hash":"%s","complete":0,"incomplete":1}`,
				msg.InfoHash,
			)
			if err = ws.Write(ctx, websocket.MessageText, []byte(raw)); err != nil {
				log.Warn().Err(err).Send()
				return
			}

			// skip if no offers (server)
			if len(msg.Offers) == 0 {
				continue
			}

			// get and check only first offer
			offer := msg.Offers[0]
			if offer.OfferId == "" || offer.Offer.Type != "offer" || offer.Offer.SDP == "" {
				continue
			}

			// send offer to all clients (one of them - server)
			raw = fmt.Sprintf(
				`{"action":"announce","info_hash":"%s","peer_id":"%s","offer_id":"%s","offer":{"type":"offer","sdp":"%s"}}`,
				msg.InfoHash, msg.PeerId, offer.OfferId, offer.Offer.SDP,
			)

			for _, server := range clients {
				if server != ws {
					_ = server.Write(ctx, websocket.MessageText, []byte(raw))
				}
			}

		case msg.OfferId != "" && msg.ToPeerId != "" && msg.Answer != nil:
			ws1, ok := clients[msg.ToPeerId]
			if !ok {
				continue
			}

			raw := fmt.Sprintf(
				`{"action":"announce","info_hash":"%s","peer_id":"%s","offer_id":"%s","answer":{"type":"answer","sdp":"%s"}}`,
				msg.InfoHash, msg.PeerId, msg.OfferId, msg.Answer.SDP,
			)
			_ = ws1.Write(ctx, websocket.MessageText, []byte(raw))
		}
	}
}
