package featureflags

import (
	"github.com/open-feature/go-sdk/openfeature"
	"github.com/thomaspoignant/go-feature-flag/modules/core/model"
)

var ToEvalContext = toEvalContext

var GoffErrorToResolutionError = goffErrorToResolutionError

func ToResolutionDetailBool(res model.VariationResult[bool], err error) openfeature.ProviderResolutionDetail {
	return toResolutionDetail(res, err)
}
