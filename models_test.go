package resource_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	resource "github.com/telia-oss/github-pr-resource"
)

func TestSourceValidateNumberAndConfigKey(t *testing.T) {
	validKey := strings.Repeat("0a", 32)
	tests := []struct {
		description string
		source      resource.Source
		wantErr     string
	}{
		{description: "number and config_key are accepted", source: resource.Source{Number: 12, ConfigKey: validKey}},
		{description: "number and branch are mutually exclusive", source: resource.Source{Number: 12, Branch: "feature"}, wantErr: "mutually exclusive"},
		{description: "invalid config_key is rejected", source: resource.Source{ConfigKey: strings.ToUpper(validKey)}, wantErr: "config_key"},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			tc.source.Repository = "itsdalmo/test-repository"
			tc.source.AccessToken = "oauthtoken"
			err := tc.source.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}
