package commoncfg

import (
	"encoding/json"
	"errors"
	"runtime/debug"
	"strings"

	"github.com/openkcm/common-sdk/pkg/utils"
)

func UpdateConfigVersion(cfg *BaseConfig, buildInfo string) error {
	if bi, ok := debug.ReadBuildInfo(); ok {
		cfg.Application.RuntimeBuildInfo = bi
	}

	decodedBuildInfo, err := utils.ExtractFromComplexValue(buildInfo)
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(decodedBuildInfo), &cfg.Application.BuildInfo)
	if err != nil {
		return err
	}

	return nil
}

func UpdateComponentsOfBuildInfo(cfg *BaseConfig, components ...string) error {
	if len(cfg.Application.BuildInfo.Components) == 0 && len(components) > 0 {
		cfg.Application.BuildInfo.Components = make([]Component, 0, len(components))
	}

	lerr := make([]error, 0)
	for _, component := range components {
		if component == "" {
			continue
		}

		decodedBuildInfo, err := utils.ExtractFromComplexValue(component)
		if err != nil {
			lerr = append(lerr, err)
			continue
		}

		if !strings.HasPrefix(decodedBuildInfo, "{") && !strings.HasSuffix(decodedBuildInfo, "}") {
			continue
		}

		comp := Component{}
		err = json.Unmarshal([]byte(decodedBuildInfo), &comp)
		if err != nil {
			lerr = append(lerr, err)
			continue
		}

		cfg.Application.BuildInfo.Components = append(cfg.Application.BuildInfo.Components, comp)
	}

	if len(lerr) > 0 {
		return errors.Join(lerr...)
	}

	return nil
}
