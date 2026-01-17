// Bible Index Page Event Handlers
// External event listeners for bible_index.html template

document.addEventListener('DOMContentLoaded', function() {
  // 1. Language filter buttons (Browse tab)
  const languageFilterButtons = document.querySelectorAll('.language-filter-btn');
  languageFilterButtons.forEach(function(button) {
    button.addEventListener('click', function(e) {
      e.preventDefault();
      const language = this.dataset.language || '';
      filterByLanguage(language);
    });
  });

  // 2. Compare checkboxes (Compare tab)
  const compareCheckboxes = document.querySelectorAll('input[name="compare-bible"]');
  compareCheckboxes.forEach(function(checkbox) {
    checkbox.addEventListener('change', function() {
      updateComparison();
    });
  });

  // 3. Book select dropdown (Compare tab)
  const bookSelect = document.getElementById('book-select');
  if (bookSelect) {
    bookSelect.addEventListener('change', function() {
      updateChapters();
    });
  }

  // 4. Chapter select dropdown (Compare tab)
  const chapterSelect = document.getElementById('chapter-select');
  if (chapterSelect) {
    chapterSelect.addEventListener('change', function() {
      updateVerses();
    });
  }

  // 5. Load Chapter button (Compare tab)
  const loadChapterBtn = document.getElementById('load-chapter-btn');
  if (loadChapterBtn) {
    loadChapterBtn.addEventListener('click', function(e) {
      e.preventDefault();
      loadChapter();
    });
  }

  // 6. Highlight differences checkbox (Compare tab)
  const highlightDiffCheckbox = document.getElementById('highlight-diff');
  if (highlightDiffCheckbox) {
    highlightDiffCheckbox.addEventListener('change', function() {
      toggleHighlight();
    });
  }

  // 7. Delete buttons (Manage tab)
  const deleteButtons = document.querySelectorAll('.delete-bible-btn');
  deleteButtons.forEach(function(button) {
    button.addEventListener('click', function(e) {
      const bibleName = this.dataset.bibleName || 'this Bible';
      if (!confirm('Delete ' + bibleName + '? This will remove the IR data.')) {
        e.preventDefault();
        return false;
      }
    });
  });

  // 8. Language filter dropdown (Manage tab)
  const manageLangFilter = document.getElementById('manage-lang-filter');
  if (manageLangFilter) {
    manageLangFilter.addEventListener('change', function() {
      const lang = this.value;
      const tag = this.dataset.tag || 'all';
      window.location.href = '/library/bibles/?tab=manage&tag=' + tag + '&lang=' + lang;
    });
  }
});
