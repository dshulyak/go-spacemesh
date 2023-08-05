package hare3

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spacemeshos/go-spacemesh/metrics"
)

const namespace = "hare"

var (
	processCounter = metrics.NewCounter(
		"process",
		namespace,
		"number of hare processes at different stages",
		[]string{"stage"},
	)
	instanceStart      = processCounter.WithLabelValues("started")
	instanceTerminated = processCounter.WithLabelValues("terminated")
	instanceCoin       = processCounter.WithLabelValues("weakcoin")
	instanceResult     = processCounter.WithLabelValues("result")

	exitErrors = metrics.NewCounter(
		"exit_errors",
		namespace,
		"number of unexpected exit errors. should remain at zero",
		[]string{},
	).WithLabelValues()
	validationError = metrics.NewCounter(
		"validation_errors",
		namespace,
		"number of validation errors. not expected to be at zero",
		[]string{"error"},
	)
	notRegisteredError = validationError.WithLabelValues("not_registered")
	malformedError     = validationError.WithLabelValues("malformed")
	signatureError     = validationError.WithLabelValues("signature")
	oracleError        = validationError.WithLabelValues("oracle")
	maliciuosError     = validationError.WithLabelValues("malicious")

	droppedMessages = metrics.NewCounter(
		"dropped_msgs",
		namespace,
		"number of messages dropped by gossip",
		[]string{},
	).WithLabelValues()

	// histograms use a bit more data than counters, use it sparingly
	validationLatency = metrics.NewHistogramWithBuckets(
		"validation_seconds",
		namespace,
		"validation time in seconds",
		[]string{"step"},
		prometheus.ExponentialBuckets(0.5, 2, 10),
	)
	oracleLatency = validationLatency.WithLabelValues("oracle")
	submitLatency = validationLatency.WithLabelValues("submit")

	protocolLatency = metrics.NewHistogramWithBuckets(
		"protocol_seconds",
		namespace,
		"protocol time in seconds",
		[]string{"step"},
		prometheus.ExponentialBuckets(0.5, 2, 10),
	)
	proposalsLatency = protocolLatency.WithLabelValues("proposals")
	activeLatency    = protocolLatency.WithLabelValues("active")
)
