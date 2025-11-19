package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
)

// https://challenge.sphinxhq.com/

const (
	BASE_URL = "https://challenge.sphinxhq.com"

	// ENDPOINTS
	START_ENDPOINT  = "/api/mortys/start/"
	PORTAL_ENDPOINT = "/api/mortys/portal/"
	STATUS_ENDPOINT = "/api/mortys/status/"
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

type MortySender interface {
	Send(client *http.Client)
}

type Planet struct {
	PlanetNumber       PlanetNumber
	CurrentMortyAmount int
	Survives           int
	SurvivalRate       float32
}

type PlanetNumber int

const (
	OnACob PlanetNumber = iota
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

	cobPlanet := Planet{
		PlanetNumber:       0,
		CurrentMortyAmount: 2,
		Survives:           0,
		SurvivalRate:       0,
	}

	cronenPlanet := Planet{
		PlanetNumber:       1,
		CurrentMortyAmount: 2,
		Survives:           0,
		SurvivalRate:       0,
	}
	purgePlanet := Planet{
		PlanetNumber:       2,
		CurrentMortyAmount: 2,
		Survives:           0,
		SurvivalRate:       0,
	}

	for mortiesCount > 0 {
		cobPlanet.Send(client)
		cronenPlanet.Send(client)
		purgePlanet.Send(client)

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

func (p *Planet) Send(client *http.Client) {
	slog.Info(
		"send morties parameters",
		"mortyCount", p.CurrentMortyAmount,
		"planet", p.PlanetNumber,
	)
	sm := &SendMorty{Planet: int(p.PlanetNumber), MortyCount: p.CurrentMortyAmount}
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

	slog.Debug("morties survived",
		"planet", p.PlanetNumber,
		"morties sent", p.CurrentMortyAmount,
		"survived", portal.Survived,
		"total survives", p.Survives,
		"TotalPlanetJessicaCount", portal.MortiesOnPlanetJessica,
	)
	if portal.Survived {
		p.Survives += p.CurrentMortyAmount
	}
	if portal.MortiesOnPlanetJessica != 0 {
		p.SurvivalRate = float32(p.Survives) / float32(portal.MortiesOnPlanetJessica)
	} else {
		p.SurvivalRate = 0
	}

	slog.Debug("morties survival rate",
		"planet", p.PlanetNumber,
		"total survives", p.Survives,
		"SurvivalRate", p.SurvivalRate,
	)
	sr := p.SurvivalRate
	if sr > 0.66 && sr <= 1.0 {
		p.CurrentMortyAmount = 3
	} else if sr > 0.33 && sr <= 0.66 {
		p.CurrentMortyAmount = 2
	} else {
		p.CurrentMortyAmount = 1
	}

	if portal.MortiesInCitadel < 3 {
		p.CurrentMortyAmount = portal.MortiesInCitadel
	}
	slog.Debug("morties count",
		"planet", p.PlanetNumber,
		"current morty amount", p.CurrentMortyAmount,
	)
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
