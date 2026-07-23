// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"strconv"

	"github.com/cssbruno/paperrune/internal/layoutengine"
)

const maxContentScratchCapacity = 64 * 1024

func (f *pdfDocument) contentCommandBuffer(capacity int) []byte {
	if capacity <= cap(f.contentScratch) {
		return f.contentScratch[:0]
	}
	return make([]byte, 0, capacity)
}

func (f *pdfDocument) retainContentCommandBuffer(buffer []byte) {
	if buffer != nil && cap(buffer) <= maxContentScratchCapacity {
		f.contentScratch = buffer[:0]
	}
}

// pageContentCommandBuffer returns appendable capacity owned by the current
// page. bytes.Buffer explicitly permits appending to AvailableBuffer followed
// by an immediate Write of that slice; this lets positioned painters build a
// command directly in its final allocation.
func (f *pdfDocument) pageContentCommandBuffer(capacity int) []byte {
	if f.state != documentStatePageOpen || f.page <= 0 || f.page >= len(f.pages) || f.pages[f.page] == nil {
		return f.contentCommandBuffer(capacity)
	}
	f.pages[f.page].Grow(capacity + 1) // include the newline appended by outbytes
	return f.pages[f.page].AvailableBuffer()
}

func appendPDFNumber(dst []byte, value float64, precision int) []byte {
	return strconv.AppendFloat(dst, value, 'f', precision, 64)
}

func appendPDFNumberSpace(dst []byte, value float64, precision int) []byte {
	dst = appendPDFNumber(dst, value, precision)
	return append(dst, ' ')
}

// appendPDFTJAdjustment emits the ten-decimal text-space correction used by a
// PDF TJ array directly from fixed-point geometry. The common path avoids the
// floating-point formatter for every glyph; extreme values fall back to the
// established formatter rather than risking integer overflow.
func appendPDFTJAdjustment(dst []byte, fontWidth int, fontSize, advance layoutengine.Fixed) []byte {
	fallback := func() []byte {
		native := float64(fontWidth) * fontSize.Points() / 1000
		adjustment := (native - advance.Points()) * 1000 / fontSize.Points()
		return appendPDFNumber(dst, adjustment, 10)
	}
	if fontWidth < 0 || fontSize <= 0 {
		return fallback()
	}
	const (
		maxInt64 = int64(^uint64(0) >> 1)
		scale    = int64(10_000_000_000)
	)
	width := int64(fontWidth)
	size := int64(fontSize)
	metric := int64(advance)
	if width != 0 && size > maxInt64/width || metric > maxInt64/1000 || metric < -maxInt64/1000 {
		return fallback()
	}
	numerator := width*size - metric*1000
	if numerator == -maxInt64-1 {
		return fallback()
	}
	negative := numerator < 0
	if negative {
		numerator = -numerator
	}
	if numerator > maxInt64/scale {
		return fallback()
	}
	product := numerator * scale
	scaled, remainder := product/size, product%size
	opposite := size - remainder
	distanceFromHalf := remainder - opposite
	if distanceFromHalf < 0 {
		distanceFromHalf = -distanceFromHalf
	}
	// The historical contract is the decimal form of a sequence of binary64
	// operations, not the mathematically exact rational. Very near a decimal
	// rounding midpoint, those operations can land on the opposite side; retain
	// the established formatter for that small cohort.
	if distanceFromHalf <= size/500+1 {
		return fallback()
	}
	// Match strconv's round-to-nearest, ties-to-even behavior.
	if remainder > opposite || remainder == opposite && scaled&1 != 0 {
		scaled++
	}
	if negative {
		dst = append(dst, '-')
	}
	dst = strconv.AppendInt(dst, scaled/scale, 10)
	dst = append(dst, '.')
	fraction := scaled % scale
	for divisor := int64(1_000_000_000); divisor != 0; divisor /= 10 {
		dst = append(dst, byte('0'+fraction/divisor)) // #nosec G115 -- fraction/divisor is a decimal digit in [0, 9].
		fraction %= divisor
	}
	return dst
}

// appendPDFFixed emits the exact ten-decimal representation previously
// produced by strconv.AppendFloat(value.Points(), 'f', 10, 64). FixedScale is
// 1024, whose reciprocal terminates after ten decimal places, so the decimal
// can be assembled without floating-point conversion or temporary storage.
func appendPDFFixed(dst []byte, value layoutengine.Fixed) []byte {
	const exactFloatIntegerLimit = layoutengine.Fixed(1 << 53)
	if value < -exactFloatIntegerLimit || value > exactFloatIntegerLimit {
		return strconv.AppendFloat(dst, value.Points(), 'f', 10, 64)
	}
	negative := value < 0
	var magnitude uint64
	if negative {
		magnitude = uint64(-(value + 1)) + 1 // #nosec G115 -- the adjusted negative magnitude is non-negative and bounded above.
		dst = append(dst, '-')
	} else {
		magnitude = uint64(value) // #nosec G115 -- this branch proves value is non-negative.
	}
	dst = strconv.AppendUint(dst, magnitude/uint64(layoutengine.FixedScale), 10)
	dst = append(dst, '.')
	return append(dst, pdfFixedFractionDigits[magnitude%uint64(layoutengine.FixedScale)][:]...)
}

func appendPDFFixedSpace(dst []byte, value layoutengine.Fixed) []byte {
	dst = appendPDFFixed(dst, value)
	return append(dst, ' ')
}

var pdfFixedFractionDigits = func() [layoutengine.FixedScale][10]byte {
	var fractions [layoutengine.FixedScale][10]byte
	for fixed := range fractions {
		fraction := uint64(fixed) * 9_765_625
		for digit, divisor := 0, uint64(1_000_000_000); divisor > 0; digit, divisor = digit+1, divisor/10 {
			fractions[fixed][digit] = byte('0' + fraction/divisor)
			fraction %= divisor
		}
	}
	return fractions
}()

var pdfColorComponents = func() [256]string {
	var components [256]string
	for value := range components {
		components[value] = strconv.FormatFloat(float64(value)/255, 'f', 10, 64)
	}
	return components
}()

func appendPDFColorComponentSpace(dst []byte, value uint8) []byte {
	dst = append(dst, pdfColorComponents[value]...)
	return append(dst, ' ')
}

func plannedGlyphRunCapacity(run layoutengine.CoreGlyphRun) int {
	const base = 96
	const perCode = 48
	if len(run.Codes) > (maxContentScratchCapacity-base)/perCode {
		return maxContentScratchCapacity
	}
	return base + len(run.Codes)*perCode
}

func appendPDFInt(dst []byte, value int) []byte {
	return strconv.AppendInt(dst, int64(value), 10)
}

func appendPDFUint(dst []byte, value uint32) []byte {
	return strconv.AppendUint(dst, uint64(value), 10)
}

func appendPDFObjectRef(dst []byte, objectID int) []byte {
	dst = appendPDFInt(dst, objectID)
	dst = append(dst, " 0 R"...)
	return dst
}

func appendPDFRectPaint(dst []byte, x, y, w, h float64, op string, trailingSpace bool) []byte {
	dst = appendPDFNumberSpace(dst, x, 2)
	dst = appendPDFNumberSpace(dst, y, 2)
	dst = appendPDFNumberSpace(dst, w, 2)
	dst = appendPDFNumberSpace(dst, h, 2)
	dst = append(dst, "re "...)
	dst = append(dst, op...)
	if trailingSpace {
		dst = append(dst, ' ')
	}
	return dst
}

func appendPDFLine(dst []byte, x1, y1, x2, y2 float64, precision int, trailingSpace bool) []byte {
	dst = appendPDFNumberSpace(dst, x1, precision)
	dst = appendPDFNumberSpace(dst, y1, precision)
	dst = append(dst, "m "...)
	dst = appendPDFNumberSpace(dst, x2, precision)
	dst = appendPDFNumberSpace(dst, y2, precision)
	dst = append(dst, "l S"...)
	if trailingSpace {
		dst = append(dst, ' ')
	}
	return dst
}

func appendPDFCubicCurve(dst []byte, cx0, cy0, cx1, cy1, x, y float64) []byte {
	dst = appendPDFNumberSpace(dst, cx0, 5)
	dst = appendPDFNumberSpace(dst, cy0, 5)
	dst = appendPDFNumberSpace(dst, cx1, 5)
	dst = appendPDFNumberSpace(dst, cy1, 5)
	dst = appendPDFNumberSpace(dst, x, 5)
	dst = appendPDFNumberSpace(dst, y, 5)
	return append(dst, 'c')
}

func appendPDFFontSelect(dst []byte, fontID string, size float64) []byte {
	dst = append(dst, "BT /F"...)
	dst = append(dst, fontID...)
	dst = append(dst, ' ')
	dst = appendPDFNumber(dst, size, 2)
	return append(dst, " Tf ET"...)
}

func (f *pdfDocument) outPDFFontSelect() {
	buf := make([]byte, 0, len(f.currentFont.i)+20)
	buf = appendPDFFontSelect(buf, f.currentFont.i, f.fontSizePt)
	f.outbytes(buf)
}

func (f *pdfDocument) outPDFLineWidth(width float64) {
	var scratch [32]byte
	buf := appendPDFNumber(scratch[:0], width, 2)
	buf = append(buf, " w"...)
	f.outbytes(buf)
}

func (f *pdfDocument) outPDFIntOperator(value int, operator byte) {
	var scratch [24]byte
	buf := appendPDFInt(scratch[:0], value)
	buf = append(buf, ' ', operator)
	f.outbytes(buf)
}

func (f *pdfDocument) outPDFObjHeader(n int) {
	var scratch [32]byte
	buf := appendPDFInt(scratch[:0], n)
	buf = append(buf, " 0 obj"...)
	f.outbytes(buf)
}

func (f *pdfDocument) outPDFXrefRange(count int) {
	var scratch [32]byte
	buf := append(scratch[:0], '0', ' ')
	buf = appendPDFInt(buf, count)
	f.outbytes(buf)
}

func (f *pdfDocument) outPDFXrefOffset(offset int) {
	var scratch [32]byte
	buf := appendPDFPaddedInt(scratch[:0], offset, 10)
	buf = append(buf, " 00000 n "...)
	f.outbytes(buf)
}

func (f *pdfDocument) outPDFIntLine(value int) {
	var scratch [24]byte
	f.outbytes(appendPDFInt(scratch[:0], value))
}

func appendPDFPaddedInt(dst []byte, value, width int) []byte {
	var scratch [32]byte
	raw := appendPDFInt(scratch[:0], value)
	for i := len(raw); i < width; i++ {
		dst = append(dst, '0')
	}
	return append(dst, raw...)
}
