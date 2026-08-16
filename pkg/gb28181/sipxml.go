package gb28181

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// MessageCmd is the CmdType inside GB28181 SIP MESSAGE XML body.
type MessageCmd struct {
	XMLName  xml.Name `xml:"Query"`
	CmdType  string   `xml:"CmdType"`
	SN       string   `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
}

// NotifyKeepalive is a Keepalive notify body (often wrapped as Notify).
type NotifyKeepalive struct {
	XMLName  xml.Name `xml:"Notify"`
	CmdType  string   `xml:"CmdType"`
	SN       string   `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	Status   string   `xml:"Status"`
}

// CatalogResponse holds DeviceList items from Catalog reply.
type CatalogResponse struct {
	XMLName    xml.Name       `xml:"Response"`
	CmdType    string         `xml:"CmdType"`
	SN         string         `xml:"SN"`
	DeviceID   string         `xml:"DeviceID"`
	SumNum     int            `xml:"SumNum"`
	DeviceList CatalogList    `xml:"DeviceList"`
}

type CatalogList struct {
	Num   string         `xml:"Num,attr"`
	Items []CatalogItem  `xml:"Item"`
}

type CatalogItem struct {
	DeviceID     string `xml:"DeviceID"`
	Name         string `xml:"Name"`
	Manufacturer string `xml:"Manufacturer"`
	Model        string `xml:"Model"`
	Owner        string `xml:"Owner"`
	CivilCode    string `xml:"CivilCode"`
	Address      string `xml:"Address"`
	Parental     string `xml:"Parental"`
	SafetyWay    string `xml:"SafetyWay"`
	RegisterWay  string `xml:"RegisterWay"`
	Secrecy      string `xml:"Secrecy"`
	Status       string `xml:"Status"`
	ParentID     string `xml:"ParentID"`
}

// ControlPTZ is DeviceControl PTZCmd payload wrapper.
type ControlPTZ struct {
	XMLName  xml.Name `xml:"Control"`
	CmdType  string   `xml:"CmdType"`
	SN       string   `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	PTZCmd   string   `xml:"PTZCmd"`
}

func sn() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000)
}

// BuildCatalogQuery builds a Catalog query MESSAGE body.
func BuildCatalogQuery(deviceID string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="GB2312"?>
<Query>
<CmdType>Catalog</CmdType>
<SN>%s</SN>
<DeviceID>%s</DeviceID>
</Query>
`, sn(), deviceID)
}

// BuildDeviceControl builds a DeviceControl MESSAGE body (PTZ etc).
func BuildDeviceControl(deviceID, ptzCmd string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="GB2312"?>
<Control>
<CmdType>DeviceControl</CmdType>
<SN>%s</SN>
<DeviceID>%s</DeviceID>
<PTZCmd>%s</PTZCmd>
</Control>
`, sn(), deviceID, ptzCmd)
}

// ParseMessageCmdType extracts CmdType from MESSAGE XML (Query/Response/Notify/Control).
func ParseMessageCmdType(body []byte) string {
	s := string(body)
	if i := strings.Index(s, "<CmdType>"); i >= 0 {
		j := strings.Index(s[i:], "</CmdType>")
		if j > 0 {
			return strings.TrimSpace(s[i+len("<CmdType>") : i+j])
		}
	}
	return ""
}

// ParseKeepaliveDeviceID returns device id from keepalive notify.
func ParseKeepaliveDeviceID(body []byte) string {
	var n NotifyKeepalive
	if err := xml.Unmarshal(stripXMLDecl(body), &n); err == nil && n.DeviceID != "" {
		return n.DeviceID
	}
	// fallback: some devices use Response wrapper
	type keepaliveResp struct {
		DeviceID string `xml:"DeviceID"`
	}
	var r keepaliveResp
	_ = xml.Unmarshal(stripXMLDecl(body), &r)
	return r.DeviceID
}

func stripXMLDecl(b []byte) []byte {
	s := string(b)
	if i := strings.Index(s, "?>"); i >= 0 && strings.Contains(s[:i+2], "<?xml") {
		return []byte(strings.TrimSpace(s[i+2:]))
	}
	return b
}

// ParseCatalogResponse parses Catalog response MESSAGE body (Response or Notify root).
func ParseCatalogResponse(body []byte) (*CatalogResponse, error) {
	raw := stripXMLDecl(body)
	var resp CatalogResponse
	if err := xml.Unmarshal(raw, &resp); err == nil &&
		(strings.EqualFold(resp.CmdType, "Catalog") || len(resp.DeviceList.Items) > 0) {
		if resp.CmdType == "" {
			resp.CmdType = "Catalog"
		}
		return &resp, nil
	}
	// Some IPC wrap Catalog reply as Notify
	type notifyCatalog struct {
		XMLName    xml.Name    `xml:"Notify"`
		CmdType    string      `xml:"CmdType"`
		SN         string      `xml:"SN"`
		DeviceID   string      `xml:"DeviceID"`
		SumNum     int         `xml:"SumNum"`
		DeviceList CatalogList `xml:"DeviceList"`
	}
	var n notifyCatalog
	if err := xml.Unmarshal(raw, &n); err != nil {
		return nil, fmt.Errorf("catalog xml: %w", err)
	}
	if n.CmdType != "" && !strings.EqualFold(n.CmdType, "Catalog") && len(n.DeviceList.Items) == 0 {
		return nil, fmt.Errorf("catalog xml: unexpected cmd %q", n.CmdType)
	}
	return &CatalogResponse{
		CmdType: n.CmdType, SN: n.SN, DeviceID: n.DeviceID,
		SumNum: n.SumNum, DeviceList: n.DeviceList,
	}, nil
}

// ChannelsFromCatalog converts catalog items to Channel list.
// When both the device itself and media channels appear, drops the device self-entry.
func ChannelsFromCatalog(deviceID string, items []CatalogItem) []*Channel {
	out := make([]*Channel, 0, len(items))
	for _, it := range items {
		id := strings.TrimSpace(it.DeviceID)
		if id == "" {
			continue
		}
		out = append(out, &Channel{
			ID:           id,
			DeviceID:     deviceID,
			Name:         it.Name,
			Manufacturer: it.Manufacturer,
			Model:        it.Model,
			Owner:        it.Owner,
			CivilCode:    it.CivilCode,
			Address:      it.Address,
			Parental:     it.Parental,
			SafetyWay:    it.SafetyWay,
			RegisterWay:  it.RegisterWay,
			Secrecy:      it.Secrecy,
			Status:       it.Status,
		})
	}
	media := make([]*Channel, 0, len(out))
	for _, ch := range out {
		if ch.ID != deviceID {
			media = append(media, ch)
		}
	}
	if len(media) > 0 {
		return media
	}
	return out
}
