package features

import "github.com/cucumber/godog"

type contractState struct{}

func InitializeScenario(ctx *godog.ScenarioContext) {
	state := &contractState{}

	ctx.Step(`^the service is running$`, state.theServiceIsRunning)
	ctx.Step(`^I request the health endpoint$`, state.iRequestTheHealthEndpoint)
	ctx.Step(`^the response status should be (\d+)$`, state.theResponseStatusShouldBe)
}

func (state *contractState) theServiceIsRunning() error {
	return godog.ErrPending
}

func (state *contractState) iRequestTheHealthEndpoint() error {
	return godog.ErrPending
}

func (state *contractState) theResponseStatusShouldBe(_ int) error {
	return godog.ErrPending
}
