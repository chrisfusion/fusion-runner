package runner

import "fmt"

type QuarkusRunner struct {
	cfg Config
}

func (r *QuarkusRunner) Setup() error {
	return fmt.Errorf("quarkus runner not yet implemented")
}

func (r *QuarkusRunner) Exec() error {
	return fmt.Errorf("quarkus runner not yet implemented")
}
