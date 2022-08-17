package plex

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
)

type Config struct {
	ID         string `json:"id"`
	TunerCount int    `json:"tuner_count"`
	Listen     string `json:"listen"`
	FFMpeg     string `json:"ffmpeg"`
	Channel    string `json:"channel"`
	Log        string `json:"log"`
}

func loadConfig(name string) (*Config, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	c := new(Config)
	err = json.NewDecoder(f).Decode(&c)
	if err != nil {
		return nil, err
	}
	if err = checkConfig(c); err != nil {
		return nil, err
	}
	return c, nil
}

func checkConfig(c *Config) error {
	c.ID = strings.TrimSpace(c.ID)
	if c.ID == "" {
		return errors.New("id missing in config file")
	}
	if c.TunerCount < 1 {
		c.TunerCount = 1
	}
	c.Listen = strings.TrimSpace(c.Listen)
	c.FFMpeg = strings.TrimSpace(c.FFMpeg)
	c.Channel = strings.TrimSpace(c.Channel)
	c.Log = strings.TrimSpace(c.Log)
	return nil
}
