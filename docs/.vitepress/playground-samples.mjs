export const playgroundSamples = [
  {
    name: 'Typed greeting',
    source: `document @hello:
  language: "en"
  title: "PaperRune playground"

  schema input:
    string name
    bool premium
    optional string note

  page @sheet:
    size: "A4"
    margin: 42pt
    body @content:
      heading @title:
        level: 1
        size: premium ? 24pt : 20pt
        color: premium ? "#2459D3" : "#171A1F"
        text: name

      paragraph @message:
        size: 11pt
        line-height: 16pt
        text: premium ? "Premium documents, planned exactly." : "Readable documents, planned exactly."

      paragraph @note:
        visible: note != null
        color: "#555B65"
        text: note != null ? note : ""
`,
    data: `{
  "name": "Hello from WebAssembly",
  "premium": true,
  "note": "Edit the source or JSON. The preview is compiled locally."
}`,
  },
  {
    name: 'Calculated layout',
    source: `document @layout:
  language: "en"

  schema metrics:
    number columns
    bool compact

  page:
    size: "A4"
    margin: 36pt
    body:
      heading:
        level: 1
        text: "Calculated layout"
      row:
        gap: compact ? 8pt : 14pt
        height: 70pt
        paragraph:
          width: columns * 1fr
          text: "Exact decimal and unit expressions"
        paragraph:
          width: 1fr
          text: "No browser layout decisions"
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
    line-height: 14pt
    color: "#344451"

  style @title:
    style: "@base"
    size: 24pt
    line-height: 30pt
    bold: true
    color: "#173F5F"

  style @label:
    style: "@base"
    size: 8pt
    line-height: 11pt
    bold: true
    color: "#687887"

  style @total:
    style: "@base"
    size: 17pt
    line-height: 22pt
    bold: true
    color: "#176B49"

  page @invoice-page:
    size: "A4"
    margin: 34pt
    page-numbers: true
    page-number-format: "Invoice | Page %d of {pages}"
    page-total-alias: "{pages}"

    header @invoice-header:
      heading @invoice-heading:
        level: 1
        style: "@title"
        text: "INVOICE"
      paragraph @number-label:
        style: "@label"
        text: "INVOICE NUMBER"
      paragraph @number:
        style: "@base"
        bind: "invoiceNumber"
        text: "INV-0000"

    body @invoice-content:
      paragraph @seller-label:
        style: "@label"
        text: "FROM"
      paragraph @seller:
        style: "@base"
        bind: "seller"
        text: "Seller"
      paragraph @seller-contact:
        style: "@base"
        bind: "sellerContact"
        text: "Contact"
      paragraph @customer-label:
        style: "@label"
        text: "BILL TO"
      paragraph @customer:
        style: "@base"
        bind: "customer"
        text: "Customer"
      paragraph @customer-address:
        style: "@base"
        bind: "customerAddress"
        text: "Address"
      paragraph @issued-label:
        style: "@label"
        text: "ISSUED"
      paragraph @issued:
        style: "@base"
        bind: "issued"
        text: "Issue date"
      paragraph @due-label:
        style: "@label"
        text: "DUE"
      paragraph @due:
        style: "@base"
        bind: "due"
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
              paragraph:
                style: "@label"
                text: "DESCRIPTION"
            cell @quantity-heading:
              paragraph:
                style: "@label"
                text: "QTY"
            cell @rate-heading:
              paragraph:
                style: "@label"
                text: "RATE"
            cell @amount-heading:
              paragraph:
                style: "@label"
                text: "AMOUNT"
        repeat @item-rows:
          source: "items"
          instance-prefix: "items"
          max-items: 100
          table-row @item-row:
            cell @description-cell:
              paragraph:
                style: "@base"
                bind: "description"
                text: "Item"
            cell @quantity-cell:
              paragraph:
                style: "@base"
                bind: "quantity"
                text: "1"
            cell @rate-cell:
              paragraph:
                style: "@base"
                bind: "rate"
                text: "$0.00"
            cell @amount-cell:
              paragraph:
                style: "@base"
                bind: "amount"
                text: "$0.00"

      paragraph @total-label:
        style: "@label"
        text: "AMOUNT DUE"
      paragraph @total-value:
        style: "@total"
        bind: "total"
        text: "$0.00"
      paragraph @note:
        style: "@base"
        text: "Thank you. Include the invoice number with your remittance."
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
    line-height: 16pt
    color: "#344451"

  component @ready-card:
    heading @ready-title:
      level: 2
      color: "#176B49"
      text: "READY TO SHIP"
    paragraph @ready-copy:
      style: "@base"
      text: "All release gates passed."

  component @blocked-card:
    heading @blocked-title:
      level: 2
      color: "#B42318"
      text: "RELEASE BLOCKED"
    paragraph @blocked-copy:
      style: "@base"
      text: "Resolve the blocker before publishing."

  page @sheet:
    size: "A4"
    margin: 42pt
    body @content:
      heading @product:
        level: 1
        size: 25pt
        text: product

      paragraph @score-label:
        style: "@base"
        bold: true
        text: "Readiness"
      paragraph @score:
        style: "@base"
        text: switch:
          case score >= 90: "Excellent"
          case score >= 70: "Review"
          default: "At risk"

      use @release-state:
        component: blocked ? @blocked-card : @ready-card

      paragraph @blocker:
        visible: blocker != null && blocked
        style: "@base"
        color: "#B42318"
        text: blocker != null ? blocker : ""
`,
    data: `{
  "product": "PaperRune 0.4",
  "score": 82,
  "blocked": true,
  "blocker": "Documentation approval is still pending."
}`,
  },
];
