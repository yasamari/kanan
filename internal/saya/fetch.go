package saya

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Channel struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	ServiceIDs []int  `json:"serviceIds"`
	SyobocalID int    `json:"syobocalId"`
	AnnictID   int    `json:"annictId"`
}

const url = "https://github.com/SlashNephy/saya-definitions/raw/refs/heads/master/definitions.json"

func fetchSayaDefinitions() ([]Channel, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Saya definitions: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var response struct {
		Channels []Channel `json:"channels"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode Saya definitions: %w", err)
	}

	return response.Channels, nil
}

func GetServiceToChannelIDMap() (map[int]int, error) {
	channels, err := fetchSayaDefinitions()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Saya definitions: %w", err)
	}

	serviceToChannelID := make(map[int]int)
	for _, ch := range channels {
		for _, sid := range ch.ServiceIDs {
			serviceToChannelID[sid] = ch.SyobocalID
		}
	}

	return serviceToChannelID, nil
}
