export const playgroundSamples = [
  {
    name: 'Typed greeting',
    source: `document @hello:
  language: "en"
  title: "A typed welcome"

  schema input:
    string name
    bool premium
    optional string note

  style @base:
    font: "Helvetica"
    size: 11pt
    line-height: 17pt
    color: "#24313D"

  style @eyebrow:
    style: "@base"
    size: 8pt
    line-height: 11pt
    bold: true
    color: "#E85D3F"

  style @display:
    style: "@base"
    size: 32pt
    line-height: 36pt
    bold: true
    color: "#132238"

  style @small:
    style: "@base"
    size: 8pt
    line-height: 12pt
    color: "#607080"

  page @sheet:
    size: "A4"
    margin: 38pt
    body @content:
      paragraph @masthead:
        style: "@eyebrow"
        padding: 12pt
        background: "#132238"
        color: "#FFB29F"
        border-radius: 8pt
        text: "PAPERRUNE  /  BROWSER EDITION"

      paragraph @edition:
        style: "@eyebrow"
        margin-top: 32pt
        text: premium ? "PREMIUM DOCUMENT WORKSPACE" : "PERSONAL DOCUMENT WORKSPACE"

      heading @title:
        level: 1
        style: "@display"
        margin-top: 8pt
        margin-bottom: 10pt
        color: premium ? "#2459D3" : "#132238"
        text: name

      paragraph @message:
        style: "@base"
        size: 14pt
        line-height: 21pt
        margin-bottom: 34pt
        color: "#536171"
        text: premium ? "Your typed data became a deterministic page: locally, instantly, beautifully." : "Readable documents, planned exactly and rendered locally."

      table @facts:
        split: "avoid"
        table-column:
          width: 33%
        table-column:
          width: 34%
        table-column:
          width: 33%
        table-row:
          cell @typed:
            style: "@small"
            padding: 14pt
            background: "#EEF2FF"
            bold: true
            color: "#3D57A3"
            text: "01  TYPED INPUT"
          cell @local:
            style: "@small"
            padding: 14pt
            background: "#FFF0EB"
            bold: true
            color: "#B94730"
            text: "02  LOCAL WASM"
          cell @exact:
            style: "@small"
            padding: 14pt
            background: "#EAF7F0"
            bold: true
            color: "#287052"
            text: "03  EXACT OUTPUT"

      paragraph @note:
        visible: note != null
        style: "@base"
        margin-top: 24pt
        padding: 18pt
        background: "#F5F2EA"
        border-left-width: 3pt
        border-color: "#E85D3F"
        color: "#45515E"
        text: note != null ? note : ""

      paragraph @footer:
        style: "@small"
        margin-top: 30pt
        border-top-width: 1pt
        border-color: "#D8DEE5"
        padding-top: 10pt
        text: "Edit the source or JSON. This page redraws itself as you type."
`,
    data: `{
  "name": "Hello, document maker.",
  "premium": true,
  "note": "A document is more than text on a page. Give it rhythm, hierarchy, and a clear reason to exist."
}`,
  },
  {
    name: 'Calculated layout',
    source: `document @layout:
  language: "en"
  title: "Geometry report"

  schema metrics:
    number columns
    bool compact

  style @base:
    font: "Helvetica"
    size: 10pt
    line-height: 15pt
    color: "#28343E"

  style @label:
    style: "@base"
    size: 8pt
    line-height: 10pt
    bold: true
    color: "#6C7680"

  style @metric:
    style: "@base"
    size: 19pt
    line-height: 23pt
    bold: true
    color: "#152A3B"

  page:
    size: "A4"
    margin: 38pt
    body:
      paragraph @report-label:
        style: "@label"
        color: "#D45C3B"
        text: "LAYOUT SYSTEMS  /  REPORT 07"

      heading:
        level: 1
        margin-top: 8pt
        size: 30pt
        line-height: 34pt
        color: "#152A3B"
        text: "Geometry that explains itself."

      paragraph @intro:
        style: "@base"
        margin-top: 10pt
        margin-bottom: 28pt
        size: 12pt
        line-height: 18pt
        color: "#596773"
        text: "Fractions, physical units, and typed expressions resolve into one stable composition."

      row @metric-columns:
        gap: compact ? 8pt : 14pt
        height: 94pt
        paragraph @fraction-card:
          width: columns * 1fr
          style: "@metric"
          color: "#3D57A3"
          text: "2fr  /  Primary column"
        paragraph @fixed-card:
          width: 1fr
          style: "@metric"
          color: "#B94730"
          text: "1fr  /  Support"

      paragraph @formula-label:
        style: "@label"
        margin-top: 30pt
        margin-bottom: 8pt
        text: "THE LAYOUT FORMULA"

      paragraph @formula:
        font: "Courier"
        size: 12pt
        line-height: 18pt
        padding: 16pt
        margin-bottom: 24pt
        background: "#152A3B"
        color: "#F7F2E8"
        border-radius: 7pt
        text: "available width - gap  >  2fr + 1fr  >  exact points"

      table @principles:
        split: "avoid"
        table-column:
          width: 33%
        table-column:
          width: 34%
        table-column:
          width: 33%
        table-row:
          cell @units:
            style: "@base"
            padding: 12pt
            background: "#EEF2FF"
            bold: true
            color: "#3D57A3"
            text: "Physical units / pt, mm, in, and percentages"
          cell @expressions:
            style: "@base"
            padding: 12pt
            background: "#FFF0EB"
            bold: true
            color: "#B94730"
            text: "Typed expressions / no stringly layout decisions"
          cell @determinism:
            style: "@base"
            padding: 12pt
            background: "#EAF7F0"
            bold: true
            color: "#287052"
            text: "Deterministic output / the browser does not improvise"
`,
    data: `{
  "columns": 2,
  "compact": false
}`,
  },
  {
    name: 'Production invoice',
    source: `document @invoice:
  title: "Invoice"
  language: "en"

  schema billing:
    string invoiceNumber
    string issued
    string due
    string seller
    string sellerContact
    string customer
    string customerAddress
    string total
    list object items:
      max-items: 100
      string description
      string quantity
      string rate
      string amount

  style @base:
    font: "Helvetica"
    size: 10pt
    line-height: 15pt
    color: "#2F3D47"

  style @title:
    style: "@base"
    size: 30pt
    line-height: 34pt
    bold: true
    color: "#142D3D"

  style @label:
    style: "@base"
    size: 8pt
    line-height: 11pt
    bold: true
    color: "#D45C3B"

  style @total:
    style: "@base"
    size: 20pt
    line-height: 24pt
    bold: true
    color: "#142D3D"

  page @invoice-page:
    size: "A4"
    margin: 36pt
    page-numbers: true
    page-number-format: "Invoice | Page %d of {pages}"
    page-total-alias: "{pages}"

    header @invoice-header:
      row @invoice-masthead:
        gap: 16pt
        align-items: "center"
        heading @invoice-heading:
          width: 2fr
          level: 1
          style: "@title"
          text: "INVOICE"
        paragraph @number:
          width: 1fr
          style: "@base"
          bind: "invoiceNumber"
          bold: true
          align: "right"
          text: "INV-0000"

    body @invoice-content:
      paragraph @project-label:
        style: "@label"
        margin-top: 18pt
        text: "DOCUMENT SYSTEMS / JULY 2026"

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
              background: "#142D3D"
              color: "#FFB19E"
              text: "FROM"
            cell:
              header-cell: true
              style: "@label"
              padding: 8pt
              background: "#142D3D"
              color: "#FFB19E"
              text: "BILL TO"
        table-row:
          cell:
            padding: 10pt
            background: "#F3F5F5"
            paragraph @seller:
              style: "@base"
              bind: "seller"
              bold: true
              text: "Seller"
          cell:
            padding: 10pt
            background: "#F3F5F5"
            paragraph @customer:
              style: "@base"
              bind: "customer"
              bold: true
              text: "Customer"
        table-row:
          cell:
            padding: 10pt
            background: "#F3F5F5"
            paragraph @seller-contact:
              style: "@base"
              bind: "sellerContact"
              color: "#65717A"
              text: "Contact"
          cell:
            padding: 10pt
            background: "#F3F5F5"
            paragraph @customer-address:
              style: "@base"
              bind: "customerAddress"
              color: "#65717A"
              text: "Address"

      row @dates:
        gap: 12pt
        paragraph @issued:
          width: 1fr
          style: "@base"
          bind: "issued"
          bold: true
          color: "#B94730"
          text: "Issue date"
        paragraph @due:
          width: 1fr
          style: "@base"
          bind: "due"
          bold: true
          color: "#3D57A3"
          text: "Due date"

      table @items-table:
        repeat-header: true
        split: "rows"
        table-column @description-column:
          width: 52%
        table-column @quantity-column:
          width: 12%
        table-column @rate-column:
          width: 18%
        table-column @amount-column:
          width: 18%
        table-header @items-header:
          table-row @items-header-row:
            cell @description-heading:
              padding: 8pt
              background: "#DDE7EA"
              paragraph:
                style: "@label"
                color: "#40515C"
                text: "DESCRIPTION"
            cell @quantity-heading:
              padding: 8pt
              background: "#DDE7EA"
              paragraph:
                style: "@label"
                color: "#40515C"
                text: "QTY"
            cell @rate-heading:
              padding: 8pt
              background: "#DDE7EA"
              paragraph:
                style: "@label"
                color: "#40515C"
                text: "RATE"
            cell @amount-heading:
              padding: 8pt
              background: "#DDE7EA"
              paragraph:
                style: "@label"
                color: "#40515C"
                align: "right"
                text: "AMOUNT"
        repeat @item-rows:
          source: "items"
          instance-prefix: "items"
          max-items: 100
          table-row @item-row:
            cell @description-cell:
              padding: 9pt
              border-bottom-width: 1pt
              border-color: "#DDE3E6"
              paragraph:
                style: "@base"
                bind: "description"
                text: "Item"
            cell @quantity-cell:
              padding: 9pt
              border-bottom-width: 1pt
              border-color: "#DDE3E6"
              paragraph:
                style: "@base"
                bind: "quantity"
                text: "1"
            cell @rate-cell:
              padding: 9pt
              border-bottom-width: 1pt
              border-color: "#DDE3E6"
              paragraph:
                style: "@base"
                bind: "rate"
                text: "$0.00"
            cell @amount-cell:
              padding: 9pt
              border-bottom-width: 1pt
              border-color: "#DDE3E6"
              paragraph:
                style: "@base"
                bind: "amount"
                align: "right"
                bold: true
                text: "$0.00"

      row @summary:
        gap: 10pt
        paragraph @summary-note:
          width: 2fr
          style: "@base"
          color: "#66737C"
          text: "Payment due within 30 days. Thank you for building better documents with us."
        paragraph @total-value:
          width: 1fr
          style: "@total"
          bind: "total"
          align: "right"
          text: "$0.00"
      paragraph @note:
        style: "@base"
        margin-top: 18pt
        padding-top: 10pt
        border-top-width: 1pt
        border-color: "#D5DDE1"
        size: 8pt
        color: "#77828A"
        text: "AMOUNT DUE / Include the invoice number with your remittance."
`,
    data: `{
  "invoiceNumber": "INV-2026-0042",
  "issued": "July 20, 2026",
  "due": "August 19, 2026",
  "seller": "PaperRune Studio",
  "sellerContact": "documents@example.test | +1 555 0124",
  "customer": "Northwind Operations",
  "customerAddress": "22 Market Street, Seattle, WA 98101",
  "total": "$1,850.00",
  "items": [
    {"description":"PDF generation platform","quantity":"1","rate":"$800.00","amount":"$800.00"},
    {"description":"Template implementation","quantity":"6","rate":"$95.00","amount":"$570.00"},
    {"description":"Automated document checks","quantity":"4","rate":"$75.00","amount":"$300.00"},
    {"description":"Support package","quantity":"1","rate":"$180.00","amount":"$180.00"}
  ]
}`,
  },
  {
    name: 'Component dispatch',
    source: `document @component-dispatch:
  title: "Release readiness"
  language: "en"

  schema release:
    string product
    number score
    bool blocked
    optional string blocker

  style @base:
    font: "Helvetica"
    size: 11pt
    line-height: 17pt
    color: "#31404A"

  style @label:
    style: "@base"
    size: 8pt
    line-height: 11pt
    bold: true
    color: "#697681"

  style @display:
    style: "@base"
    size: 29pt
    line-height: 34pt
    bold: true
    color: "#152D3D"

  component @ready-card:
    heading @ready-title:
      level: 2
      margin-top: 22pt
      padding: 15pt
      background: "#DDF4E8"
      border-radius: 8pt
      color: "#176B49"
      text: "READY TO SHIP"
    paragraph @ready-copy:
      style: "@base"
      padding: 14pt
      background: "#F0FAF5"
      color: "#35644F"
      text: "All release gates passed. The package is clear for publication."

  component @blocked-card:
    heading @blocked-title:
      level: 2
      margin-top: 22pt
      padding: 15pt
      background: "#FFE3DC"
      border-radius: 8pt
      color: "#B42318"
      text: "RELEASE BLOCKED"
    paragraph @blocked-copy:
      style: "@base"
      padding: 14pt
      background: "#FFF3F0"
      color: "#7E3A30"
      text: "One gate needs attention. Resolve it before publishing this release."

  page @sheet:
    size: "A4"
    margin: 38pt
    body @content:
      row @report-header:
        gap: 12pt
        align-items: "center"
        paragraph @report-name:
          width: 2fr
          style: "@label"
          color: "#D45C3B"
          text: "RELEASE CONTROL / LIVE REPORT"
        paragraph @report-version:
          width: 1fr
          style: "@label"
          align: "right"
          text: "PAPERRUNE 0.4"

      heading @product:
        level: 1
        style: "@display"
        margin-top: 22pt
        text: product

      paragraph @subtitle:
        style: "@base"
        margin-top: 8pt
        margin-bottom: 28pt
        color: "#66747E"
        text: "A compact, typed decision report assembled from reusable components."

      row @scoreboard:
        gap: 12pt
        paragraph @score:
          width: 1fr
          style: "@display"
          bind: "score"
          format: "decimal"
          format-locale: "en-US"
          format-min-fraction: 0
          format-max-fraction: 0
          color: "#3D57A3"
          text: "0"
        paragraph @rating:
          width: 2fr
          style: "@base"
          bold: true
          text: switch:
            case score >= 90: "EXCELLENT / all systems are ready"
            case score >= 70: "REVIEW / almost there, inspect open gates"
            default: "AT RISK / intervention is required"

      paragraph @score-label:
        style: "@label"
        margin-top: 8pt
        text: "READINESS SCORE / 100"

      use @release-state:
        component: blocked ? @blocked-card : @ready-card

      paragraph @blocker:
        visible: blocker != null && blocked
        style: "@base"
        margin-top: 10pt
        padding: 14pt
        border-left-width: 3pt
        border-color: "#B42318"
        color: "#B42318"
        text: blocker != null ? blocker : ""

      paragraph @checks-label:
        style: "@label"
        margin-top: 28pt
        margin-bottom: 8pt
        text: "SYSTEM CHECKS"

      table @checks:
        split: "avoid"
        table-column:
          width: 33%
        table-column:
          width: 34%
        table-column:
          width: 33%
        table-row:
          cell @types:
            style: "@base"
            padding: 13pt
            background: "#EEF2FF"
            bold: true
            color: "#3D57A3"
            text: "01  TYPES / Schema valid"
          cell @layout:
            style: "@base"
            padding: 13pt
            background: "#F8F0E2"
            bold: true
            color: "#9A6438"
            text: "02  LAYOUT / Plan stable"
          cell @output:
            style: "@base"
            padding: 13pt
            background: "#E9F6EF"
            bold: true
            color: "#287052"
            text: "03  OUTPUT / Hash exact"
`,
    data: `{
  "product": "PaperRune 0.4",
  "score": 82,
  "blocked": true,
  "blocker": "Documentation approval is still pending."
}`,
  },
];
