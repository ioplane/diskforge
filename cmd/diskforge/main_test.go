package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/ioplane/diskforge"
)

type fakeEngine struct {
	inspectRequest diskforge.InspectRequest
	stageRequest   diskforge.StageRequest
	writeRequest   diskforge.WriteRequest
	progress       func(diskforge.Progress)
	result         diskforge.WriteResult
	err            error
}

func (engine *fakeEngine) Inspect(
	_ context.Context,
	request diskforge.InspectRequest,
) (diskforge.Inspection, error) {
	engine.inspectRequest = request

	return diskforge.Inspection{Mode: request.Mode}, engine.err
}

func (engine *fakeEngine) Stage(
	_ context.Context,
	request diskforge.StageRequest,
) (*diskforge.StagedImage, error) {
	engine.stageRequest = request

	return &diskforge.StagedImage{
		Path:            request.Destination,
		SHA256:          request.SHA256,
		Format:          "raw",
		CompressedBytes: 1024,
	}, engine.err
}

func (engine *fakeEngine) Write(
	_ context.Context,
	request diskforge.WriteRequest,
) (diskforge.WriteResult, error) {
	engine.writeRequest = request
	if engine.progress != nil {
		engine.progress(diskforge.Progress{ExpectedBytes: request.ExpectedBytes})
		engine.progress(diskforge.Progress{
			WrittenBytes:  request.ExpectedBytes,
			ExpectedBytes: request.ExpectedBytes,
		})
	}

	return engine.result, engine.err
}

func factoryFor(engine *fakeEngine) engineFactory {
	return func(progress func(diskforge.Progress)) (engineAPI, error) {
		engine.progress = progress

		return engine, nil
	}
}

func TestRunInspectEmitsStableJSON(t *testing.T) {
	t.Parallel()

	engine := &fakeEngine{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"inspect",
		"--mode=rescue",
		"--target=/dev/vda",
		"--image=/images/image.raw",
		"--sha256=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"--expected-bytes=2048",
	}, stdout, stderr, factoryFor(engine))
	if exitCode != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("run() exit=%d stderr=%q", exitCode, stderr.String())
	}
	wantRequest := diskforge.InspectRequest{
		Mode:          diskforge.ModeRescue,
		TargetPath:    "/dev/vda",
		ImagePath:     "/images/image.raw",
		SHA256:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpectedBytes: 2048,
	}
	if !reflect.DeepEqual(engine.inspectRequest, wantRequest) {
		t.Fatalf("Inspect() request = %#v", engine.inspectRequest)
	}
	var result diskforge.Inspection
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.Mode != diskforge.ModeRescue {
		t.Fatalf("stdout=%q result=%#v error=%v", stdout.String(), result, err)
	}
}

func TestRunWriteEmitsResultAndBoundedProgress(t *testing.T) {
	t.Parallel()

	engine := &fakeEngine{result: diskforge.WriteResult{WrittenBytes: 2048}}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(t.Context(), []string{
		"write",
		"--mode=rescue",
		"--target=/dev/vda",
		"--image=/images/image.raw",
		"--sha256=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"--expected-bytes=2048",
		"--confirmation=confirm-v1-vda-aaaaaaaaaaaa-testbinding0000",
	}, stdout, stderr, factoryFor(engine))
	if exitCode != exitSuccess {
		t.Fatalf("run() exit=%d stderr=%q", exitCode, stderr.String())
	}
	var result diskforge.WriteResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.WrittenBytes != 2048 {
		t.Fatalf("stdout=%q result=%#v error=%v", stdout.String(), result, err)
	}
	lines := bytes.Count(stderr.Bytes(), []byte("\n"))
	if lines != 2 || !bytes.Contains(stderr.Bytes(), []byte(`"written_bytes":2048`)) {
		t.Fatalf("stderr progress = %q", stderr.String())
	}
}

func TestRunMapsUsageGateCancellationAndOperationalErrors(t *testing.T) {
	t.Parallel()

	validInspect := []string{
		"inspect",
		"--mode=rescue",
		"--target=/dev/vda",
		"--image=/images/image.raw",
		"--sha256=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"--expected-bytes=2048",
	}
	tests := map[string]struct {
		args []string
		err  error
		want int
	}{
		"usage":        {args: []string{"unknown"}, want: exitUsage},
		"gate":         {args: validInspect, err: &diskforge.GateError{Code: diskforge.GateNotRoot}, want: exitGate},
		"cancellation": {args: validInspect, err: context.Canceled, want: exitCanceled},
		"operation":    {args: validInspect, err: errors.New("injected operation"), want: exitOperational},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			stderr := &bytes.Buffer{}
			engine := &fakeEngine{err: test.err}
			got := run(t.Context(), test.args, &bytes.Buffer{}, stderr, factoryFor(engine))
			if got != test.want || stderr.Len() == 0 {
				t.Fatalf("run() exit=%d stderr=%q, want exit=%d", got, stderr.String(), test.want)
			}
		})
	}
}

func TestRunVersionJSONDoesNotConstructEngine(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	factory := func(func(diskforge.Progress)) (engineAPI, error) {
		t.Fatal("engine factory called for version")

		return nil, errors.New("engine factory called for version")
	}
	exitCode := run(t.Context(), []string{"version", "--json"}, stdout, &bytes.Buffer{}, factory)
	if exitCode != exitSuccess {
		t.Fatalf("run() exit = %d", exitCode)
	}
	var result map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("version JSON = %q: %v", stdout.String(), err)
	}
	for _, field := range []string{"version", "commit", "build_date", "go_version", "os", "arch"} {
		if result[field] == "" {
			t.Fatalf("version field %q = empty; result=%#v", field, result)
		}
	}
}
