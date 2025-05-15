package health_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/health"
)

func TestEvaluateAvailabilityStatus(t *testing.T) {
	tests := []struct {
		name           string
		expectedStatus health.AvailabilityStatus
		maxTimeInError time.Duration
		maxFails       uint
		state          health.CheckState
	}{
		{
			name:           "NoChecksMadeYetThenStatusUnknown",
			expectedStatus: health.StatusUnknown,
			maxTimeInError: 0,
			maxFails:       0,
			state: health.CheckState{
				LastCheckedAt: time.Time{},
			},
		},
		{
			name:           "NoErrorThenStatusUp",
			expectedStatus: health.StatusUp,
			maxTimeInError: 0,
			maxFails:       0,
			state: health.CheckState{
				LastCheckedAt: time.Now(),
			},
		},
		{
			name:           "ErrorThenStatusDown",
			expectedStatus: health.StatusDown,
			maxTimeInError: 0,
			maxFails:       0,
			state: health.CheckState{
				LastCheckedAt: time.Now(),
				Result:        errors.New("example error"),
			},
		},
		{
			name:           "ErrorAndMaxFailuresThresholdNotCrossedThenStatusWarn",
			expectedStatus: health.StatusUp,
			maxTimeInError: 1 * time.Second,
			maxFails:       10,
			state: health.CheckState{
				LastCheckedAt:       time.Now(),
				Result:              errors.New("example error"),
				FirstCheckStartedAt: time.Now().Add(-2 * time.Minute),
				LastSuccessAt:       time.Now().Add(-3 * time.Minute),
				ContiguousFails:     1,
			},
		},
		{
			name:           "ErrorAndMaxTimeInErrorThresholdNotCrossedThenStatusWarn",
			expectedStatus: health.StatusUp,
			maxTimeInError: 1 * time.Hour,
			maxFails:       1,
			state: health.CheckState{
				LastCheckedAt:       time.Now(),
				Result:              errors.New("example error"),
				FirstCheckStartedAt: time.Now().Add(-3 * time.Minute),
				LastSuccessAt:       time.Now().Add(-2 * time.Minute),
				ContiguousFails:     100,
			},
		},
		{
			name:           "ErrorAndAllThresholdsCrossedThenStatusDown",
			expectedStatus: health.StatusDown,
			maxTimeInError: 1 * time.Second,
			maxFails:       1,
			state: health.CheckState{
				LastCheckedAt:       time.Now(),
				Result:              errors.New("example error"),
				FirstCheckStartedAt: time.Now().Add(-3 * time.Minute),
				LastSuccessAt:       time.Now().Add(-2 * time.Minute),
				ContiguousFails:     5,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := health.EvaluateCheckStatus(&tc.state, tc.maxTimeInError, tc.maxFails)

			// Assert
			assert.Equal(t, tc.expectedStatus, result)
		})
	}
}

func TestStatusUnknownBeforeStatusUp(t *testing.T) {
	// Arrange
	testData := map[string]health.CheckState{"check1": {Status: health.StatusUp}, "check2": {Status: health.StatusUnknown}}

	// Act
	result := health.AggregateStatus(testData)

	// Assert
	assert.Equal(t, health.StatusUnknown, result)
}

func TestStatusDownBeforeStatusUnknown(t *testing.T) {
	// Arrange
	testData := map[string]health.CheckState{"check1": {Status: health.StatusDown}, "check2": {Status: health.StatusUnknown}}

	// Act
	result := health.AggregateStatus(testData)

	// Assert
	assert.Equal(t, health.StatusDown, result)
}

func TestStartStopManualPeriodicChecks(t *testing.T) {
	ckr := health.NewChecker(
		health.WithDisabledAutostart(),
		health.WithPeriodicCheck(50*time.Minute, 0, health.Check{
			Name: "check",
			Check: func(ctx context.Context) error {
				return nil
			},
		}))

	assert.Equal(t, 0, ckr.GetRunningPeriodicCheckCount())

	ckr.Start()
	assert.Equal(t, 1, ckr.GetRunningPeriodicCheckCount())

	ckr.Stop()
	assert.Equal(t, 0, ckr.GetRunningPeriodicCheckCount())
}

func TestCheckerCheck(t *testing.T) {
	tests := []struct {
		name           string
		expectedStatus health.AvailabilityStatus
		updateInterval time.Duration
		err            error
	}{
		{
			name:           "ChecksExecutedThenAggregatedResultUp",
			expectedStatus: health.StatusUp,
			updateInterval: 0,
			err:            nil,
		},
		{
			name:           "OneCheckFailedThenAggregatedResultDown",
			expectedStatus: health.StatusDown,
			updateInterval: 0,
			err:            errors.New("this is a check error"),
		},
		{
			name:           "CheckSuccessNotAllChecksExecutedYet",
			expectedStatus: health.StatusUnknown,
			updateInterval: 5 * time.Hour,
			err:            nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			ckr := health.NewChecker(
				health.WithTimeout(10*time.Second),
				health.WithCheck(health.Check{
					Name: "check1",
					Check: func(ctx context.Context) error {
						return nil
					},
				}),
				health.WithPeriodicCheck(tc.updateInterval, 0, health.Check{
					Name: "check2",
					Check: func(ctx context.Context) error {
						return tc.err
					},
				}),
			)

			// Act
			res := ckr.Check(t.Context())

			// Assert
			require.NotNil(t, res.Details)
			assert.Equal(t, tc.expectedStatus, res.Status)
			for _, checkName := range []string{"check1", "check2"} {
				_, checkResultExists := res.Details[checkName]
				assert.True(t, checkResultExists)
			}
		})
	}
}

func TestPanicRecovery(t *testing.T) {
	// Arrange
	expectedPanicMsg := "test message"
	ckr := health.NewChecker(
		health.WithCheck(health.Check{
			Name: "iPanic",
			Check: func(ctx context.Context) error {
				panic(expectedPanicMsg)
			},
		}),
	)

	// Act
	res := ckr.Check(t.Context())

	// Assert
	require.NotNil(t, res.Details)
	assert.Equal(t, health.StatusDown, res.Status)

	checkRes, checkResultExists := res.Details["iPanic"]
	assert.True(t, checkResultExists)
	assert.Error(t, checkRes.Error)
	assert.Equal(t, expectedPanicMsg, (checkRes.Error).Error())
}
