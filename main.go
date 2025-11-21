package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
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
	EPSILON         = 0.4
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

type PlanetNumber int

const (
	OnACob PlanetNumber = iota
	CronenBergWorld
	PurgePlanet
)

type Planet struct {
	PlanetNumber       PlanetNumber
	CurrentMortyAmount int
	Survives           int
	TotalSent          int
	SurvivalRate       float32
}

type Action struct {
	avgSurvivalRate     float32
	survivalRateHistory []float32
}

func main() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	AUTH_HEADER = os.Getenv("AUTH_HEADER")

	client := &http.Client{}

	start := StartEpisode(client)

	slog.Info(fmt.Sprintf("StartState: %+v", start))

	mortiesCount := start.MortiesInCitadel

	var actions = map[[3]int]*Action{{2, 2, 2}: {avgSurvivalRate: 0.1, survivalRateHistory: []float32{0.1}}}

	for mortiesCount > 0 {
		randomChance := rand.Float32()
		slog.Debug("chance", "chance<epsilon", float32(randomChance) < EPSILON)
		if float32(randomChance) < EPSILON {
			slog.Debug("PERFORM RANDOM ACTION")
			randCombo := RandomCombo()

			if mortiesCount < 3 {
				randCombo = [3]int{mortiesCount, 0, 0}
			}
			rate := Send(client, randCombo)
			slog.Debug("best survival rate",
				"randCombo", randCombo,
				"rate with combo", rate,
			)

			if _, ok := actions[randCombo]; ok {
				actions[randCombo].survivalRateHistory = append(actions[randCombo].survivalRateHistory, rate)
				actions[randCombo].avgSurvivalRate = Average(actions[randCombo].survivalRateHistory)
			} else {
				actions[randCombo] = &Action{rate, []float32{rate}}
			}
		} else {
			slog.Debug("PERFORM BEST PERFOMING ACTION")
			bestCombo := FindBestSurvivalCombo(actions)
			if mortiesCount < 3 {
				bestCombo = [3]int{mortiesCount, 0, 0}
			}
			rate := Send(client, bestCombo)
			slog.Debug("best survival rate",
				"bestCombo", bestCombo,
				"rate with combo", rate,
			)
			if _, ok := actions[bestCombo]; ok {
				actions[bestCombo].survivalRateHistory = append(actions[bestCombo].survivalRateHistory, rate)
				actions[bestCombo].avgSurvivalRate = Average(actions[bestCombo].survivalRateHistory)
			} else {
				actions[bestCombo] = &Action{rate, []float32{rate}}
			}
		}

		status := GetEpisodeStatus(client)

		rate := float32(status.MortiesOnPlanetJessica) / float32(1000)
		slog.Info("Status",
			"MortiesInCitadel",
			status.MortiesInCitadel,
			"MortiesOnPlanetJessica",
			status.MortiesOnPlanetJessica,
			"RATE",
			rate,
		)

		// Update mortyCount
		mortiesCount = status.MortiesInCitadel
	}
}

type SendMorty struct {
	Planet     int `json:"planet"`
	MortyCount int `json:"morty_count"`
}

func Send(client *http.Client, combo [3]int) float32 {
	var count int
	var total int
	for i, v := range combo {
		sm := &SendMorty{Planet: i, MortyCount: v}
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
		total += v
		if portal.Survived {
			count += v
		}
	}
	return float32(count) / float32(total)

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

func RandomCombo() [3]int {
	return [3]int{rand.IntN(3) + 1, rand.IntN(3) + 1, rand.IntN(3) + 1}
}

func FindMax(f []int) int {
	slog.Debug("FIND MAX FLOAT", "length of slice", len(f))
	if len(f) == 0 {
		return 0
	}

	highest := f[0]
	for _, v := range f {
		if v > highest {
			highest = v
		}
	}
	slog.Debug("FindMaxFloat", "values", f, "highest", highest)
	return highest
}
func FindBestSurvivalCombo(actions map[[3]int]*Action) [3]int {
	slog.Debug("FindBestSurvivalCombo")
	var highest float32
	var bestCombo [3]int
	if len(actions) == 0 {
		return [3]int{rand.IntN(3) + 1, rand.IntN(3) + 1, rand.IntN(3) + 1}
	}
	for i, v := range actions {
		if v.avgSurvivalRate > highest {
			highest = v.avgSurvivalRate
			bestCombo = i
		}

	}
	slog.Debug("returned combo", "bestCombo", bestCombo)
	return bestCombo
}

func Average(f []float32) float32 {
	if len(f) == 0 {
		return 0
	}

	var total float32
	var count int
	for _, v := range f {
		count++
		total += v
	}

	return total / float32(count)

}
