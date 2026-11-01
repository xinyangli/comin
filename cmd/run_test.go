package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfigurationName(t *testing.T) {
	tests := []struct {
		name                     string
		hostname                 string
		homeManagerConfiguration string
		homeManager              bool
		expected                 string
		expectedError            string
	}{
		{
			name:                     "Home Manager configuration",
			hostname:                 "machine",
			homeManagerConfiguration: "testuser@laptop",
			homeManager:              true,
			expected:                 "testuser@laptop",
		},
		{
			name:        "system configuration",
			hostname:    "machine",
			homeManager: false,
			expected:    "machine",
		},
		{
			name:          "missing Home Manager configuration",
			hostname:      "machine",
			homeManager:   true,
			expectedError: "home-manager configuration name must be set with home_manager_configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, err := getConfigurationName(tt.hostname, tt.homeManagerConfiguration, tt.homeManager)
			assert.Equal(t, tt.expected, name)
			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedError)
			}
		})
	}
}
