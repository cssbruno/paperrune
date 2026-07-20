// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0

package document

// TestPDFDocument exposes the private PDF engine only to this package's
// external black-box tests. It is absent from normal builds.
type TestPDFDocument = pdfDocument

type HTMLTemplateValuesForTest = htmlTemplateValues
type HTMLTemplateRawForTest = htmlTemplateRaw
type HTMLTemplateImageForTest = htmlTemplateImage

func CompileHTMLForTest(source string) (*compiledHTML, error) { return compileHTML(source) }

func CompileHTMLTemplateForTest(source string) (*compiledHTMLTemplate, error) {
	return compileHTMLTemplate(source)
}

func RenderHTMLTemplateForTest(source string, values HTMLTemplateValuesForTest) (string, error) {
	return renderHTMLTemplate(source, values)
}

func (f *pdfDocument) HTMLNewForTest() htmlRenderer { return f.htmlNew() }

func NewTestPDFDocument(options ...Option) (*TestPDFDocument, error) {
	return newPDFDocument(options...)
}

func MustNewTestPDFDocument(options ...Option) *TestPDFDocument {
	return mustNewPDFDocument(options...)
}

func NewTestPDFDocumentWithDefaults(defaults Defaults, options ...Option) (*TestPDFDocument, error) {
	return newPDFDocumentWithDefaults(defaults, options...)
}
