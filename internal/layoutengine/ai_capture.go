// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package layoutengine

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

var ErrAICaptureEmpty = errors.New("layoutengine: AI capture plan has no pages")

// AICaptureMode selects either the non-disclosing geometry artifact or the
// paint-ready core-text preview artifact for every page in one bundle.
type AICaptureMode string

const (
	AICaptureGeometry AICaptureMode = "geometry_svg"
	AICaptureCoreText AICaptureMode = "core_text_svg"
)

func (mode AICaptureMode) valid() bool {
	return mode == AICaptureGeometry || mode == AICaptureCoreText
}

// AICaptureDisclosure is an explicit artifact disclosure classification.
type AICaptureDisclosure string

const (
	AIDisclosureGeometryOnly     AICaptureDisclosure = "geometry_only"
	AIDisclosureContainsUserText AICaptureDisclosure = "contains_user_text"
)

// AIPageArtifactMetadata is the canonical manifest entry for one page SVG.
// SHA256 is the lowercase digest of the exact detached artifact bytes.
type AIPageArtifactMetadata struct {
	Page             uint32              `json:"page"`
	Name             string              `json:"name"`
	Kind             AICaptureMode       `json:"kind"`
	MediaType        string              `json:"media_type"`
	FormatVersion    uint16              `json:"format_version"`
	PageBounds       Rect                `json:"page_bounds"`
	CaptureBounds    Rect                `json:"capture_bounds"`
	FixedScale       int64               `json:"fixed_scale"`
	Disclosure       AICaptureDisclosure `json:"disclosure"`
	ContainsUserText bool                `json:"contains_user_text"`
	ByteLength       uint64              `json:"byte_length"`
	SHA256           string              `json:"sha256"`
}

func captureAIPlanPage(plan LayoutPlan, page uint32, mode AICaptureMode, disclosure AICaptureDisclosure, containsUserText bool) (AIPageArtifactMetadata, []byte, error) {
	var (
		formatVersion uint16
		pageBounds    Rect
		captureBounds Rect
		svg           []byte
		err           error
	)
	switch mode {
	case AICaptureGeometry:
		capture, captureErr := plan.CaptureDebugGeometrySVGPage(page)
		if captureErr != nil {
			err = captureErr
			break
		}
		formatVersion, pageBounds, captureBounds, svg = capture.FormatVersion, capture.PageBounds, capture.CanvasBounds, capture.SVG
	case AICaptureCoreText:
		capture, captureErr := CaptureCorePlanSVG(plan, page)
		if captureErr != nil {
			err = captureErr
			break
		}
		formatVersion, pageBounds, captureBounds, svg = capture.FormatVersion, capture.PageBounds, capture.PageBounds, capture.SVG
	}
	if err != nil {
		return AIPageArtifactMetadata{}, nil, fmt.Errorf("layoutengine: capture AI page %d: %w", page, err)
	}
	digest := sha256.Sum256(svg)
	metadata := AIPageArtifactMetadata{
		Page: page, Name: fmt.Sprintf("page-%04d.%s.svg", page, mode), Kind: mode,
		MediaType: "image/svg+xml", FormatVersion: formatVersion,
		PageBounds: pageBounds, CaptureBounds: captureBounds, FixedScale: FixedScale,
		Disclosure: disclosure, ContainsUserText: containsUserText,
		ByteLength: uint64(len(svg)), SHA256: hex.EncodeToString(digest[:]),
	}
	return metadata, svg, nil
}
