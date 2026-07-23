export const playgroundSamples = [
  {
    name: 'Academic article',
    source: `document @academic-article:
  language: "en"
  title: "Academic article"

  schema paper:
    string title
    string authors
    string affiliation
    string abstract
    string doi
    number participants
    number improvement
    number confidence
    bool peerReviewed
    string measureLabel
    string endpointLabel
    string browserFlowLabel
    string plannedLabel
    string changeLabel
    string confidenceIntervalLabel
    string subgroupLabel
    string sampleSizeLabel
    string flowVarianceLabel

  style @body:
    font: "Times"
    size: 8.8pt
    line-height: 12.8pt
    color: "#202A33"

  style @sans:
    font: "Helvetica"
    size: 8pt
    line-height: 11pt
    color: "#202A33"

  style @label:
    style: "@sans"
    size: 6.8pt
    line-height: 9pt
    bold: true
    color: "#315D7A"

  style @title:
    font: "Times"
    size: 25pt
    line-height: 28pt
    bold: true
    color: "#17232D"

  style @author:
    font: "Times"
    size: 9.5pt
    line-height: 13pt
    bold: true
    color: "#253541"

  style @affiliation:
    font: "Times"
    size: 8.3pt
    line-height: 11.5pt
    italic: true
    color: "#64727C"

  style @section:
    style: "@sans"
    size: 7.4pt
    line-height: 10pt
    bold: true
    color: "#315D7A"

  style @stat:
    style: "@sans"
    size: 19pt
    line-height: 21pt
    bold: true
    color: "#17232D"

  style @caption:
    font: "Times"
    size: 7.4pt
    line-height: 10.5pt
    color: "#5F6C75"

  style @page-title:
    font: "Times"
    size: 20pt
    line-height: 24pt
    bold: true
    color: "#17232D"

  style @subhead:
    style: "@sans"
    size: 9pt
    line-height: 12pt
    bold: true
    color: "#17232D"

  page @article:
    size: "A4"
    margin: 44pt
    page-numbers: true
    page-number-format: "AURORA METHODS / %d of {pages}"
    page-total-alias: "{pages}"
    page-number-align: "outer"

    body @article-body:
      table @journal-head-page-1:
        split: "avoid"
        table-column @journal-left-column-1:
          width: 68%
        table-column @journal-right-column-1:
          width: 32%
        table-row @journal-row-1:
          cell @journal-left-1:
            style: "@label"
            padding-bottom: 7pt
            border-bottom-width: 1pt
            border-color: "#8EA4B2"
            text: "AURORA JOURNAL OF COMPUTATIONAL METHODS"
          cell @journal-right-1:
            style: "@label"
            padding-bottom: 7pt
            border-bottom-width: 1pt
            border-color: "#8EA4B2"
            align: "right"
            text: peerReviewed ? "RESEARCH ARTICLE / PEER REVIEWED" : "RESEARCH ARTICLE / PREPRINT"

      paragraph @eyebrow:
        style: "@label"
        margin-top: 18pt
        text: "VOLUME 12 / ISSUE 4 / JULY 2026"

      heading @paper-title:
        level: 1
        style: "@title"
        bind: "title"
        margin-top: 7pt

      paragraph @authors:
        style: "@author"
        bind: "authors"
        margin-top: 10pt

      paragraph @affiliation-line:
        style: "@affiliation"
        bind: "affiliation"
        margin-top: 2pt

      paragraph @identity-gap:
        size: 1pt
        line-height: 14pt
        color: "#FFFFFF"
        text: "."

      table @identity:
        split: "avoid"
        table-column:
          width: 17%
        table-column:
          width: 58%
        table-column:
          width: 25%
        table-row:
          cell:
            style: "@label"
            padding: 9pt
            background: "#315D7A"
            color: "#FFFFFF"
            text: "ABSTRACT"
          cell:
            style: "@body"
            bind: "abstract"
            padding: 9pt
            background: "#EEF2F3"
          cell:
            style: "@caption"
            bind: "doi"
            padding: 9pt
            background: "#E2EAED"

      paragraph @keywords:
        style: "@caption"
        margin-top: 6pt
        italic: true
        text: "Keywords: deterministic layout; reproducible research; WebAssembly; pagination; scientific publishing"

      paragraph @evidence-gap:
        size: 1pt
        line-height: 17pt
        color: "#FFFFFF"
        text: "."

      table @evidence:
        split: "avoid"
        table-column:
          width: 33.333%
        table-column:
          width: 33.333%
        table-column:
          width: 33.334%
        table-row:
          cell:
            padding: 10pt
            border-top-width: 2pt
            border-bottom-width: 1pt
            border-color: "#315D7A"
            paragraph @participants-value:
              style: "@stat"
              bind: "participants"
              format: "decimal"
              format-locale: "en-US"
              format-min-fraction: 0
              format-max-fraction: 0
            paragraph @participants-label:
              style: "@label"
              text: "DOCUMENTS / TEST CORPUS"
          cell:
            padding: 10pt
            border-top-width: 2pt
            border-bottom-width: 1pt
            border-color: "#315D7A"
            paragraph @improvement-value:
              style: "@stat"
              bind: "improvement"
              format: "decimal"
              format-locale: "en-US"
              format-min-fraction: 1
              format-max-fraction: 1
            paragraph @improvement-label:
              style: "@label"
              text: "PERCENT FEWER LAYOUT SHIFTS"
          cell:
            padding: 10pt
            border-top-width: 2pt
            border-bottom-width: 1pt
            border-color: "#315D7A"
            paragraph @confidence-value:
              style: "@stat"
              bind: "confidence"
              format: "decimal"
              format-locale: "en-US"
              format-min-fraction: 0
              format-max-fraction: 0
            paragraph @confidence-label:
              style: "@label"
              text: "PERCENT CONFIDENCE"

      paragraph @reading-grid-gap:
        size: 1pt
        line-height: 17pt
        color: "#FFFFFF"
        text: "."

      table @reading-grid:
        split: "avoid"
        table-column:
          width: 50%
        table-column:
          width: 50%
        table-row:
          cell:
            padding-right: 14pt
            border-right-width: 1pt
            border-color: "#C9D2D7"
            paragraph @introduction-heading:
              style: "@section"
              text: "01 / INTRODUCTION"
            paragraph @introduction-copy:
              style: "@body"
              text: "Scientific documents are often treated as fluid browser pages until the moment of export. That late transition introduces hidden measurements, unstable line breaks, and review artifacts that are difficult to reproduce across systems."
            paragraph @method-heading:
              style: "@section"
              text: "02 / METHOD"
            paragraph @method-copy:
              style: "@body"
              text: "We compiled the same typed source under a pinned 144 dpi raster profile. Physical units, font metrics, fractional columns, and pagination constraints were resolved into an immutable layout plan before paint. Output identity was verified by plan hash and pixel comparison."
            paragraph @method-note:
              style: "@caption"
              italic: true
              text: "Protocol. Five independent runs per document; cold runtime initialization excluded; geometry compared at exact device-pixel boundaries."
          cell:
            padding-left: 14pt
            paragraph @results-heading:
              style: "@section"
              text: "03 / RESULTS"
            paragraph @results-copy:
              style: "@body"
              text: "Constraint-first planning eliminated cross-run geometry variance in the evaluated corpus. The largest improvement appeared in mixed table and prose layouts, where browser-dependent sizing had previously shifted both rules and baselines."
            paragraph @finding:
              style: "@body"
              bold: true
              color: "#315D7A"
              text: "Primary finding: identical inputs produced identical page geometry and raster dimensions in every measured run."
            paragraph @discussion-heading:
              style: "@section"
              text: "04 / DISCUSSION"
            paragraph @discussion-copy:
              style: "@body"
              text: "The approach turns layout into inspectable evidence rather than an incidental side effect. It is especially useful for manuscripts, regulated reports, and archival documents whose visual form must remain stable during review."
            paragraph @references-heading:
              style: "@section"
              text: "REFERENCES"
            paragraph @references:
              style: "@caption"
              text: "1. Knuth DE, Plass MF. Breaking paragraphs into lines. 2. W3C. CSS Paged Media Module. 3. ISO 32000-2. Document management - PDF 2.0."

      paragraph @benchmark-heading:
        style: "@section"
        margin-top: 13pt
        margin-bottom: 5pt
        text: "TABLE 1 / REPRODUCIBILITY OUTCOMES"

      table @benchmark:
        split: "avoid"
        table-column:
          width: 36%
        table-column:
          width: 22%
        table-column:
          width: 22%
        table-column:
          width: 20%
        table-header:
          table-row:
            cell:
              style: "@label"
              bind: "measureLabel"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
            cell:
              style: "@label"
              bind: "browserFlowLabel"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              align: "right"
            cell:
              style: "@label"
              bind: "plannedLabel"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              align: "right"
            cell:
              style: "@label"
              bind: "changeLabel"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              align: "right"
        table-row:
          cell:
            style: "@caption"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#C9D2D7"
            text: "Cross-run geometry variance"
          cell:
            style: "@caption"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#C9D2D7"
            align: "right"
            text: "observed"
          cell:
            style: "@caption"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#C9D2D7"
            align: "right"
            bold: true
            text: "none"
          cell:
            style: "@caption"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#C9D2D7"
            align: "right"
            color: "#315D7A"
            bold: true
            text: "stabilized"
        table-row:
          cell:
            style: "@caption"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#C9D2D7"
            text: "Layout shifts per 1,000 pages"
          cell:
            style: "@caption"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#C9D2D7"
            align: "right"
            text: "18.4"
          cell:
            style: "@caption"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#C9D2D7"
            align: "right"
            bold: true
            text: "0.0"
          cell:
            style: "@caption"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#C9D2D7"
            align: "right"
            color: "#315D7A"
            bold: true
            text: "-100%"

      paragraph @benchmark-note:
        style: "@caption"
        margin-top: 4pt
        italic: true
        text: "Values report the evaluation corpus at the pinned raster profile; lower is better."

      paragraph @conclusion-gap:
        size: 1pt
        line-height: 12pt
        color: "#FFFFFF"
        text: "."

      table @conclusion:
        split: "avoid"
        table-column:
          width: 17%
        table-column:
          width: 83%
        table-row:
          cell:
            style: "@label"
            padding: 9pt
            background: "#17232D"
            color: "#FFFFFF"
            text: "CONCLUSION"
          cell:
            style: "@body"
            padding: 9pt
            background: "#E8EEF0"
            bold: true
            text: "A deterministic plan makes scientific layout reviewable, repeatable, and independent of browser heuristics."

      page-break @methods-page:

      table @journal-head-page-2:
        split: "avoid"
        table-column @journal-left-column-2:
          width: 68%
        table-column @journal-right-column-2:
          width: 32%
        table-row @journal-row-2:
          cell @journal-left-2:
            style: "@label"
            padding-bottom: 7pt
            border-bottom-width: 1pt
            border-color: "#8EA4B2"
            text: "AURORA JOURNAL OF COMPUTATIONAL METHODS"
          cell @journal-right-2:
            style: "@label"
            padding-bottom: 7pt
            border-bottom-width: 1pt
            border-color: "#8EA4B2"
            align: "right"
            text: peerReviewed ? "RESEARCH ARTICLE / PEER REVIEWED" : "RESEARCH ARTICLE / PREPRINT"

      paragraph @methods-eyebrow:
        style: "@label"
        margin-top: 18pt
        text: "RESEARCH DESIGN / METHODS"

      heading @methods-title:
        level: 1
        style: "@page-title"
        margin-top: 5pt
        text: "Materials and methods"

      paragraph @methods-lead:
        style: "@body"
        margin-top: 7pt
        margin-bottom: 13pt
        size: 10pt
        line-height: 14pt
        color: "#4C5B65"
        text: "A preregistered benchmark compared browser-dependent document flow with a constraint-first layout planner under identical source data, typography, and raster conditions."

      table @study-profile:
        split: "avoid"
        table-column:
          width: 24%
        table-column:
          width: 26%
        table-column:
          width: 24%
        table-column:
          width: 26%
        table-header:
          table-row:
            cell:
              style: "@label"
              padding: 7pt
              background: "#17232D"
              color: "#FFFFFF"
              text: "DESIGN"
            cell:
              style: "@label"
              padding: 7pt
              background: "#17232D"
              color: "#FFFFFF"
              text: "PRIMARY ENDPOINT"
            cell:
              style: "@label"
              padding: 7pt
              background: "#17232D"
              color: "#FFFFFF"
              text: "RASTER PROFILE"
            cell:
              style: "@label"
              padding: 7pt
              background: "#17232D"
              color: "#FFFFFF"
              text: "ANALYSIS"
        table-row:
          cell:
            style: "@body"
            padding: 8pt
            background: "#EEF2F3"
            text: "Paired repeated measures"
          cell:
            style: "@body"
            padding: 8pt
            background: "#EEF2F3"
            text: "Cross-run geometry variance"
          cell:
            style: "@body"
            padding: 8pt
            background: "#EEF2F3"
            text: "A4 / 144 dpi / sRGB"
          cell:
            style: "@body"
            padding: 8pt
            background: "#EEF2F3"
            text: "Two-sided, alpha = 0.05"

      paragraph @method-grid-gap:
        size: 1pt
        line-height: 16pt
        color: "#FFFFFF"
        text: "."

      table @method-grid:
        split: "avoid"
        table-column:
          width: 50%
        table-column:
          width: 50%
        table-row:
          cell:
            padding-right: 14pt
            border-right-width: 1pt
            border-color: "#C9D2D7"
            paragraph @corpus-heading:
              style: "@section"
              text: "2.1 / CORPUS CONSTRUCTION"
            paragraph @corpus-copy:
              style: "@body"
              text: "The corpus contained 1,248 synthetic but structurally representative manuscripts. Documents were stratified by page count, table density, heading depth, and the presence of repeated headers. Source fixtures were frozen before analysis and assigned stable content digests."
            paragraph @randomization-heading:
              style: "@section"
              text: "2.2 / RANDOMIZATION"
            paragraph @randomization-copy:
              style: "@body"
              text: "Render order was randomized within each of five runs. A deterministic seed generated the schedule, and the operator was blinded to pipeline identity until all raster evidence had been captured."
            paragraph @exclusion-heading:
              style: "@section"
              text: "2.3 / EXCLUSION CRITERIA"
            paragraph @exclusion-copy:
              style: "@body"
              text: "Documents were excluded only when the authored source was invalid or required an unsupported glyph repertoire. No completed render was removed on the basis of visual outcome or execution time."
          cell:
            padding-left: 14pt
            paragraph @planner-heading:
              style: "@section"
              text: "2.4 / LAYOUT PLANNER"
            paragraph @planner-copy:
              style: "@body"
              text: "The experimental pipeline resolved page geometry, font advances, line breaks, table tracks, and pagination into an immutable plan. Painting consumed only retained commands; it did not query browser layout or perform late measurement."
            paragraph @outcome-heading:
              style: "@section"
              text: "2.5 / OUTCOME MEASURES"
            paragraph @outcome-copy:
              style: "@body"
              text: "The primary outcome was the proportion of documents with any cross-run pixel-boundary displacement. Secondary outcomes included line-break divergence, table-rule displacement, plan identity, compilation latency, and peak memory."
            paragraph @masking-heading:
              style: "@section"
              text: "2.6 / QUALITY CONTROL"
            paragraph @masking-copy:
              style: "@body"
              text: "Every artifact was validated for dimensions, page count, renderer contract, and PNG signature. Ten percent of difference maps were independently inspected against the retained display plan."

      paragraph @corpus-table-heading:
        style: "@section"
        margin-top: 16pt
        margin-bottom: 5pt
        text: "TABLE 2 / BENCHMARK CORPUS"

      table @corpus-table:
        split: "avoid"
        table-column:
          width: 38%
        table-column:
          width: 18%
        table-column:
          width: 22%
        table-column:
          width: 22%
        table-header:
          table-row:
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              text: "DOCUMENT CLASS"
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              align: "right"
              text: "N"
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              align: "right"
              text: "MEDIAN PAGES"
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              align: "right"
              text: "TABLE DENSITY"
        table-row:
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Research articles"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "416"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "8"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "moderate"
        table-row:
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Clinical reports"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "312"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "5"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "high"
        table-row:
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Technical appendices"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "280"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "12"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "low"
        table-row:
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Regulatory summaries"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "240"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "7"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "moderate"

      paragraph @analysis-note:
        style: "@body"
        margin-top: 14pt
        padding: 10pt
        background: "#E8EEF0"
        border-width: 1pt
        border-color: "#315D7A"
        border-radius: 4pt
        text: "Statistical analysis. Paired bootstrap confidence intervals used 10,000 resamples at the document level. Family-wise error was controlled with the Holm procedure for secondary endpoints."

      paragraph @environment-heading:
        style: "@section"
        margin-top: 16pt
        margin-bottom: 5pt
        text: "REPRODUCIBILITY ENVIRONMENT"

      table @environment:
        split: "avoid"
        table-column:
          width: 34%
        table-column:
          width: 33%
        table-column:
          width: 33%
        table-header:
          table-row:
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              text: "EXECUTION"
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              text: "RENDER CONTRACT"
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              text: "IDENTITY EVIDENCE"
        table-row:
          cell:
            style: "@body"
            padding: 7pt
            background: "#EEF2F3"
            text: "Go WebAssembly / isolated worker"
          cell:
            style: "@body"
            padding: 7pt
            background: "#EEF2F3"
            text: "Pinned profile / 144 dpi"
          cell:
            style: "@body"
            padding: 7pt
            background: "#EEF2F3"
            text: "Plan SHA-256 / PNG SHA-256"

      paragraph @ethics-note:
        style: "@caption"
        margin-top: 7pt
        italic: true
        text: "Ethics. The study used generated documents and no human participants, personal data, or confidential manuscripts; institutional review was therefore not required."

      page-break @results-page:

      table @journal-head-page-3:
        split: "avoid"
        table-column @journal-left-column-3:
          width: 68%
        table-column @journal-right-column-3:
          width: 32%
        table-row @journal-row-3:
          cell @journal-left-3:
            style: "@label"
            padding-bottom: 7pt
            border-bottom-width: 1pt
            border-color: "#8EA4B2"
            text: "AURORA JOURNAL OF COMPUTATIONAL METHODS"
          cell @journal-right-3:
            style: "@label"
            padding-bottom: 7pt
            border-bottom-width: 1pt
            border-color: "#8EA4B2"
            align: "right"
            text: peerReviewed ? "RESEARCH ARTICLE / PEER REVIEWED" : "RESEARCH ARTICLE / PREPRINT"

      paragraph @results-eyebrow:
        style: "@label"
        margin-top: 18pt
        text: "EVIDENCE / RESULTS"

      heading @results-page-title:
        level: 1
        style: "@page-title"
        margin-top: 5pt
        text: "Results and model diagnostics"

      paragraph @results-page-lead:
        style: "@body"
        margin-top: 7pt
        margin-bottom: 13pt
        size: 10pt
        line-height: 14pt
        color: "#4C5B65"
        text: "All 1,248 eligible documents completed five planned runs in both conditions, yielding 12,480 page-set observations and no missing primary-outcome data."

      table @result-summary:
        split: "avoid"
        table-column:
          width: 35%
        table-column:
          width: 21%
        table-column:
          width: 21%
        table-column:
          width: 23%
        table-header:
          table-row:
            cell:
              style: "@label"
              bind: "endpointLabel"
              padding: 7pt
              background: "#17232D"
              color: "#FFFFFF"
            cell:
              style: "@label"
              bind: "browserFlowLabel"
              padding: 7pt
              background: "#17232D"
              color: "#FFFFFF"
              align: "right"
            cell:
              style: "@label"
              bind: "plannedLabel"
              padding: 7pt
              background: "#17232D"
              color: "#FFFFFF"
              align: "right"
            cell:
              style: "@label"
              bind: "confidenceIntervalLabel"
              padding: 7pt
              background: "#17232D"
              color: "#FFFFFF"
              align: "right"
        table-row:
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Any geometry variance"
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "18.4%"
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            bold: true
            color: "#315D7A"
            text: "0.0%"
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "-20.6 to -16.2"
        table-row:
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Line-break divergence"
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "11.7%"
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            bold: true
            color: "#315D7A"
            text: "0.0%"
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "-13.5 to -9.9"
        table-row:
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Median compile time"
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "42.8 ms"
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            bold: true
            color: "#315D7A"
            text: "39.1 ms"
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "-4.4 to -3.1"
        table-row:
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Peak memory"
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "21.4 MiB"
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            bold: true
            color: "#315D7A"
            text: "22.1 MiB"
          cell:
            style: "@body"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "+0.4 to +1.0"

      paragraph @diagnostic-heading:
        style: "@section"
        margin-top: 17pt
        margin-bottom: 5pt
        text: "FIGURE 1 / ERROR CONCENTRATION BY DOCUMENT COMPLEXITY"

      table @diagnostic-matrix:
        split: "avoid"
        table-column:
          width: 28%
        table-column:
          width: 18%
        table-column:
          width: 18%
        table-column:
          width: 18%
        table-column:
          width: 18%
        table-header:
          table-row:
            cell:
              style: "@label"
              padding: 6pt
              background: "#E8EEF0"
              text: "COMPLEXITY QUARTILE"
            cell:
              style: "@label"
              padding: 6pt
              background: "#E8EEF0"
              align: "center"
              text: "Q1"
            cell:
              style: "@label"
              padding: 6pt
              background: "#D6E2E7"
              align: "center"
              text: "Q2"
            cell:
              style: "@label"
              padding: 6pt
              background: "#AFC5D1"
              align: "center"
              text: "Q3"
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              align: "center"
              text: "Q4"
        table-row:
          cell:
            style: "@body"
            padding: 7pt
            text: "Browser-flow variance"
          cell:
            style: "@body"
            padding: 7pt
            background: "#EEF2F3"
            align: "center"
            text: "4.8%"
          cell:
            style: "@body"
            padding: 7pt
            background: "#DCE7EB"
            align: "center"
            text: "10.6%"
          cell:
            style: "@body"
            padding: 7pt
            background: "#B9CFD9"
            align: "center"
            text: "19.1%"
          cell:
            style: "@body"
            padding: 7pt
            background: "#527B94"
            color: "#FFFFFF"
            align: "center"
            bold: true
            text: "39.0%"
        table-row:
          cell:
            style: "@body"
            padding: 7pt
            text: "Planned-layout variance"
          cell:
            style: "@body"
            padding: 7pt
            background: "#EEF2F3"
            align: "center"
            text: "0.0%"
          cell:
            style: "@body"
            padding: 7pt
            background: "#EEF2F3"
            align: "center"
            text: "0.0%"
          cell:
            style: "@body"
            padding: 7pt
            background: "#EEF2F3"
            align: "center"
            text: "0.0%"
          cell:
            style: "@body"
            padding: 7pt
            background: "#EEF2F3"
            align: "center"
            bold: true
            color: "#315D7A"
            text: "0.0%"

      paragraph @diagnostic-caption:
        style: "@caption"
        margin-top: 5pt
        italic: true
        text: "Darker cells indicate a larger proportion of documents with at least one displaced pixel boundary. Complexity combines page count, table count, and heading depth."

      paragraph @ablation-heading:
        style: "@section"
        margin-top: 16pt
        margin-bottom: 5pt
        text: "TABLE 3 / ABLATION ANALYSIS"

      table @ablation:
        split: "avoid"
        table-column:
          width: 48%
        table-column:
          width: 17%
        table-column:
          width: 17%
        table-column:
          width: 18%
        table-header:
          table-row:
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              text: "PLANNER CONDITION"
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              align: "right"
              text: "VARIANCE"
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              align: "right"
              text: "LATENCY"
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              align: "right"
              text: "PLAN MATCH"
        table-row:
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Full constraint-first planner"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "0.0%"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "39.1 ms"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            bold: true
            color: "#315D7A"
            text: "100%"
        table-row:
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Without pinned font metrics"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "9.6%"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "37.8 ms"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "90.4%"
        table-row:
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Without fixed track allocation"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "14.2%"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "36.9 ms"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "85.8%"

      paragraph @result-interpretation:
        style: "@body"
        margin-top: 14pt
        padding: 10pt
        background: "#E8EEF0"
        border-width: 1pt
        border-color: "#315D7A"
        border-radius: 4pt
        bold: true
        text: "Interpretation. Font metrics and fractional track allocation were independently necessary; removing either restored measurable cross-run instability."

      paragraph @subgroup-heading:
        style: "@section"
        margin-top: 16pt
        margin-bottom: 5pt
        text: "TABLE 4 / PRESPECIFIED SUBGROUP ANALYSIS"

      table @subgroup:
        split: "avoid"
        table-column:
          width: 40%
        table-column:
          width: 20%
        table-column:
          width: 20%
        table-column:
          width: 20%
        table-header:
          table-row:
            cell:
              style: "@label"
              bind: "subgroupLabel"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
            cell:
              style: "@label"
              bind: "sampleSizeLabel"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              align: "right"
            cell:
              style: "@label"
              bind: "flowVarianceLabel"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              align: "right"
            cell:
              style: "@label"
              bind: "plannedLabel"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              align: "right"
        table-row:
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Documents with repeated headers"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "438"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "24.7%"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            bold: true
            color: "#315D7A"
            text: "0.0%"
        table-row:
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Documents with dense tables"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "351"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "28.5%"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            bold: true
            color: "#315D7A"
            text: "0.0%"
        table-row:
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Documents longer than 10 pages"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "294"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "31.6%"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            bold: true
            color: "#315D7A"
            text: "0.0%"

      paragraph @sensitivity-note:
        style: "@caption"
        margin-top: 6pt
        italic: true
        text: "Sensitivity analyses at 96 and 192 dpi produced the same direction and magnitude of the primary effect. No interaction crossed the prespecified significance threshold."

      page-break @discussion-page:

      table @journal-head-page-4:
        split: "avoid"
        table-column @journal-left-column-4:
          width: 68%
        table-column @journal-right-column-4:
          width: 32%
        table-row @journal-row-4:
          cell @journal-left-4:
            style: "@label"
            padding-bottom: 7pt
            border-bottom-width: 1pt
            border-color: "#8EA4B2"
            text: "AURORA JOURNAL OF COMPUTATIONAL METHODS"
          cell @journal-right-4:
            style: "@label"
            padding-bottom: 7pt
            border-bottom-width: 1pt
            border-color: "#8EA4B2"
            align: "right"
            text: peerReviewed ? "RESEARCH ARTICLE / PEER REVIEWED" : "RESEARCH ARTICLE / PREPRINT"

      paragraph @discussion-eyebrow:
        style: "@label"
        margin-top: 18pt
        text: "INTERPRETATION / OPEN SCIENCE"

      heading @discussion-page-title:
        level: 1
        style: "@page-title"
        margin-top: 5pt
        text: "Discussion and reproducibility"

      paragraph @discussion-page-lead:
        style: "@body"
        margin-top: 7pt
        margin-bottom: 13pt
        size: 10pt
        line-height: 14pt
        color: "#4C5B65"
        text: "The findings support layout planning as a scientific-control mechanism: it reduces environmental variance without trading away authorial structure or operational performance."

      table @discussion-grid:
        split: "avoid"
        table-column:
          width: 50%
        table-column:
          width: 50%
        table-row:
          cell:
            padding-right: 14pt
            border-right-width: 1pt
            border-color: "#C9D2D7"
            paragraph @discussion-main-heading:
              style: "@section"
              text: "4.1 / PRINCIPAL FINDINGS"
            paragraph @discussion-main-copy:
              style: "@body"
              text: "Constraint-first pagination removed detectable cross-run geometry variance across every corpus stratum. The effect was not explained by reduced content complexity: the planned condition preserved the same prose, tables, rules, and physical page dimensions."
            paragraph @mechanism-heading:
              style: "@section"
              text: "4.2 / MECHANISM"
            paragraph @mechanism-copy:
              style: "@body"
              text: "The dominant mechanism was elimination of late measurement. Once font advances and track widths were fixed, downstream line breaking and pagination became pure consequences of the authored plan rather than properties of the host browser."
            paragraph @generalization-heading:
              style: "@section"
              text: "4.3 / GENERALIZABILITY"
            paragraph @generalization-copy:
              style: "@body"
              text: "The corpus emphasized Latin-script scientific and regulated documents. The same architecture should generalize to other domains, but broader writing systems require explicit shaping and font-repertoire evaluation."
          cell:
            padding-left: 14pt
            paragraph @limitations-heading:
              style: "@section"
              text: "4.4 / LIMITATIONS"
            paragraph @limitations-copy:
              style: "@body"
              text: "The benchmark used deterministic synthetic fixtures rather than confidential production manuscripts. Raster equivalence was assessed at one pinned profile, and the study did not compare assistive-technology output or every PDF consumer."
            paragraph @implications-heading:
              style: "@section"
              text: "4.5 / PRACTICAL IMPLICATIONS"
            paragraph @implications-copy:
              style: "@body"
              text: "Teams can treat plan hashes, page counts, and display rasters as review evidence. This enables meaningful visual regression checks for manuscripts, protocols, submissions, and archival publications."
            paragraph @future-heading:
              style: "@section"
              text: "4.6 / FUTURE WORK"
            paragraph @future-copy:
              style: "@body"
              text: "Future studies should evaluate complex-script shaping, mathematical notation, tagged-PDF semantics, and cross-engine agreement under embedded-font workflows."

      paragraph @open-science-heading:
        style: "@section"
        margin-top: 17pt
        margin-bottom: 5pt
        text: "OPEN SCIENCE STATEMENT"

      table @open-science:
        split: "avoid"
        table-column:
          width: 28%
        table-column:
          width: 54%
        table-column:
          width: 18%
        table-header:
          table-row:
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              text: "ARTIFACT"
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              text: "AVAILABILITY"
            cell:
              style: "@label"
              padding: 6pt
              background: "#315D7A"
              color: "#FFFFFF"
              align: "right"
              text: "STATUS"
        table-row:
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Preregistration"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Protocol and analysis plan deposited before rendering"
          cell:
            style: "@label"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "AVAILABLE"
        table-row:
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Source corpus"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Synthetic fixtures, schemas, and stable content digests"
          cell:
            style: "@label"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "AVAILABLE"
        table-row:
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Analysis code"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Compiler, renderer profile, and verification scripts"
          cell:
            style: "@label"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "AVAILABLE"
        table-row:
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Raster evidence"
          cell:
            style: "@body"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            text: "Page PNGs, hashes, dimensions, and difference maps"
          cell:
            style: "@label"
            padding: 6pt
            border-bottom-width: 1pt
            border-color: "#D5DDE1"
            align: "right"
            text: "AVAILABLE"

      paragraph @contributions-heading:
        style: "@section"
        margin-top: 16pt
        text: "AUTHOR CONTRIBUTIONS"

      paragraph @contributions-copy:
        style: "@body"
        margin-top: 4pt
        text: "M.C.: conceptualization, methodology, writing. E.R.: software, validation, visualization. N.W.: formal analysis, supervision, review and editing. All authors approved the final manuscript."

      paragraph @references-page-heading:
        style: "@section"
        margin-top: 15pt
        text: "REFERENCES"

      table @references-grid:
        split: "avoid"
        table-column:
          width: 50%
        table-column:
          width: 50%
        table-row:
          cell:
            padding-right: 14pt
            paragraph @references-left:
              style: "@caption"
              text: "1. Knuth DE, Plass MF. Breaking paragraphs into lines. Software Pract Exper. 1981;11:1119-1184. 2. W3C. CSS Paged Media Module Level 3. Working Draft. 3. ISO. ISO 32000-2:2020 Document management - PDF 2.0. 4. Munafo MR et al. A manifesto for reproducible science. Nat Hum Behav. 2017;1:0021."
          cell:
            padding-left: 14pt
            paragraph @references-right:
              style: "@caption"
              text: "5. Peng RD. Reproducible research in computational science. Science. 2011;334:1226-1227. 6. Sandve GK et al. Ten simple rules for reproducible computational research. PLoS Comput Biol. 2013;9:e1003285. 7. Wilkinson MD et al. The FAIR guiding principles. Sci Data. 2016;3:160018. 8. Boettiger C. An introduction to Docker for reproducible research. ACM SIGOPS Oper Syst Rev. 2015;49:71-79."

      paragraph @integrity-note:
        style: "@body"
        margin-top: 16pt
        padding: 10pt
        background: "#17232D"
        border-radius: 4pt
        color: "#FFFFFF"
        text: "Competing interests: none declared. Funding: Northbridge Open Methods Initiative. Data and code: doi:10.5281/aurora.2026.0417. Correspondence: m.chen@northbridge.example"
`,
    data: `{
  "title": "Constraint-Based Pagination for Reproducible Scientific Publishing",
  "authors": "Maya Chen, PhD; Elias Romero, MSc; Noor Williams, PhD",
  "affiliation": "Center for Computational Publishing, Northbridge Institute / Correspondence: m.chen@northbridge.example",
  "abstract": "We evaluate a layout pipeline that resolves physical units, type metrics, and pagination before rasterization. Across a heterogeneous document corpus, deterministic planning removed cross-run geometry variance while preserving authored hierarchy.",
  "doi": "doi:10.5281/aurora.2026.0417",
  "participants": 1248,
  "improvement": 18.4,
  "confidence": 95,
  "peerReviewed": true,
  "measureLabel": "MEASURE",
  "endpointLabel": "ENDPOINT",
  "browserFlowLabel": "BROWSER FLOW",
  "plannedLabel": "PLANNED",
  "changeLabel": "CHANGE",
  "confidenceIntervalLabel": "95% CI",
  "subgroupLabel": "SUBGROUP",
  "sampleSizeLabel": "N",
  "flowVarianceLabel": "FLOW VARIANCE"
}`,
  },
  {
    name: 'Editorial welcome',
    source: `document @welcome:
  language: "en"
  title: "Editorial welcome"

  schema input:
    string name
    string deck
    bool premium
    optional string note
    string principleOne
    string principleTwo
    string principleThree
    string closing

  style @base:
    font: "Helvetica"
    size: 10pt
    line-height: 15pt
    color: "#25313A"

  style @label:
    style: "@base"
    size: 8pt
    line-height: 10pt
    bold: true
    color: "#D94F2B"

  style @display:
    style: "@base"
    size: 39pt
    line-height: 42pt
    bold: true
    color: "#111A21"

  style @small:
    style: "@base"
    size: 8pt
    line-height: 12pt
    color: "#687078"

  page @sheet:
    size: "A4"
    margin: 54pt
    page-numbers: true
    page-number-format: "FIELD NOTE / %d of {pages}"
    page-total-alias: "{pages}"
    page-number-align: "outer"
    body @content:
      paragraph @masthead:
        style: "@label"
        padding-bottom: 11pt
        border-bottom-width: 2pt
        border-color: "#111A21"
        color: "#111A21"
        text: "PAPERRUNE / FIELD NOTE 01                         2026"

      paragraph @edition:
        style: "@label"
        margin-top: 38pt
        text: premium ? "THE PREMIUM DOCUMENT WORKSPACE" : "THE DOCUMENT WORKSPACE"

      heading @title:
        level: 1
        style: "@display"
        bind: "name"
        margin-top: 8pt
        padding: 22pt
        background: "#111A21"
        color: "#FFFFFF"
        border-left-width: 7pt
        border-color: "#D94F2B"

      paragraph @deck:
        style: "@base"
        bind: "deck"
        size: 14pt
        line-height: 21pt
        margin-top: 16pt
        margin-bottom: 26pt
        color: "#56616A"

      paragraph @manifesto:
        style: "@base"
        bind: "note"
        bind-required: false
        padding: 15pt
        padding-left: 18pt
        background: "#F2EEE6"
        border-width: 1pt
        border-color: "#D94F2B"
        border-radius: 5pt
        size: 13pt
        line-height: 20pt
        bold: true

      paragraph @principles-label:
        style: "@label"
        margin-top: 36pt
        margin-bottom: 8pt
        text: "THREE RULES FOR A SERIOUS DOCUMENT"

      table @principles:
        split: "avoid"
        table-column:
          width: 23%
        table-column:
          width: 77%
        table-row:
          cell:
            style: "@label"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "01 / STRUCTURE"
          cell:
            style: "@base"
            bind: "principleOne"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
        table-row:
          cell:
            style: "@label"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "02 / RHYTHM"
          cell:
            style: "@base"
            bind: "principleTwo"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
        table-row:
          cell:
            style: "@label"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "03 / PROOF"
          cell:
            style: "@base"
            bind: "principleThree"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"

      paragraph @closing-label:
        style: "@label"
        margin-top: 34pt
        text: "DESIGNED IN PAPER / RENDERED IN GO WASM"

      heading @closing:
        level: 2
        bind: "closing"
        margin-top: 7pt
        size: 23pt
        line-height: 28pt
        color: "#111A21"

      paragraph @footer:
        style: "@small"
        margin-top: 42pt
        padding-top: 10pt
        border-top-width: 1pt
        border-color: "#CDC7BD"
        text: "Edit the Paper source or JSON. The WebAssembly renderer rebuilds this page automatically."
`,
    data: `{
  "name": "Make the page impossible to ignore.",
  "deck": "A field note on building publication systems with a clear voice, measurable rhythm, and no accidental layout decisions.",
  "premium": true,
  "note": "A document should feel authored, not assembled. Give it one voice, one hierarchy, and no accidental decoration.",
  "principleOne": "Lead with meaning. Let spacing and scale establish the reading order before decoration begins.",
  "principleTwo": "Alternate dense evidence with quiet space so every section has a distinct reading tempo.",
  "principleThree": "Compile the same inputs into the same pixels, then preserve that visual contract in review.",
  "closing": "The browser is the press. The plan is the contract."
}`,
  },
  {
    name: 'Layout specimen',
    source: `document @layout-specimen:
  language: "en"
  title: "Layout specimen"

  schema metrics:
    string title
    string intro
    number columns
    bool compact

  style @base:
    font: "Helvetica"
    size: 10pt
    line-height: 15pt
    color: "#25313A"

  style @label:
    style: "@base"
    size: 8pt
    line-height: 10pt
    bold: true
    color: "#D94F2B"

  style @display:
    style: "@base"
    size: 34pt
    line-height: 38pt
    bold: true
    color: "#111A21"

  style @mono:
    font: "Courier"
    size: 9pt
    line-height: 13pt
    color: "#111A21"

  page @sheet:
    size: "A4"
    margin: 54pt
    page-numbers: true
    page-number-format: "GEOMETRY / %d of {pages}"
    page-total-alias: "{pages}"
    page-number-align: "outer"
    body @content:
      paragraph @masthead:
        style: "@label"
        padding: 14pt
        background: "#D94F2B"
        color: "#FFFFFF"
        text: "02 / GEOMETRY                         PAPERRUNE"

      paragraph @section:
        style: "@label"
        margin-top: 36pt
        text: "LAYOUT AS AN EXPLICIT SYSTEM"

      heading @title:
        level: 1
        style: "@display"
        bind: "title"
        margin-top: 8pt

      paragraph @intro:
        style: "@base"
        bind: "intro"
        margin-top: 10pt
        margin-bottom: 32pt
        size: 13pt
        line-height: 19pt
        color: "#59636B"

      row @ratio:
        gap: compact ? 8pt : 16pt
        table @primary:
          width: columns == 2 ? 2fr : 3fr
          split: "avoid"
          table-column:
            width: 100%
          table-row:
            cell:
              style: "@display"
              padding: 18pt
              background: "#111A21"
              color: "#FFFFFF"
              text: columns == 2 ? "2fr / PRIMARY" : "3fr / PRIMARY"
        table @support:
          width: 1fr
          split: "avoid"
          table-column:
            width: 100%
          table-row:
            cell:
              style: "@display"
              padding: 18pt
              background: "#F2EEE6"
              color: "#D94F2B"
              text: "1fr"

      paragraph @formula-label:
        style: "@label"
        margin-top: 24pt
        margin-bottom: 8pt
        text: "THE DECLARED RELATIONSHIP"

      paragraph @formula:
        style: "@mono"
        padding: 17pt
        margin-bottom: 30pt
        background: "#111A21"
        border-radius: 4pt
        color: "#F7F2E9"
        text: compact ? (columns == 2 ? "available width - 8pt gap = 2fr primary + 1fr support" : "available width - 8pt gap = 3fr primary + 1fr support") : (columns == 2 ? "available width - 16pt gap = 2fr primary + 1fr support" : "available width - 16pt gap = 3fr primary + 1fr support")

      table @measurements:
        split: "avoid"
        table-column:
          width: 31%
        table-column:
          width: 24%
        table-column:
          width: 45%
        table-header:
          table-row:
            cell:
              header-cell: true
              style: "@label"
              padding: 9pt
              background: "#D94F2B"
              color: "#FFFFFF"
              text: "TOKEN"
            cell:
              header-cell: true
              style: "@label"
              padding: 9pt
              background: "#D94F2B"
              color: "#FFFFFF"
              text: "VALUE"
            cell:
              header-cell: true
              style: "@label"
              padding: 9pt
              background: "#D94F2B"
              color: "#FFFFFF"
              text: "PURPOSE"
        table-row:
          cell:
            style: "@mono"
            padding: 10pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "page.margin"
          cell:
            style: "@base"
            padding: 10pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "54pt"
          cell:
            style: "@base"
            padding: 10pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Stable outer rhythm"
        table-row:
          cell:
            style: "@mono"
            padding: 10pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "row.gap"
          cell:
            style: "@base"
            padding: 10pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: compact ? "8pt" : "16pt"
          cell:
            style: "@base"
            padding: 10pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Data-dependent spacing"
        table-row:
          cell:
            style: "@mono"
            padding: 10pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "column.width"
          cell:
            style: "@base"
            padding: 10pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: columns == 2 ? "2fr / 1fr" : "3fr / 1fr"
          cell:
            style: "@base"
            padding: 10pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Exact proportional allocation"
        table-row:
          cell:
            style: "@mono"
            padding: 10pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "renderer.dpi"
          cell:
            style: "@base"
            padding: 10pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "144"
          cell:
            style: "@base"
            padding: 10pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Pinned WASM raster profile"

      paragraph @footer:
        style: "@label"
        margin-top: 38pt
        padding-top: 10pt
        border-top-width: 1pt
        border-color: "#CDC7BD"
        text: "NO BROWSER LAYOUT DECISIONS / NO HIDDEN MEASUREMENTS"
`,
    data: `{
  "title": "A measured page.",
  "intro": "Fractions, physical units, and typed expressions resolve into an inspectable plan before a single pixel is painted.",
  "columns": 2,
  "compact": false
}`,
  },
  {
    name: 'Studio invoice',
    source: `document @invoice:
  title: "Studio invoice"
  language: "en"

  schema billing:
    string invoiceNumber
    string issued
    string due
    string seller
    string sellerContact
    string customer
    string customerAddress
    string status
    string subtotal
    string tax
    string total
    string paymentReference
    string remittance
    list object items:
      max-items: 100
      string description
      string quantity
      string rate
      string amount

  style @base:
    font: "Helvetica"
    size: 9.5pt
    line-height: 14pt
    color: "#25313A"

  style @label:
    style: "@base"
    size: 7pt
    line-height: 10pt
    bold: true
    color: "#D94F2B"

  style @display:
    style: "@base"
    size: 31pt
    line-height: 35pt
    bold: true
    color: "#111A21"

  style @total:
    style: "@base"
    size: 19pt
    line-height: 23pt
    bold: true
    color: "#FFFFFF"

  page @invoice-page:
    size: "A4"
    margin: 48pt
    page-numbers: true
    page-number-format: "Invoice / Page %d of {pages}"
    page-total-alias: "{pages}"

    body @invoice-content:
      paragraph @masthead:
        style: "@label"
        padding-bottom: 12pt
        margin-bottom: 30pt
        background: "#111A21"
        color: "#F7F2E9"
        border-bottom-width: 5pt
        border-color: "#D94F2B"
        text: "PAPERRUNE STUDIO                         BILLING / 2026"

      row @title-row:
        gap: 16pt
        align-items: "end"
        heading @invoice-heading:
          width: 2fr
          level: 1
          style: "@display"
          text: "INVOICE"
        paragraph @number:
          width: 1fr
          style: "@base"
          bind: "invoiceNumber"
          bold: true
          align: "right"

      paragraph @title-gap:
        size: 1pt
        line-height: 20pt
        color: "#FFFFFF"
        text: "."

      table @parties:
        split: "avoid"
        table-column:
          width: 50%
        table-column:
          width: 50%
        table-header:
          table-row:
            cell:
              header-cell: true
              style: "@label"
              padding: 8pt
              border-bottom-width: 2pt
              border-color: "#111A21"
              text: "FROM"
            cell:
              header-cell: true
              style: "@label"
              padding: 8pt
              border-bottom-width: 2pt
              border-color: "#111A21"
              text: "BILL TO"
        table-row:
          cell:
            padding: 10pt
            paragraph @seller:
              style: "@base"
              bind: "seller"
              bold: true
          cell:
            padding: 10pt
            paragraph @customer:
              style: "@base"
              bind: "customer"
              bold: true
        table-row:
          cell:
            padding: 10pt
            paragraph @seller-contact:
              style: "@base"
              bind: "sellerContact"
              color: "#697078"
          cell:
            padding: 10pt
            paragraph @customer-address:
              style: "@base"
              bind: "customerAddress"
              color: "#697078"

      paragraph @parties-gap:
        size: 1pt
        line-height: 14pt
        color: "#FFFFFF"
        text: "."

      table @dates:
        split: "avoid"
        table-column:
          width: 33.333%
        table-column:
          width: 33.333%
        table-column:
          width: 33.334%
        table-row:
          cell:
            style: "@label"
            padding: 9pt
            background: "#F2EEE6"
            text: "ISSUED"
          cell:
            style: "@label"
            padding: 9pt
            background: "#F2EEE6"
            text: "DUE"
          cell:
            style: "@label"
            padding: 9pt
            background: "#F2EEE6"
            text: "STATUS"
        table-row:
          cell:
            style: "@base"
            bind: "issued"
            padding: 9pt
            bold: true
          cell:
            style: "@base"
            bind: "due"
            padding: 9pt
            bold: true
          cell:
            style: "@base"
            bind: "status"
            padding: 9pt
            bold: true
            color: "#D94F2B"

      paragraph @dates-gap:
        size: 1pt
        line-height: 24pt
        color: "#FFFFFF"
        text: "."

      table @items-table:
        repeat-header: true
        split: "rows"
        table-column:
          width: 52%
        table-column:
          width: 12%
        table-column:
          width: 18%
        table-column:
          width: 18%
        table-header:
          table-row:
            cell:
              header-cell: true
              style: "@label"
              padding: 9pt
              background: "#111A21"
              color: "#FFFFFF"
              text: "DESCRIPTION"
            cell:
              header-cell: true
              style: "@label"
              padding: 9pt
              background: "#111A21"
              color: "#FFFFFF"
              text: "QTY"
            cell:
              header-cell: true
              style: "@label"
              padding: 9pt
              background: "#111A21"
              color: "#FFFFFF"
              text: "RATE"
            cell:
              header-cell: true
              style: "@label"
              padding: 9pt
              background: "#111A21"
              color: "#FFFFFF"
              align: "right"
              text: "AMOUNT"
        repeat @item-rows:
          source: "items"
          instance-prefix: "items"
          max-items: 100
          table-row:
            cell:
              padding: 10pt
              border-bottom-width: 1pt
              border-color: "#D5CFC5"
              paragraph:
                style: "@base"
                bind: "description"
            cell:
              padding: 10pt
              border-bottom-width: 1pt
              border-color: "#D5CFC5"
              paragraph:
                style: "@base"
                bind: "quantity"
            cell:
              padding: 10pt
              border-bottom-width: 1pt
              border-color: "#D5CFC5"
              paragraph:
                style: "@base"
                bind: "rate"
            cell:
              padding: 10pt
              border-bottom-width: 1pt
              border-color: "#D5CFC5"
              paragraph:
                style: "@base"
                bind: "amount"
                align: "right"
                bold: true

      paragraph @financials-gap:
        size: 1pt
        line-height: 14pt
        color: "#FFFFFF"
        text: "."

      table @financials:
        split: "avoid"
        table-column:
          width: 72%
        table-column:
          width: 28%
        table-row:
          cell:
            style: "@label"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5CFC5"
            align: "right"
            text: "SUBTOTAL"
          cell:
            style: "@base"
            bind: "subtotal"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5CFC5"
            align: "right"
            bold: true
        table-row:
          cell:
            style: "@label"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5CFC5"
            align: "right"
            text: "TAX"
          cell:
            style: "@base"
            bind: "tax"
            padding: 7pt
            border-bottom-width: 1pt
            border-color: "#D5CFC5"
            align: "right"
            bold: true
        table-row:
          cell:
            style: "@label"
            padding: 7pt
            align: "right"
            text: "PAYMENT REFERENCE"
          cell:
            style: "@base"
            bind: "paymentReference"
            padding: 7pt
            align: "right"
            bold: true

      table @summary:
        split: "avoid"
        table-column:
          width: 65%
        table-column:
          width: 35%
        table-row:
          cell:
            style: "@base"
            padding: 15pt
            background: "#F2EEE6"
            color: "#596169"
            text: "Payment due within 30 days. Reference the invoice number with your remittance."
          cell:
            style: "@total"
            bind: "total"
            padding: 15pt
            background: "#D94F2B"
            align: "right"

      paragraph @terms:
        style: "@base"
        bind: "remittance"
        margin-top: 28pt
        padding-top: 10pt
        border-top-width: 1pt
        border-color: "#CDC7BD"
        size: 8pt
        color: "#687078"

      paragraph @footer:
        style: "@label"
        margin-top: 34pt
        text: "PAPERRUNE / TYPED BILLING / DETERMINISTIC OUTPUT"
`,
    data: `{
  "invoiceNumber": "INV-2026-0042",
  "issued": "July 20, 2026",
  "due": "August 19, 2026",
  "seller": "PaperRune Studio",
  "sellerContact": "accounts@paperrune.studio / +1 555 0124",
  "customer": "Northwind Operations",
  "customerAddress": "22 Market Street, Seattle, WA 98101",
  "status": "DUE",
  "subtotal": "$1,850.00",
  "tax": "$0.00",
  "total": "$1,850.00",
  "paymentReference": "NW-8421-PR",
  "remittance": "Remit by bank transfer within 30 days and include NW-8421-PR. Questions: accounts@paperrune.studio",
  "items": [
    {"description":"Document platform license","quantity":"1","rate":"$800.00","amount":"$800.00"},
    {"description":"Editorial template system","quantity":"6","rate":"$95.00","amount":"$570.00"},
    {"description":"Automated output checks","quantity":"4","rate":"$75.00","amount":"$300.00"},
    {"description":"Launch support","quantity":"1","rate":"$180.00","amount":"$180.00"}
  ]
}`,
  },
  {
    name: 'Release brief',
    source: `document @release-brief:
  title: "Release brief"
  language: "en"

  schema release:
    string product
    string releaseId
    string owner
    string targetDate
    string environment
    string evidenceHash
    string deploymentWindow
    string approver
    number score
    bool blocked
    optional string blocker

  style @base:
    font: "Helvetica"
    size: 10pt
    line-height: 15pt
    color: "#25313A"

  style @label:
    style: "@base"
    size: 8pt
    line-height: 10pt
    bold: true
    color: "#D94F2B"

  style @display:
    style: "@base"
    size: 36pt
    line-height: 40pt
    bold: true
    color: "#111A21"

  style @score:
    style: "@base"
    size: 46pt
    line-height: 50pt
    bold: true
    color: "#D94F2B"

  component @ready:
    paragraph @ready-label:
      style: "@label"
      margin-top: 22pt
      padding: 13pt
      background: "#111A21"
      color: "#FFFFFF"
      text: "DECISION / READY TO SHIP"
    paragraph @ready-copy:
      style: "@base"
      padding: 15pt
      background: "#F2EEE6"
      text: "Every release gate is closed. Publication can proceed."

  component @blocked:
    paragraph @blocked-label:
      style: "@label"
      margin-top: 22pt
      padding: 13pt
      background: "#D94F2B"
      color: "#FFFFFF"
      text: "DECISION / HOLD RELEASE"
    paragraph @blocked-copy:
      style: "@base"
      padding: 15pt
      background: "#F2EEE6"
      text: "One gate remains open. Resolve the named blocker before publication."

  page @sheet:
    size: "A4"
    margin: 54pt
    page-numbers: true
    page-number-format: "RELEASE CONTROL / %d of {pages}"
    page-total-alias: "{pages}"
    page-number-align: "outer"
    body @content:
      paragraph @masthead:
        style: "@label"
        padding: 14pt
        background: "#111A21"
        color: "#F7F2E9"
        border-bottom-width: 5pt
        border-color: "#D94F2B"
        text: "04 / RELEASE CONTROL                         PAPERRUNE"

      paragraph @brief-label:
        style: "@label"
        margin-top: 36pt
        text: "SHIP / HOLD DECISION"

      heading @product:
        level: 1
        style: "@display"
        bind: "product"
        margin-top: 8pt

      paragraph @subtitle:
        style: "@base"
        margin-top: 10pt
        margin-bottom: 30pt
        size: 13pt
        line-height: 19pt
        color: "#59636B"
        text: "A typed decision record for the people responsible for publication."

      table @release-metadata:
        split: "avoid"
        table-column:
          width: 25%
        table-column:
          width: 25%
        table-column:
          width: 25%
        table-column:
          width: 25%
        table-header:
          table-row:
            cell:
              style: "@label"
              padding: 7pt
              background: "#111A21"
              color: "#FFFFFF"
              text: "RELEASE"
            cell:
              style: "@label"
              padding: 7pt
              background: "#111A21"
              color: "#FFFFFF"
              text: "OWNER"
            cell:
              style: "@label"
              padding: 7pt
              background: "#111A21"
              color: "#FFFFFF"
              text: "TARGET"
            cell:
              style: "@label"
              padding: 7pt
              background: "#111A21"
              color: "#FFFFFF"
              text: "ENVIRONMENT"
        table-row:
          cell:
            style: "@base"
            bind: "releaseId"
            padding: 8pt
            background: "#F2EEE6"
            bold: true
          cell:
            style: "@base"
            bind: "owner"
            padding: 8pt
            background: "#F2EEE6"
            bold: true
          cell:
            style: "@base"
            bind: "targetDate"
            padding: 8pt
            background: "#F2EEE6"
            bold: true
          cell:
            style: "@base"
            bind: "environment"
            padding: 8pt
            background: "#F2EEE6"
            bold: true

      paragraph @score-gap:
        size: 1pt
        line-height: 18pt
        color: "#FFFFFF"
        text: "."

      table @scoreboard:
        split: "avoid"
        table-column:
          width: 34%
        table-column:
          width: 66%
        table-row:
          cell:
            style: "@score"
            padding: 16pt
            background: "#D94F2B"
            color: "#FFFFFF"
            bind: "score"
            format: "decimal"
            format-locale: "en-US"
            format-min-fraction: 0
            format-max-fraction: 0
          cell:
            style: "@base"
            padding: 20pt
            background: "#F2EEE6"
            bold: true
            text: switch:
              case score >= 90: "EXCELLENT / all release evidence is complete"
              case score >= 70: "REVIEW / inspect the remaining open gate"
              default: "AT RISK / intervention is required"

      paragraph @score-label:
        style: "@label"
        margin-top: 8pt
        text: "READINESS SCORE / 100"

      use @decision:
        component: blocked ? @blocked : @ready

      paragraph @blocker:
        visible: blocker != null && blocked
        style: "@base"
        bind: "blocker"
        bind-required: false
        margin-top: 12pt
        padding: 14pt
        background: "#FFF4F0"
        border-width: 1pt
        border-color: "#D94F2B"
        border-radius: 4pt
        bold: true

      page-break @release-evidence-page:

      paragraph @evidence-masthead:
        style: "@label"
        padding: 14pt
        background: "#111A21"
        color: "#F7F2E9"
        border-bottom-width: 5pt
        border-color: "#D94F2B"
        text: "04 / RELEASE CONTROL                         EVIDENCE"

      paragraph @gates-label:
        style: "@label"
        margin-top: 30pt
        margin-bottom: 8pt
        text: "RELEASE GATES"

      table @gates:
        split: "avoid"
        table-column:
          width: 34%
        table-column:
          width: 38%
        table-column:
          width: 28%
        table-header:
          table-row:
            cell:
              header-cell: true
              style: "@label"
              padding: 9pt
              background: "#111A21"
              color: "#FFFFFF"
              text: "GATE"
            cell:
              header-cell: true
              style: "@label"
              padding: 9pt
              background: "#111A21"
              color: "#FFFFFF"
              text: "EVIDENCE"
            cell:
              header-cell: true
              style: "@label"
              padding: 9pt
              background: "#111A21"
              color: "#FFFFFF"
              text: "STATE"
        table-row:
          cell:
            style: "@base"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Typed source"
          cell:
            style: "@base"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Schema and expressions"
          cell:
            style: "@label"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            color: "#25313A"
            text: "PASSED"
        table-row:
          cell:
            style: "@base"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Layout plan"
          cell:
            style: "@base"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Stable page geometry"
          cell:
            style: "@label"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            color: "#25313A"
            text: "PASSED"
        table-row:
          cell:
            style: "@base"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Publication"
          cell:
            style: "@base"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: blocked ? "Approval outstanding" : "Approval recorded"
          cell:
            style: "@label"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            color: "#D94F2B"
            text: blocked ? "OPEN" : "PASSED"

      paragraph @sequence-label:
        style: "@label"
        margin-top: 30pt
        margin-bottom: 8pt
        text: "DEPLOYMENT SEQUENCE"

      table @sequence:
        split: "avoid"
        table-column:
          width: 30%
        table-column:
          width: 28%
        table-column:
          width: 24%
        table-column:
          width: 18%
        table-header:
          table-row:
            cell:
              style: "@label"
              padding: 8pt
              background: "#111A21"
              color: "#FFFFFF"
              text: "STEP"
            cell:
              style: "@label"
              padding: 8pt
              background: "#111A21"
              color: "#FFFFFF"
              text: "OWNER"
            cell:
              style: "@label"
              padding: 8pt
              background: "#111A21"
              color: "#FFFFFF"
              text: "WINDOW"
            cell:
              style: "@label"
              padding: 8pt
              background: "#111A21"
              color: "#FFFFFF"
              text: "STATE"
        table-row:
          cell:
            style: "@base"
            padding: 9pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Freeze release candidate"
          cell:
            style: "@base"
            padding: 9pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Publishing Platform"
          cell:
            style: "@base"
            bind: "deploymentWindow"
            padding: 9pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
          cell:
            style: "@label"
            padding: 9pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            color: "#25313A"
            text: "COMPLETE"
        table-row:
          cell:
            style: "@base"
            padding: 9pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Promote signed artifacts"
          cell:
            style: "@base"
            padding: 9pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Release Engineering"
          cell:
            style: "@base"
            bind: "targetDate"
            padding: 9pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
          cell:
            style: "@label"
            padding: 9pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            color: "#D94F2B"
            text: blocked ? "PAUSED" : "READY"
        table-row:
          cell:
            style: "@base"
            padding: 9pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Verify production plan"
          cell:
            style: "@base"
            bind: "approver"
            padding: 9pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
          cell:
            style: "@base"
            padding: 9pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Post-deploy"
          cell:
            style: "@label"
            padding: 9pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            color: "#687078"
            text: "PENDING"

      paragraph @signoff:
        style: "@base"
        margin-top: 26pt
        padding: 13pt
        background: "#F2EEE6"
        border-width: 1pt
        border-color: "#CDC7BD"
        border-radius: 4pt
        text: blocked ? "Sign-off is withheld until the open publication gate is closed." : "All named evidence is complete; final sign-off may proceed."

      paragraph @footer:
        style: "@label"
        margin-top: 38pt
        padding-top: 10pt
        border-top-width: 1pt
        border-color: "#CDC7BD"
        text: "DECISION RECORD / GENERATED LOCALLY / HASHED EXACTLY"

      paragraph @evidence-hash:
        font: "Courier"
        size: 8pt
        line-height: 11pt
        bind: "evidenceHash"
        color: "#687078"
`,
    data: `{
  "product": "PaperRune 0.4",
  "releaseId": "REL-2026.07.22",
  "owner": "Publishing Platform",
  "targetDate": "July 24, 2026",
  "environment": "Production",
  "evidenceHash": "sha256:7cb8d4f6a31e92c0",
  "deploymentWindow": "July 22 / 18:00 UTC",
  "approver": "Noor Williams",
  "score": 82,
  "blocked": true,
  "blocker": "Documentation approval is still pending."
}`,
  },
];
