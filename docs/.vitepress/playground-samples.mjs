export const playgroundSamples = [
  {
    name: 'Editorial welcome',
    source: `document @welcome:
  language: "en"
  title: "Editorial welcome"

  schema input:
    string name
    bool premium
    optional string note

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
        margin-top: 8pt
        padding: 22pt
        background: "#111A21"
        color: "#FFFFFF"
        border-left-width: 7pt
        border-color: "#D94F2B"
        text: name

      paragraph @deck:
        style: "@base"
        size: 14pt
        line-height: 21pt
        margin-top: 16pt
        margin-bottom: 26pt
        color: "#56616A"
        text: "A page with a point of view, compiled from typed data and exact decisions."

      paragraph @manifesto:
        style: "@base"
        padding: 15pt
        padding-left: 18pt
        background: "#F2EEE6"
        border-left-width: 4pt
        border-color: "#D94F2B"
        size: 13pt
        line-height: 20pt
        bold: true
        text: note != null ? note : "Make the hierarchy obvious. Make every measurement intentional."

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
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Lead with meaning. Let spacing and scale establish the reading order."
        table-row:
          cell:
            style: "@label"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "02 / RHYTHM"
          cell:
            style: "@base"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Alternate dense evidence with quiet space so the page can breathe."
        table-row:
          cell:
            style: "@label"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "03 / PROOF"
          cell:
            style: "@base"
            padding: 11pt
            border-bottom-width: 1pt
            border-color: "#CDC7BD"
            text: "Compile the same inputs into the same pixels, every single time."

      paragraph @closing-label:
        style: "@label"
        margin-top: 34pt
        text: "DESIGNED IN PAPER / RENDERED IN GO WASM"

      heading @closing:
        level: 2
        margin-top: 7pt
        size: 23pt
        line-height: 28pt
        color: "#111A21"
        text: "The browser is the press. The plan is the contract."

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
  "premium": true,
  "note": "A document should feel authored, not assembled. Give it one voice, one hierarchy, and no accidental decoration."
}`,
  },
  {
    name: 'Layout specimen',
    source: `document @layout-specimen:
  language: "en"
  title: "Layout specimen"

  schema metrics:
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
        margin-top: 8pt
        text: "A measured page."

      paragraph @intro:
        style: "@base"
        margin-top: 10pt
        margin-bottom: 32pt
        size: 13pt
        line-height: 19pt
        color: "#59636B"
        text: "Fractions, physical units, and typed expressions resolve before a single pixel is painted."

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
    string total
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
          text: "INV-0000"

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
              text: "Seller"
          cell:
            padding: 10pt
            paragraph @customer:
              style: "@base"
              bind: "customer"
              bold: true
              text: "Customer"
        table-row:
          cell:
            padding: 10pt
            paragraph @seller-contact:
              style: "@base"
              bind: "sellerContact"
              color: "#697078"
              text: "Contact"
          cell:
            padding: 10pt
            paragraph @customer-address:
              style: "@base"
              bind: "customerAddress"
              color: "#697078"
              text: "Address"

      paragraph @parties-gap:
        size: 1pt
        line-height: 14pt
        color: "#FFFFFF"
        text: "."

      table @dates:
        split: "avoid"
        table-column:
          width: 50%
        table-column:
          width: 50%
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
        table-row:
          cell:
            style: "@base"
            bind: "issued"
            padding: 9pt
            bold: true
            text: "Issue date"
          cell:
            style: "@base"
            bind: "due"
            padding: 9pt
            bold: true
            text: "Due date"

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
                text: "Item"
            cell:
              padding: 10pt
              border-bottom-width: 1pt
              border-color: "#D5CFC5"
              paragraph:
                style: "@base"
                bind: "quantity"
                text: "1"
            cell:
              padding: 10pt
              border-bottom-width: 1pt
              border-color: "#D5CFC5"
              paragraph:
                style: "@base"
                bind: "rate"
                text: "$0.00"
            cell:
              padding: 10pt
              border-bottom-width: 1pt
              border-color: "#D5CFC5"
              paragraph:
                style: "@base"
                bind: "amount"
                align: "right"
                bold: true
                text: "$0.00"

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
            text: "$0.00"

      paragraph @terms:
        style: "@base"
        margin-top: 28pt
        padding-top: 10pt
        border-top-width: 1pt
        border-color: "#CDC7BD"
        size: 8pt
        color: "#687078"
        text: "Thank you for the collaboration. Questions: accounts@paperrune.studio"

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
  "total": "$1,850.00",
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
        margin-top: 8pt
        text: product

      paragraph @subtitle:
        style: "@base"
        margin-top: 10pt
        margin-bottom: 30pt
        size: 13pt
        line-height: 19pt
        color: "#59636B"
        text: "A typed decision record for the people responsible for publication."

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
            text: "0"
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
        margin-top: 12pt
        padding: 14pt
        border-left-width: 4pt
        border-color: "#D94F2B"
        bold: true
        text: blocker != null ? blocker : ""

      paragraph @gates-label:
        style: "@label"
        margin-top: 34pt
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

      paragraph @footer:
        style: "@label"
        margin-top: 38pt
        padding-top: 10pt
        border-top-width: 1pt
        border-color: "#CDC7BD"
        text: "DECISION RECORD / GENERATED LOCALLY / HASHED EXACTLY"
`,
    data: `{
  "product": "PaperRune 0.4",
  "score": 82,
  "blocked": true,
  "blocker": "Documentation approval is still pending."
}`,
  },
];
