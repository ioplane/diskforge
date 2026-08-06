// Package naming enforces the portable identifiers used by release tooling.
package naming

import (
	"errors"
	"fmt"
	"strings"
)

const (
	maxArtifactNameLength = 255
	maxLabelLength        = 63
	semverComponentCount  = 3
)

var ErrInvalidArtifactName = errors.New("invalid artifact name")

// ArtifactName returns the canonical release archive name for a stable tag.
func ArtifactName(version, goos, goarch, extension string) (string, error) {
	if !ValidSemverTag(version) {
		return "", fmt.Errorf("%w: stable release tag %q", ErrInvalidArtifactName, version)
	}
	if !ValidLabel(goos) {
		return "", fmt.Errorf("%w: operating system label %q", ErrInvalidArtifactName, goos)
	}
	if !ValidLabel(goarch) {
		return "", fmt.Errorf("%w: architecture label %q", ErrInvalidArtifactName, goarch)
	}
	if !validExtension(extension) {
		return "", fmt.Errorf("%w: artifact extension %q", ErrInvalidArtifactName, extension)
	}

	name := fmt.Sprintf(
		"diskforge_%s_%s_%s.%s",
		strings.TrimPrefix(version, "v"),
		goos,
		goarch,
		extension,
	)
	if len(name) > maxArtifactNameLength {
		return "", fmt.Errorf("%w: exceeds %d bytes", ErrInvalidArtifactName, maxArtifactNameLength)
	}

	return name, nil
}

// ValidSemverTag reports whether value is a stable vMAJOR.MINOR.PATCH tag.
func ValidSemverTag(value string) bool {
	if len(value) < len("v0.0.0") || value[0] != 'v' {
		return false
	}

	components := strings.Split(value[1:], ".")
	if len(components) != semverComponentCount {
		return false
	}

	for _, component := range components {
		if !validNumericIdentifier(component) {
			return false
		}
	}

	return true
}

func validNumericIdentifier(value string) bool {
	if value == "" || len(value) > 1 && value[0] == '0' {
		return false
	}

	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}

	return true
}

// ValidLabel reports whether value is a lowercase RFC 1123-style label.
func ValidLabel(value string) bool {
	if value == "" || len(value) > maxLabelLength {
		return false
	}
	if !isLowerAlphanumeric(value[0]) || !isLowerAlphanumeric(value[len(value)-1]) {
		return false
	}

	for index := 1; index < len(value)-1; index++ {
		if !isLowerAlphanumeric(value[index]) && value[index] != '-' {
			return false
		}
	}

	return true
}

func validExtension(value string) bool {
	if value == "" || len(value) > maxLabelLength {
		return false
	}

	previousWasSeparator := true
	for index := range len(value) {
		character := value[index]
		if isLowerAlphanumeric(character) {
			previousWasSeparator = false
			continue
		}
		if (character != '.' && character != '-') || previousWasSeparator {
			return false
		}

		previousWasSeparator = true
	}

	return !previousWasSeparator
}

func isLowerAlphanumeric(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
}
