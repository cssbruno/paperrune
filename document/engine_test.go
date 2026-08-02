// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0

package document

// TestPDFDocument exposes the private PDF engine only to this package's
// external black-box tests. It is absent from normal builds.
type TestPDFDocument = pdfDocument

func MustNewTestPDFDocument(options ...Option) *TestPDFDocument {
	return mustNewPDFDocument(options...)
}

func NewTestPDFDocumentWithDefaults(defaults Defaults, options ...Option) (*TestPDFDocument, error) {
	return newPDFDocumentWithDefaults(defaults, options...)
}
