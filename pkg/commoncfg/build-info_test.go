package commoncfg_test

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

func TestUpdateConfigVersion(t *testing.T) {
	plainJson :=
		`{
			"branch":"main",
			"org": "openkcm",
			"product": "common-sdk",
			"repo": "github.com/openkcm/common-sdk",
			"sha": "abc123def456",
			"version": "1.2.3",
			"buildTime": "2024-01-01T12:00:00Z"
		}`
	b64 := base64.StdEncoding.EncodeToString([]byte(plainJson))

	wantBuildInfo := commoncfg.BuildInfo{
		Component: commoncfg.Component{
			Branch:    "main",
			Org:       "openkcm",
			Product:   "common-sdk",
			Repo:      "github.com/openkcm/common-sdk",
			SHA:       "abc123def456",
			Version:   "1.2.3",
			BuildTime: "2024-01-01T12:00:00Z",
		},
	}

	plainJsonWithComponents :=
		`{
			"branch":"main",
			"org": "openkcm",
			"product": "common-sdk",
			"repo": "github.com/openkcm/common-sdk",
			"sha": "abc123def456",
			"version": "1.2.3",
			"buildTime": "2024-01-01T12:00:00Z",
			"components": [
				{
					"branch":"dev",
					"org": "openkcm",
					"product": "component-a",
					"repo": "github.com/openkcm/component-a",
					"sha": "def789ghi012",
					"version": "0.9.0",
					"buildTime": "2024-01-01T12:00:00Z"
				},
				{
					"branch":"release",
					"org": "openkcm",
					"product": "component-b",
					"repo": "github.com/openkcm/component-b",
					"sha": "ghi345jkl678",
					"version": "2.1.0",
					"buildTime": "2024-01-01T12:00:00Z"	
				}
			]
		}`

	b64WithComponents := base64.StdEncoding.EncodeToString([]byte(plainJsonWithComponents))

	wantBuildInfoWithComponents := commoncfg.BuildInfo{
		Component: commoncfg.Component{
			Branch:    "main",
			Org:       "openkcm",
			Product:   "common-sdk",
			Repo:      "github.com/openkcm/common-sdk",
			SHA:       "abc123def456",
			Version:   "1.2.3",
			BuildTime: "2024-01-01T12:00:00Z",
		},
		Components: []commoncfg.Component{
			{
				Branch:    "dev",
				Org:       "openkcm",
				Product:   "component-a",
				Repo:      "github.com/openkcm/component-a",
				SHA:       "def789ghi012",
				Version:   "0.9.0",
				BuildTime: "2024-01-01T12:00:00Z",
			},
			{
				Branch:    "release",
				Org:       "openkcm",
				Product:   "component-b",
				Repo:      "github.com/openkcm/component-b",
				SHA:       "ghi345jkl678",
				Version:   "2.1.0",
				BuildTime: "2024-01-01T12:00:00Z",
			},
		},
	}

	tests := []struct {
		name          string
		input         string
		wantErr       bool
		wantBuildInfo commoncfg.BuildInfo
	}{
		{
			name:          "plain json",
			input:         plainJson,
			wantErr:       false,
			wantBuildInfo: wantBuildInfo,
		},
		{
			name:          "base64 wrapped",
			input:         fmt.Sprintf("base64(%s)", b64),
			wantErr:       false,
			wantBuildInfo: wantBuildInfo,
		},
		{
			name:          "plain json with components",
			input:         plainJsonWithComponents,
			wantErr:       false,
			wantBuildInfo: wantBuildInfoWithComponents,
		},
		{
			name:          "base64 wrapped with components",
			input:         fmt.Sprintf("base64(%s)", b64WithComponents),
			wantErr:       false,
			wantBuildInfo: wantBuildInfoWithComponents,
		},
		{
			name:    "invalid json",
			input:   "not-a-json",
			wantErr: true,
		},
		{
			name:    "invalid base64",
			input:   "base64(invalid***)",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &commoncfg.BaseConfig{}

			err := commoncfg.UpdateConfigVersion(cfg, tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantBuildInfo, cfg.Application.BuildInfo)
		})
	}
}

func TestUpdateBuildInfoComponents(t *testing.T) {
	main := `{
		"branch":"main",
		"org": "openkcm",
		"product": "common-sdk",
		"repo": "github.com/openkcm/common-sdk",
		"sha": "abc123def456",
		"version": "1.2.3",
		"buildTime": "2024-01-01T12:00:00Z"
	}`

	wantComponent := commoncfg.Component{
		Branch:    "main",
		Org:       "openkcm",
		Product:   "common-sdk",
		Repo:      "github.com/openkcm/common-sdk",
		SHA:       "abc123def456",
		Version:   "1.2.3",
		BuildTime: "2024-01-01T12:00:00Z",
	}

	components := []string{
		`{
			"branch":"dev",
			"org": "openkcm",
			"product": "component-a",
			"repo": "github.com/openkcm/component-a",
			"sha": "def789ghi012",
			"version": "0.9.0",
			"buildTime": "2024-01-01T12:00:00Z"
		}`,
		`{
				"branch":"release",
				"org": "openkcm",
				"product": "component-b",
				"repo": "github.com/openkcm/component-b",
				"sha": "ghi345jkl678",
				"version": "2.1.0",
				"buildTime": "2024-01-01T12:00:00Z"
			}`,
	}

	wantBuildInfoWithComponents := []commoncfg.Component{
		{
			Branch:    "dev",
			Org:       "openkcm",
			Product:   "component-a",
			Repo:      "github.com/openkcm/component-a",
			SHA:       "def789ghi012",
			Version:   "0.9.0",
			BuildTime: "2024-01-01T12:00:00Z",
		},
		{
			Branch:    "release",
			Org:       "openkcm",
			Product:   "component-b",
			Repo:      "github.com/openkcm/component-b",
			SHA:       "ghi345jkl678",
			Version:   "2.1.0",
			BuildTime: "2024-01-01T12:00:00Z",
		},
	}

	tests := []struct {
		name              string
		main              string
		components        []string
		wantComps         commoncfg.BuildInfo
		wantMainErr       bool
		wantComponentsErr bool
	}{
		{
			name:       "valid main component (plain JSON)",
			main:       main,
			components: []string{},
			wantComps: commoncfg.BuildInfo{
				Component: wantComponent,
			},
		},
		{
			name:       "multiple components from JSON with components field",
			main:       main,
			components: components,
			wantComps: commoncfg.BuildInfo{
				Component:  wantComponent,
				Components: wantBuildInfoWithComponents,
			},
		},
		{
			name:        "invalid main JSON returns error",
			main:        `{not-json`,
			components:  []string{},
			wantMainErr: true,
		},
		{
			name: "invalid component JSON returns error",
			main: main,
			components: []string{
				`{not-json`,
			},
			wantComponentsErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &commoncfg.BaseConfig{}

			err := commoncfg.UpdateConfigVersion(cfg, tt.main)
			if tt.wantMainErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			err = commoncfg.UpdateComponentsOfBuildInfo(cfg, tt.components...)
			if tt.wantComponentsErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			assert.Equal(t, tt.wantComps, cfg.Application.BuildInfo)
		})
	}
}
