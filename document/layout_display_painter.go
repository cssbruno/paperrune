// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/cssbruno/paperrune/internal/layoutengine"
)

type plannedImageSources map[layoutengine.ImageContentDigest][]byte
type plannedFontSources map[layoutengine.CoreFontMetricsDigest][]byte

func chargePlannedFontSourceBytes(seen map[layoutengine.CoreFontMetricsDigest]bool, digest layoutengine.CoreFontMetricsDigest, size, limit uint64, used *uint64) error {
	if seen[digest] {
		return nil
	}
	if used == nil || *used > limit || size > limit-*used {
		return fmt.Errorf("%w: cumulative embedded font source bytes exceed limit", errCoreLayoutPlanPaintUnsupported)
	}
	*used += size
	seen[digest] = true
	return nil
}

type plannedImageLookupBudget struct {
	remainingLookups uint64
	remainingBytes   uint64
}

func newPlannedImageLookupBudget(resources int, maxBytes int) (plannedImageLookupBudget, error) {
	if resources < 0 || maxBytes <= 0 {
		return plannedImageLookupBudget{}, fmt.Errorf("%w: invalid planned image lookup limits", errCoreLayoutPlanPaintUnsupported)
	}
	return plannedImageLookupBudget{remainingLookups: uint64(resources), remainingBytes: uint64(maxBytes)}, nil
}

func lookupPlannedImageSourceContext(ctx context.Context, sources plannedImageSources, digest layoutengine.ImageContentDigest, budget *plannedImageLookupBudget) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if budget == nil || budget.remainingLookups == 0 {
		return nil, fmt.Errorf("%w: planned image lookup count exceeds limit", errCoreLayoutPlanPaintUnsupported)
	}
	budget.remainingLookups--
	encoded, exists := sources[digest]
	if !exists || len(encoded) == 0 {
		return nil, fmt.Errorf("%w: image %s bytes are unavailable", errCoreLayoutPlanPaintUnsupported, digest)
	}
	if uint64(len(encoded)) > budget.remainingBytes {
		return nil, fmt.Errorf("%w: cumulative planned image source bytes exceed limit", errCoreLayoutPlanPaintUnsupported)
	}
	budget.remainingBytes -= uint64(len(encoded))
	return encoded, nil
}

type preparedDisplayImage struct {
	resource   layoutengine.ImageResource
	key        string
	info       *ImageInfo
	minVersion string
}

type preparedDisplaySemantic struct {
	id               layoutengine.SemanticNodeID
	role             string
	alt              string
	actual           string
	lang             string
	header           bool
	scope            string
	rowSpan, colSpan uint32
}

type preparedDisplayImageCrop struct {
	enabled                        bool
	clipX, clipY, clipW, clipH     float64
	imageX, imageY, imageW, imageH float64
}

type preparedDisplayPlanPDF struct {
	fonts            map[layoutengine.FontResourceID]preparedCorePlanFont
	fontOrder        []layoutengine.FontResourceID
	images           map[layoutengine.ImageResourceID]preparedDisplayImage
	imageOrder       []layoutengine.ImageResourceID
	projection       layoutengine.LayoutPlanProjection
	imageCrops       []preparedDisplayImageCrop
	semanticPaths    map[layoutengine.FragmentID][]preparedDisplaySemantic
	documentLanguage string
}

// paintDisplayLayoutPlanPDF is the initial mixed text/image production sink.
// All plan validation, byte-digest verification, image decoding, intrinsic
// dimension checks, and resource preparation complete before target mutation.
func (f *pdfDocument) paintDisplayLayoutPlanPDF(plan layoutengine.LayoutPlan, sources plannedImageSources) error {
	prepared, err := f.preflightDisplayLayoutPlanPDF(plan, sources)
	if err != nil {
		return err
	}
	return f.paintPreparedDisplayLayoutPlanPDF(prepared)
}

func (f *pdfDocument) paintPreparedDisplayLayoutPlanPDF(prepared preparedDisplayPlanPDF) error {
	return f.paintPreparedDisplayLayoutPlanPDFAtCurrentPage(prepared, false, 0, false)
}

func typedPDFSemanticRole(node layoutengine.SemanticNode) string {
	switch node.Role {
	case layoutengine.SemanticRoleSection:
		return "Sect"
	case layoutengine.SemanticRoleHeading:
		if node.Attributes.HeadingLevel >= 1 && node.Attributes.HeadingLevel <= 6 {
			return fmt.Sprintf("H%d", node.Attributes.HeadingLevel)
		}
		return "H"
	case layoutengine.SemanticRoleParagraph:
		if strings.Contains(string(node.Key), "caption") {
			return "Caption"
		}
		return "P"
	case layoutengine.SemanticRoleList:
		return "L"
	case layoutengine.SemanticRoleListItem:
		return "LI"
	case layoutengine.SemanticRoleTable:
		return "Table"
	case layoutengine.SemanticRoleRow:
		return "TR"
	case layoutengine.SemanticRoleCell:
		if node.Attributes.TableHeader {
			return "TH"
		}
		return "TD"
	case layoutengine.SemanticRoleFigure:
		return "Figure"
	case layoutengine.SemanticRoleLink:
		return "Link"
	default:
		return ""
	}
}

// ensureTaggedListBody adds the PDF/UA list-item children that are implicit in
// the renderer-neutral semantic tree. The planned text remains owned by the
// existing list item content, but the tagged PDF gets the required Lbl/LBody
// siblings without changing layout geometry or plan identity.
func (f *pdfDocument) ensureTaggedListBody(item *taggedElement) *taggedElement {
	if item == nil || item.Role != taggedRoleLI {
		return item
	}
	var body *taggedElement
	for _, child := range item.Children {
		if child == nil {
			continue
		}
		switch child.Role {
		case taggedRoleLbl:
		case taggedRoleLBody:
			body = child
		}
	}
	if body != nil {
		return body
	}
	label := &taggedElement{Role: taggedRoleLbl, MCID: -1, Parent: item}
	body = &taggedElement{Role: taggedRoleLBody, MCID: -1, Parent: item}
	item.Children = append(item.Children, label, body)
	f.tagged.elems = append(f.tagged.elems, label, body)
	return body
}

func (f *pdfDocument) beginPreparedSemantic(path []preparedDisplaySemantic, elements map[layoutengine.SemanticNodeID]*taggedElement) func() {
	if !f.tagged.enabled || len(path) == 0 {
		return func() {}
	}
	leaf := path[len(path)-1]
	if leaf.role == "Artifact" {
		f.BeginArtifact()
		return f.EndArtifact
	}
	var parent *taggedElement
	for _, semantic := range path {
		var elem *taggedElement
		if semantic.id.Valid() {
			elem = elements[semantic.id]
		}
		if elem == nil {
			elem = &taggedElement{Role: semantic.role, MCID: -1, Alt: semantic.alt,
				ActualText: semantic.actual, Lang: semantic.lang}
			if semantic.header || semantic.scope != "" || semantic.rowSpan > 1 || semantic.colSpan > 1 {
				elem.Table = normalizeTaggedTableAttributes(semantic.role, taggedTableAttributes{
					Scope: semantic.scope, RowSpan: semantic.rowSpan, ColSpan: semantic.colSpan,
				})
			}
			if parent != nil {
				elem.Parent = parent
				parent.Children = append(parent.Children, elem)
			}
			f.tagged.elems = append(f.tagged.elems, elem)
			if semantic.id.Valid() {
				elements[semantic.id] = elem
			}
		}
		if semantic.role == taggedRoleLI {
			parent = f.ensureTaggedListBody(elem)
			continue
		}
		parent = elem
	}
	if parent == nil {
		return func() {}
	}
	mcid := f.registerPreparedSemanticElement(parent)
	if mcid < 0 {
		return func() {}
	}
	if leaf.role == taggedRoleLink {
		f.tagged.pendingLinkElem = parent
	}
	begin := f.beginPreparedTaggedContent(parent.Role, mcid)
	if len(begin) != 0 {
		f.outbytes(begin)
	}
	return func() {
		if len(begin) != 0 {
			f.outbytes(taggedEndMarkedContent)
		}
	}
}

func preparedDisplaySemanticPath(nodes []layoutengine.SemanticNode, leafID layoutengine.SemanticNodeID, destination []preparedDisplaySemantic) []preparedDisplaySemantic {
	if !leafID.Valid() {
		return destination
	}
	leaf := nodes[leafID-1]
	if leaf.Role == layoutengine.SemanticRoleArtifact {
		return append(destination, preparedDisplaySemantic{id: leaf.ID, role: "Artifact"})
	}
	for id := leafID; id.Valid(); {
		node := nodes[id-1]
		if node.Role != layoutengine.SemanticRoleDocument && node.Role != layoutengine.SemanticRoleArtifact {
			role := typedPDFSemanticRole(node)
			if role != "" {
				destination = append(destination, preparedDisplaySemantic{id: node.ID, role: role,
					alt: node.Attributes.AlternateText, actual: node.Attributes.ActualText,
					lang: node.Attributes.Language, header: node.Attributes.TableHeader,
					scope: node.Attributes.TableScope, rowSpan: node.Attributes.TableRowSpan,
					colSpan: node.Attributes.TableColumnSpan})
			}
		}
		id = node.Parent
	}
	for left, right := 0, len(destination)-1; left < right; left, right = left+1, right-1 {
		destination[left], destination[right] = destination[right], destination[left]
	}
	return destination
}

func plannedDisplayPageContentCapacity(projection layoutengine.LayoutPlanProjection, page layoutengine.PlannedPage, tagged bool) int {
	const maximum = 8 << 20
	total := 128
	add := func(amount int) {
		if amount <= 0 || total == maximum {
			return
		}
		if amount >= maximum-total {
			total = maximum
			return
		}
		total += amount
	}
	pathCost := func(path layoutengine.PlannedPath) int {
		return 32 + len(path.Segments)*72
	}
	for commandOffset := uint32(0); commandOffset < page.Commands.Count; commandOffset++ {
		commandIndex := page.Commands.Start + commandOffset
		command := projection.Commands[commandIndex]
		add(1) // command newline
		if tagged && (command.Kind == layoutengine.CommandGlyphRun || command.Kind == layoutengine.CommandImage || command.Kind == layoutengine.CommandLink) {
			add(63) // marked-content wrappers
		}
		switch command.Kind {
		case layoutengine.CommandSaveState, layoutengine.CommandRestoreState:
			add(4)
		case layoutengine.CommandTransform:
			add(128)
		case layoutengine.CommandClip:
			clip := projection.Clips[command.Payload]
			add(pathCost(projection.Paths[clip.Path]) + 16)
		case layoutengine.CommandFillPath:
			fill := projection.Fills[command.Payload]
			add(pathCost(projection.Paths[fill.Path]) + 64)
		case layoutengine.CommandStrokePath:
			stroke := projection.Strokes[command.Payload]
			add(pathCost(projection.Paths[stroke.Path]) + 128 + len(stroke.Dash)*24)
		case layoutengine.CommandGlyphRun:
			add(plannedGlyphRunCapacity(projection.GlyphRuns[command.Payload]))
		case layoutengine.CommandImage:
			add(256)
		case layoutengine.CommandLink:
			add(96)
		}
	}
	return total
}

func (f *pdfDocument) paintPreparedDisplayLayoutPlanPDFAtCurrentPage(prepared preparedDisplayPlanPDF, reuseCurrent bool, pageOffset int, preserveAuthoredText bool) error {
	projection := prepared.projection
	resources := f.ensureResourceStore()
	for _, id := range prepared.fontOrder {
		font := prepared.fonts[id]
		resources.setFont(font.key, font.font)
	}
	for _, id := range prepared.imageOrder {
		image := prepared.images[id]
		resources.setImage(image.key, image.info)
		f.requirePDFVersion(image.minVersion)
	}
	destinationLinks := make(map[layoutengine.DestinationID]int)
	if len(projection.Destinations) != 0 {
		destinationLinks = make(map[layoutengine.DestinationID]int, len(projection.Destinations))
	}
	semanticElements := make(map[layoutengine.SemanticNodeID]*taggedElement)
	if f.tagged.enabled && len(projection.SemanticNodes) != 0 {
		semanticElements = make(map[layoutengine.SemanticNodeID]*taggedElement, len(projection.SemanticNodes))
	}
	f.tagged.documentLanguage = prepared.documentLanguage
	for _, destination := range projection.Destinations {
		destinationLinks[destination.ID] = f.AddLink()
	}
	for pageIndex, page := range projection.Pages {
		size := Size{Wd: f.PointConvert(page.Size.Width.Points()), Ht: f.PointConvert(page.Size.Height.Points())}
		if !reuseCurrent || pageIndex != 0 {
			f.AddPageFormat("P", size)
			if f.err != nil {
				return f.err
			}
		}
		_ = f.pageContentCommandBuffer(plannedDisplayPageContentCapacity(projection, page, f.tagged.enabled))
		var previousRun layoutengine.CoreGlyphRun
		previousRunSet := false
		for commandOffset := uint32(0); commandOffset < page.Commands.Count; commandOffset++ {
			commandIndex := page.Commands.Start + commandOffset
			command := projection.Commands[commandIndex]
			switch command.Kind {
			case layoutengine.CommandSaveState:
				f.out("q")
			case layoutengine.CommandRestoreState:
				f.out("Q")
			case layoutengine.CommandTransform:
				f.paintPlannedTransform(page.Size.Height, projection.Transforms[command.Payload])
			case layoutengine.CommandClip:
				clip := projection.Clips[command.Payload]
				f.paintPlannedClip(page.Size.Height, projection.Paths[clip.Path], clip)
			case layoutengine.CommandFillPath:
				fill := projection.Fills[command.Payload]
				f.paintPlannedFill(page.Size.Height, projection.Paths[fill.Path], fill)
			case layoutengine.CommandStrokePath:
				stroke := projection.Strokes[command.Payload]
				f.paintPlannedStroke(page.Size.Height, projection.Paths[stroke.Path], stroke)
			case layoutengine.CommandGlyphRun:
				run := projection.GlyphRuns[command.Payload]
				font := prepared.fonts[run.Font]
				if preserveAuthoredText && !run.LeadingSpace && previousRunSet {
					run.LeadingSpace = previousRun.TrailingSpace
				}
				if run.Opacity != 0 {
					f.out("q")
					f.SetAlpha(run.Opacity.Points(), "Normal")
				}
				var closeSemantic func()
				if f.tagged.enabled {
					closeSemantic = f.beginPreparedSemantic(prepared.semanticPaths[command.Fragment], semanticElements)
				}
				content := f.pageContentCommandBuffer(plannedGlyphRunCapacity(run))
				if font.resource.EmbeddedUTF8 != nil {
					if preserveAuthoredText {
						content = appendPlannedUTF8GlyphRunActualText(content, font.font, page.Size.Height, run)
					} else {
						content = appendPlannedUTF8GlyphRun(content, font.font, page.Size.Height, run)
					}
				} else if preserveAuthoredText {
					content = appendPlannedCoreGlyphRunExactTJ(content, font.font, page.Size.Height, run)
				} else {
					content = appendPlannedCoreGlyphRun(content, font.font, page.Size.Height, run)
				}
				f.outbytes(content)
				if closeSemantic != nil {
					closeSemantic()
				}
				if run.Opacity != 0 {
					f.out("Q")
				}
				previousRun, previousRunSet = projection.GlyphRuns[command.Payload], true
			case layoutengine.CommandImage:
				image := projection.Images[command.Payload]
				asset := prepared.images[image.Resource]
				crop := prepared.imageCrops[command.Payload]
				var closeSemantic func()
				if f.tagged.enabled {
					closeSemantic = f.beginPreparedSemantic(prepared.semanticPaths[command.Fragment], semanticElements)
				}
				if image.Opacity != 0 {
					f.out("q")
					f.SetAlpha(image.Opacity.Points(), "Normal")
				}
				if crop.enabled {
					f.ClipRect(crop.clipX, crop.clipY, crop.clipW, crop.clipH, false)
					f.drawImageXObject(asset.info.i, crop.imageX, crop.imageY, crop.imageW, crop.imageH)
					f.ClipEnd()
				} else {
					bounds := image.Bounds
					f.drawImageXObject(asset.info.i,
						f.PointConvert(bounds.X.Points()), f.PointConvert(bounds.Y.Points()),
						f.PointConvert(bounds.Width.Points()), f.PointConvert(bounds.Height.Points()))
				}
				if image.Opacity != 0 {
					f.out("Q")
				}
				if closeSemantic != nil {
					closeSemantic()
				}
			case layoutengine.CommandLink:
				link := projection.Links[command.Payload]
				var closeSemantic func()
				if f.tagged.enabled {
					baseSemantic := prepared.semanticPaths[command.Fragment]
					var semanticScratch [17]preparedDisplaySemantic
					semantic := append(semanticScratch[:0], baseSemantic...)
					semantic = append(semantic, preparedDisplaySemantic{role: "Link"})
					closeSemantic = f.beginPreparedSemantic(semantic, semanticElements)
				}
				bounds := link.Bounds
				x := f.PointConvert(bounds.X.Points())
				y := f.PointConvert(bounds.Y.Points())
				width := f.PointConvert(bounds.Width.Points())
				height := f.PointConvert(bounds.Height.Points())
				if link.Destination.Valid() {
					f.newLink(x, y, width, height, destinationLinks[link.Destination], "")
				} else {
					f.newLink(x, y, width, height, 0, link.URI)
				}
				if closeSemantic != nil {
					closeSemantic()
				}
			}
		}
	}
	for _, destination := range projection.Destinations {
		f.setPlannedLink(destinationLinks[destination.ID], f.PointConvert(destination.Point.X.Points()),
			f.PointConvert(destination.Point.Y.Points()), int(destination.Page)+pageOffset)
	}
	return f.err
}

func (f *pdfDocument) preflightDisplayLayoutPlanPDF(plan layoutengine.LayoutPlan, sources plannedImageSources) (preparedDisplayPlanPDF, error) {
	return f.preflightDisplayLayoutPlanPDFContext(context.Background(), plan, sources)
}

func (f *pdfDocument) preflightDisplayLayoutPlanPDFContext(ctx context.Context, plan layoutengine.LayoutPlan, sources plannedImageSources) (preparedDisplayPlanPDF, error) {
	return f.preflightDisplayLayoutPlanPDFResourcesContextForTarget(ctx, plan, sources, nil, false, f.tagged.enabled)
}

func (f *pdfDocument) preflightDisplayLayoutPlanPDFContextForTarget(ctx context.Context, plan layoutengine.LayoutPlan, sources plannedImageSources, allowActivePage bool) (preparedDisplayPlanPDF, error) {
	return f.preflightDisplayLayoutPlanPDFResourcesContextForTarget(ctx, plan, sources, nil, allowActivePage, f.tagged.enabled)
}

func (f *pdfDocument) preflightDisplayLayoutPlanPDFResourcesContextForTarget(ctx context.Context, plan layoutengine.LayoutPlan, sources plannedImageSources, fontSources plannedFontSources, allowActivePage, taggedOutput bool) (preparedDisplayPlanPDF, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return preparedDisplayPlanPDF{}, err
	}
	if f == nil || f.err != nil || (!allowActivePage && (f.page != 0 || f.state != documentStateUnopened)) ||
		(allowActivePage && (f.page <= 0 || f.state != documentStatePageOpen)) ||
		f.k <= 0 || !isFiniteFloat(f.k) || f.clipNest != 0 || f.transformNest != 0 {
		return preparedDisplayPlanPDF{}, fmt.Errorf("%w: requires a fresh error-free document", errCoreLayoutPlanPaintUnsupported)
	}
	if f.headerFnc != nil || f.footerFnc != nil || f.footerFncLpi != nil || f.pageAddGuard != nil ||
		len(f.aliasMap) != 0 || f.aliasNbPagesStr != "" {
		return preparedDisplayPlanPDF{}, fmt.Errorf("%w: custom or deferred page behavior is present", errCoreLayoutPlanPaintUnsupported)
	}
	if err := layoutengine.ValidateTrustedDisplayPaintPlan(plan, layoutengine.DefaultDisplayPaintLimits()); err != nil {
		return preparedDisplayPlanPDF{}, fmt.Errorf("document: preflight display plan: %w", err)
	}
	projection := plan.ReadOnlyProjection()
	var semanticPaths map[layoutengine.FragmentID][]preparedDisplaySemantic
	documentLanguage := ""
	if taggedOutput {
		semanticPaths = make(map[layoutengine.FragmentID][]preparedDisplaySemantic, len(projection.SemanticFragments))
		for _, node := range projection.SemanticNodes {
			if node.Role == layoutengine.SemanticRoleDocument {
				documentLanguage = node.Attributes.Language
			}
		}
		for _, association := range projection.SemanticFragments {
			semanticPaths[association.Fragment] = preparedDisplaySemanticPath(projection.SemanticNodes, association.Semantic, make([]preparedDisplaySemantic, 0, 4))
		}
	}
	if f.limits.MaxPages > 0 && len(projection.Pages) > f.limits.MaxPages {
		return preparedDisplayPlanPDF{}, fmt.Errorf("%w: %d > %d", ErrPageLimitExceeded, len(projection.Pages), f.limits.MaxPages)
	}
	prepared := preparedDisplayPlanPDF{
		projection:       projection,
		semanticPaths:    semanticPaths,
		documentLanguage: documentLanguage,
	}
	if len(projection.Fonts) != 0 {
		prepared.fonts = make(map[layoutengine.FontResourceID]preparedCorePlanFont, len(projection.Fonts))
		prepared.fontOrder = make([]layoutengine.FontResourceID, 0, len(projection.Fonts))
	}
	if len(projection.ImageResources) != 0 {
		prepared.images = make(map[layoutengine.ImageResourceID]preparedDisplayImage, len(projection.ImageResources))
		prepared.imageOrder = make([]layoutengine.ImageResourceID, 0, len(projection.ImageResources))
	}
	if len(projection.Images) != 0 {
		prepared.imageCrops = make([]preparedDisplayImageCrop, len(projection.Images))
	}
	var fontSourceBytes uint64
	seenFontSources := make(map[layoutengine.CoreFontMetricsDigest]bool, len(projection.Fonts))
	for index, resource := range projection.Fonts {
		if index&31 == 0 {
			if err := ctx.Err(); err != nil {
				return preparedDisplayPlanPDF{}, err
			}
		}
		if resource.EmbeddedUTF8 != nil {
			data := fontSources[resource.EmbeddedUTF8.Digest]
			if err := chargePlannedFontSourceBytes(seenFontSources, resource.EmbeddedUTF8.Digest, uint64(len(data)), uint64(maxFontSourceBytes), &fontSourceBytes); err != nil {
				return preparedDisplayPlanPDF{}, err
			}
		}
		font, err := f.preflightPlanFontContext(ctx, resource, fontSources)
		if err != nil {
			return preparedDisplayPlanPDF{}, err
		}
		prepared.fonts[resource.ID] = font
		prepared.fontOrder = append(prepared.fontOrder, resource.ID)
	}
	lookupBudget, err := newPlannedImageLookupBudget(len(projection.ImageResources), f.imageSourceLimit())
	if err != nil {
		return preparedDisplayPlanPDF{}, err
	}
	var decodedTotal uint64
	for index, resource := range projection.ImageResources {
		if index&15 == 0 {
			if err := ctx.Err(); err != nil {
				return preparedDisplayPlanPDF{}, err
			}
		}
		encoded, err := lookupPlannedImageSourceContext(ctx, sources, resource.Digest, &lookupBudget)
		if err != nil {
			return preparedDisplayPlanPDF{}, err
		}
		pixels := uint64(resource.PixelWidth) * uint64(resource.PixelHeight)
		if pixels > ^uint64(0)/4 {
			return preparedDisplayPlanPDF{}, fmt.Errorf("%w: planned image decoded size overflows", errCoreLayoutPlanPaintUnsupported)
		}
		decoded := pixels * 4
		if decoded > uint64(f.imageDecodedLimit())-decodedTotal { // #nosec G115 -- fixed-width conversion is bounded by the surrounding parser, planner, or resource invariant
			return preparedDisplayPlanPDF{}, fmt.Errorf("%w: cumulative planned image decoded bytes exceed limit", errCoreLayoutPlanPaintUnsupported)
		}
		decodedTotal += decoded
		asset, err := f.preflightDisplayImageContext(ctx, resource, encoded)
		if err != nil {
			return preparedDisplayPlanPDF{}, err
		}
		prepared.images[resource.ID] = asset
		prepared.imageOrder = append(prepared.imageOrder, resource.ID)
	}
	for pageIndex, page := range projection.Pages {
		if pageIndex&31 == 0 {
			if err := ctx.Err(); err != nil {
				return preparedDisplayPlanPDF{}, err
			}
		}
		commandEnd := uint64(page.Commands.Start) + uint64(page.Commands.Count)
		if commandEnd > uint64(len(projection.Commands)) {
			return preparedDisplayPlanPDF{}, errors.New("document: display plan page has invalid command range")
		}
		for index := uint64(page.Commands.Start); index < commandEnd; index++ {
			command := projection.Commands[index]
			switch command.Kind {
			case layoutengine.CommandSaveState, layoutengine.CommandRestoreState:
			case layoutengine.CommandTransform, layoutengine.CommandClip, layoutengine.CommandFillPath,
				layoutengine.CommandStrokePath, layoutengine.CommandGlyphRun:
			case layoutengine.CommandImage:
				crop, cropErr := f.preflightDisplayImageCrop(projection.Images[command.Payload])
				if cropErr != nil {
					return preparedDisplayPlanPDF{}, cropErr
				}
				prepared.imageCrops[command.Payload] = crop
			case layoutengine.CommandLink:
				link := projection.Links[command.Payload]
				if link.URI != "" {
					checked, checkErr := checkedExternalLinkTarget(link.URI)
					if checkErr != nil || checked != link.URI {
						if checkErr == nil {
							checkErr = errors.New("target is not canonical")
						}
						return preparedDisplayPlanPDF{}, fmt.Errorf("document: invalid planned external link target: %w", checkErr)
					}
				}
			default:
				return preparedDisplayPlanPDF{}, errors.New("document: display plan contains an unsupported command")
			}
		}
	}
	return prepared, nil
}

func (f *pdfDocument) preflightDisplayImageCrop(image layoutengine.PlannedImage) (preparedDisplayImageCrop, error) {
	if image.Crop == nil {
		return preparedDisplayImageCrop{}, nil
	}
	destination := image.Crop.Clip
	source := image.Crop.Source
	intrinsic := image.Crop.Intrinsic
	destX := f.PointConvert(destination.X.Points())
	destY := f.PointConvert(destination.Y.Points())
	destW := f.PointConvert(destination.Width.Points())
	destH := f.PointConvert(destination.Height.Points())
	imageW := destW * float64(intrinsic.Width) / float64(source.Width)
	imageH := destH * float64(intrinsic.Height) / float64(source.Height)
	imageX := destX - destW*float64(source.X)/float64(source.Width)
	imageY := destY - destH*float64(source.Y)/float64(source.Height)
	if !finiteNumbers(destX, destY, destW, destH, imageX, imageY, imageW, imageH) ||
		destW <= 0 || destH <= 0 || imageW <= 0 || imageH <= 0 {
		return preparedDisplayImageCrop{}, fmt.Errorf("%w: invalid planned image crop transform", errCoreLayoutPlanPaintUnsupported)
	}
	return preparedDisplayImageCrop{
		enabled: true,
		clipX:   destX, clipY: destY, clipW: destW, clipH: destH,
		imageX: imageX, imageY: imageY, imageW: imageW, imageH: imageH,
	}, nil
}

func (f *pdfDocument) preflightDisplayImageContext(ctx context.Context, resource layoutengine.ImageResource, encoded []byte) (preparedDisplayImage, error) {
	if len(encoded) == 0 {
		return preparedDisplayImage{}, fmt.Errorf("%w: image %s bytes are unavailable", errCoreLayoutPlanPaintUnsupported, resource.Digest)
	}
	digest := sha256.New()
	for offset := 0; offset < len(encoded); offset += 64 << 10 {
		if err := ctx.Err(); err != nil {
			return preparedDisplayImage{}, err
		}
		end := offset + (64 << 10)
		if end > len(encoded) {
			end = len(encoded)
		}
		_, _ = digest.Write(encoded[offset:end])
	}
	if hex.EncodeToString(digest.Sum(nil)) != string(resource.Digest) {
		return preparedDisplayImage{}, fmt.Errorf("%w: image %s content digest mismatch", errCoreLayoutPlanPaintUnsupported, resource.Digest)
	}
	imageType := string(resource.Format)
	if resource.Format == layoutengine.ImageJPEG {
		imageType = "jpg"
	}
	info, minVersion, err := parseImageOptionsReaderWithLimitsContext(ctx,
		ImageOptions{ImageType: imageType}, bytes.NewReader(encoded), f.k, f.compressLevel, f.pdfVersion,
		f.imageSourceLimit(), f.imageDecodedLimit(),
	)
	if err != nil {
		return preparedDisplayImage{}, fmt.Errorf("document: decode planned image %s: %w", resource.Digest, err)
	}
	if info.w != float64(resource.PixelWidth) || info.h != float64(resource.PixelHeight) {
		return preparedDisplayImage{}, fmt.Errorf("%w: image %s intrinsic dimensions mismatch", errCoreLayoutPlanPaintUnsupported, resource.Digest)
	}
	if info.i, err = generateImageID(info); err != nil {
		return preparedDisplayImage{}, fmt.Errorf("document: identify planned image %s: %w", resource.Digest, err)
	}
	key := "plan-image-" + string(resource.Digest)
	if f.resources != nil {
		if existing, exists := f.resources.image(key); exists && (existing == nil || existing.i != info.i) {
			return preparedDisplayImage{}, fmt.Errorf("%w: document image shadows %s", errCoreLayoutPlanPaintUnsupported, resource.Digest)
		}
	}
	return preparedDisplayImage{resource: resource, key: key, info: info, minVersion: minVersion}, nil
}
