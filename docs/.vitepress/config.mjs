import {defineConfig} from 'vitepress';

const repository = process.env.GITHUB_REPOSITORY?.split('/')[1];
const base = process.env.PAPERRUNE_DOCS_BASE || (process.env.GITHUB_ACTIONS && repository ? `/${repository}/` : '/');

export default defineConfig({
  title: 'PaperRune',
  description: 'Typed templates for deterministic PDF and HTML.',
  lang: 'en-US',
  base,
  cleanUrls: true,
  lastUpdated: true,
  head: [
    ['meta', {name: 'theme-color', content: '#f4f0e7'}],
    ['meta', {name: 'color-scheme', content: 'light dark'}],
  ],
  markdown: {
    lineNumbers: true,
    languageAlias: {paper: 'yaml'},
    languageLabel: {paper: 'Paper'},
  },
  themeConfig: {
    logo: '/mark.svg',
    siteTitle: 'PaperRune',
    nav: [
      {text: 'Guide', link: '/guide/getting-started'},
      {text: 'Language', link: '/reference/language'},
      {text: 'Playground', link: '/playground'},
      {text: 'CLI', link: '/reference/cli'},
    ],
    sidebar: {
      '/guide/': [
        {text: 'Start', items: [
          {text: 'Getting started', link: '/guide/getting-started'},
          {text: 'Projects and data', link: '/guide/projects-and-data'},
          {text: 'Assets and imports', link: '/paper-assets'},
          {text: 'Documentation site', link: '/guide/documentation-site'},
        ]},
      ],
      '/reference/': [
        {text: 'Start here', items: [
          {text: 'Language overview', link: '/reference/language'},
          {text: 'Node index', link: '/reference/nodes'},
          {text: 'Syntax and values', link: '/reference/syntax'},
          {text: 'Expressions', link: '/reference/expressions'},
        ]},
        {text: 'Document', items: [
          {text: 'Document and pages', link: '/reference/document'},
          {text: 'Content', link: '/reference/content'},
          {text: 'Layout', link: '/reference/layout'},
        ]},
        {text: 'Data and reuse', items: [
          {text: 'Schemas and expansion', link: '/reference/data'},
          {text: 'Components', link: '/reference/components'},
          {text: 'Styles and themes', link: '/reference/design'},
        ]},
        {text: 'Tools', items: [
          {text: 'Command line', link: '/reference/cli'},
          {text: 'Project file', link: '/reference/project-file'},
        ]},
      ],
      '/playground': [
        {text: 'Browser tools', items: [
          {text: 'WASM playground', link: '/playground'},
          {text: 'Language reference', link: '/reference/language'},
          {text: 'Expressions', link: '/reference/expressions'},
        ]},
      ],
    },
    socialLinks: [
      {icon: 'github', link: 'https://github.com/cssbruno/paperrune'},
    ],
    search: {provider: 'local'},
    outline: {level: [2, 3]},
    editLink: {
      pattern: 'https://github.com/cssbruno/paperrune/edit/main/docs/:path',
      text: 'Edit this page on GitHub',
    },
    footer: {
      copyright: 'PaperRune Health-Sector Restricted License 1.0',
    },
  },
});
