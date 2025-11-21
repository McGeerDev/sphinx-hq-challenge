# Go Code Review: main.go

## Executive Summary

**Overall Assessment: NEEDS SIGNIFICANT REFACTORING**

This code implements an epsilon-greedy reinforcement learning algorithm but contains multiple **critical bugs** that can cause runtime panics, resource leaks, and undefined behavior. The code is non-idiomatic, untestable, and violates fundamental Go best practices. While the core algorithm logic appears sound, the implementation is fundamentally flawed.

**Severity Breakdown:**
- **Critical Issues (Program-Breaking):** 8
- **Major Issues (Non-Idiomatic/Poor Design):** 12
- **Minor Issues (Code Quality):** 6

---

## Critical Issues (Must Fix Immediately)

### 1. **Nil Pointer Dereferences Throughout** 🚨
**Location:** Lines 210, 258, 285, 292

**Problem:**
```go
res, err := client.Do(req)
if err != nil {
    slog.Error("error sending request", "error", err.Error())
}
defer res.Body.Close() // PANIC: res is nil if error occurred
```

**Why This is Critical:** When `client.Do()` fails, `res` is `nil`. The code logs the error but continues execution, causing a nil pointer dereference on the next line.

**Correct Pattern:**
```go
res, err := client.Do(req)
if err != nil {
    return Status{}, fmt.Errorf("sending request: %w", err)
}
defer res.Body.Close()
```

### 2. **Resource Leak: Defer in Loop** 🚨
**Location:** Line 210 in `Send()`

**Problem:**
```go
for i, v := range combo {
    // ... HTTP request setup ...
    res, err := client.Do(req)
    defer res.Body.Close() // BUG: Doesn't execute until function returns
}
```

**Why This is Critical:** `defer` executes at function return, not loop iteration end. All 3 HTTP response bodies remain open until `Send()` completes, causing file descriptor exhaustion in long-running programs.

**Correct Pattern:**
```go
// Option 1: Close immediately
res.Body.Close()

// Option 2: Extract to function
func sendToSinglePlanet(...) error {
    defer res.Body.Close()
    // ...
}
```

### 3. **Division By Zero** 🚨
**Location:** Lines 234, 373

**Problem:**
```go
// Line 234
return float32(count) / float32(total) // total could be 0 if combo is [0,0,0]

// Line 373
return total / float32(count) // count could be 0 if check removed
```

**Why This is Critical:** Causes program crash or `+Inf`/`NaN` propagation.

**Fix:**
```go
if total == 0 {
    return 0
}
return float32(count) / float32(total)
```

### 4. **No Timeout on HTTP Client** 🚨
**Location:** Line 95

**Problem:**
```go
client := &http.Client{} // No timeout - can hang forever
```

**Why This is Critical:** Network requests can hang indefinitely, making the program unresponsive.

**Fix:**
```go
client := &http.Client{
    Timeout: 30 * time.Second,
}
```

### 5. **Error Handling Violates Go Philosophy** 🚨
**Location:** Throughout entire codebase

**Problem:** All functions log errors but never return them. The program continues executing in invalid states after fatal errors.

```go
func StartEpisode(client *http.Client) Status {
    req, err := http.NewRequest(...)
    if err != nil {
        slog.Error("error", "error", err.Error()) // Logged but not returned!
    }
    req.Header.Set(...) // req could be nil - PANIC
}
```

**Why This is Wrong:** Go's explicit error handling is a core language feature. Logging errors without propagating them violates the principle "don't just check errors, handle them gracefully."

**Correct Pattern:**
```go
func StartEpisode(client *http.Client) (Status, error) {
    req, err := http.NewRequest(...)
    if err != nil {
        return Status{}, fmt.Errorf("creating request: %w", err)
    }
    // ...
}
```

### 6. **Blank Identifier Error Suppression** 🚨
**Location:** Line 286

**Problem:**
```go
req, _ := http.NewRequest("GET", ...) // Silently ignoring error
req.Header.Set(...) // req could be nil
```

**Why This is Critical:** `http.NewRequest` CAN fail (e.g., invalid URL). Ignoring the error leads to nil pointer dereference.

### 7. **Algorithm Bug in FindBestSurvivalCombo** 🚨
**Location:** Lines 340, 347

**Problem:**
```go
var highest float32 // Defaults to 0
for i, v := range actions {
    if v.avgSurvivalRate > highest { // If all rates <= 0, returns [0,0,0]
        bestCombo = i
    }
}
```

**Why This is Critical:** If all survival rates are negative or zero, the function returns the zero value `[0,0,0]`, which is not a valid combo from the map.

**Fix:**
```go
highest := float32(math.Inf(-1))
// Or initialize to first element's rate
```

### 8. **No Context for HTTP Requests** 🚨
**Location:** All HTTP request creation

**Problem:** Using `http.NewRequest` instead of `http.NewRequestWithContext`.

**Why This Matters:** Cannot cancel requests, set deadlines, or propagate cancellation signals. Essential for production code.

---

## Major Issues (Non-Idiomatic Go)

### 9. **Global Mutable State**
**Location:** Lines 31-33

**Problem:**
```go
var (
    AUTH_HEADER string // Global mutable variable
)

func main() {
    AUTH_HEADER = os.Getenv("AUTH_HEADER")
}
```

**Why This is Bad:**
- Untestable (must mutate global state in tests)
- Race conditions in concurrent programs
- Violates dependency injection principles
- Makes code flow unclear

**Idiomatic Solution:**
```go
type Config struct {
    AuthHeader string
    BaseURL    string
}

func NewConfig() Config {
    return Config{
        AuthHeader: os.Getenv("AUTH_HEADER"),
        BaseURL:    "https://challenge.sphinxhq.com",
    }
}

// Pass config to functions that need it
func StartEpisode(client *http.Client, cfg Config) (Status, error)
```

### 10. **Non-Idiomatic Naming Conventions**
**Location:** Lines 19-25

**Problem:**
```go
const (
    BASE_URL = "..." // SCREAMING_SNAKE_CASE
    START_ENDPOINT = "..."
)
```

**Why This is Wrong:** Go style guide mandates `mixedCaps` or `MixedCaps`, never underscores.

**Correct:**
```go
const (
    baseURL        = "https://challenge.sphinxhq.com"
    startEndpoint  = "/api/mortys/start/"
    portalEndpoint = "/api/mortys/portal/"
    statusEndpoint = "/api/mortys/status/"
    epsilon        = 0.4
)
```

### 11. **Unnecessary Pointer Allocations**
**Location:** Lines 182, 268, 298

**Problem:**
```go
sm := &SendMorty{Planet: i, MortyCount: v}
jsonBody, err := json.Marshal(*sm) // Immediately dereference

start := &Status{}
// ... populate ...
return *start // Immediately dereference
```

**Why This is Inefficient:**
- Unnecessary heap allocations increase GC pressure
- Pointers should only be used when mutation is needed or for large structs
- Violates Go idiom: "return values, not pointers to values"

**Correct:**
```go
sm := SendMorty{Planet: i, MortyCount: v}
jsonBody, err := json.Marshal(sm)

var start Status
// ... populate ...
return start
```

### 12. **Code Duplication (DRY Violation)**
**Location:** Lines 128-132 and 146-150

**Problem:** Identical 14-line blocks for updating the actions map.

**Correct:**
```go
func updateActions(actions map[[3]int]*Action, combo [3]int, rate float32) {
    if action, ok := actions[combo]; ok {
        action.survivalRateHistory = append(action.survivalRateHistory, rate)
        action.avgSurvivalRate = Average(action.survivalRateHistory)
    } else {
        actions[combo] = &Action{
            avgSurvivalRate:     rate,
            survivalRateHistory: []float32{rate},
        }
    }
}
```

### 13. **Incorrect Structured Logging Usage**
**Location:** Line 101

**Problem:**
```go
slog.Info(fmt.Sprintf("StartState: %+v", start))
```

**Why This is Wrong:** Defeats the purpose of structured logging. The entire struct is concatenated into a single string field, making it unsearchable.

**Correct:**
```go
slog.Info("episode started",
    "morties_in_citadel", start.MortiesInCitadel,
    "morties_on_planet_jessica", start.MortiesOnPlanetJessica,
)
```

### 14. **No Package Organization**
**Problem:** Everything lives in `package main` with no separation of concerns.

**Correct Structure:**
```
morty-express/
├── main.go              # Entry point only
├── internal/
│   ├── client/          # HTTP client wrapper
│   ├── rl/              # Reinforcement learning logic
│   └── models/          # Data structures
└── go.mod
```

### 15. **Dead Code**
**Location:** Lines 54-56, 68-74, 321-335

**Problem:**
- `MortySender` interface: Defined but never implemented
- `Planet` struct: Defined but never used
- `FindMax()` function: Never called

**Fix:** Delete unused code.

### 16. **Unexported Struct Fields**
**Location:** Lines 79-80

**Problem:**
```go
type Action struct {
    avgSurvivalRate     float32  // Unexported
    survivalRateHistory []float32 // Unexported
}
```

**Why This is Bad:** Impossible to initialize from other packages. Forces composite literal initialization in same package only.

**Fix:** Either export fields or provide a constructor:
```go
type Action struct {
    AvgSurvivalRate     float32
    SurvivalRateHistory []float32
}

// Or
func NewAction(rate float32) *Action {
    return &Action{
        avgSurvivalRate:     rate,
        survivalRateHistory: []float32{rate},
    }
}
```

### 17. **Missing Function Comments**
**Problem:** Exported functions lack doc comments (required by Go conventions).

**Correct:**
```go
// Send attempts to send Morties through portals and returns the survival rate.
// It makes 3 HTTP requests, one per planet in the combo.
func Send(client *http.Client, combo [3]int) (float32, error) {
```

### 18. **No Testing**
**Problem:** Zero test coverage. Code is untestable due to global state and lack of interfaces.

**Solution:**
```go
// Define interface for testing
type MortyClient interface {
    StartEpisode(ctx context.Context) (Status, error)
    Send(ctx context.Context, planet, count int) (Portal, error)
    GetStatus(ctx context.Context) (Status, error)
}

// Implementation can be mocked in tests
```

### 19. **Magic Numbers**
**Location:** Lines 107, 157

**Problem:**
```go
var actions = map[[3]int]*Action{{2, 2, 2}: {avgSurvivalRate: 0.1, ...}}
rate := float32(status.MortiesOnPlanetJessica) / float32(1000)
```

**Fix:**
```go
const (
    initialMortyCount = 1000
    seedCombo = [3]int{2, 2, 2}
    seedRate = 0.1
)
```

### 20. **No Graceful Shutdown**
**Problem:** No signal handling. Program cannot be cleanly interrupted.

**Solution:**
```go
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()
```

---

## Minor Issues

### 21. **Redundant Type Conversion**
**Location:** Line 113

```go
if float32(randomChance) < EPSILON { // randomChance is already float32
```

### 22. **Inefficient Averaging**
**Location:** Lines 365-369

```go
var count int
for _, v := range f {
    count++  // Just use len(f)
    total += v
}
```

### 23. **Inconsistent Variable Naming**
**Location:** Line 363

```go
func Average(f []float32) float32 { // 'f' is unclear
```

Better: `func Average(values []float32) float32`

### 24. **Misleading Function Name**
**Location:** Line 321

```go
func FindMax(f []int) int { // Logs say "FLOAT" but operates on int
    slog.Debug("FIND MAX FLOAT", ...)
}
```

### 25. **Redundant Blank Line**
**Location:** Line 351

Single blank line is sufficient; no need for extra spacing.

### 26. **Missing HTTP Status Code Checks**
**Problem:** Never validates `res.StatusCode`. Assumes all requests succeed.

**Fix:**
```go
if res.StatusCode != http.StatusOK {
    return Status{}, fmt.Errorf("unexpected status: %d", res.StatusCode)
}
```

---

## Thought Patterns Analysis

### ✅ What You Got Right

1. **Algorithm Selection:** Epsilon-greedy is appropriate for this exploration/exploitation problem.
2. **Structured Logging:** Using `slog` is correct (though implementation needs work).
3. **Basic Code Organization:** Functions are reasonably small and focused.
4. **Type Safety:** Using custom types like `PlanetNumber` shows type-safety awareness.

### ❌ What You Got Wrong

1. **Error Handling Philosophy:** Fundamental misunderstanding of Go's error handling. Logging ≠ handling.
2. **Resource Management:** Not understanding defer's scope (function vs. loop).
3. **Pointer Usage:** Over-using pointers without understanding stack vs. heap allocation.
4. **Global State:** Violates dependency injection and testability principles.
5. **Naming Conventions:** Not following Go style guide.
6. **HTTP Best Practices:** No timeouts, no context, no status code validation.
7. **Testing Mindset:** Code is written without testability in mind.

---

## Recommended Refactoring Priorities

### Phase 1: Critical Bug Fixes (Do First)
1. Fix nil pointer dereferences (add early returns on errors)
2. Fix defer-in-loop resource leak
3. Add HTTP client timeout
4. Fix division by zero
5. Remove blank identifier error suppression

### Phase 2: Idiomatic Go (Do Second)
1. Remove global variables
2. Return errors from functions
3. Rename constants to mixedCaps
4. Remove unnecessary pointers
5. Extract duplicated code

### Phase 3: Architecture (Do Third)
1. Separate packages (client, rl, models)
2. Add interfaces for testability
3. Add context.Context to all functions
4. Write unit tests
5. Add HTTP status code validation

---

## Example: How StartEpisode Should Look

**Before:**
```go
func StartEpisode(client *http.Client) Status {
    req, err := http.NewRequest("POST", fmt.Sprintf("%s%s", BASE_URL, START_ENDPOINT), nil)
    if err != nil {
        slog.Error("error creating request", "error", err.Error())
    }
    req.Header.Set("Authorization", AUTH_HEADER)
    res, err := client.Do(req)
    if err != nil {
        slog.Error("error sending request", "error", err.Error())
    }
    defer res.Body.Close()
    b, err := io.ReadAll(res.Body)
    if err != nil {
        slog.Error("error reading response body", "error", err.Error())
    }
    start := &Status{}
    err = json.Unmarshal(b, start)
    if err != nil {
        slog.Error("error unmarshalling response body", "error", err.Error())
    }
    return *start
}
```

**After:**
```go
func (c *Client) StartEpisode(ctx context.Context) (Status, error) {
    url := fmt.Sprintf("%s%s", c.baseURL, c.startEndpoint)

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
    if err != nil {
        return Status{}, fmt.Errorf("creating request: %w", err)
    }

    req.Header.Set("Authorization", c.authHeader)

    res, err := c.httpClient.Do(req)
    if err != nil {
        return Status{}, fmt.Errorf("sending request: %w", err)
    }
    defer res.Body.Close()

    if res.StatusCode != http.StatusOK {
        return Status{}, fmt.Errorf("unexpected status code: %d", res.StatusCode)
    }

    var status Status
    if err := json.NewDecoder(res.Body).Decode(&status); err != nil {
        return Status{}, fmt.Errorf("decoding response: %w", err)
    }

    return status, nil
}
```

**Key Improvements:**
- Returns `(Status, error)` instead of just `Status`
- Uses `context.Context` for cancellation
- Checks HTTP status code
- Uses `json.NewDecoder` (more efficient than ReadAll + Unmarshal)
- No global variables
- No unnecessary pointer allocations
- Proper error wrapping with `%w`

---

## Conclusion

This code demonstrates understanding of the problem domain (reinforcement learning) but lacks fundamental Go proficiency. The core issues are:

1. **Not understanding error handling** - the most critical problem
2. **Resource management bugs** - defer in loops, no timeouts
3. **Non-idiomatic style** - global state, wrong naming, unnecessary pointers
4. **Untestable design** - no interfaces, global dependencies

**Recommended Action:** Complete rewrite following the refactoring priorities above. Focus on understanding Go's error handling philosophy, resource management patterns, and standard library best practices before proceeding.

**Estimated Effort:** 4-6 hours for proper refactoring with tests.

**Learning Resources:**
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Standard library `net/http` package documentation](https://pkg.go.dev/net/http)
- ["Errors are values" by Rob Pike](https://go.dev/blog/errors-are-values)
