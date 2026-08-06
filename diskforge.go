package diskforge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/ioplane/diskforge/internal/image"
	linuxsystem "github.com/ioplane/diskforge/internal/linux"
	"github.com/ioplane/diskforge/internal/policy"
)

const defaultHTTPTimeout = 6 * time.Hour

// Engine coordinates reusable inspection, staging, and guarded writes.
type Engine struct {
	client   *http.Client
	system   linuxsystem.Boundary
	progress func(Progress)
}

// Option configures an Engine during construction.
type Option func(*Engine) error

// WithHTTPClient configures a reusable client with a mandatory total timeout.
func WithHTTPClient(client *http.Client) Option {
	return func(engine *Engine) error {
		if client == nil {
			return fmt.Errorf("%w: HTTP client is nil", ErrInvalidRequest)
		}
		if client.Timeout <= 0 {
			return fmt.Errorf("%w: HTTP client timeout must be positive", ErrInvalidRequest)
		}
		clone := *client
		engine.client = &clone

		return nil
	}
}

// WithProgress installs a callback for monotonic write progress.
func WithProgress(callback func(Progress)) Option {
	return func(engine *Engine) error {
		engine.progress = callback

		return nil
	}
}

func withBoundary(boundary linuxsystem.Boundary) Option {
	return func(engine *Engine) error {
		if boundary == nil {
			return fmt.Errorf("%w: Linux boundary is nil", ErrInvalidRequest)
		}
		engine.system = boundary

		return nil
	}
}

// New constructs an immutable reusable Engine.
func New(options ...Option) (*Engine, error) {
	engine := &Engine{
		client: &http.Client{Timeout: defaultHTTPTimeout},
		system: linuxsystem.DefaultSystem(),
	}
	for index, option := range options {
		if option == nil {
			return nil, fmt.Errorf("%w: option %d is nil", ErrInvalidRequest, index)
		}
		if err := option(engine); err != nil {
			return nil, fmt.Errorf("apply option %d: %w", index, err)
		}
	}

	return engine, nil
}

// Close releases the verified descriptor owned by a staged image.
func (staged *StagedImage) Close() error {
	if staged == nil || staged.close == nil {
		return nil
	}

	return staged.close()
}

// ConfirmationToken returns consent bound to the complete target and image identity.
func ConfirmationToken(target TargetIdentity, source ImageIdentity) (string, error) {
	token, err := policy.ConfirmationToken(toPolicyTarget(target), toPolicyImage(source))

	return token, publicError(err)
}

// Inspect evaluates the current Linux host without opening the target writable.
func (engine *Engine) Inspect(
	ctx context.Context,
	request InspectRequest,
) (Inspection, error) {
	if err := validateContext(ctx); err != nil {
		return Inspection{}, err
	}
	if err := validateInspectRequest(request); err != nil {
		return Inspection{}, err
	}
	inspection, err := engine.system.Inspect(ctx, linuxRequest(request))

	return fromPolicyInspection(inspection), publicError(err)
}

// Stage downloads, verifies, and atomically publishes one bounded image.
func (engine *Engine) Stage(
	ctx context.Context,
	request StageRequest,
) (*StagedImage, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if err := validateStageRequest(request); err != nil {
		return nil, err
	}
	verified, err := image.Stage(
		ctx,
		engine.client,
		request.URL,
		request.Destination,
		request.SHA256,
		request.MaximumBytes,
	)
	if err != nil {
		return nil, publicError(err)
	}

	return &StagedImage{
		Path:            verified.Path,
		SHA256:          verified.SHA256,
		Format:          string(verified.Format),
		CompressedBytes: verified.CompressedBytes,
		close:           verified.Close,
	}, nil
}

// Write performs a complete dry run or the guarded destructive sequence.
func (engine *Engine) Write(
	ctx context.Context,
	request WriteRequest,
) (WriteResult, error) {
	if err := validateContext(ctx); err != nil {
		return WriteResult{}, err
	}
	if err := validateWriteRequest(request); err != nil {
		return WriteResult{}, err
	}
	internalRequest := linuxWriteRequest(request)
	if request.DryRun {
		inspection, verified, err := linuxsystem.Prepare(ctx, internalRequest, engine.system)
		if err != nil {
			return WriteResult{}, publicError(err)
		}
		if err := verified.Close(); err != nil {
			return WriteResult{}, fmt.Errorf("close verified dry-run source: %w", err)
		}

		return WriteResult{
			DryRun:     true,
			Inspection: fromPolicyInspection(inspection),
		}, nil
	}

	result, err := linuxsystem.Execute(ctx, internalRequest, engine.system, engine.progressCallback())
	if err != nil {
		return WriteResult{}, publicError(err)
	}

	return WriteResult{WrittenBytes: result.WrittenBytes}, nil
}

func (engine *Engine) progressCallback() func(image.Progress) {
	if engine.progress == nil {
		return nil
	}

	return func(update image.Progress) {
		engine.progress(Progress{
			WrittenBytes:  update.WrittenBytes,
			ExpectedBytes: update.ExpectedBytes,
		})
	}
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("%w: context is nil", ErrInvalidRequest)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	return nil
}

func validateInspectRequest(request InspectRequest) error {
	if err := validateMode(request.Mode); err != nil {
		return err
	}
	if err := validateTargetPath(request.TargetPath); err != nil {
		return err
	}
	if err := validateImagePath(request.ImagePath); err != nil {
		return err
	}
	if err := validateDigest(request.SHA256); err != nil {
		return err
	}
	if request.ExpectedBytes <= 0 {
		return &GateError{Code: GateInvalidImageSize, Message: "expected image size must be positive"}
	}

	return nil
}

func validateStageRequest(request StageRequest) error {
	parsed, err := url.ParseRequestURI(request.URL)
	if err != nil || parsed.Host == "" || parsed.User != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%w: stage URL must be an absolute HTTP(S) URL without userinfo", ErrInvalidRequest)
	}
	if err := validateImagePath(request.Destination); err != nil {
		return err
	}
	if err := validateDigest(request.SHA256); err != nil {
		return err
	}
	if request.MaximumBytes <= 0 || request.MaximumBytes == math.MaxInt64 {
		return fmt.Errorf("%w: maximum stage size must be positive and bounded", ErrInvalidRequest)
	}

	return nil
}

func validateWriteRequest(request WriteRequest) error {
	if err := validateInspectRequest(InspectRequest{
		Mode:          request.Mode,
		TargetPath:    request.TargetPath,
		ImagePath:     request.ImagePath,
		SHA256:        request.SHA256,
		ExpectedBytes: request.ExpectedBytes,
	}); err != nil {
		return err
	}
	if !request.DryRun && request.Confirmation == "" {
		return &GateError{Code: GateConfirmation, Message: "confirmation token is required"}
	}
	if request.Mode == ModeRescue && request.Reboot {
		return fmt.Errorf("%w: reboot is valid only in live mode", ErrInvalidRequest)
	}

	return nil
}

func validateMode(mode Mode) error {
	if mode != ModeRescue && mode != ModeLive {
		return &GateError{Code: GateInvalidMode, Message: fmt.Sprintf("unsupported write mode %q", mode)}
	}

	return nil
}

func validateTargetPath(path string) error {
	cleaned := filepath.Clean(path)
	if cleaned != path || !filepath.IsAbs(path) || filepath.Dir(path) != "/dev" || filepath.Base(path) == "." {
		return &GateError{Code: GateInvalidTarget, Message: "target must be a canonical /dev/<name> path"}
	}

	return nil
}

func validateImagePath(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("%w: image path must be canonical and absolute", ErrInvalidRequest)
	}

	return nil
}

func validateDigest(value string) error {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size || len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return &GateError{
			Code:    GateInvalidDigest,
			Message: "SHA-256 digest must contain exactly 64 lowercase hexadecimal characters",
		}
	}

	return nil
}

func linuxRequest(request InspectRequest) linuxsystem.Request {
	return linuxsystem.Request{
		Mode:          policy.Mode(request.Mode),
		TargetPath:    request.TargetPath,
		ImagePath:     request.ImagePath,
		SHA256:        request.SHA256,
		ExpectedBytes: request.ExpectedBytes,
	}
}

func linuxWriteRequest(request WriteRequest) linuxsystem.Request {
	internal := linuxRequest(InspectRequest{
		Mode:          request.Mode,
		TargetPath:    request.TargetPath,
		ImagePath:     request.ImagePath,
		SHA256:        request.SHA256,
		ExpectedBytes: request.ExpectedBytes,
	})
	internal.Confirmation = request.Confirmation
	internal.Reboot = request.Reboot

	return internal
}

func toPolicyTarget(target TargetIdentity) policy.TargetIdentity {
	return policy.TargetIdentity{
		CanonicalPath: target.CanonicalPath,
		Serial:        target.Serial,
		WWN:           target.WWN,
		SizeBytes:     target.SizeBytes,
		KName:         target.KName,
		IsPartition:   target.IsPartition,
		Descendants:   append([]string(nil), target.Descendants...),
	}
}

func toPolicyImage(source ImageIdentity) policy.ImageIdentity {
	return policy.ImageIdentity{
		Path:              source.Path,
		SHA256:            source.SHA256,
		Format:            source.Format,
		CompressedBytes:   source.CompressedBytes,
		UncompressedBytes: source.UncompressedBytes,
	}
}

func fromPolicyInspection(inspection policy.Inspection) Inspection {
	return Inspection{
		Mode:              Mode(inspection.Mode),
		Target:            fromPolicyTarget(inspection.Target),
		Image:             fromPolicyImage(inspection.Image),
		ConfirmationToken: inspection.ConfirmationToken,
	}
}

func fromPolicyTarget(target policy.TargetIdentity) TargetIdentity {
	return TargetIdentity{
		CanonicalPath: target.CanonicalPath,
		Serial:        target.Serial,
		WWN:           target.WWN,
		SizeBytes:     target.SizeBytes,
		KName:         target.KName,
		IsPartition:   target.IsPartition,
		Descendants:   append([]string(nil), target.Descendants...),
	}
}

func fromPolicyImage(source policy.ImageIdentity) ImageIdentity {
	return ImageIdentity{
		Path:              source.Path,
		SHA256:            source.SHA256,
		Format:            source.Format,
		CompressedBytes:   source.CompressedBytes,
		UncompressedBytes: source.UncompressedBytes,
	}
}

func publicError(err error) error {
	if err == nil {
		return nil
	}
	var gate *policy.GateError
	if errors.As(err, &gate) {
		return &GateError{Code: GateCode(gate.Code), Message: gate.Message, Cause: err}
	}
	if errors.Is(err, image.ErrInvalidDigest) || errors.Is(err, image.ErrDigestMismatch) {
		return &GateError{Code: GateInvalidDigest, Message: err.Error(), Cause: err}
	}
	if errors.Is(err, image.ErrSizeMismatch) {
		return &GateError{Code: GateInvalidImageSize, Message: err.Error(), Cause: err}
	}

	return err
}
