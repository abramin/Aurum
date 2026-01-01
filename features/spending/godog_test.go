package spending

import (
	"os"
	"testing"

	"github.com/cucumber/godog"
)

func TestMain(m *testing.M) {
	options := godog.Options{
		Output: os.Stdout,
		Format: "pretty",
		Paths:  []string{"."},
	}

	status := godog.TestSuite{
		Name:                "spending",
		ScenarioInitializer: InitializeSpendingScenario,
		Options:             &options,
	}.Run()

	if testStatus := m.Run(); testStatus > status {
		status = testStatus
	}

	os.Exit(status)
}
