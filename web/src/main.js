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
      import('@codemirror/theme-one-dark'),
    ]).then(([cm, lang, theme]) => ({ ...cm, ...lang, ...theme }));
  }
  return _cmPromise;
};
