// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import "sync"

type documentPlanningCacheState struct {
	planningCaches *documentPlanningCaches
}

type documentPlanningCaches struct {
	measurementScratch     sync.Pool // reusable concurrent-safe planner metric state
	typedCoreFontResources sync.Map  // immutable exact core metrics by backing table
}

func (f *pdfDocument) planningCache() *documentPlanningCaches {
	if f.planningCaches == nil {
		f.planningCaches = new(documentPlanningCaches)
	}
	return f.planningCaches
}

// acquireTypedMeasurementScratch returns planner-only font metric state scoped to the
// owning document. Table planning can measure hundreds of cells, and creating
// a complete PDF document (resource stores, page maps, caches, and policy
// state) for every cell was both unnecessary and allocation-heavy.
func (f *pdfDocument) acquireTypedMeasurementScratch() *pdfDocument {
	cache := f.planningCache()
	scratch, _ := cache.measurementScratch.Get().(*pdfDocument)
	if scratch == nil {
		scratch = documentNew("P", f.unitStr, "", f.fontDirStr, Size{Wd: f.w, Ht: f.h})
	}
	scratch.cMargin = f.cMargin
	scratch.ws = f.ws
	scratch.fontFamily = f.fontFamily
	scratch.fontStyle = f.fontStyle
	scratch.fontSizePt = f.fontSizePt
	scratch.fontSize = f.fontSizePt / scratch.k
	scratch.underline = false
	scratch.strikeout = false
	return scratch
}

func (f *pdfDocument) releaseTypedMeasurementScratch(scratch *pdfDocument) {
	if scratch != nil && scratch.err == nil {
		f.planningCache().measurementScratch.Put(scratch)
	}
}
