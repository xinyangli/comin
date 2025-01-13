package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/nlewo/comin/internal/deployment"
	"github.com/nlewo/comin/internal/generation"
	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/internal/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func generationStatus(g generation.Generation) {
	fmt.Printf("  Current Generation\n")
	switch g.Status {
	case generation.Init:
		fmt.Printf("    Status: initializated\n")
	case generation.Evaluating:
		fmt.Printf("    Status: evaluating (since %s)\n", humanize.Time(g.EvalStartedAt))
	case generation.EvaluationSucceeded:
		fmt.Printf("    Status: evaluated (%s)\n", humanize.Time(g.EvalEndedAt))
	case generation.Building:
		fmt.Printf("    Status: building (since %s)\n", humanize.Time(g.BuildStartedAt))
	case generation.BuildSucceeded:
		fmt.Printf("    Status: built (%s)\n", humanize.Time(g.BuildEndedAt))
	case generation.EvaluationFailed:
		fmt.Printf("    Status: evaluation failed (%s)\n", humanize.Time(g.EvalEndedAt))
	case generation.BuildFailed:
		fmt.Printf("    Status: build failed (%s)\n", humanize.Time(g.BuildEndedAt))
	}
	printCommit(g.SelectedRemoteName, g.SelectedBranchName, g.SelectedCommitId, g.SelectedCommitMsg)
}

func deploymentStatus(d deployment.Deployment) {
	fmt.Printf("  Current Deployment\n")
	fmt.Printf("    Operation: %s\n", d.Operation)
	switch d.Status {
	case deployment.Init:
		fmt.Printf("    Status: initializated\n")
	case deployment.Running:
		fmt.Printf("    Status: running (since %s)\n", humanize.Time(d.StartAt))
	case deployment.Done:
		fmt.Printf("    Status: succeeded (%s)\n", humanize.Time(d.EndAt))
	case deployment.Failed:
		fmt.Printf("    Status: failed (%s)\n", humanize.Time(d.EndAt))
	}
	printCommit(d.Generation.SelectedRemoteName, d.Generation.SelectedBranchName, d.Generation.SelectedCommitId, d.Generation.SelectedCommitMsg)
}

func printCommit(selectedRemoteName, selectedBranchName, selectedCommitId, selectedCommitMsg string) {
	fmt.Printf("    Commit %s from '%s/%s'\n",
		selectedCommitId,
		selectedRemoteName,
		selectedBranchName,
	)
	fmt.Printf("      %s\n",
		utils.FormatCommitMsg(selectedCommitMsg),
	)
}

func getStatus() (status manager.State, err error) {
	url := "http://localhost:4242/api/status"
	client := http.Client{
		Timeout: time.Second * 2,
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return
	}
	res, err := client.Do(req)
	if err != nil {
		return
	}
	if res.Body != nil {
		defer res.Body.Close()
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &status)
	if err != nil {
		return
	}
	return
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get the status of the local machine",
	Args:  cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		status, err := getStatus()
		if err != nil {
			logrus.Fatal(err)
		}
		fmt.Printf("Status of the machine %s\n", status.Builder.Hostname)
		needToReboot := "no"
		if status.NeedToReboot {
			needToReboot = "yes"
		}
		fmt.Printf("  Need to reboot: %s\n", needToReboot)
		for _, r := range status.Fetcher.RepositoryStatus.Remotes {
			fmt.Printf("  Remote %s fetched %s\n",
				r.Url, humanize.Time(r.FetchedAt),
			)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
