package features

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/cucumber/godog"
)

type contractState struct {
	server   *httptest.Server
	response *http.Response
}

func InitializeScenario(sc *godog.ScenarioContext) {
	state := &contractState{}

	sc.Step(`^the service is running$`, state.theServiceIsRunning)
	sc.Step(`^I request the health endpoint$`, state.iRequestTheHealthEndpoint)
	sc.Step(`^the response status should be (\d+)$`, state.theResponseStatusShouldBe)

	sc.After(func(ctx context.Context, scenario *godog.Scenario, err error) (context.Context, error) {
		if state.server != nil {
			state.server.Close()
		}
		if state.response != nil {
			state.response.Body.Close()
		}
		return ctx, nil
	})
}

func (s *contractState) theServiceIsRunning() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	s.server = httptest.NewServer(mux)
	return nil
}

func (s *contractState) iRequestTheHealthEndpoint() error {
	if s.server == nil {
		return fmt.Errorf("server not running")
	}
	resp, err := http.Get(s.server.URL + "/health")
	if err != nil {
		return fmt.Errorf("failed to request health endpoint: %w", err)
	}
	s.response = resp
	return nil
}

func (s *contractState) theResponseStatusShouldBe(expected int) error {
	if s.response == nil {
		return fmt.Errorf("no response received")
	}
	if s.response.StatusCode != expected {
		return fmt.Errorf("expected status %d, got %d", expected, s.response.StatusCode)
	}
	return nil
}
