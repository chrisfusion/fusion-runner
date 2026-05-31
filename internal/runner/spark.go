package runner

import "fmt"

type SparkRunner struct {
	cfg Config
}

func (r *SparkRunner) Setup() error {
	return fmt.Errorf("spark runner not yet implemented")
}

func (r *SparkRunner) Exec() error {
	return fmt.Errorf("spark runner not yet implemented")
}
