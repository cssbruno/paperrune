// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// CompiledHTML stores the reusable parse products for an HTML fragment. It is
// safe to reuse across documents.
type compiledHTML struct {
	sourceBytes       int
	tokens            []htmlSegmentType
	cssRules          []htmlCSSRule
	styleDeclarations map[string]map[string]string
	nodeIndexes       []compiledHTMLNode
	tokenNode         []int
	elementEnd        []int
	elementDecl       []map[string]string
	tableStyleKeys    []string
	elementText       []compiledHTMLText
	elementMeta       []htmlElementMetadata
	maxDepth          int
	tables            map[int]compiledHTMLTable
	inlineSVGs        map[int]compiledInlineSVG
	dataImages        map[int]compiledHTMLDataImage
	recovery          []compiledHTMLRecoveryIssue
	// unifiedResolved is populated only on the detached snapshot consumed by
	// the HTML-to-IR adapter. Selector matching and cascade resolution happen
	// while creating that snapshot; the planner never receives CSS rules.
	unifiedResolved []htmlUnifiedResolvedElement
}

type compiledHTMLTable struct {
	table htmlTableType
	end   int
	start int
}

type compiledInlineSVG struct {
	svg *SVG
	end int
}

type compiledHTMLText struct {
	plain     string
	preserved string
	ok        bool
}

type compiledHTMLNode struct {
	Parent      int
	FirstChild  int
	NextSibling int
	Token       int
	EndToken    int
}

type compiledHTMLDataImage struct {
	name    string
	options ImageOptions
	data    []byte
}

// CompiledHTMLStats summarizes reusable parse products in a compiled fragment.
type compiledHTMLStats struct {
	Tokens       int
	Nodes        int
	Tables       int
	Images       int
	InlineSVGs   int
	CSSRules     int
	Recovery     int
	MaxDepth     int
	CachedText   int
	CachedStyles int
}

// CompiledHTMLRecoveryIssue describes one malformed-fragment recovery decision
// made while building the compiled node model.
type compiledHTMLRecoveryIssue struct {
	Kind  string
	Tag   string
	Token int
}

// CompileHTML tokenizes an HTML fragment, parses CSS rules, records element
// boundaries, pre-parses tables, and pre-parses inline SVGs for repeated
// rendering.
func compileHTML(htmlStr string, cacheReusableData ...bool) (*compiledHTML, error) {
	cache := true
	if len(cacheReusableData) != 0 {
		cache = cacheReusableData[0]
	}
	return compileHTMLWithDataImageLimitContext(context.Background(), htmlStr, cache, htmlDefaultMaxDataImageBytes)
}

// CompileHTMLContext tokenizes and compiles an HTML fragment while checking ctx
// during tokenization, data-image decoding, and inline-SVG parsing.
func compileHTMLContext(ctx context.Context, htmlStr string) (*compiledHTML, error) {
	if len(htmlStr) > htmlDefaultMaxHTMLBytes {
		return nil, errHTMLLimitExceeded
	}
	return compileHTMLWithDataImageLimitContext(ctx, htmlStr, true, htmlDefaultMaxDataImageBytes)
}

func compileHTMLWithDataImageLimitContext(ctx context.Context, htmlStr string, cacheReusableData bool, maxDataImageBytes int) (*compiledHTML, error) {
	tokens, err := htmlTokenizeContext(ctx, htmlStr, make(map[string]map[string]string))
	if err != nil {
		return nil, err
	}
	compiled := compileHTMLTokens(tokens, cacheReusableData)
	compiled.sourceBytes = len(htmlStr)
	if err := outputCanceledError(ctx); err != nil {
		return nil, err
	}
	if err := compiled.compileDataImagesContext(ctx, maxDataImageBytes); err != nil {
		return nil, err
	}
	if err := compiled.compileInlineSVGsContext(ctx); err != nil {
		return nil, err
	}
	return compiled, nil
}

func compileHTMLTokens(tokens []htmlSegmentType, cacheReusableData bool) *compiledHTML {
	compiled := &compiledHTML{
		tokens:     tokens,
		cssRules:   htmlCollectCSSRules(tokens),
		tokenNode:  make([]int, len(tokens)),
		elementEnd: make([]int, len(tokens)),
		tables:     make(map[int]compiledHTMLTable),
		inlineSVGs: make(map[int]compiledInlineSVG),
		dataImages: make(map[int]compiledHTMLDataImage),
	}
	if cacheReusableData {
		compiled.styleDeclarations = make(map[string]map[string]string)
		compiled.elementDecl = make([]map[string]string, len(tokens))
		compiled.tableStyleKeys = make([]string, len(tokens))
		compiled.elementText = make([]compiledHTMLText, len(tokens))
		compiled.elementMeta = make([]htmlElementMetadata, len(tokens))
	}
	for i := range compiled.tokenNode {
		compiled.tokenNode[i] = -1
	}
	for i := range compiled.elementEnd {
		compiled.elementEnd[i] = i
	}
	compiled.buildNodeIndexes()

	for i, token := range tokens {
		if token.Cat == 'O' && cacheReusableData {
			compiled.compileStyleDeclarations(token.Attr)
			compiled.elementMeta[i] = htmlElementMetadataFromSegment(token)
			compiled.compileElementText(i, token.Str)
		}
		if token.Cat == 'O' && token.Str == "table" {
			table, end := parseHTMLTable(tokens, i)
			compiled.tables[i] = compiledHTMLTable{table: table, end: end, start: i}
		}
	}
	if cacheReusableData {
		compiled.compileElementDeclarations()
		compiled.compileTableStyleKeys()
	}
	return compiled
}

func (compiled *compiledHTML) compileTableStyleKeys() {
	for tokenIndex, token := range compiled.tokens {
		if token.Cat != 'O' || (token.Str != "tr" && token.Str != "td" && token.Str != "th") {
			continue
		}
		compiled.tableStyleKeys[tokenIndex] = htmlTableCellStyleDeclarationKey(compiled.elementDecl[tokenIndex])
	}
}

func (compiled *compiledHTML) buildNodeIndexes() {
	compiled.nodeIndexes = make([]compiledHTMLNode, 0, len(compiled.tokens))
	stack := make([]int, 0, 16)
	lastChildByParent := make([]int, 0, len(compiled.tokens))
	for i, token := range compiled.tokens {
		switch token.Cat {
		case 'O':
			if !htmlClosePops(token.Str) {
				continue
			}
			parent := -1
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			nodeIndex := len(compiled.nodeIndexes)
			compiled.nodeIndexes = append(compiled.nodeIndexes, compiledHTMLNode{
				Parent:      parent,
				FirstChild:  -1,
				NextSibling: -1,
				Token:       i,
				EndToken:    i,
			})
			lastChildByParent = append(lastChildByParent, -1)
			compiled.tokenNode[i] = nodeIndex
			if parent >= 0 {
				if compiled.nodeIndexes[parent].FirstChild < 0 {
					compiled.nodeIndexes[parent].FirstChild = nodeIndex
				} else {
					compiled.nodeIndexes[lastChildByParent[parent]].NextSibling = nodeIndex
				}
				lastChildByParent[parent] = nodeIndex
			}
			stack = append(stack, nodeIndex)
			if len(stack) > compiled.maxDepth {
				compiled.maxDepth = len(stack)
			}
		case 'C':
			matched := false
			for len(stack) > 0 {
				nodeIndex := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				node := &compiled.nodeIndexes[nodeIndex]
				open := compiled.tokens[node.Token]
				node.EndToken = i
				compiled.elementEnd[node.Token] = i
				if open.Str != token.Str {
					compiled.recovery = append(compiled.recovery, compiledHTMLRecoveryIssue{Kind: "misnested", Tag: open.Str, Token: node.Token})
					continue
				}
				matched = true
				break
			}
			if !matched {
				compiled.recovery = append(compiled.recovery, compiledHTMLRecoveryIssue{Kind: "unexpected-close", Tag: token.Str, Token: i})
			}
		}
	}
	for _, nodeIndex := range stack {
		node := &compiled.nodeIndexes[nodeIndex]
		node.EndToken = len(compiled.tokens) - 1
		compiled.elementEnd[node.Token] = node.EndToken
		compiled.recovery = append(compiled.recovery, compiledHTMLRecoveryIssue{Kind: "unclosed", Tag: compiled.tokens[node.Token].Str, Token: node.Token})
	}
}

func (compiled *compiledHTML) compileElementDeclarations() {
	ancestors := make([]htmlSegmentType, 0, compiled.maxDepth)
	ancestorMeta := make([]htmlElementMetadata, 0, compiled.maxDepth)
	for i, token := range compiled.tokens {
		switch token.Cat {
		case 'O':
			compiled.elementDecl[i] = htmlElementDeclarationsWithStyleMeta(token, compiled.elementMeta[i], compiled.cssRules, compiled.inlineStyleDeclarations(token.Attr), ancestors, ancestorMeta)
			if htmlClosePops(token.Str) {
				ancestors = append(ancestors, token)
				ancestorMeta = append(ancestorMeta, compiled.elementMeta[i])
			}
		case 'C':
			if !htmlClosePops(token.Str) {
				continue
			}
			for len(ancestors) > 0 {
				last := len(ancestors) - 1
				open := ancestors[last]
				ancestors = ancestors[:last]
				ancestorMeta = ancestorMeta[:last]
				if open.Str == token.Str {
					break
				}
			}
			continue
		}
	}
}

func (compiled *compiledHTML) inlineStyleDeclarations(attrs map[string]string) map[string]string {
	if attrs == nil {
		return nil
	}
	style := attrs["style"]
	if strings.TrimSpace(style) == "" || !strings.Contains(style, ":") {
		return nil
	}
	if compiled.styleDeclarations == nil {
		return parseStyleDeclarations(style)
	}
	if declarations, ok := compiled.styleDeclarations[style]; ok {
		return declarations
	}
	declarations := parseStyleDeclarations(style)
	compiled.styleDeclarations[style] = declarations
	return declarations
}

func (compiled *compiledHTML) compileElementText(start int, tag string) {
	if !htmlCompiledTextTag(tag) {
		return
	}
	tokens, _ := compiled.collectElementTokens(start, tag)
	if len(tokens) < 2 {
		return
	}
	inner := tokens[1 : len(tokens)-1]
	if htmlCompiledTextHasNestedBlock(inner) {
		return
	}
	compiled.elementText[start] = compiledHTMLText{
		plain:     htmlPlainTextWithMode(inner, false),
		preserved: htmlPlainTextWithMode(inner, true),
		ok:        true,
	}
}

func htmlCompiledTextHasNestedBlock(tokens []htmlSegmentType) bool {
	for _, token := range tokens {
		if token.Cat == 'O' && htmlCompiledNestedTextTag(token.Str) {
			return true
		}
	}
	return false
}

func htmlCompiledNestedTextTag(tag string) bool {
	switch tag {
	case "p", "div", "section", "article", "header", "footer", "figure", "figcaption",
		"li", "caption", "h1", "h2", "h3", "h4", "h5", "h6", "dt", "dd":
		return true
	default:
		return false
	}
}

func htmlCompiledTextTag(tag string) bool {
	switch tag {
	case "p", "figcaption", "li", "caption", "h1", "h2", "h3", "h4", "h5", "h6", "dt", "dd":
		return true
	default:
		return false
	}
}

func (compiled *compiledHTML) compileStyleDeclarations(attrs map[string]string) {
	if attrs == nil {
		return
	}
	style := attrs["style"]
	if strings.TrimSpace(style) == "" || !strings.Contains(style, ":") {
		return
	}
	if _, ok := compiled.styleDeclarations[style]; ok {
		return
	}
	compiled.styleDeclarations[style] = parseStyleDeclarations(style)
}

func (compiled *compiledHTML) compileInlineSVGsContext(ctx context.Context) error {
	var cache map[string]*SVG
	for i := 0; i < len(compiled.tokens); i++ {
		if i%128 == 0 {
			if err := outputCanceledError(ctx); err != nil {
				return err
			}
		}
		token := compiled.tokens[i]
		if token.Cat != 'O' || token.Str != "svg" {
			if token.Cat == 'O' && (token.Str == "style" || token.Str == "script" || token.Str == "head") {
				i = compiled.skipElement(i, token.Str)
			}
			continue
		}
		svgTokens, end := compiled.collectElementTokens(i, "svg")
		if len(svgTokens) == 0 {
			continue
		}
		svgText := htmlSerializeTokens(svgTokens)
		svg, ok := cache[svgText]
		if !ok {
			parsed, err := SVGParseContext(ctx, []byte(svgText))
			if err != nil {
				return err
			}
			svg = &parsed
			if cache == nil {
				cache = make(map[string]*SVG)
			}
			cache[svgText] = svg
		}
		compiled.inlineSVGs[i] = compiledInlineSVG{svg: svg, end: end}
		i = end
	}
	return nil
}

func (compiled *compiledHTML) compileDataImagesContext(ctx context.Context, maxBytes int) error {
	if compiled == nil {
		return nil
	}
	var cache map[string]compiledHTMLDataImage
	for i, token := range compiled.tokens {
		if i%128 == 0 {
			if err := outputCanceledError(ctx); err != nil {
				return err
			}
		}
		if token.Cat != 'O' || token.Str != "img" {
			continue
		}
		src := strings.TrimSpace(token.Attr["src"])
		if cache != nil {
			if source, ok := cache[src]; ok {
				compiled.dataImages[i] = source
				continue
			}
		}
		source, ok, err := compileHTMLDataImageSourceContext(ctx, src, maxBytes)
		if err != nil {
			return err
		}
		if ok {
			if cache == nil {
				cache = make(map[string]compiledHTMLDataImage)
			}
			cache[src] = source
			compiled.dataImages[i] = source
		}
	}
	return nil
}

func compileHTMLDataImageSource(src string, maxBytes int) (compiledHTMLDataImage, bool, error) {
	return compileHTMLDataImageSourceContext(context.Background(), src, maxBytes)
}

func compileHTMLDataImageSourceContext(ctx context.Context, src string, maxBytes int) (compiledHTMLDataImage, bool, error) {
	if err := outputCanceledError(ctx); err != nil {
		return compiledHTMLDataImage{}, false, err
	}
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(strings.ToLower(src), "data:") {
		return compiledHTMLDataImage{}, false, nil
	}
	media, data, ok := strings.Cut(src[5:], ",")
	if !ok {
		return compiledHTMLDataImage{}, false, errors.New("invalid HTML image data URI")
	}
	parts := strings.Split(media, ";")
	mimeType := strings.ToLower(strings.TrimSpace(parts[0]))
	base64Encoded := false
	for _, part := range parts[1:] {
		if strings.EqualFold(strings.TrimSpace(part), "base64") {
			base64Encoded = true
		}
	}
	if !base64Encoded {
		return compiledHTMLDataImage{}, false, errors.New("HTML image data URI must be base64 encoded")
	}
	imageType := htmlImageTypeFromMime(mimeType)
	if imageType == "" {
		return compiledHTMLDataImage{}, false, fmt.Errorf("unsupported HTML image type: %s", mimeType)
	}
	if maxBytes <= 0 {
		maxBytes = htmlDefaultMaxDataImageBytes
	}
	if base64.StdEncoding.DecodedLen(len(data)) > maxBytes {
		return compiledHTMLDataImage{}, false, errors.New("HTML image data URI exceeds maximum size")
	}
	if err := outputCanceledError(ctx); err != nil {
		return compiledHTMLDataImage{}, false, err
	}
	buf, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return compiledHTMLDataImage{}, false, fmt.Errorf("invalid HTML image data URI: %w", err)
	}
	if err := outputCanceledError(ctx); err != nil {
		return compiledHTMLDataImage{}, false, err
	}
	if len(buf) > maxBytes {
		return compiledHTMLDataImage{}, false, errors.New("HTML image data URI exceeds maximum size")
	}
	name := compiledHTMLDataImageName(buf)
	return compiledHTMLDataImage{
		name:    name,
		options: ImageOptions{ImageType: imageType, ReadDpi: true},
		data:    buf,
	}, true, nil
}

func compiledHTMLDataImageName(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("html-data-image-%x", sum)
}

func (compiled *compiledHTML) declarations(start int) (map[string]string, bool) {
	if compiled == nil || start < 0 || start >= len(compiled.elementDecl) {
		return nil, false
	}
	decl := compiled.elementDecl[start]
	return decl, decl != nil
}

// Tokens returns a copy of the token stream used by the compiled HTML fragment.
func (compiled *compiledHTML) Tokens() []htmlSegmentType {
	if compiled == nil {
		return nil
	}
	return cloneHTMLTokens(compiled.tokens)
}

// Stats returns diagnostics for the reusable parse products stored in the
// compiled HTML fragment.
func (compiled *compiledHTML) Stats() compiledHTMLStats {
	if compiled == nil {
		return compiledHTMLStats{}
	}
	stats := compiledHTMLStats{
		Tokens:       len(compiled.tokens),
		Nodes:        len(compiled.nodeIndexes),
		Tables:       len(compiled.tables),
		Images:       len(compiled.dataImages),
		InlineSVGs:   len(compiled.inlineSVGs),
		CSSRules:     len(compiled.cssRules),
		Recovery:     len(compiled.recovery),
		MaxDepth:     compiled.maxDepth,
		CachedStyles: len(compiled.styleDeclarations),
	}
	for _, text := range compiled.elementText {
		if text.ok {
			stats.CachedText++
		}
	}
	return stats
}

// RecoveryIssues returns malformed-fragment recovery decisions from
// compilation. The returned slice is a copy.
func (compiled *compiledHTML) RecoveryIssues() []compiledHTMLRecoveryIssue {
	if compiled == nil || len(compiled.recovery) == 0 {
		return nil
	}
	out := make([]compiledHTMLRecoveryIssue, len(compiled.recovery))
	copy(out, compiled.recovery)
	return out
}

// DebugDump returns a compact tree dump for diagnostics.
func (compiled *compiledHTML) DebugDump() string {
	if compiled == nil {
		return ""
	}
	var out strings.Builder
	for i, node := range compiled.nodeIndexes {
		if node.Parent < 0 {
			compiled.debugDumpNode(&out, i, 0)
		}
	}
	return out.String()
}

func (compiled *compiledHTML) debugDumpNode(out *strings.Builder, nodeIndex, depth int) {
	if nodeIndex < 0 || nodeIndex >= len(compiled.nodeIndexes) {
		return
	}
	node := compiled.nodeIndexes[nodeIndex]
	token := compiled.tokens[node.Token]
	out.WriteString(strings.Repeat("  ", depth))
	out.WriteString(token.Str)
	out.WriteString(" token=")
	_, _ = fmt.Fprint(out, node.Token)
	out.WriteString(" end=")
	_, _ = fmt.Fprint(out, node.EndToken)
	out.WriteByte('\n')
	for child := node.FirstChild; child >= 0; child = compiled.nodeIndexes[child].NextSibling {
		compiled.debugDumpNode(out, child, depth+1)
	}
}

func (compiled *compiledHTML) validate() error {
	if compiled == nil {
		return errors.New("compiled HTML is nil")
	}
	return nil
}

func (compiled *compiledHTML) collectElementTokens(start int, tag string) ([]htmlSegmentType, int) {
	if compiled == nil || start < 0 || start >= len(compiled.tokens) {
		return nil, 0
	}
	end := compiled.elementEnd[start]
	if start < len(compiled.tokenNode) {
		if nodeIndex := compiled.tokenNode[start]; nodeIndex >= 0 && nodeIndex < len(compiled.nodeIndexes) {
			end = compiled.nodeIndexes[nodeIndex].EndToken
		}
	}
	if end < start || end >= len(compiled.tokens) {
		return htmlCollectElementTokens(compiled.tokens, start, tag)
	}
	return compiled.tokens[start : end+1], end
}

func (compiled *compiledHTML) skipElement(start int, tag string) int {
	_, end := compiled.collectElementTokens(start, tag)
	return end
}

func (compiled *compiledHTML) dataImage(start int) (compiledHTMLDataImage, bool) {
	if compiled == nil {
		return compiledHTMLDataImage{}, false
	}
	img, ok := compiled.dataImages[start]
	return img, ok
}

func (compiled *compiledHTML) validateDataImageLimit(maxBytes int) error {
	if compiled == nil {
		return nil
	}
	if maxBytes <= 0 {
		maxBytes = htmlDefaultMaxDataImageBytes
	}
	for _, img := range compiled.dataImages {
		if len(img.data) > maxBytes {
			return errors.New("HTML image data URI exceeds maximum size")
		}
	}
	return nil
}

func validateHTMLImageSource(src string) error {
	src = strings.TrimSpace(src)
	if strings.HasPrefix(strings.ToLower(src), "data:") {
		return nil
	}
	if u, err := url.Parse(src); err == nil && u.Scheme != "" {
		switch strings.ToLower(u.Scheme) {
		case "http", "https":
			return errors.New("remote HTML images are disabled")
		case "file":
			return errors.New("file URL HTML images are disabled")
		}
	}
	return nil
}

func cloneHTMLTokens(tokens []htmlSegmentType) []htmlSegmentType {
	if len(tokens) == 0 {
		return nil
	}
	out := make([]htmlSegmentType, len(tokens))
	for i, token := range tokens {
		out[i] = token
		if len(token.Attr) == 0 {
			continue
		}
		out[i].Attr = make(map[string]string, len(token.Attr))
		for key, value := range token.Attr {
			out[i].Attr[key] = value
		}
	}
	return out
}
