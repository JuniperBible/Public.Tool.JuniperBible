// SWORD Module Table JavaScript
// Handles filtering, selection, and table interactions for SWORD module tables

// Debounce timer for search input
let debounceTimer = null;

function debounce(func, delay) {
  return function() {
    const context = this;
    const args = arguments;
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => func.apply(context, args), delay);
  };
}

function applyFilters() {
  const typeFilter = document.getElementById('filter-type')?.value || '';
  const langFilter = document.getElementById('filter-language')?.value || '';
  const searchFilter = (document.getElementById('filter-search')?.value || '').toLowerCase();
  // Find the visible/active table with sword-module-table class
  const tables = document.querySelectorAll('.sword-module-table');
  let rows = [];
  tables.forEach(table => {
    // Check if table is visible (in an active tab)
    const parent = table.closest('.tab-content');
    if (!parent || parent.classList.contains('active')) {
      rows = rows.concat(Array.from(table.querySelectorAll('tbody tr')));
    }
  });
  let visibleCount = 0;

  rows.forEach(row => {
    const rowType = row.dataset.type || '';
    const rowLang = row.dataset.language || '';
    const rowName = row.dataset.name || '';
    const rowDesc = row.dataset.description || '';
    const typeMatch = !typeFilter || rowType === typeFilter;
    const langMatch = !langFilter || rowLang === langFilter;
    // Search all fields: name, description, type, language
    const searchText = (rowName + ' ' + rowDesc + ' ' + rowType + ' ' + rowLang).toLowerCase();
    const searchMatch = !searchFilter || searchText.includes(searchFilter);

    if (typeMatch && langMatch && searchMatch) {
      row.style.display = '';
      visibleCount++;
    } else {
      row.style.display = 'none';
    }
  });

  const filterCount = document.getElementById('filter-count');
  if (filterCount) {
    filterCount.textContent = `Showing ${visibleCount} of ${rows.length} modules`;
  }

  updateTagStates();
}

// Debounced version for search input (250ms delay)
const applyFiltersDebounced = debounce(applyFilters, 250);

function updateTagStates() {
  const typeFilter = document.getElementById('filter-type')?.value || '';
  const langFilter = document.getElementById('filter-language')?.value || '';

  document.querySelectorAll('.filter-tag').forEach(tag => {
    const filterType = tag.dataset.filterType;
    const filterValue = tag.dataset.filterValue;
    let isActive = false;

    if (filterType === 'type' && typeFilter === filterValue) {
      isActive = true;
    } else if (filterType === 'language' && langFilter === filterValue) {
      isActive = true;
    }

    if (isActive) {
      tag.style.fontWeight = 'bold';
      tag.style.boxShadow = '0 0 0 2px #007bff';
    } else {
      tag.style.fontWeight = 'normal';
      tag.style.boxShadow = 'none';
    }
  });
}

function toggleTag(tag) {
  const filterType = tag.dataset.filterType;
  const filterValue = tag.dataset.filterValue;
  const selectId = 'filter-' + filterType;
  const select = document.getElementById(selectId);

  if (select) {
    if (select.value === filterValue) {
      select.value = '';
    } else {
      select.value = filterValue;
    }
    applyFilters();
  }
}

function setFilter(filterType, filterValue) {
  const selectId = 'filter-' + filterType;
  const select = document.getElementById(selectId);
  if (select) {
    select.value = filterValue;
    applyFilters();
  }
}

function clearFilters() {
  const typeEl = document.getElementById('filter-type');
  const langEl = document.getElementById('filter-language');
  const searchEl = document.getElementById('filter-search');
  if (typeEl) typeEl.value = '';
  if (langEl) langEl.value = '';
  if (searchEl) searchEl.value = '';
  applyFilters();
}

// Ingest selection functions
function selectAllVisible() {
  document.querySelectorAll('.sword-module-table tbody tr').forEach(row => {
    if (row.style.display !== 'none') {
      const cb = row.querySelector('input[type="checkbox"]');
      if (cb) cb.checked = true;
    }
  });
  updateSelectAllState();
}

function selectNone() {
  document.querySelectorAll('input[name="modules"]').forEach(cb => cb.checked = false);
  updateSelectAllState();
}

function toggleSelectAll(checkbox) {
  if (checkbox.checked) {
    selectAllVisible();
  } else {
    selectNone();
  }
}

function updateSelectAllState() {
  const visibleCheckboxes = [];
  document.querySelectorAll('.sword-module-table tbody tr').forEach(row => {
    if (row.style.display !== 'none') {
      const cb = row.querySelector('input[type="checkbox"]');
      if (cb) visibleCheckboxes.push(cb);
    }
  });
  const allChecked = visibleCheckboxes.length > 0 && visibleCheckboxes.every(cb => cb.checked);
  const selectAll = document.getElementById('select-all');
  if (selectAll) selectAll.checked = allChecked;
}

function ingestSingle(moduleId) {
  selectNone();
  const cb = document.querySelector('input[value="' + CSS.escape(moduleId) + '"]');
  if (cb) {
    cb.checked = true;
    // Find and submit the associated form
    const formId = cb.getAttribute('form') || 'ingest-form';
    const form = document.getElementById(formId);
    if (form) form.submit();
  }
}

// Confirmation dialog handler for delete forms
// Uses accessible modal if available, falls back to browser confirm()
function confirmDelete(form) {
  const moduleNameInput = form.querySelector('.module-name');
  const moduleName = moduleNameInput ? moduleNameInput.value : 'this module';
  const message = 'Delete ' + moduleName + '? This cannot be undone.';

  // Use accessible modal if showConfirmDialog is available
  if (typeof showConfirmDialog === 'function') {
    return showConfirmDialog(message, form);
  }

  // Fallback to browser confirm()
  return confirm(message);
}

// Initialize all event listeners when DOM is loaded
document.addEventListener('DOMContentLoaded', function() {
  // Apply initial filters
  applyFilters();

  // Filter controls - apply filters on change
  const filterType = document.getElementById('filter-type');
  const filterLanguage = document.getElementById('filter-language');
  const filterSearch = document.getElementById('filter-search');

  if (filterType) {
    filterType.addEventListener('change', applyFilters);
  }

  if (filterLanguage) {
    filterLanguage.addEventListener('change', applyFilters);
  }

  if (filterSearch) {
    filterSearch.addEventListener('input', applyFiltersDebounced);
  }

  // Clear filters button
  const clearFiltersBtn = document.querySelector('[data-action="clear-filters"]');
  if (clearFiltersBtn) {
    clearFiltersBtn.addEventListener('click', clearFilters);
  }

  // Select all/none buttons
  const selectAllBtn = document.querySelector('[data-action="select-all"]');
  const selectNoneBtn = document.querySelector('[data-action="select-none"]');

  if (selectAllBtn) {
    selectAllBtn.addEventListener('click', selectAllVisible);
  }

  if (selectNoneBtn) {
    selectNoneBtn.addEventListener('click', selectNone);
  }

  // Filter tags - toggle filters
  document.querySelectorAll('[data-action="toggle-tag"]').forEach(tag => {
    tag.addEventListener('click', function() {
      toggleTag(this);
    });
  });

  // Set filter tags (in table cells)
  document.querySelectorAll('[data-action="set-filter"]').forEach(tag => {
    tag.addEventListener('click', function() {
      const filterType = this.dataset.filterType;
      const filterValue = this.dataset.filterValue;
      setFilter(filterType, filterValue);
    });
  });

  // Select-all checkbox
  const selectAllCheckbox = document.getElementById('select-all');
  if (selectAllCheckbox) {
    selectAllCheckbox.addEventListener('change', function() {
      toggleSelectAll(this);
    });
  }

  // Individual module checkboxes
  document.querySelectorAll('input[name="modules"]').forEach(cb => {
    cb.addEventListener('change', updateSelectAllState);
  });

  // Ingest single buttons
  document.querySelectorAll('[data-action="ingest-single"]').forEach(btn => {
    btn.addEventListener('click', function() {
      const moduleId = this.dataset.moduleId;
      ingestSingle(moduleId);
    });
  });

  // Delete confirmation forms
  document.querySelectorAll('form[data-action="confirm-delete"]').forEach(form => {
    form.addEventListener('submit', function(e) {
      if (!confirmDelete(this)) {
        e.preventDefault();
        return false;
      }
    });
  });
});
