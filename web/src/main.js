import './styles.css';
import { createIcons, icons } from 'lucide';

// Expose Lucide on the global so the existing inline scripts in index.html
// (which use `window.lucide.createIcons(...)`) keep working.
window.lucide = {
  createIcons: (opts = {}) => createIcons({ icons, ...opts }),
  icons,
};

// Initial icon swap once the DOM is ready, in case the inline scripts run
// before this module finishes loading.
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', () => window.lucide.createIcons());
} else {
  window.lucide.createIcons();
}

// Lazy-loaded CodeMirror bundle for the YAML editor.
// Kept in this module (not the inline HTML script) so that Vite handles
// the dynamic imports as proper code-split chunks.
let _cmPromise = null;
window.loadCodeMirror = function loadCodeMirror() {
  if (!_cmPromise) {
    _cmPromise = Promise.all([
      import('codemirror'),
      import('@codemirror/lang-yaml'),
      import('@codemirror/language'),
      import('@lezer/highlight'),
    ]).then(([cm, lang, { HighlightStyle, syntaxHighlighting }, { tags: t }]) => ({
      ...cm,
      ...lang,
      // High-contrast editor: black background, white values, bright keys.
      yamlTheme: [
        cm.EditorView.theme({
          '&': { backgroundColor: '#000', color: '#fff', fontSize: '13px' },
          '.cm-content': { caretColor: '#fff', fontFamily: 'var(--font-mono)' },
          '.cm-cursor, .cm-dropCursor': { borderLeftColor: '#fff' },
          '.cm-gutters': { backgroundColor: '#000', color: '#9ca3af', border: 'none', borderRight: '1px solid #262626' },
          '.cm-activeLine': { backgroundColor: '#141414' },
          '.cm-activeLineGutter': { backgroundColor: '#141414', color: '#fff' },
          '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': { backgroundColor: '#1e3a8a !important' },
          '.cm-selectionMatch': { backgroundColor: '#1e3a8a80' },
          '.cm-foldPlaceholder': { backgroundColor: '#262626', color: '#fff', border: 'none' },
        }, { dark: true }),
        syntaxHighlighting(HighlightStyle.define([
          { tag: t.definition(t.propertyName), color: '#7dd3fc' },
          { tag: [t.content, t.string, t.attributeValue], color: '#ffffff' },
          { tag: [t.separator, t.punctuation, t.squareBracket, t.brace], color: '#d4d4d4' },
          { tag: t.lineComment, color: '#a3a3a3', fontStyle: 'italic' },
          { tag: [t.labelName, t.typeName, t.keyword, t.meta], color: '#fcd34d' },
        ])),
      ],
    }));
  }
  return _cmPromise;
};
