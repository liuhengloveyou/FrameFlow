package gb28181

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// RecordItem is one recording segment from a RecordInfo response.
type RecordItem struct {
	DeviceID   string `xml:"DeviceID" json:"device_id,omitempty"`
	Name       string `xml:"Name" json:"name,omitempty"`
	FilePath   string `xml:"FilePath" json:"file_path,omitempty"`
	Address    string `xml:"Address" json:"address,omitempty"`
	StartTime  string `xml:"StartTime" json:"start_time"`
	EndTime    string `xml:"EndTime" json:"end_time"`
	Secrecy    int    `xml:"Secrecy" json:"secrecy,omitempty"`
	Type       string `xml:"Type" json:"type,omitempty"`
	RecorderID string `xml:"RecorderID" json:"recorder_id,omitempty"`
	FileSize   string `xml:"FileSize" json:"file_size,omitempty"`
}

// RecordInfoResponse is a RecordInfo MESSAGE body (possibly one of many packets).
type RecordInfoResponse struct {
	XMLName  xml.Name     `xml:"Response"`
	CmdType  string       `xml:"CmdType"`
	SN       string       `xml:"SN"`
	DeviceID string       `xml:"DeviceID"`
	Name     string       `xml:"Name"`
	SumNum   int          `xml:"SumNum"`
	List     RecordList   `xml:"RecordList"`
}

type RecordList struct {
	Num   string       `xml:"Num,attr"`
	Items []RecordItem `xml:"Item"`
}

// BuildRecordInfoQuery builds a RecordInfo query MESSAGE body.
// Times use RFC3339-like local form without timezone (GB28181 common).
func BuildRecordInfoQuery(channelID string, start, end time.Time) (snVal, body string) {
	snVal = sn()
	body = fmt.Sprintf(`<?xml version="1.0" encoding="GB2312"?>
<Query>
<CmdType>RecordInfo</CmdType>
<SN>%s</SN>
<DeviceID>%s</DeviceID>
<StartTime>%s</StartTime>
<EndTime>%s</EndTime>
<Secrecy>0</Secrecy>
<Type>all</Type>
</Query>
`, snVal, channelID, start.Format("2006-01-02T15:04:05"), end.Format("2006-01-02T15:04:05"))
	return snVal, body
}

// ParseRecordInfoResponse parses a RecordInfo response MESSAGE.
func ParseRecordInfoResponse(body []byte) (*RecordInfoResponse, error) {
	raw := stripXMLDecl(body)
	var resp RecordInfoResponse
	if err := xml.Unmarshal(raw, &resp); err == nil &&
		(strings.EqualFold(resp.CmdType, "RecordInfo") || len(resp.List.Items) > 0) {
		if resp.CmdType == "" {
			resp.CmdType = "RecordInfo"
		}
		return &resp, nil
	}
	type notifyRecord struct {
		XMLName  xml.Name   `xml:"Notify"`
		CmdType  string     `xml:"CmdType"`
		SN       string     `xml:"SN"`
		DeviceID string     `xml:"DeviceID"`
		Name     string     `xml:"Name"`
		SumNum   int        `xml:"SumNum"`
		List     RecordList `xml:"RecordList"`
	}
	var n notifyRecord
	if err := xml.Unmarshal(raw, &n); err != nil {
		return nil, fmt.Errorf("recordinfo xml: %w", err)
	}
	if n.CmdType != "" && !strings.EqualFold(n.CmdType, "RecordInfo") && len(n.List.Items) == 0 {
		return nil, fmt.Errorf("recordinfo xml: unexpected cmd %q", n.CmdType)
	}
	return &RecordInfoResponse{
		CmdType: n.CmdType, SN: n.SN, DeviceID: n.DeviceID,
		Name: n.Name, SumNum: n.SumNum, List: n.List,
	}, nil
}

// ParseRecordTime parses common GB28181 record time strings.
func ParseRecordTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
	}
	var last error
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, s, time.Local)
		if err == nil {
			return t, nil
		}
		last = err
	}
	return time.Time{}, last
}
