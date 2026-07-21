// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

func (f *pdfDocument) newobj() {
	objectNumber := f.allocateObject(f.finalOutputOffset())
	if f.hooks.OnOutputObject != nil {
		f.hooks.OnOutputObject(objectNumber, "object")
	}
	f.outPDFObjHeader(objectNumber)
}

func (f *pdfDocument) beginPDFObject(objNum int) {
	f.recordObject(objNum, f.finalOutputOffset())
	f.outPDFObjHeader(objNum)
}

func (f *pdfDocument) newPDFDictObject() {
	f.newobj()
	f.beginPDFDict()
}

func (f *pdfDocument) beginPDFDict() {
	f.out("<<")
}

func (f *pdfDocument) endPDFDict() {
	f.out(">>")
}

func (f *pdfDocument) endPDFObject() {
	f.out("endobj")
}

func (f *pdfDocument) beginPDFStream() {
	f.out("stream")
}

func (f *pdfDocument) endPDFStream() {
	f.out("endstream")
}

func (f *pdfDocument) putstream(b []byte) {
	f.beginPDFStream()
	f.outbytes(b)
	f.endPDFStream()
}
