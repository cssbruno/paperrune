// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0

package document

import (
	"reflect"
	"sort"
	"testing"
)

func TestPublicDocumentSurfaceAllowsOnlyPaperConfigurationAndOutput(t *testing.T) {
	want := []string{
		"ClearError", "Close", "EnableTaggedPDF", "Err", "Error", "Ok",
		"Output", "OutputContext", "OutputFile", "OutputFileAndClose", "OutputFileContext", "OutputFileStream",
		"OutputFileStreamContext", "OutputFileStreamWithOptions", "OutputFileStreamWithOptionsContext",
		"OutputFileWithOptions", "OutputFileWithOptionsContext", "OutputSigned", "OutputSignedContext",
		"OutputSignedFile", "OutputSignedFileContext", "OutputStream", "OutputStreamContext",
		"OutputStreamWithOptions", "OutputStreamWithOptionsContext", "OutputWithOptions", "OutputWithOptionsContext",
		"PageCount", "SetAttachments", "SetAuthor", "SetComplianceMetadata", "SetLimits", "SetOutputIntent",
		"SetProductionPolicy", "SetSecurityPolicy", "SetSubject", "SetTitle",
		"WritePaper", "WritePaperJSON", "WritePaperJSONWithAssetsAndImports", "WritePaperJSONWithOptions",
		"WritePaperPlan", "WritePaperScenario", "WritePaperScenarioWithAssets", "WritePaperScenarioWithAssetsAndImports",
		"WritePaperScenarioWithImports", "WritePaperWithAssets", "WritePaperWithAssetsAndImports", "WritePaperWithImports",
	}
	typeOf := reflect.TypeOf((*Document)(nil))
	got := make([]string, typeOf.NumMethod())
	for index := range got {
		got[index] = typeOf.Method(index).Name
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("public Document method surface drifted:\n got %v\nwant %v", got, want)
	}
}
