package nix

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nlewo/comin/internal/profile"
	"github.com/sirupsen/logrus"
)

// GetExpectedMachineId evals
// nixosConfigurations.MACHINE.config.services.comin.machineId and
// returns (machine-id, nil) is comin.machineId is set, ("", nil) otherwise.
func getExpectedMachineId(path, hostname string) (machineId string, err error) {
	machineId = ""
	return
	expr := fmt.Sprintf("%s#nixosConfigurations.%s.config.services.comin.machineId", path, hostname)
	args := []string{
		"eval",
		expr,
		"--json",
	}
	var stdout bytes.Buffer
	err = runNixCommand(args, &stdout, os.Stderr)
	if err != nil {
		return
	}
	var machineIdPtr *string
	err = json.Unmarshal(stdout.Bytes(), &machineIdPtr)
	if err != nil {
		return
	}
	if machineIdPtr != nil {
		logrus.Debugf("Getting comin.machineId = %s", *machineIdPtr)
		machineId = *machineIdPtr
	} else {
		logrus.Debugf("Getting comin.machineId = null (not set)")
		machineId = ""
	}
	return
}

func runNixCommand(args []string, stdout, stderr io.Writer) (err error) {
	commonArgs := []string{"--extra-experimental-features", "nix-command", "--extra-experimental-features", "flakes", "--accept-flake-config"}
	args = append(commonArgs, args...)
	cmdStr := fmt.Sprintf("nix %s", strings.Join(args, " "))
	logrus.Infof("nix: running '%s'", cmdStr)
	cmd := exec.Command("nix", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Command '%s' fails with %s", cmdStr, err)
	}
	return nil
}

func Eval(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, machineId string, err error) {
	drvPath, outPath, err = ShowDerivation(ctx, flakeUrl, hostname)
	if err != nil {
		return
	}
	machineId, err = getExpectedMachineId(flakeUrl, hostname)
	return
}

func ShowDerivation(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, err error) {
	resp, err := http.Get(fmt.Sprintf("https://raw.githubusercontent.com/xinyangli/nixos-config/refs/heads/deploy-comin-eval/eval/%s.json", hostname))
	// TODO: Error message
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var output map[string]Derivation
	err = json.Unmarshal(body, &output)
	if err != nil {
		return
	}
	keys := make([]string, 0, len(output))
	for key := range output {
		keys = append(keys, key)
	}
	drvPath = keys[0]
	outPath = output[drvPath].Outputs.Out.Path
	logrus.Infof("nix: the derivation path is %s", drvPath)
	logrus.Infof("nix: the output path is %s", outPath)
	return
}

type Path struct {
	Path string `json:"path"`
}

type Output struct {
	Out Path `json:"out"`
}

type Derivation struct {
	Outputs Output `json:"outputs"`
}

type Show struct {
	NixosConfigurations map[string]struct{} `json:"nixosConfigurations"`
}

func List(flakeUrl string) (hosts []string, err error) {
	args := []string{
		"flake",
		"show",
		"--json",
		flakeUrl,
	}
	var stdout bytes.Buffer
	err = runNixCommand(args, &stdout, os.Stderr)
	if err != nil {
		return
	}

	var output Show
	err = json.Unmarshal(stdout.Bytes(), &output)
	if err != nil {
		return
	}
	hosts = make([]string, 0, len(output.NixosConfigurations))
	for key := range output.NixosConfigurations {
		hosts = append(hosts, key)
	}
	return
}

func Build(ctx context.Context, drvPath string) (err error) {
	args := []string{
		"build",
		fmt.Sprintf("%s^*", drvPath),
		"-L",
		"--no-link"}
	err = runNixCommand(args, os.Stdout, os.Stderr)
	if err != nil {
		return
	}
	return
}

func RemoteBuild(ctx context.Context, outPath string) (err error) {
	args := []string{
		"build",
		fmt.Sprintf("%s", outPath),
		"-L",
		"--no-link"}
	err = runNixCommand(args, os.Stdout, os.Stderr)
	if err != nil {
		return
	}
	return
}

func cominUnitFileHash() string {
	logrus.Infof("nix: generating the comin.service unit file sha256: 'systemctl cat comin.service | sha256sum'")
	cmd := exec.Command("systemctl", "cat", "comin.service")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Infof("nix: command 'systemctl cat comin.service' fails with '%s'", err)
		return ""
	}
	sum := sha256.Sum256(stdout.Bytes())
	hash := fmt.Sprintf("%x", sum)
	logrus.Infof("nix: the comin.service unit file sha256 is '%s'", hash)
	return hash
}

func switchToConfiguration(operation string, outPath string, dryRun bool) error {
	switchToConfigurationExe := filepath.Join(outPath, "bin", "switch-to-configuration")
	logrus.Infof("nix: running '%s %s'", switchToConfigurationExe, operation)
	cmd := exec.Command(switchToConfigurationExe, operation)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if dryRun {
		logrus.Infof("nix: dry-run enabled: '%s switch' has not been executed", switchToConfigurationExe)
	} else {
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Command %s switch fails with %s", switchToConfigurationExe, err)
		}
		logrus.Infof("nix: switch successfully terminated")
	}
	return nil
}

func Deploy(ctx context.Context, expectedMachineId, outPath, operation string) (needToRestartComin bool, profilePath string, err error) {
	beforeCominUnitFileHash := cominUnitFileHash()

	// This is required to write boot entries
	// Only do this is operation is switch or boot
	if profilePath, err = profile.SetSystemProfile(operation, outPath, false); err != nil {
		return
	}

	if err = switchToConfiguration(operation, outPath, false); err != nil {
		return
	}

	afterCominUnitFileHash := cominUnitFileHash()

	if beforeCominUnitFileHash != afterCominUnitFileHash {
		needToRestartComin = true
	}

	logrus.Infof("nix: deployment ended")

	return
}
