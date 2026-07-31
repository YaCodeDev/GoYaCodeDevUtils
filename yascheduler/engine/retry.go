package engine

import (
	"math"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

func maxFunctionAttempts(
	spec protocol.RetrySpec,
) (attempts store.FunctionAttempts) {
	switch spec.Policy {
	case protocol.RetryPolicyInherit:
		return store.FunctionAttempts(protocol.DefaultMaxRetries) + firstAttemptNumber
	case protocol.RetryPolicyNone:
		return firstAttemptNumber
	case protocol.RetryPolicyImmediate,
		protocol.RetryPolicyFixed,
		protocol.RetryPolicyExponential:
		return store.FunctionAttempts(spec.MaxRetries) + firstAttemptNumber
	default:
		return store.FunctionAttempts(protocol.DefaultMaxRetries) + firstAttemptNumber
	}
}

func retryDelay(
	spec protocol.RetrySpec,
	consumedAttempts store.FunctionAttempts,
	cfg *Config,
) (delay time.Duration) {
	initial := millisDuration(spec.InitialDelayMillis)
	if initial <= 0 {
		initial = cfg.RetryInitialDelay
	}

	maxDelay := millisDuration(spec.MaxDelayMillis)
	if maxDelay <= 0 {
		maxDelay = cfg.RetryMaxDelay
	}

	multiplier := math.Float64frombits(spec.MultiplierBits)
	if multiplier <= 1 || math.IsNaN(multiplier) || math.IsInf(multiplier, 0) {
		multiplier = DefaultRetryMultiplier
	}

	switch spec.Policy {
	case protocol.RetryPolicyImmediate:
		return 0
	case protocol.RetryPolicyFixed:
		return initial
	case protocol.RetryPolicyNone:
		return 0
	case protocol.RetryPolicyInherit, protocol.RetryPolicyExponential:
		exponent := float64(consumedAttempts) - firstAttemptNumber
		if exponent < 0 {
			exponent = 0
		}

		scaled := float64(initial) * math.Pow(multiplier, exponent)
		if scaled > float64(maxDelay) {
			return maxDelay
		}

		return time.Duration(scaled)
	default:
		return initial
	}
}
