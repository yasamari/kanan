package syoboi

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Program struct {
	LastUpdate  string `xml:"LastUpdate"`
	ID          int    `xml:"PID"`
	TitleID     int    `xml:"TID"`
	StartTime   string `xml:"StTime"`
	StartOffset int    `xml:"StOffset"`
	EndTime     string `xml:"EdTime"`
	Count       int    `xml:"Count"`
	STSubTitle  string `xml:"STSubTitle"`
	Comment     string `xml:"ProgComment"`
	Flag        int    `xml:"Flag"`
	Deleted     int    `xml:"Deleted"`
	Warn        int    `xml:"Warn"`
	ChannelID   int    `xml:"ChID"`
	Revision    int    `xml:"Revision"`
}

type Channel struct {
	ID       int    `xml:"ChID"`
	Name     string `xml:"ChName"`
	IEPGName string `xml:"ChiEPGName"`
	URL      string `xml:"ChURL"`
	EPGURL   string `xml:"ChEPGURL"`
	Comment  string `xml:"ChComment"`
	GroupID  int    `xml:"ChGID"`
	Number   int    `xml:"ChNumber"`
}

type Title struct {
	ID            int    `xml:"TID"`
	LastUpdate    string `xml:"LastUpdate"`
	Title         string `xml:"Title"`
	ShortTitle    string `xml:"ShortTitle"`
	TitleYomi     string `xml:"TitleYomi"`
	TitleEN       string `xml:"TitleEN"`
	Comment       string `xml:"Comment"`
	Cat           int    `xml:"Cat"`
	TitleFlag     int    `xml:"TitleFlag"`
	FirstYear     string `xml:"FirstYear"`
	FirstMonth    string `xml:"FirstMonth"`
	FirstEndYear  string `xml:"FirstEndYear"`
	FirstEndMonth string `xml:"FirstEndMonth"`
	FirstChannels string `xml:"FirstCh"`
	Keywords      string `xml:"Keywords"`
	UserPoint     int    `xml:"UserPoint"`
	UserPointRank int    `xml:"UserPointRank"`

	// *01*まさかの上京！ どうなる芽衣子!?
	// *02*ヘッジホッグ閉店!? 経営を立て直せ！
	// *03*因縁！ 猫vsハリネズミ
	SubTitles string `xml:"SubTitles"`
}

type Client interface {
	SearchProgramsByChannelAndTime(channelID int, startTime, endTime time.Time) ([]Program, error)
	GetTitleByID(titleID int64) (*Title, error)
	GetChannels() ([]Channel, error)
}

const (
	baseURL      = "http://cal.syoboi.jp/db.php"
	stTimeFormat = "20060102_150405"
)

type client struct {
	client http.Client
}

var _ Client = (*client)(nil)

func NewClient(httpClient http.Client) *client {
	return &client{
		client: httpClient,
	}
}

type Result struct {
	Code    int    `xml:"Code"`
	Message string `xml:"Message"`
}

func createRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Syoboi Client/1.0")

	return req, nil
}

func (c *client) SearchProgramsByChannelAndTime(channelID int, startTime, endTime time.Time) ([]Program, error) {
	// 20260102_000000-20261002_235959
	stTime := fmt.Sprintf("%s-%s", startTime.Format(stTimeFormat), endTime.Format(stTimeFormat))

	u, _ := url.Parse(baseURL)
	q := u.Query()
	q.Add("Command", "ProgLookup")
	q.Add("JOIN", "SubTitles")
	q.Add("ChID", strconv.Itoa(channelID))
	q.Add("StTime", stTime)
	u.RawQuery = q.Encode()

	req, err := createRequest(u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch programs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch programs: %d", resp.StatusCode)
	}

	var progLookupResponse struct {
		XMLName   xml.Name  `xml:"ProgLookupResponse"`
		ProgItems []Program `xml:"ProgItems>ProgItem"`
		Result    Result    `xml:"Result"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&progLookupResponse); err != nil {
		return nil, fmt.Errorf("failed to decode programs: %w", err)
	}

	return progLookupResponse.ProgItems, nil
}

func (c *client) GetTitleByID(titleID int64) (*Title, error) {
	u, _ := url.Parse(baseURL)
	q := u.Query()
	q.Add("Command", "TitleLookup")
	q.Add("TID", strconv.FormatInt(titleID, 10))
	u.RawQuery = q.Encode()

	req, err := createRequest(u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch title: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch title: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var titleLookupResponse struct {
		XMLName    xml.Name `xml:"TitleLookupResponse"`
		TitleItems []Title  `xml:"TitleItems>TitleItem"`
		Result     Result   `xml:"Result"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&titleLookupResponse); err != nil {
		return nil, fmt.Errorf("failed to decode title: %w", err)
	}

	if len(titleLookupResponse.TitleItems) == 0 {
		return nil, nil
	}

	return &titleLookupResponse.TitleItems[0], nil
}

func (c *client) GetChannels() ([]Channel, error) {
	u, _ := url.Parse(baseURL)
	q := u.Query()
	q.Add("Command", "ChLookup")
	u.RawQuery = q.Encode()

	req, err := createRequest(u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channels: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch channels: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var chLookupResponse struct {
		XMLName xml.Name  `xml:"ChLookupResponse"`
		ChItems []Channel `xml:"ChItems>ChItem"`
		Result  Result    `xml:"Result"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&chLookupResponse); err != nil {
		return nil, fmt.Errorf("failed to decode channels: %w", err)
	}

	return chLookupResponse.ChItems, nil
}
