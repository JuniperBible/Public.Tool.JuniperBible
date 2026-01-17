/**
 * Common JavaScript utilities for Juniper Bible Web UI
 */

// Theme Manager - handles light/dark theme toggling
class ThemeManager {
  constructor() {
    this.html = document.documentElement;
    this.currentTheme = this.loadTheme();
    this.applyTheme(this.currentTheme);
  }

  loadTheme() {
    const saved = localStorage.getItem('theme');
    if (saved) return saved;
    // Respect system preference if no saved preference
    if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
      return 'dark';
    }
    return 'light';
  }

  saveTheme(theme) {
    localStorage.setItem('theme', theme);
  }

  applyTheme(theme) {
    this.html.setAttribute('data-theme', theme);
    this.currentTheme = theme;
  }

  toggle() {
    const newTheme = this.currentTheme === 'dark' ? 'light' : 'dark';
    this.applyTheme(newTheme);
    this.saveTheme(newTheme);
    return newTheme;
  }

  getTheme() {
    return this.currentTheme;
  }
}

// Settings Manager - handles settings visibility (dev mode)
class SettingsManager {
  constructor() {
    this.visible = this.loadVisibility();
  }

  loadVisibility() {
    return localStorage.getItem('settingsVisible') === 'true';
  }

  saveVisibility(visible) {
    localStorage.setItem('settingsVisible', visible ? 'true' : 'false');
    this.visible = visible;
  }

  toggle() {
    const newVisibility = !this.visible;
    this.saveVisibility(newVisibility);
    return newVisibility;
  }

  isVisible() {
    return this.visible;
  }

  applyToElement(element) {
    if (element && this.visible) {
      element.style.display = '';
    }
  }
}

// Global instances
window.themeManager = new ThemeManager();
window.settingsManager = new SettingsManager();

// Bible Reader Navigation
class BibleReaderNav {
  constructor() {
    this.nav = document.querySelector('.bible-nav[data-bible]');
    if (!this.nav) return;

    this.bibleId = this.nav.dataset.bible;
    this.bookId = this.nav.dataset.book;
    this.chapter = parseInt(this.nav.dataset.chapter) || 1;

    this.bibleSelect = document.getElementById('bible-select');
    this.bookSelect = document.getElementById('book-select');
    this.chapterSelect = document.getElementById('chapter-select');

    this.initSelects();
    this.initKeyboardNav();
  }

  initSelects() {
    if (this.bibleSelect) {
      this.bibleSelect.addEventListener('change', () => {
        var val = this.bibleSelect.options[this.bibleSelect.selectedIndex].value;
        window.location.href = '/bible/' + val + '/' + this.bookId + '/' + this.chapter;
      });
    }

    if (this.bookSelect) {
      this.bookSelect.addEventListener('change', () => {
        var opt = this.bookSelect.options[this.bookSelect.selectedIndex];
        var maxCh = parseInt(opt.dataset.chapters) || 1;
        var ch = Math.min(this.chapter, maxCh);
        window.location.href = '/bible/' + this.bibleId + '/' + opt.value + '/' + ch;
      });
    }

    if (this.chapterSelect) {
      this.chapterSelect.addEventListener('change', () => {
        var val = this.chapterSelect.options[this.chapterSelect.selectedIndex].value;
        window.location.href = '/bible/' + this.bibleId + '/' + this.bookId + '/' + val;
      });
    }
  }

  initKeyboardNav() {
    var readerData = document.getElementById('bible-reader-data');
    if (!readerData) return;

    var prevUrl = readerData.dataset.prevUrl;
    var nextUrl = readerData.dataset.nextUrl;

    document.addEventListener('keydown', function(e) {
      if (e.target.matches('input, select, textarea')) return;
      if (e.key === 'ArrowLeft' && prevUrl) {
        window.location.href = prevUrl;
      } else if (e.key === 'ArrowRight' && nextUrl) {
        window.location.href = nextUrl;
      }
    });
  }
}

// Initialize theme toggle button (if present)
document.addEventListener('DOMContentLoaded', function() {
  // Initialize Bible Reader Navigation
  new BibleReaderNav();

  // Initialize manage tab language filter dropdown
  const manageLangFilter = document.getElementById('manage-lang-filter');
  if (manageLangFilter) {
    manageLangFilter.addEventListener('change', function() {
      const tag = this.dataset.tag || 'all';
      window.location.href = '/library/bibles/?tab=manage&tag=' + tag + '&lang=' + this.value;
    });
  }
  const toggle = document.getElementById('theme-toggle');
  const sunIcon = document.getElementById('sun-icon');
  const moonIcon = document.getElementById('moon-icon');

  if (toggle && sunIcon && moonIcon) {
    // Update icons based on current theme
    function updateIcons(theme) {
      if (theme === 'dark') {
        sunIcon.style.display = 'none';
        moonIcon.style.display = 'block';
      } else {
        sunIcon.style.display = 'block';
        moonIcon.style.display = 'none';
      }
    }

    updateIcons(window.themeManager.getTheme());

    toggle.addEventListener('click', function() {
      const newTheme = window.themeManager.toggle();
      updateIcons(newTheme);
    });
  }

  // Apply settings visibility (Developer Tools link)
  const settingsLink = document.getElementById('settings-link');
  if (settingsLink) {
    settingsLink.style.display = window.settingsManager.isVisible() ? '' : 'none';
  }

  // Easter egg: click theme toggle 11 times within 20 seconds to show/hide Developer Tools
  if (toggle) {
    let clickTimes = [];

    function handleEasterEgg() {
      const now = Date.now();
      clickTimes.push(now);
      clickTimes = clickTimes.filter(t => now - t < 20000);

      if (clickTimes.length >= 11) {
        const newVisibility = window.settingsManager.toggle();
        if (settingsLink) {
          settingsLink.style.display = newVisibility ? '' : 'none';
        }
        clickTimes = [];
      }
    }

    toggle.addEventListener('click', handleEasterEgg);
  }

  // Search result highlighting
  const queryData = document.getElementById('search-query-data');
  if (queryData) {
    const query = queryData.dataset.query || '';
    if (query) {
      const searchTerm = query.replace(/^"|"$/g, '');
      if (searchTerm) {
        const results = document.querySelectorAll('.result-text');
        results.forEach(el => {
          const regex = new RegExp('(' + escapeRegex(searchTerm) + ')', 'gi');
          el.innerHTML = el.innerHTML.replace(regex, '<mark>$1</mark>');
        });
      }
    }
  }

  // Initialize custom source on repository page
  const sourceSelect = document.getElementById('source');
  if (sourceSelect && sourceSelect.value === 'Custom') {
    const customContainer = document.getElementById('custom-url-container');
    const sourceUrl = document.getElementById('source-url');
    if (customContainer) {
      customContainer.style.display = 'flex';
      customContainer.style.gap = '0.5rem';
      customContainer.style.alignItems = 'center';
    }
    if (sourceUrl) {
      sourceUrl.style.display = 'none';
    }
  }

  // Initialize counts on load for capsules tab
  const generateTable = document.getElementById('generate-table');
  const exportTable = document.getElementById('export-table');
  if (generateTable) updateGenerateCount();
  if (exportTable) updateExportCount();

  // Chapter jump select handler - navigates to selected chapter
  document.querySelectorAll('.chapter-jump-select').forEach(select => {
    select.addEventListener('change', function() {
      if (this.value) {
        window.location.href = this.value;
      }
    });
  });

  // Delete form confirmation handler
  document.querySelectorAll('.delete-confirm-form').forEach(form => {
    form.addEventListener('submit', function(e) {
      e.preventDefault();
      const moduleNameInput = this.querySelector('.module-name');
      const moduleName = moduleNameInput ? moduleNameInput.value : 'this module';
      const message = 'Delete ' + moduleName + '? This cannot be undone.';

      // Use accessible modal if showConfirmDialog is available
      if (typeof showConfirmDialog === 'function') {
        showConfirmDialog(message, this);
      } else {
        // Fallback to browser confirm()
        if (confirm(message)) {
          this.submit();
        }
      }
    });
  });

  // Disable submit buttons on form submission to prevent double clicks
  document.querySelectorAll('form').forEach(form => {
    form.addEventListener('submit', function(e) {
      const btn = this.querySelector('.submit-btn');
      if (btn && !btn.disabled) {
        btn.disabled = true;
        const loadingText = btn.getAttribute('data-loading-text');
        if (loadingText) {
          btn.textContent = loadingText;
        }
        btn.setAttribute('aria-busy', 'true');
      }
    });
  });

  // Repoman refresh cooldown
  const loadBtn = document.getElementById('load-btn');
  const btnText = document.getElementById('btn-text');
  const btnCountdown = document.getElementById('btn-countdown');
  const loadForm = document.getElementById('load-form');

  if (loadBtn && btnText && btnCountdown && loadForm) {
    const sourceInput = loadForm.querySelector('input[name="source"]');
    const source = sourceInput ? sourceInput.value : 'default';
    const COOLDOWN_KEY = 'repoman_refresh_cooldown_' + source;
    const COOLDOWN_SECONDS = 30;

    function formatTime(seconds) {
      return seconds + 's';
    }

    function updateButtonState() {
      const cooldownEnd = localStorage.getItem(COOLDOWN_KEY);
      if (cooldownEnd) {
        const remaining = Math.ceil((parseInt(cooldownEnd) - Date.now()) / 1000);
        if (remaining > 0) {
          loadBtn.disabled = true;
          loadBtn.setAttribute('aria-busy', 'true');
          btnText.style.display = 'none';
          btnCountdown.style.display = 'inline';
          btnCountdown.textContent = 'Refresh available in ' + formatTime(remaining);
          return true;
        } else {
          localStorage.removeItem(COOLDOWN_KEY);
        }
      }
      loadBtn.disabled = false;
      loadBtn.removeAttribute('aria-busy');
      btnText.style.display = 'inline';
      btnCountdown.style.display = 'none';
      return false;
    }

    function startCooldown() {
      const cooldownEnd = Date.now() + (COOLDOWN_SECONDS * 1000);
      localStorage.setItem(COOLDOWN_KEY, cooldownEnd.toString());
    }

    if (updateButtonState()) {
      const interval = setInterval(function() {
        if (!updateButtonState()) {
          clearInterval(interval);
        }
      }, 1000);
    }

    loadForm.addEventListener('submit', function() {
      startCooldown();
    });
  }
});

// Utility function for regex escaping
function escapeRegex(string) {
  return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// Repository source selection
function handleSourceChange(select) {
  const customContainer = document.getElementById('custom-url-container');
  const sourceUrl = document.getElementById('source-url');
  if (!customContainer || !sourceUrl) return;

  if (select.value === 'Custom') {
    customContainer.style.display = 'flex';
    customContainer.style.gap = '0.5rem';
    customContainer.style.alignItems = 'center';
    sourceUrl.style.display = 'none';
    const customUrlInput = document.getElementById('custom-url');
    if (customUrlInput) customUrlInput.focus();
  } else {
    customContainer.style.display = 'none';
    sourceUrl.style.display = '';
    select.form.submit();
  }
}

// Generate IR tab functions
function selectAllGenerate() {
  document.querySelectorAll('#generate-table input[name="source"]').forEach(cb => cb.checked = true);
  updateGenerateCount();
  updateSelectAllGenerateState();
}

function selectNoneGenerate() {
  document.querySelectorAll('#generate-table input[name="source"]').forEach(cb => cb.checked = false);
  updateGenerateCount();
  updateSelectAllGenerateState();
}

function toggleSelectAllGenerate(checkbox) {
  if (checkbox.checked) {
    selectAllGenerate();
  } else {
    selectNoneGenerate();
  }
}

function updateGenerateCount() {
  const checked = document.querySelectorAll('#generate-table input[name="source"]:checked').length;
  const el = document.getElementById('generate-selected-count');
  if (el) el.textContent = checked + ' selected';
}

function updateSelectAllGenerateState() {
  const all = document.querySelectorAll('#generate-table input[name="source"]');
  const checked = document.querySelectorAll('#generate-table input[name="source"]:checked');
  const selectAll = document.getElementById('select-all-generate');
  if (selectAll) selectAll.checked = all.length > 0 && all.length === checked.length;
}

// Export tab functions
function selectAllExport() {
  document.querySelectorAll('#export-table input[name="source"]').forEach(cb => cb.checked = true);
  updateExportCount();
  updateSelectAllExportState();
}

function selectNoneExport() {
  document.querySelectorAll('#export-table input[name="source"]').forEach(cb => cb.checked = false);
  updateExportCount();
  updateSelectAllExportState();
}

function toggleSelectAllExport(checkbox) {
  if (checkbox.checked) {
    selectAllExport();
  } else {
    selectNoneExport();
  }
}

function updateExportCount() {
  const checked = document.querySelectorAll('#export-table input[name="source"]:checked').length;
  const el = document.getElementById('export-selected-count');
  if (el) el.textContent = checked + ' selected';
}

function updateSelectAllExportState() {
  const all = document.querySelectorAll('#export-table input[name="source"]');
  const checked = document.querySelectorAll('#export-table input[name="source"]:checked');
  const selectAll = document.getElementById('select-all-export');
  if (selectAll) selectAll.checked = all.length > 0 && all.length === checked.length;
}

// Convert tab switching
function showTab(tabName) {
  document.querySelectorAll('.tab-content').forEach(el => {
    el.classList.remove('active');
  });
  document.querySelectorAll('.tab').forEach(el => {
    el.classList.remove('active');
  });
  const tabContent = document.getElementById('tab-' + tabName);
  if (tabContent) {
    tabContent.classList.add('active');
  }
  if (typeof event !== 'undefined' && event.target) {
    event.target.classList.add('active');
  }
}

/**
 * Accessible Confirmation Dialog
 * Replaces browser confirm() with an accessible HTML5 <dialog> modal
 * Requires {{template "confirmDialog"}} to be included in the page
 */
function showConfirmDialog(message, formToSubmit) {
  const dialog = document.getElementById('confirm-dialog');
  if (!dialog) {
    console.warn('Confirm dialog not found. Falling back to browser confirm().');
    return confirm(message);
  }

  const messageEl = document.getElementById('confirm-message');
  const cancelBtn = dialog.querySelector('[data-action="cancel"]');
  const confirmBtn = dialog.querySelector('[data-action="confirm"]');

  if (messageEl) {
    messageEl.textContent = message;
  }

  // Handle confirm button click
  const handleConfirm = function() {
    cleanup();
    if (formToSubmit) {
      formToSubmit.submit();
    }
  };

  // Handle cancel button click
  const handleCancel = function() {
    cleanup();
  };

  // Handle ESC key (default dialog behavior)
  const handleClose = function() {
    cleanup();
  };

  // Clean up event listeners
  const cleanup = function() {
    confirmBtn.removeEventListener('click', handleConfirm);
    cancelBtn.removeEventListener('click', handleCancel);
    dialog.removeEventListener('close', handleClose);
    closeDialog(dialog);
  };

  // Attach event listeners
  confirmBtn.addEventListener('click', handleConfirm);
  cancelBtn.addEventListener('click', handleCancel);
  dialog.addEventListener('close', handleClose);

  // Open the dialog
  openDialog(dialog);
  return false; // Prevent form submission
}

/**
 * Opens an accessible dialog with proper focus management
 */
function openDialog(dialog) {
  dialog.showModal();
  document.body.classList.add('modal-is-open');
}

/**
 * Closes an accessible dialog
 */
function closeDialog(dialog) {
  document.body.classList.remove('modal-is-open');
  dialog.close();
}
