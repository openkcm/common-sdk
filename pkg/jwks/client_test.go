package jwks_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/common-sdk/pkg/jwks"
)

func TestClient(t *testing.T) {
	t.Run("init", func(t *testing.T) {
		t.Run("should return error if url is invalid", func(t *testing.T) {
			// given
			endpoint := "invalid url"

			// when
			res, err := jwks.NewClient(endpoint)

			// then
			assert.Error(t, err)
			assert.ErrorIs(t, err, jwks.ErrInvalidURL)
			assert.Nil(t, res)
		})

		t.Run("should not return error if url is invalid", func(t *testing.T) {
			// given
			endpoint := "https://someurl.calling.com/endpoint"

			// when
			res, err := jwks.NewClient(endpoint)

			// then
			assert.NoError(t, err)
			assert.NotNil(t, res)
		})
	})

	t.Run("Get", func(t *testing.T) {
		// given
		expJWKS := jwks.JWKS{
			Keys: []jwks.Key{
				{
					Kty:    "kty",
					Alg:    "alg",
					Use:    "use",
					KeyOps: []string{"keyop"},
					Kid:    "kid",
					X5c:    []string{"x5c"},
					N:      "N",
					E:      "E",
				},
			},
		}

		t.Run("should return JWKS", func(t *testing.T) {
			// given
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, err := json.Marshal(expJWKS)
				assert.NoError(t, err)

				_, err = w.Write(b)
				assert.NoError(t, err)
			}))

			defer srv.Close()

			subj, err := jwks.NewClient(srv.URL)
			assert.NoError(t, err)

			// when
			res, err := subj.Get(t.Context())

			// then
			assert.NoError(t, err)
			assert.NotNil(t, res)
			assert.Equal(t, &expJWKS, res)
		})

		t.Run("should return error", func(t *testing.T) {
			t.Run("if jwks server returns a http status other than http.StatusOK", func(t *testing.T) {
				// given
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
				defer srv.Close()

				subj, err := jwks.NewClient(srv.URL)
				assert.NoError(t, err)

				// when
				res, err := subj.Get(t.Context())

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, jwks.ErrHTTPStatusNotOK)
				assert.Nil(t, res)
			})

			t.Run("if response from server is not a valid json", func(t *testing.T) {
				// given
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					b, err := json.Marshal("invalid")
					assert.NoError(t, err)

					_, err = w.Write(b)
					assert.NoError(t, err)
				}))

				defer srv.Close()

				subj, err := jwks.NewClient(srv.URL)
				assert.NoError(t, err)

				// when
				res, err := subj.Get(t.Context())

				// then
				expErr := &json.UnmarshalTypeError{Value: "string", Type: reflect.TypeFor[jwks.JWKS]()}

				assert.Error(t, err)
				assert.ErrorContains(t, err, expErr.Error())
				assert.Nil(t, res)
			})

			t.Run("if response from server is not there", func(t *testing.T) {
				// given
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				}))

				defer srv.Close()

				subj, err := jwks.NewClient(srv.URL)
				assert.NoError(t, err)

				// when
				res, err := subj.Get(t.Context())

				// then

				assert.Error(t, err)
				assert.ErrorContains(t, err, "unexpected end of JSON input")
				assert.Nil(t, res)
			})
		})
	})
}
