// Capsules page functionality

// Confirmation dialog handler for delete forms
function confirmDeleteCapsule(form) {
  const capsuleNameInput = form.querySelector('.capsule-name');
  const capsuleName = capsuleNameInput ? capsuleNameInput.value : 'this capsule';
  return confirm('Delete ' + capsuleName + '? This cannot be undone.');
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

// Initialize on load
document.addEventListener('DOMContentLoaded', function() {
  updateGenerateCount();
  updateExportCount();

  // Delete capsule confirmation forms
  document.querySelectorAll('form[action="/capsules/delete"]').forEach(function(form) {
    form.addEventListener('submit', function(e) {
      const capsuleNameInput = this.querySelector('.capsule-name');
      const capsuleName = capsuleNameInput ? capsuleNameInput.value : 'this capsule';
      if (!confirm('Delete ' + capsuleName + '? This cannot be undone.')) {
        e.preventDefault();
      }
    });
  });

  // Generate tab: Select All button
  const selectAllGenerateBtn = document.querySelector('#tab-generate button[data-action="select-all"]');
  if (selectAllGenerateBtn) {
    selectAllGenerateBtn.addEventListener('click', selectAllGenerate);
  }

  // Generate tab: Select None button
  const selectNoneGenerateBtn = document.querySelector('#tab-generate button[data-action="select-none"]');
  if (selectNoneGenerateBtn) {
    selectNoneGenerateBtn.addEventListener('click', selectNoneGenerate);
  }

  // Generate tab: Select all checkbox
  const selectAllGenerateCheckbox = document.getElementById('select-all-generate');
  if (selectAllGenerateCheckbox) {
    selectAllGenerateCheckbox.addEventListener('change', function() {
      toggleSelectAllGenerate(this);
    });
  }

  // Generate tab: Individual checkboxes
  document.querySelectorAll('#generate-table input[name="source"]').forEach(function(cb) {
    cb.addEventListener('change', updateGenerateCount);
  });

  // Export tab: Select All button
  const selectAllExportBtn = document.querySelector('#tab-export button[data-action="select-all"]');
  if (selectAllExportBtn) {
    selectAllExportBtn.addEventListener('click', selectAllExport);
  }

  // Export tab: Select None button
  const selectNoneExportBtn = document.querySelector('#tab-export button[data-action="select-none"]');
  if (selectNoneExportBtn) {
    selectNoneExportBtn.addEventListener('click', selectNoneExport);
  }

  // Export tab: Select all checkbox
  const selectAllExportCheckbox = document.getElementById('select-all-export');
  if (selectAllExportCheckbox) {
    selectAllExportCheckbox.addEventListener('change', function() {
      toggleSelectAllExport(this);
    });
  }

  // Export tab: Individual checkboxes
  document.querySelectorAll('#export-table input[name="source"]').forEach(function(cb) {
    cb.addEventListener('change', updateExportCount);
  });
});
