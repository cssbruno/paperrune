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
];
