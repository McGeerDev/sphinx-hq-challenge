package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// https://challenge.sphinxhq.com/
// 1000 morties on starting point
// 3 planets with varying survival rates
// 0 morties on the end point
//

const (
	BASE_URL = "https://challenge.sphinxhq.com"

	// ENDPOINTS
	START_ENDPOINT  = "/api/mortys/start/"
	PORTAL_ENDPOINT = "/api/mortys/portal/"
	STATUS_ENDPOINT = "/api/mortys/status/"
	// UTILS
	RATE_LIMIT = 30 * time.Second
)

var (
	AUTH_HEADER string
)

type Status struct {
	MortiesInCitadel       int    `json:"morties_in_citadel"`
	MortiesOnPlanetJessica int    `json:"morties_on_planet_jessica"`
	MortiesLost            int    `json:"morties_lost"`
	StepsTaken             int    `json:"steps_taken"`
	StatusMessage          string `json:"status_message"`
}

type Portal struct {
	MortiesSent            int  `json:"morties_sent"`
	Survived               bool `json:"survived"`
	MortiesInCitadel       int  `json:"morties_in_citadel"`
	MortiesOnPlanetJessica int  `json:"morties_on_planet_jessica"`
	MortiesLost            int  `json:"morties_lost"`
	StepsTaken             int  `json:"steps_taken"`
}

type Planet int

const (
	OnACob Planet = iota
	CronenBergWorld
	PurgePlanet
)

func main() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	AUTH_HEADER = os.Getenv("AUTH_HEADER")

	client := &http.Client{}

	start := StartEpisode(client)
	slog.Info(fmt.Sprintf("StartState: %+v", start))
	mortiesCount := start.MortiesInCitadel
	cobMorties := 2
	cronMorties := 2
	purgeMorties := 2

	for mortiesCount > 0 {
		if cobMorties+cronMorties+purgeMorties > mortiesCount {
			cobMorties = 3
			cronMorties = mortiesCount - cobMorties
			purgeMorties = mortiesCount - cronMorties
		}
		onACobSurvived := SendMorties(client, cobMorties, OnACob)
		if onACobSurvived && cobMorties < 3 {
			cobMorties++
		} else if !onACobSurvived && cobMorties > 1 {
			cobMorties--
		}

		cronenbergWorldSurvived := SendMorties(client, cronMorties, CronenBergWorld)
		if cronenbergWorldSurvived && cronMorties < 3 {
			cronMorties++
		} else if !cronenbergWorldSurvived && cronMorties > 1 {
			cronMorties--
		}

		purgePlanetSurvived := SendMorties(client, purgeMorties, PurgePlanet)
		if purgePlanetSurvived && purgeMorties < 3 {
			purgeMorties++
		} else if !purgePlanetSurvived && purgeMorties > 1 {
			purgeMorties--
		}

		status := GetEpisodeStatus(client)
		slog.Info("Status", "value", fmt.Sprintf("%+v", status))

		// Update mortyCount
		mortiesCount = status.MortiesInCitadel
	}
}

type SendMorty struct {
	Planet     int `json:"planet"`
	MortyCount int `json:"morty_count"`
}

func SendMorties(client *http.Client, mortyCount int, planet Planet) bool {
	slog.Info(
		"send morties parameters",
		"mortyCount", mortyCount,
		"planet", planet,
	)
	sm := &SendMorty{Planet: 0, MortyCount: 2}
	jsonBody, err := json.Marshal(*sm)
	if err != nil {
		slog.Error(err.Error())
	}

	bytesReader := bytes.NewReader(jsonBody)

	req, err := http.NewRequest("POST", fmt.Sprintf("%s%s", BASE_URL, PORTAL_ENDPOINT), bytesReader)
	if err != nil {
		slog.Error(err.Error())
	}

	req.Header.Set("Authorization", AUTH_HEADER)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		slog.Error(err.Error())
	}
	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		slog.Error("error reading response body",
			"error", err.Error(),
		)
	}

	portal := &Portal{}
	err = json.Unmarshal(b, portal)
	if err != nil {
		slog.Error("error unmarshalling response body",
			"error", err.Error(),
			"response body", string(b),
		)
	}
	return portal.Survived
}

func StartEpisode(client *http.Client) Status {
	slog.Debug("Starting Episode")
	req, err := http.NewRequest("POST", fmt.Sprintf("%s%s", BASE_URL, START_ENDPOINT), nil)
	if err != nil {
		slog.Error(
			"error creating request",
			"error", err.Error())
	}
	req.Header.Set("Authorization", AUTH_HEADER)
	res, err := client.Do(req)
	if err != nil {
		slog.Error("error sending request",
			"error", err.Error())
	}

	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		slog.Error("error reading response body",
			"error", err.Error(),
		)
	}

	start := &Status{}
	err = json.Unmarshal(b, start)
	if err != nil {
		slog.Error("error unmarshalling response body",
			"error", err.Error(),
			"response body", string(b),
		)
	}

	return *start
}

func GetEpisodeStatus(client *http.Client) Status {
	slog.Debug("Episode Status")
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s%s", BASE_URL, STATUS_ENDPOINT), nil)
	req.Header.Set("Authorization", AUTH_HEADER)
	res, err := client.Do(req)
	if err != nil {
		slog.Error("error sending request",
			"error", err.Error())
	}

	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		slog.Error("error reading response body",
			"error", err.Error(),
		)
	}

	status := &Status{}
	err = json.Unmarshal(b, status)
	if err != nil {
		slog.Error("error unmarshalling response body",
			"error", err.Error(),
			"response body", string(b),
		)
	}

	return *status
}

// strat:
// Send 3 morties to each planet
// Get responses
// When a morty dies send less morties on next request
// if morties survive send more on next request

// Plan:
// Request all state from
