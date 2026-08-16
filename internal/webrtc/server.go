package webrtc

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/webrtc"
	pion "github.com/pion/webrtc/v4"
)

const MimeSDP = "application/sdp"

var sessions = map[string]*webrtc.Conn{}

func syncHandler(w http.ResponseWriter, r *http.Request) {
	remote := r.RemoteAddr
	switch r.Method {
	case "POST":
		query := r.URL.Query()
		if query.Get("src") != "" {
			// WHEP or JSON SDP or raw SDP exchange
			outputWebRTC(w, r)
		} else if query.Get("dst") != "" {
			// WHIP SDP exchange
			inputWebRTC(w, r)
		} else {
			log.Warn().
				Str("remote", remote).
				Str("method", r.Method).
				Msg("[webrtc] http: reject post without src or dst")
			http.Error(w, "", http.StatusBadRequest)
		}

	case "PATCH":
		log.Info().Str("remote", remote).Msg("[webrtc] http: patch not implemented (whip/whep)")
		http.Error(w, "", http.StatusMethodNotAllowed)

	case "DELETE":
		if id := r.URL.Query().Get("id"); id != "" {
			if conn, ok := sessions[id]; ok {
				delete(sessions, id)
				_ = conn.Close()
				log.Info().
					Str("session_id", id).
					Str("remote", remote).
					Msg("[webrtc] http whip: session deleted")
			} else {
				log.Warn().
					Str("session_id", id).
					Str("remote", remote).
					Msg("[webrtc] http whip: delete unknown session")
				http.Error(w, "", http.StatusNotFound)
			}
		} else {
			log.Warn().Str("remote", remote).Msg("[webrtc] http whip: delete missing id")
			http.Error(w, "", http.StatusBadRequest)
		}

	case "OPTIONS":
		log.Debug().Str("remote", remote).Msg("[webrtc] http: options")
		w.WriteHeader(http.StatusNoContent)

	default:
		log.Warn().
			Str("remote", remote).
			Str("method", r.Method).
			Msg("[webrtc] http: method not allowed")
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
}

// outputWebRTC support API depending on Content-Type:
// 1. application/json - receive {"type":"offer","sdp":"v=0\r\n..."} and response {"type":"answer","sdp":"v=0\r\n..."}
// 2. application/sdp - receive/response SDP via WebRTC-HTTP Egress Protocol (WHEP)
// 3. other - receive/response raw SDP
func outputWebRTC(w http.ResponseWriter, r *http.Request) {
	u := r.URL.Query().Get("src")
	remote := r.RemoteAddr

	mediaType := r.Header.Get("Content-Type")
	if mediaType != "" {
		mediaType, _, _ = strings.Cut(mediaType, ";")
		mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	}

	log.Info().
		Str("src", u).
		Str("content_type", mediaType).
		Str("remote", remote).
		Str("method", r.Method).
		Msg("[webrtc] http pull: request")

	stream := streams.Get(u)
	if stream == nil {
		log.Warn().Str("src", u).Str("remote", remote).Msg("[webrtc] http pull: stream not found")
		http.Error(w, api.StreamNotFound, http.StatusNotFound)
		return
	}

	var offer string

	switch mediaType {
	case "application/json":
		var desc pion.SessionDescription
		if err := json.NewDecoder(r.Body).Decode(&desc); err != nil {
			log.Error().Err(err).Str("src", u).Str("remote", remote).Caller().Send()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		offer = desc.SDP

	case "application/x-www-form-urlencoded":
		if err := r.ParseForm(); err != nil {
			log.Error().Err(err).Str("src", u).Str("remote", remote).Caller().Send()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		offerB64 := r.Form.Get("data")
		b, err := base64.StdEncoding.DecodeString(offerB64)
		if err != nil {
			log.Error().Err(err).Str("src", u).Str("remote", remote).Caller().Send()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		offer = string(b)

	default:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error().Err(err).Str("src", u).Str("remote", remote).Caller().Send()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		offer = string(body)
	}

	var desc string

	switch mediaType {
	case "application/json":
		desc = "webrtc/json"
	case MimeSDP:
		desc = "webrtc/whep"
	default:
		desc = "webrtc/post"
	}

	answer, err := ExchangeSDP(stream, offer, desc, r.UserAgent(), u)
	if err != nil {
		log.Error().
			Err(err).
			Str("src", u).
			Str("desc", desc).
			Str("remote", remote).
			Int("offer_len", len(offer)).
			Caller().Send()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("src", u).
		Str("desc", desc).
		Str("remote", remote).
		Int("offer_len", len(offer)).
		Int("answer_len", len(answer)).
		Msg("[webrtc] http pull: sdp exchanged ok")

	switch mediaType {
	case "application/json":
		w.Header().Set("Content-Type", mediaType)

		v := pion.SessionDescription{
			Type: pion.SDPTypeAnswer, SDP: answer,
		}
		err = json.NewEncoder(w).Encode(v)

	case "application/x-www-form-urlencoded":
		w.Header().Set("Content-Type", mediaType)
		answerB64 := base64.StdEncoding.EncodeToString([]byte(answer))
		_, err = w.Write([]byte(answerB64))

	case MimeSDP:
		w.Header().Set("Content-Type", mediaType)
		w.WriteHeader(http.StatusCreated)

		_, err = w.Write([]byte(answer))

	default:
		w.Header().Set("Content-Type", mediaType)

		_, err = w.Write([]byte(answer))
	}

	if err != nil {
		log.Error().
			Err(err).
			Str("src", u).
			Str("remote", remote).
			Str("content_type", mediaType).
			Caller().Send()
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func inputWebRTC(w http.ResponseWriter, r *http.Request) {
	dst := r.URL.Query().Get("dst")
	remote := r.RemoteAddr
	ct := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))

	log.Info().
		Str("dst", dst).
		Str("remote", remote).
		Str("content_type", ct).
		Msg("[webrtc] http push: whip request")

	stream := streams.Get(dst)
	if stream == nil {
		log.Warn().Str("dst", dst).Str("remote", remote).Msg("[webrtc] http push: stream not found")
		http.Error(w, api.StreamNotFound, http.StatusNotFound)
		return
	}

	// 1. Get offer
	offer, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Str("dst", dst).Str("remote", remote).Caller().Send()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Trace().Msgf("[webrtc] WHIP offer\n%s", offer)

	pc, err := PeerConnection(false)
	if err != nil {
		log.Error().Err(err).Str("dst", dst).Str("remote", remote).Caller().Send()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// create new webrtc instance
	prod := webrtc.NewConn(pc)
	prod.Mode = core.ModePassiveProducer
	prod.Protocol = "http"
	prod.UserAgent = r.UserAgent()

	if err = prod.SetOffer(string(offer)); err != nil {
		log.Warn().Err(err).Str("dst", dst).Str("remote", remote).Caller().Send()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	answer, err := prod.GetCompleteAnswer(GetCandidates(), FilterCandidate)
	if err != nil {
		log.Warn().Err(err).Str("dst", dst).Str("remote", remote).Caller().Send()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Trace().Msgf("[webrtc] WHIP answer\n%s", answer)

	id := strconv.FormatInt(time.Now().UnixNano(), 36)
	sessions[id] = prod

	prod.Listen(func(msg any) {
		switch msg := msg.(type) {
		case pion.PeerConnectionState:
			if msg == pion.PeerConnectionStateClosed {
				stream.RemoveProducer(prod)
				delete(sessions, id)
				log.Info().
					Str("session_id", id).
					Str("dst", dst).
					Msg("[webrtc] http push: producer pc closed")
			}
		}
	})

	stream.AddProducer(prod)

	w.Header().Set("Content-Type", MimeSDP)
	w.Header().Set("Location", "webrtc?id="+id)
	w.WriteHeader(http.StatusCreated)

	if _, err = w.Write([]byte(answer)); err != nil {
		log.Warn().Err(err).Str("dst", dst).Str("session_id", id).Str("remote", remote).Caller().Send()
		return
	}

	log.Info().
		Str("dst", dst).
		Str("session_id", id).
		Str("remote", remote).
		Int("offer_len", len(offer)).
		Int("answer_len", len(answer)).
		Msg("[webrtc] http push: whip answer sent")
}
