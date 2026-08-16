package hass

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type API struct {
	ws *websocket.Conn
}

func NewAPI(rawURL, token string) (*API, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ws, _, err := websocket.Dial(ctx, rawURL, nil)
	if err != nil {
		return nil, err
	}

	api := &API{ws: ws}
	if err = api.Auth(token); err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, "")
		return nil, err
	}

	return api, nil
}

func (a *API) Auth(token string) error {
	ctx := context.Background()
	var res ResponseAuth

	if err := wsjson.Read(ctx, a.ws, &res); err != nil {
		return err
	}
	if res.Type != "auth_required" {
		return errors.New("hass: wrong type: " + res.Type)
	}

	s := `{"type":"auth","access_token":"` + token + `"}`
	if err := a.ws.Write(ctx, websocket.MessageText, []byte(s)); err != nil {
		return err
	}
	if err := wsjson.Read(ctx, a.ws, &res); err != nil {
		return err
	}
	if res.Type != "auth_ok" {
		return errors.New("hass: wrong type: " + res.Type)
	}

	return nil
}

func (a *API) Close() error {
	return a.ws.Close(websocket.StatusNormalClosure, "")
}

func (a *API) ExchangeSDP(entityID, offer string) (string, error) {
	ctx := context.Background()
	var msg = map[string]any{
		"id":        1,
		"type":      "camera/web_rtc_offer",
		"entity_id": entityID,
		"offer":     offer,
	}
	if err := wsjson.Write(ctx, a.ws, msg); err != nil {
		return "", err
	}

	var res ResponseOffer
	if err := wsjson.Read(ctx, a.ws, &res); err != nil {
		return "", err
	}

	if res.Type != "result" || !res.Success {
		return "", errors.New("hass: wrong response")
	}

	return res.Result.Answer, nil
}

func (a *API) GetWebRTCEntities() (map[string]string, error) {
	ctx := context.Background()
	s := `{"id":1,"type":"get_states"}`
	if err := a.ws.Write(ctx, websocket.MessageText, []byte(s)); err != nil {
		return nil, err
	}

	var res ResponseStates
	if err := wsjson.Read(ctx, a.ws, &res); err != nil {
		return nil, err
	}
	if res.Type != "result" || !res.Success {
		return nil, errors.New("hass: wrong response")
	}

	entities := map[string]string{}

	for _, entity := range res.Result {
		if entity.Attributes.FrontendStreamType == "web_rtc" {
			entities[entity.Attributes.FriendlyName] = entity.EntityId
		}
	}

	return entities, nil
}

type ResponseAuth struct {
	Type string `json:"type"`
}

type ResponseStates struct {
	Type    string `json:"type"`
	Success bool   `json:"success"`
	Result  []struct {
		EntityId   string `json:"entity_id"`
		Attributes struct {
			FrontendStreamType string `json:"frontend_stream_type"`
			FriendlyName       string `json:"friendly_name"`
		} `json:"attributes"`
	} `json:"result"`
}

type ResponseOffer struct {
	Type    string `json:"type"`
	Success bool   `json:"success"`
	Result  struct {
		Answer string `json:"answer"`
	} `json:"result"`
}

func SupervisorToken() string {
	return os.Getenv("SUPERVISOR_TOKEN")
}
