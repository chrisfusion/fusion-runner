package runner

import "fmt"

// Runner prepares the runtime environment and replaces the process with the target application.
type Runner interface {
	Setup() error
	Exec() error
}

func New(cfg Config) (Runner, error) {
	switch cfg.RunnerType {
	case "python":
		return &PythonRunner{cfg: cfg}, nil
	case "streamlit":
		return &StreamlitRunner{PythonRunner: PythonRunner{cfg: cfg}}, nil
	case "python-index":
		return &IndexPythonRunner{PythonRunner: PythonRunner{cfg: cfg}}, nil
	case "spark":
		return &SparkRunner{cfg: cfg}, nil
	case "quarkus":
		return &QuarkusRunner{cfg: cfg}, nil
	default:
		return nil, fmt.Errorf("unsupported runner type %q (set via WEAVE_RUNNER_TYPE)", cfg.RunnerType)
	}
}
