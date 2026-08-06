package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ioplane/diskforge"
	"github.com/ioplane/diskforge/internal/version"
)

const (
	exitSuccess     = 0
	exitOperational = 1
	exitUsage       = 2
	exitGate        = 3
	exitCanceled    = 4
	commandInspect  = "inspect"
	commandStage    = "stage"
	commandWrite    = "write"
	commandVersion  = "version"
)

type engineAPI interface {
	Inspect(ctx context.Context, request diskforge.InspectRequest) (diskforge.Inspection, error)
	Stage(ctx context.Context, request diskforge.StageRequest) (*diskforge.StagedImage, error)
	Write(ctx context.Context, request diskforge.WriteRequest) (diskforge.WriteResult, error)
}

type engineFactory func(func(diskforge.Progress)) (engineAPI, error)

type commonFlags struct {
	mode          string
	targetPath    string
	imagePath     string
	digest        string
	expectedBytes int64
}

type errorEnvelope struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Kind    string             `json:"kind"`
	Code    diskforge.GateCode `json:"code,omitempty"`
	Message string             `json:"message"`
}

func main() {
	os.Exit(run(
		context.Background(),
		os.Args[1:],
		os.Stdout,
		os.Stderr,
		productionFactory,
	))
}

func productionFactory(progress func(diskforge.Progress)) (engineAPI, error) {
	return diskforge.New(diskforge.WithProgress(progress))
}

func run(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	newEngine engineFactory,
) int {
	if len(args) == 0 {
		return usageError(stderr, "a command is required: inspect, stage, write, or version")
	}

	switch args[0] {
	case commandInspect:
		return runInspect(ctx, args[1:], stdout, stderr, newEngine)
	case commandStage:
		return runStage(ctx, args[1:], stdout, stderr, newEngine)
	case commandWrite:
		return runWrite(ctx, args[1:], stdout, stderr, newEngine)
	case commandVersion:
		return runVersion(args[1:], stdout, stderr)
	default:
		return usageError(stderr, fmt.Sprintf("unknown command %q", args[0]))
	}
}

func runInspect(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	newEngine engineFactory,
) int {
	flags, values := newCommonFlagSet(commandInspect)
	if err := flags.Parse(args); err != nil {
		return usageError(stderr, err.Error())
	}
	if flags.NArg() != 0 {
		return usageError(stderr, "inspect does not accept positional arguments")
	}
	engine, err := newEngine(nil)
	if err != nil {
		return operationError(stderr, err)
	}
	result, err := engine.Inspect(ctx, values.inspectRequest())
	if err != nil {
		return apiError(stderr, err)
	}

	return encodeResult(stdout, stderr, result)
}

func runStage(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	newEngine engineFactory,
) int {
	flags := newFlagSet(commandStage)
	sourceURL := flags.String("url", "", "absolute HTTP(S) source URL")
	destination := flags.String("destination", "", "canonical absolute destination path")
	digest := flags.String("sha256", "", "expected lowercase SHA-256")
	maximumBytes := flags.Int64("maximum-bytes", 0, "maximum compressed bytes")
	if err := flags.Parse(args); err != nil {
		return usageError(stderr, err.Error())
	}
	if flags.NArg() != 0 {
		return usageError(stderr, "stage does not accept positional arguments")
	}
	engine, err := newEngine(nil)
	if err != nil {
		return operationError(stderr, err)
	}
	staged, err := engine.Stage(ctx, diskforge.StageRequest{
		URL:          *sourceURL,
		Destination:  *destination,
		SHA256:       *digest,
		MaximumBytes: *maximumBytes,
	})
	if err != nil {
		return apiError(stderr, err)
	}
	if err := staged.Close(); err != nil {
		return operationError(stderr, fmt.Errorf("close staged image: %w", err))
	}

	return encodeResult(stdout, stderr, staged)
}

func runWrite(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	newEngine engineFactory,
) int {
	flags, values := newCommonFlagSet(commandWrite)
	confirmation := flags.String("confirmation", "", "target-bound confirmation token")
	reboot := flags.Bool("reboot", false, "approve immediate live-mode reboot")
	dryRun := flags.Bool("dry-run", false, "verify all non-destructive gates only")
	if err := flags.Parse(args); err != nil {
		return usageError(stderr, err.Error())
	}
	if flags.NArg() != 0 {
		return usageError(stderr, "write does not accept positional arguments")
	}
	progressEncoder := json.NewEncoder(stderr)
	engine, err := newEngine(func(progress diskforge.Progress) {
		if encodeErr := progressEncoder.Encode(progress); encodeErr != nil {
			return
		}
	})
	if err != nil {
		return operationError(stderr, err)
	}
	request := values.writeRequest()
	request.Confirmation = *confirmation
	request.Reboot = *reboot
	request.DryRun = *dryRun
	result, err := engine.Write(ctx, request)
	if err != nil {
		return apiError(stderr, err)
	}

	return encodeResult(stdout, stderr, result)
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(commandVersion)
	jsonOutput := flags.Bool("json", false, "emit stable JSON metadata")
	if err := flags.Parse(args); err != nil {
		return usageError(stderr, err.Error())
	}
	if flags.NArg() != 0 {
		return usageError(stderr, "version does not accept positional arguments")
	}
	info := version.Current()
	if *jsonOutput {
		return encodeResult(stdout, stderr, info)
	}
	if _, err := fmt.Fprintf(stdout, "diskforge %s (%s)\n", info.Version, info.Commit); err != nil {
		return operationError(stderr, fmt.Errorf("write version: %w", err))
	}

	return exitSuccess
}

func newCommonFlagSet(name string) (*flag.FlagSet, *commonFlags) {
	flags := newFlagSet(name)
	values := &commonFlags{}
	flags.StringVar(&values.mode, "mode", "", "write mode: rescue or live")
	flags.StringVar(&values.targetPath, "target", "", "canonical whole-disk path")
	flags.StringVar(&values.imagePath, "image", "", "canonical source image path")
	flags.StringVar(&values.digest, "sha256", "", "expected lowercase SHA-256")
	flags.Int64Var(&values.expectedBytes, "expected-bytes", 0, "expanded image bytes")

	return flags, values
}

func newFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	return flags
}

func (values commonFlags) inspectRequest() diskforge.InspectRequest {
	return diskforge.InspectRequest{
		Mode:          diskforge.Mode(values.mode),
		TargetPath:    values.targetPath,
		ImagePath:     values.imagePath,
		SHA256:        values.digest,
		ExpectedBytes: values.expectedBytes,
	}
}

func (values commonFlags) writeRequest() diskforge.WriteRequest {
	return diskforge.WriteRequest{
		Mode:          diskforge.Mode(values.mode),
		TargetPath:    values.targetPath,
		ImagePath:     values.imagePath,
		SHA256:        values.digest,
		ExpectedBytes: values.expectedBytes,
	}
}

func encodeResult(stdout, stderr io.Writer, value any) int {
	if err := json.NewEncoder(stdout).Encode(value); err != nil {
		return operationError(stderr, fmt.Errorf("encode result: %w", err))
	}

	return exitSuccess
}

func usageError(stderr io.Writer, message string) int {
	writeError(stderr, errorDetail{Kind: "usage", Message: message})

	return exitUsage
}

func operationError(stderr io.Writer, err error) int {
	writeError(stderr, errorDetail{Kind: "operational", Message: err.Error()})

	return exitOperational
}

func apiError(stderr io.Writer, err error) int {
	var gate *diskforge.GateError
	if errors.As(err, &gate) {
		writeError(stderr, errorDetail{Kind: "gate", Code: gate.Code, Message: gate.Error()})

		return exitGate
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		writeError(stderr, errorDetail{Kind: "canceled", Message: err.Error()})

		return exitCanceled
	}

	return operationError(stderr, err)
}

func writeError(stderr io.Writer, detail errorDetail) {
	if err := json.NewEncoder(stderr).Encode(errorEnvelope{Error: detail}); err != nil {
		return
	}
}
