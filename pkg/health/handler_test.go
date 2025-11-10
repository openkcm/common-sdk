package health_test

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/openkcm/common-sdk/pkg/health"
)

type checkerMock struct {
	mock.Mock
}

func (ck *checkerMock) Start() {
	ck.Called()
}

func (ck *checkerMock) Stop() {
	ck.Called()
}

func (ck *checkerMock) Check(ctx context.Context) health.Result {
	r, _ := ck.Called(ctx).Get(0).(health.Result)
	return r
}

func (ck *checkerMock) GetRunningPeriodicCheckCount() int {
	r, _ := ck.Called().Get(0).(int)
	return r
}

func (ck *checkerMock) IsStarted() bool {
	r, _ := ck.Called().Get(0).(bool)
	return r
}

func TestSuite(t *testing.T) {
	tests := []struct {
		name               string
		expectedStatus     health.Result
		statusCodeUp       int
		statusCodeDown     int
		expectedStatusCode int
	}{
		{
			name: "CheckSucceedsThenRespondWithAvailable",
			expectedStatus: health.Result{
				Status: health.StatusUp,
				Details: map[string]health.CheckResult{
					"check1": {Status: health.StatusUp, Timestamp: time.Now().UTC(), Error: nil},
				},
			},
			statusCodeUp:       http.StatusNoContent,
			statusCodeDown:     http.StatusTeapot,
			expectedStatusCode: http.StatusNoContent,
		},
		{
			name: "AuthFailsThenReturnNoDetails",
			expectedStatus: health.Result{
				Status:  health.StatusDown,
				Details: nil,
			},
			statusCodeUp:       http.StatusNoContent,
			statusCodeDown:     http.StatusTeapot,
			expectedStatusCode: http.StatusTeapot,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "https://localhost/foo", nil)

			ckr := checkerMock{}
			ckr.On("IsStarted").Return(false)
			ckr.On("Start")
			ckr.On("Check", mock.Anything).Return(tc.expectedStatus)

			handler := health.NewHandler(&ckr, health.WithStatusCodeUp(tc.statusCodeUp), health.WithStatusCodeDown(tc.statusCodeDown))

			// Act
			handler.ServeHTTP(response, request)

			// Assert
			ckr.AssertNumberOfCalls(t, "Check", 1)
			assert.Equal(t, "application/json; charset=utf-8", response.Header().Get("content-type"))
			assert.Equal(t, tc.expectedStatusCode, response.Result().StatusCode)

			result := health.Result{}
			_ = json.Unmarshal(response.Body.Bytes(), &result)
			log.Printf("returned %+v, want %+v", result.Details, tc.expectedStatus.Details)

			assert.True(t, reflect.DeepEqual(result, tc.expectedStatus))
		})
	}
}

func TestWhenChecksEmptyThenHandlerResultContainNoChecksMap(t *testing.T) {
	// Arrange
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	// Act
	health.NewHandler(health.NewChecker()).ServeHTTP(w, r)

	// Assert
	if w.Body.String() != "{\"status\":\"up\"}" {
		t.Errorf("response does not contain the expected result")
	}
}
