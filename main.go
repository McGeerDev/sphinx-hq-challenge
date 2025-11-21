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
	// CRITICAL: Non-idiomatic naming - Go uses mixedCaps/MixedCaps, not SCREAMING_SNAKE_CASE
	// Should be: baseURL, startEndpoint, portalEndpoint, statusEndpoint, epsilon
	BASE_URL = "https://challenge.sphinxhq.com"

	// ENDPOINTS
	START_ENDPOINT  = "/api/mortys/start/"
	PORTAL_ENDPOINT = "/api/mortys/portal/"
	STATUS_ENDPOINT = "/api/mortys/status/"
	EPSILON         = 0.4
)

// CRITICAL: Global mutable state is an anti-pattern in Go
// Problems: untestable, race conditions, violates dependency injection
// Solution: Pass as parameter or use a config struct
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

// ISSUE: Dead code - interface defined but never implemented or used
// Either implement it or remove it
type MortySender interface {
	Send(client *http.Client)
}

type PlanetNumber int

const (
	OnACob PlanetNumber = iota
	CronenBergWorld
	PurgePlanet
)

// ISSUE: Dead code - type defined but never used
// All fields remain uninitialized throughout the program
type Planet struct {
	PlanetNumber       PlanetNumber
	CurrentMortyAmount int
	Survives           int
	TotalSent          int
	SurvivalRate       float32
}

// ISSUE: Unexported fields make this impossible to initialize from other packages
// Either export fields or provide a constructor function
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

	// ISSUE: Mutating global variable
	AUTH_HEADER = os.Getenv("AUTH_HEADER")

	// CRITICAL: No timeout configured - can hang indefinitely
	// Should be: client := &http.Client{Timeout: 30 * time.Second}
	client := &http.Client{}

	start := StartEpisode(client)

	// ISSUE: Using fmt.Sprintf with structured logging defeats slog's purpose
	// Should be: slog.Info("StartState", "status", start)
	slog.Info(fmt.Sprintf("StartState: %+v", start))

	mortiesCount := start.MortiesInCitadel

	// ISSUE: Magic numbers {2,2,2} and 0.1 with no explanation
	// Why initialize with this specific combination?
	var actions = map[[3]int]*Action{{2, 2, 2}: {avgSurvivalRate: 0.1, survivalRateHistory: []float32{0.1}}}

	for mortiesCount > 0 {
		randomChance := rand.Float32()
		slog.Debug("chance", "chance<epsilon", float32(randomChance) < EPSILON)
		// ISSUE: Redundant type conversion - randomChance is already float32
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

			// CRITICAL: Code duplication - lines 121-126 and 137-142 are identical
			// Extract to updateActions(actions, combo, rate) function
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
			// CRITICAL: Identical code block - DRY violation
			if _, ok := actions[bestCombo]; ok {
				actions[bestCombo].survivalRateHistory = append(actions[bestCombo].survivalRateHistory, rate)
				actions[bestCombo].avgSurvivalRate = Average(actions[bestCombo].survivalRateHistory)
			} else {
				actions[bestCombo] = &Action{rate, []float32{rate}}
			}
		}

		status := GetEpisodeStatus(client)

		// ISSUE: Magic number 1000 should be named constant (e.g., initialMortyCount)
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
		// ISSUE: Unnecessary pointer allocation - sm doesn't need to escape to heap
		sm := &SendMorty{Planet: i, MortyCount: v}
		// ISSUE: Dereferencing pointer immediately - should use sm without pointer
		jsonBody, err := json.Marshal(*sm)
		// CRITICAL: Error logged but not returned - function continues with invalid state
		if err != nil {
			slog.Error(err.Error())
		}

		bytesReader := bytes.NewReader(jsonBody)

		req, err := http.NewRequest("POST", fmt.Sprintf("%s%s", BASE_URL, PORTAL_ENDPOINT), bytesReader)
		// CRITICAL: Error logged but not handled - req could be nil
		if err != nil {
			slog.Error(err.Error())
		}

		req.Header.Set("Authorization", AUTH_HEADER)
		req.Header.Set("Content-Type", "application/json")

		res, err := client.Do(req)
		// CRITICAL: Error logged but not handled - res could be nil
		if err != nil {
			slog.Error(err.Error())
		}
		// CRITICAL: defer in loop - resources not released until function returns
		// All 3 HTTP response bodies stay open until Send() completes
		// Solution: close immediately or extract to separate function
		// CRITICAL: If client.Do fails, res is nil and this panics with nil pointer dereference
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
	// CRITICAL: Division by zero if combo is [0,0,0]
	// Should check: if total == 0 { return 0 }
	return float32(count) / float32(total)

}

// ISSUE: Should return (Status, error) for proper error handling
// CRITICAL: No context.Context - can't cancel or set timeout on request
func StartEpisode(client *http.Client) Status {
	slog.Debug("Starting Episode")
	req, err := http.NewRequest("POST", fmt.Sprintf("%s%s", BASE_URL, START_ENDPOINT), nil)
	// CRITICAL: Error not returned - req could be nil, next line panics
	if err != nil {
		slog.Error(
			"error creating request",
			"error", err.Error())
	}
	req.Header.Set("Authorization", AUTH_HEADER)
	res, err := client.Do(req)
	// CRITICAL: Error not returned - res could be nil
	if err != nil {
		slog.Error("error sending request",
			"error", err.Error())
	}

	// CRITICAL: If client.Do failed, res is nil - panic on nil pointer dereference
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		slog.Error("error reading response body",
			"error", err.Error(),
		)
	}

	// ISSUE: Unnecessary pointer - allocates on heap then immediately dereferences
	// Should be: var start Status; json.Unmarshal(b, &start); return start
	start := &Status{}
	err = json.Unmarshal(b, start)
	if err != nil {
		slog.Error("error unmarshalling response body",
			"error", err.Error(),
			"response body", string(b),
		)
	}

	// ISSUE: Dereferencing pointer immediately - pointer was unnecessary
	return *start
}

// ISSUE: Should return (Status, error) for proper error handling
func GetEpisodeStatus(client *http.Client) Status {
	slog.Debug("Episode Status")
	// CRITICAL: Silently ignoring error with _ - http.NewRequest CAN fail
	// If it fails, req is nil and next line panics
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

// ISSUE: Dead code - function never called
// ISSUE: Function name says "Float" but operates on []int
// ISSUE: Debug log says "FIND MAX FLOAT" but it's finding max int
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
	// CRITICAL: highest defaults to 0 - if all rates are negative or zero, returns [0,0,0]
	// Should initialize to math.MinFloat32 or first element's rate
	var highest float32
	var bestCombo [3]int
	if len(actions) == 0 {
		return [3]int{rand.IntN(3) + 1, rand.IntN(3) + 1, rand.IntN(3) + 1}
	}
	for i, v := range actions {
		// ISSUE: Algorithm bug - if all survival rates are <= 0, returns zero value [0,0,0]
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
	// ISSUE: Redundant - count will always equal len(f)
	// Just use len(f) instead of counting in loop
	var count int
	for _, v := range f {
		count++
		total += v
	}

	// ISSUE: count == len(f), so this is just total / float32(len(f))
	// CRITICAL: If len(f) was 0 and check was removed, division by zero
	return total / float32(count)

}
