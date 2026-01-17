/**
 * SWORD Repository Manager - Event Handlers and Filter Functions
 */

// Filter functions
function applyFilters() {
  const typeFilter = document.getElementById('filter-type').value;
  const langFilter = document.getElementById('filter-language').value;
  const searchFilter = document.getElementById('filter-search').value.toLowerCase();
  const rows = document.querySelectorAll('#modules-table tbody tr');
  let visibleCount = 0;

  rows.forEach(row => {
    const rowType = row.dataset.type || '';
    const rowLang = row.dataset.language || '';
    const rowName = row.dataset.name || '';
    const typeMatch = !typeFilter || rowType === typeFilter;
    const langMatch = !langFilter || rowLang === langFilter;
    const searchMatch = !searchFilter || rowName.toLowerCase().includes(searchFilter);

    if (typeMatch && langMatch && searchMatch) {
      row.style.display = '';
      visibleCount++;
    } else {
      row.style.display = 'none';
    }
  });

  document.getElementById('filter-count').textContent =
    `Showing ${visibleCount} of ${rows.length} modules`;

  // Update tag active states
  updateTagStates();
}

function updateTagStates() {
  const typeFilter = document.getElementById('filter-type').value;
  const langFilter = document.getElementById('filter-language').value;

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
      select.value = ''; // Clear if already selected
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
  document.getElementById('filter-type').value = '';
  document.getElementById('filter-language').value = '';
  document.getElementById('filter-search').value = '';
  applyFilters();
}

// Initialize event listeners when DOM is ready
document.addEventListener('DOMContentLoaded', function() {
  // 1. Source select - submit form on change
  const sourceSelect = document.getElementById('source');
  if (sourceSelect) {
    sourceSelect.addEventListener('change', function() {
      this.form.submit();
    });
  }

  // 2. Filter selects - apply filters on change
  const filterType = document.getElementById('filter-type');
  if (filterType) {
    filterType.addEventListener('change', applyFilters);
  }

  const filterLanguage = document.getElementById('filter-language');
  if (filterLanguage) {
    filterLanguage.addEventListener('change', applyFilters);
  }

  // Filter search input - apply filters on input
  const filterSearch = document.getElementById('filter-search');
  if (filterSearch) {
    filterSearch.addEventListener('input', applyFilters);
  }

  // 3. Clear Filters button
  const clearButton = document.getElementById('clear-filters-btn');
  if (clearButton) {
    clearButton.addEventListener('click', clearFilters);
  }

  // 4. Filter tags - toggle on click
  const filterTags = document.querySelectorAll('.filter-tag');
  filterTags.forEach(tag => {
    tag.addEventListener('click', function() {
      toggleTag(this);
    });
  });

  // 5. Type and language tags in table - set filter on click
  const typeTags = document.querySelectorAll('.tag-type');
  typeTags.forEach(tag => {
    tag.addEventListener('click', function() {
      const filterValue = this.dataset.filterValue;
      setFilter('type', filterValue);
    });
  });

  const languageTags = document.querySelectorAll('.tag-language');
  languageTags.forEach(tag => {
    tag.addEventListener('click', function() {
      const filterValue = this.dataset.filterValue;
      setFilter('language', filterValue);
    });
  });

  // 6. Delete forms - confirm before submit
  const deleteForms = document.querySelectorAll('.delete-form');
  deleteForms.forEach(form => {
    form.addEventListener('submit', function(event) {
      const moduleName = this.dataset.moduleName;
      if (!confirm(`Delete ${moduleName}? This cannot be undone.`)) {
        event.preventDefault();
      }
    });
  });

  // Initialize filter count on page load
  applyFilters();
});
