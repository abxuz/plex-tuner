package plex

import "encoding/json"

type Channel struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Type string `json:"type"`
	Icon string `json:"icon"`
}

func getChannel(p string) ([]*Channel, error) {
	data, err := getContent(p)
	if err != nil {
		return nil, err
	}
	list := make([]*Channel, 0)
	err = json.Unmarshal(data, &list)
	if err != nil {
		return nil, err
	}
	return list, nil
}
