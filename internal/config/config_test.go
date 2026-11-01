package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nlewo/comin/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestHomeManagerConfiguration(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "configuration.yaml")
	contents := []byte("hostname: machine\nhome_manager_configuration: testuser@laptop\nrepository_type: flake\n")
	assert.NoError(t, os.WriteFile(configPath, contents, 0o600))

	config, err := Read(configPath)
	assert.NoError(t, err)
	assert.Equal(t, "machine", config.Hostname)
	assert.Equal(t, "testuser@laptop", config.HomeManagerConfiguration)
}

func TestConfig(t *testing.T) {
	configPath := "./configuration.yaml"
	expected := types.Configuration{
		Hostname:              "machine",
		StateDir:              "/var/lib/comin",
		StateFilepath:         "/var/lib/comin/state.json",
		PostDeploymentCommand: "/some/path",
		RepositoryType:        "flake",
		RepositorySubdir:      ".",
		Remotes: []types.Remote{
			{
				Name: "origin",
				URL:  "https://framagit.org/owner/infra",
				Auth: types.Auth{
					AccessToken:     "my-secret",
					AccessTokenPath: "./secret",
					Username:        "comin",
				},
				Timeout: 300,
				Branches: types.Branches{
					Main:    types.Branch{Operation: "switch"},
					Testing: types.Branch{Operation: "test"},
				},
			},
			{
				Name: "local",
				URL:  "/home/owner/git/infra",
				Auth: types.Auth{
					AccessToken:     "",
					AccessTokenPath: "",
					Username:        "comin",
				},
				Timeout: 300,
				Branches: types.Branches{
					Main:    types.Branch{Operation: "switch"},
					Testing: types.Branch{Operation: "test"},
				},
			},
		},
		ApiServer: types.HttpServer{
			ListenAddress: "127.0.0.1",
			Port:          4242,
		},
		Exporter: types.HttpServer{
			ListenAddress: "0.0.0.0",
			Port:          4243,
		},
		Grpc: types.Grpc{
			UnixSocketPath: "/var/lib/comin/grpc.sock",
		},
		EvalTimeout:     1800,
		BuildTimeout:    1800,
		BuildRetryTimer: 30,
	}
	config, err := Read(configPath)
	assert.Nil(t, err)
	assert.Equal(t, expected, config)
}
