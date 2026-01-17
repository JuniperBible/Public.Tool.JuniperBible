/**
 * Juniper Ingest Page - Event Handlers and Filtering
 */

/**
 * Apply filters to the modules table based on selected language, category, and license
 */
function applyFilters() {
  const langFilter = document.getElementById('filter-language').value;
  const categoryFilter = document.getElementById('filter-category').value;
  const licenseFilter = document.getElementById('filter-license').value;
  const rows = document.querySelectorAll('#modules-table tbody tr');
  let visibleCount = 0;

  rows.forEach(row => {
    const rowLang = row.dataset.language || '';
    const rowCategory = row.dataset.category || '';
    const rowLicense = row.dataset.license || '';
    const langMatch = !langFilter || rowLang === langFilter;
    const categoryMatch = !categoryFilter || rowCategory === categoryFilter;
    const licenseMatch = !licenseFilter || rowLicense === licenseFilter;

    if (langMatch && categoryMatch && licenseMatch) {
      row.style.display = '';
      visibleCount++;
    } else {
      row.style.display = 'none';
      // Uncheck hidden rows
      const checkbox = row.querySelector('input[type="checkbox"]');
      if (checkbox) checkbox.checked = false;
    }
  });

  document.getElementById('filter-count').textContent =
    `Showing ${visibleCount} of ${rows.length} modules`;

  // Update tag active states
  updateTagStates();
}

/**
 * Update visual states of filter tags based on current filter selections
 */
function updateTagStates() {
  const langFilter = document.getElementById('filter-language').value;
  const categoryFilter = document.getElementById('filter-category').value;

  document.querySelectorAll('.filter-tag').forEach(tag => {
    const filterType = tag.dataset.filterType;
    const filterValue = tag.dataset.filterValue;
    let isActive = false;

    if (filterType === 'language' && langFilter === filterValue) {
      isActive = true;
    } else if (filterType === 'category' && categoryFilter === filterValue) {
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

/**
 * Toggle a filter tag on/off
 * @param {HTMLElement} tag - The tag element that was clicked
 */
function toggleTag(tag) {
  const filterType = tag.dataset.filterType;
  const filterValue = tag.dataset.filterValue;
  const selectId = 'filter-' + filterType;
  const select = document.getElementById(selectId);

  if (select) {
    if (select.value === filterValue) {
      select.value = ''; // Clear if already selected
    } else {
      select.value = filterValue;
    }
    applyFilters();
  }
}

/**
 * Set a specific filter to a value
 * @param {string} filterType - The type of filter (language, category, license)
 * @param {string} filterValue - The value to set
 */
function setFilter(filterType, filterValue) {
  const selectId = 'filter-' + filterType;
  const select = document.getElementById(selectId);
  if (select) {
    select.value = filterValue;
    applyFilters();
  }
}

/**
 * Clear all active filters
 */
function clearFilters() {
  document.getElementById('filter-language').value = '';
  document.getElementById('filter-category').value = '';
  document.getElementById('filter-license').value = '';
  applyFilters();
}

/**
 * Select all visible (non-filtered) module checkboxes
 */
function selectAllVisible() {
  document.querySelectorAll('#modules-table tbody tr').forEach(row => {
    if (row.style.display !== 'none') {
      const checkbox = row.querySelector('input[type="checkbox"]');
      if (checkbox) checkbox.checked = true;
    }
  });
}

/**
 * Deselect all module checkboxes
 */
function selectNone() {
  document.querySelectorAll('input[name="modules"]').forEach(cb => cb.checked = false);
}

/**
 * Ingest a single module by selecting only its checkbox and submitting the form
 * @param {string} moduleId - The ID of the module to ingest
 */
function ingestSingle(moduleId) {
  selectNone();
  const checkbox = document.getElementById('mod-' + moduleId);
  if (checkbox) {
    checkbox.checked = true;
    document.getElementById('ingest-form').submit();
  }
}

/**
 * Initialize event listeners when the DOM is ready
 */
document.addEventListener('DOMContentLoaded', function() {
  // Filter select dropdowns
  const filterLanguage = document.getElementById('filter-language');
  const filterCategory = document.getElementById('filter-category');
  const filterLicense = document.getElementById('filter-license');

  if (filterLanguage) {
    filterLanguage.addEventListener('change', applyFilters);
  }
  if (filterCategory) {
    filterCategory.addEventListener('change', applyFilters);
  }
  if (filterLicense) {
    filterLicense.addEventListener('change', applyFilters);
  }

  // Clear filters button
  const clearFiltersBtn = document.getElementById('clear-filters-btn');
  if (clearFiltersBtn) {
    clearFiltersBtn.addEventListener('click', clearFilters);
  }

  // Quick filter tags (category tags at the top)
  document.querySelectorAll('.filter-tag').forEach(tag => {
    tag.addEventListener('click', function() {
      toggleTag(this);
    });
  });

  // Inline filter tags in the table (language and category cells)
  document.querySelectorAll('.filter-tag-inline').forEach(tag => {
    tag.addEventListener('click', function() {
      const filterType = this.dataset.filterType;
      const filterValue = this.dataset.filterValue;
      setFilter(filterType, filterValue);
    });
  });

  // Single ingest buttons
  document.querySelectorAll('.ingest-single-btn').forEach(btn => {
    btn.addEventListener('click', function() {
      const moduleId = this.dataset.moduleId;
      ingestSingle(moduleId);
    });
  });

  // Select all visible button
  const selectAllVisibleBtn = document.getElementById('select-all-visible-btn');
  if (selectAllVisibleBtn) {
    selectAllVisibleBtn.addEventListener('click', selectAllVisible);
  }

  // Select none button
  const selectNoneBtn = document.getElementById('select-none-btn');
  if (selectNoneBtn) {
    selectNoneBtn.addEventListener('click', selectNone);
  }

  // Initialize filter count on page load
  applyFilters();
});
