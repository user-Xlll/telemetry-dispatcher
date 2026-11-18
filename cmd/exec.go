package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

var waitFileFlag []string
var waitRetriesFlag int
var waitIntervalFlag time.Duration

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:   "exec [flags...] <binary-path> [child-args...]",
	Short: "Execute a child binary and forward stdout/stderr",
	Long: `Runs the specified binary as a child process. All stdout is forwarded
to the parent stdout, and all stderr is forwarded to the parent stderr.

Arguments after "--" are passed to the child binary. The first argument after
"--" is treated as the binary path; the rest are its arguments.

If no "--" separator is provided, all remaining arguments are passed to the
child binary, with the first one treated as the binary path.

Examples:
  telemetry-dispatcher exec -- /path/to/binary arg1 arg2
  telemetry-dispatcher --wait-file /some/file exec /path/to/binary arg1 arg2
  telemetry-dispatcher --wait-file /some/file --wait-interval 7s exec -- /path/to/binary arg1 --flag1
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		binaryPath := args[0]
		childArgs := args[1:]

		fmt.Printf("start waiting for file(s) [%v]\n", strings.Join(waitFileFlag, ", "))

		if err := waitForFiles(waitFileFlag, waitRetriesFlag, waitIntervalFlag); err != nil {
			return err
		}

		fmt.Printf("file(s) [%v] exist, executing binary %s...\n", strings.Join(waitFileFlag, ", "), binaryPath)

		c := exec.Command(binaryPath, childArgs...)

		stdout, err := c.StdoutPipe()
		if err != nil {
			return fmt.Errorf("creating stdout pipe: %w", err)
		}

		stderr, err := c.StderrPipe()
		if err != nil {
			return fmt.Errorf("creating stderr pipe: %w", err)
		}

		if err := c.Start(); err != nil {
			return fmt.Errorf("starting command: %w", err)
		}

		var wg sync.WaitGroup
		var stdoutErr, stderrErr error

		wg.Add(2)

		go func() {
			defer wg.Done()
			_, stdoutErr = io.Copy(os.Stdout, stdout)
		}()

		go func() {
			defer wg.Done()
			_, stderrErr = io.Copy(os.Stderr, stderr)
		}()

		wg.Wait()

		if stdoutErr != nil {
			return fmt.Errorf("reading stdout: %w", stdoutErr)
		}
		if stderrErr != nil {
			return fmt.Errorf("reading stderr: %w", stderrErr)
		}

		if err := c.Wait(); err != nil {
			return fmt.Errorf("command exited with error: %w", err)
		}

		return nil
	},
}

// Wait for a list of files to appear, retrying up to retries times with interval delay between checks.
// Returns nil as soon as all files exist, or an error after all retries are exhausted.
func waitForFiles(files []string, retries int, interval time.Duration) error {
	var missingFiles []string

	for i := range retries {
		missingFilesInThisAttempt, err := getMissingFiles(files)
		if err != nil {
			return err
		}

		missingFiles = missingFilesInThisAttempt

		if len(missingFiles) == 0 {
			return nil
		}

		fmt.Printf(
			"file(s) [%v] don't exist (next check in %v, %d attemps remaining)\n",
			strings.Join(missingFilesInThisAttempt, ", "), interval, retries-i-1,
		)

		if i != retries-1 {
			time.Sleep(interval)
		}
	}

	if len(missingFiles) > 0 {
		return fmt.Errorf(
			"file(s) [%v] still don't exist after %d attemps",
			strings.Join(missingFiles, " "), retries,
		)
	}

	return nil
}

// getMissingFiles checks which files from the given list do not exist and returns them.
// Returns an error if a non-NotExist stat error occurs (e.g., permission denied).
func getMissingFiles(files []string) ([]string, error) {
	missingFiles := []string{}

	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			if os.IsNotExist(err) {
				missingFiles = append(missingFiles, file)
			} else {
				return nil, fmt.Errorf("unable to get file %s stat: %v", file, err)
			}
		}
	}

	return missingFiles, nil
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringArrayVar(&waitFileFlag, "wait-file", []string{}, "Path to a file to wait for before executing the binary (can be specified multiple times)")
	execCmd.Flags().IntVar(&waitRetriesFlag, "wait-retries", 3, "Maximum number of retry attempts to wait for files to appear")
	execCmd.Flags().DurationVar(&waitIntervalFlag, "wait-interval", time.Second, "Time interval between retry attempts")
}
