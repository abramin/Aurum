package features

import (
	"os"
	"testing"

	"github.com/cucumber/godog"
)

func TestMain(m *testing.M) {
	options := godog.Options{
		Output: os.Stdout,
		Format: "pretty",
		Paths:  []string{"contract.feature"},
	}

	status := godog.TestSuite{
		Name:                "contract",
		ScenarioInitializer: InitializeScenario,
		Options:             &options,
	}.Run()

	if testStatus := m.Run(); testStatus > status {
		status = testStatus
	}

	os.Exit(status)
}
