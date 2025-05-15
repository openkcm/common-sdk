package health

import (
	"net/http"
	"time"

	"github.com/stretchr/testify/mock"
)

func AggregateStatus(results map[string]CheckState) AvailabilityStatus {
	return aggregateStatus(results)
}

func EvaluateCheckStatus(state *CheckState, maxTimeInError time.Duration, maxFails uint) AvailabilityStatus {
	return evaluateCheckStatus(state, maxTimeInError, maxFails)
}

type resultWriterMock struct {
	mock.Mock
}

func (ck *resultWriterMock) Write(result *Result, statusCode int, w http.ResponseWriter, r *http.Request) error {
	err, _ := ck.Called(result, statusCode, w, r).Get(0).(error)
	return err
}
