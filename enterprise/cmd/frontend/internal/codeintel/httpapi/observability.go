package httpapi

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type Operations struct {
	handleEnqueue *observation.Operation
}

func NewOperations(observationContext *observation.Context) *Operations {
	metrics := metrics.NewREDMetrics(
		observationContext.Registerer,
		"codeintel_httpapi",
		metrics.WithLabels("op"),
		metrics.WithCountHelp("Total number of method invocations."),
	)

	op := func(name string) *observation.Operation {
		return observationContext.Operation(observation.Op{
			Name:              fmt.Sprintf("codeintel.httpapi.%s", name),
			MetricLabelValues: []string{name},
			Metrics:           metrics,
		})
	}

	return &Operations{
		handleEnqueue: op("HandleEnqueue"),
	}
}
