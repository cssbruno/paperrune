// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package layoutengine

import (
	"errors"
	"fmt"
)

// DisplayItem selects one typed payload for the final global command order.
// The compositor supports positioned graphics, core glyph runs, images, and
// links without performing layout.
type DisplayItem struct {
	Kind    DisplayCommandKind
	Payload uint32
	Page    uint32
}

type DisplayListInput struct {
	Fonts          []CoreFontResource
	GlyphRuns      []CoreGlyphRun
	ImageResources []ImageResource
	Images         []PlannedImage
	Destinations   []PlannedDestination
	Links          []PlannedLink
	Paths          []PlannedPath
	Transforms     []Transform
	Clips          []PlannedClip
	Fills          []PlannedFill
	Strokes        []PlannedStroke
	Items          []DisplayItem
}

// AttachDisplayList composes already planned graphics, text, and image payloads onto a
// resource-free geometry plan. Items must be in page paint order. The function
// derives command geometry and ownership; it performs no layout or sizing.
func AttachDisplayList(plan LayoutPlan, input DisplayListInput) (LayoutPlan, error) {
	if len(plan.pages) == 0 {
		return LayoutPlan{}, errors.New("layoutengine: display-list attachment requires a non-empty plan")
	}
	projection := plan.ReadOnlyProjection()
	if len(projection.Commands) != 0 || len(projection.Fonts) != 0 || len(projection.GlyphRuns) != 0 ||
		len(projection.ImageResources) != 0 || len(projection.Images) != 0 ||
		len(projection.Destinations) != 0 || len(projection.Links) != 0 || len(projection.Paths) != 0 ||
		len(projection.Transforms) != 0 || len(projection.Clips) != 0 || len(projection.Fills) != 0 || len(projection.Strokes) != 0 {
		return LayoutPlan{}, errors.New("layoutengine: display-list attachment requires a resource-free geometry plan")
	}
	if uint64(len(input.Items)) > uint64(^uint32(0)) {
		return LayoutPlan{}, errors.New("layoutengine: display item count exceeds plan index capacity")
	}
	fragmentPages := make(map[FragmentID]uint32, len(projection.Fragments))
	for _, fragment := range projection.Fragments {
		fragmentPages[fragment.ID] = fragment.Page
	}
	commands := make([]DisplayCommand, 0, len(input.Items))
	pageCounts := make([]uint32, len(projection.Pages))
	var previousPage uint32
	for index, item := range input.Items {
		var command DisplayCommand
		switch item.Kind {
		case CommandSaveState, CommandRestoreState:
			if item.Payload != 0 || item.Page == 0 {
				return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d]", index), "state command requires one page and no payload")
			}
			command = DisplayCommand{Kind: item.Kind}
		case CommandTransform:
			if uint64(item.Payload) >= uint64(len(input.Transforms)) || item.Page == 0 {
				return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d]", index), "references a missing transform or page")
			}
			command = DisplayCommand{Kind: item.Kind, Payload: item.Payload}
		case CommandClip:
			if uint64(item.Payload) >= uint64(len(input.Clips)) {
				return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d].payload", index), "references a missing clip")
			}
			clip := input.Clips[item.Payload]
			if uint64(clip.Path) >= uint64(len(input.Paths)) {
				return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d]", index), "clip references a missing path")
			}
			command = DisplayCommand{Kind: item.Kind, Fragment: clip.Fragment, Bounds: input.Paths[clip.Path].Bounds, Payload: item.Payload}
		case CommandFillPath:
			if uint64(item.Payload) >= uint64(len(input.Fills)) {
				return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d].payload", index), "references a missing fill")
			}
			fill := input.Fills[item.Payload]
			if uint64(fill.Path) >= uint64(len(input.Paths)) {
				return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d]", index), "fill references a missing path")
			}
			command = DisplayCommand{Kind: item.Kind, Fragment: fill.Fragment, Bounds: input.Paths[fill.Path].Bounds, Payload: item.Payload}
		case CommandStrokePath:
			if uint64(item.Payload) >= uint64(len(input.Strokes)) {
				return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d].payload", index), "references a missing stroke")
			}
			stroke := input.Strokes[item.Payload]
			if uint64(stroke.Path) >= uint64(len(input.Paths)) {
				return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d]", index), "stroke references a missing path")
			}
			command = DisplayCommand{Kind: item.Kind, Fragment: stroke.Fragment, Bounds: input.Paths[stroke.Path].Bounds, Payload: item.Payload}
		case CommandGlyphRun:
			if uint64(item.Payload) >= uint64(len(input.GlyphRuns)) {
				return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d].payload", index), "references a missing glyph run")
			}
			run := input.GlyphRuns[item.Payload]
			if uint64(run.Line) >= uint64(len(projection.Lines)) {
				return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d]", index), "glyph run references a missing line")
			}
			line := projection.Lines[run.Line]
			command = DisplayCommand{Kind: item.Kind, Fragment: line.Fragment, Bounds: line.Bounds, Payload: item.Payload}
		case CommandImage:
			if uint64(item.Payload) >= uint64(len(input.Images)) {
				return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d].payload", index), "references a missing planned image")
			}
			image := input.Images[item.Payload]
			command = DisplayCommand{Kind: item.Kind, Fragment: image.Fragment, Bounds: image.Bounds, Payload: item.Payload}
		case CommandLink:
			if uint64(item.Payload) >= uint64(len(input.Links)) {
				return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d].payload", index), "references a missing planned link")
			}
			link := input.Links[item.Payload]
			command = DisplayCommand{Kind: item.Kind, Fragment: link.Fragment, Bounds: link.Bounds, Payload: item.Payload}
		default:
			return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d].kind", index), "is not supported by the initial compositor")
		}
		page := fragmentPages[command.Fragment]
		if page == 0 {
			page = item.Page
		}
		if item.Page != 0 && page != item.Page {
			return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d].page", index), "does not match payload ownership")
		}
		if page == 0 || uint64(page) > uint64(len(pageCounts)) {
			return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d]", index), "payload has no owning page")
		}
		if previousPage > page {
			return LayoutPlan{}, planError(fmt.Sprintf("display_items[%d]", index), "commands are not in page paint order")
		}
		previousPage = page
		pageCounts[page-1]++
		commands = append(commands, command)
	}
	pages := cloneSlice(projection.Pages)
	var commandStart uint32
	for index := range pages {
		pages[index].Commands = IndexRange{Start: commandStart, Count: pageCounts[index]}
		commandStart += pageCounts[index]
	}
	result := plan
	result.pages = pages
	result.fonts = cloneFontResources(input.Fonts)
	result.glyphRuns = cloneCoreGlyphRuns(input.GlyphRuns)
	result.imageResources = cloneSlice(input.ImageResources)
	result.images = clonePlannedImages(input.Images)
	result.destinations = cloneSlice(input.Destinations)
	result.links = cloneSlice(input.Links)
	result.paths = clonePlannedPaths(input.Paths)
	result.transforms = cloneSlice(input.Transforms)
	result.clips = cloneSlice(input.Clips)
	result.fills = cloneSlice(input.Fills)
	result.strokes = clonePlannedStrokes(input.Strokes)
	result.commands = commands
	if plan.hasDeterministicInputs {
		result.deterministicInputs = DeterministicInputManifest{}
		result.hasDeterministicInputs = false
	}
	if err := result.Validate(); err != nil {
		return LayoutPlan{}, err
	}
	if !plan.hasDeterministicInputs {
		return result, nil
	}
	return rebindDeterministicResources(result, &plan.deterministicInputs)
}
