// Bible Index and Compare Tab JavaScript
// Handles Browse tab filtering, Search tab highlighting, and Compare tab functionality

// Browse tab functionality
function filterByLanguage(lang) {
  const cards = document.querySelectorAll('.bible-card');
  cards.forEach(card => {
    if (!lang || card.dataset.language === lang) {
      card.style.display = '';
    } else {
      card.style.display = 'none';
    }
  });
}

// Highlight search terms in results
document.addEventListener('DOMContentLoaded', function() {
  const queryData = document.getElementById('search-query-data');
  const query = queryData ? queryData.dataset.query : '';
  if (!query) return;

  const searchTerm = query.replace(/^"|"$/g, '');
  if (!searchTerm) return;

  const results = document.querySelectorAll('.result-text');
  results.forEach(el => {
    const regex = new RegExp(`(${escapeRegex(searchTerm)})`, 'gi');
    el.innerHTML = el.innerHTML.replace(regex, '<mark>$1</mark>');
  });
});

function escapeRegex(string) {
  return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// Compare tab functionality
let bibles = [];
let currentBooks = [];
let currentChapter = null;
let currentVerses = [];

// Initialize bibles array from template data
function initBibles(biblesData) {
  bibles = biblesData || [];
}

// Load bibles from data attribute
function loadBiblesFromDataAttribute() {
  const biblesDataEl = document.getElementById('bibles-data');
  if (biblesDataEl && biblesDataEl.dataset.bibles) {
    try {
      bibles = JSON.parse('[' + biblesDataEl.dataset.bibles + ']');
    } catch (e) {
      console.error('Failed to parse bibles data:', e);
    }
  }
}

// Initialize compare tab
async function initCompare() {
  if (bibles.length > 0) {
    await loadBooks(bibles[0].id);
  }

  // Check URL params for compare state
  const params = new URLSearchParams(window.location.search);
  const ref = params.get('ref');
  const selectedBibles = params.get('bibles');

  if (selectedBibles) {
    selectedBibles.split(',').forEach(id => {
      const cb = document.querySelector(`input[name="bible"][value="${id}"]`);
      if (cb) cb.checked = true;
    });
  }

  if (ref) {
    parseAndLoadRef(ref);
  }
}

async function loadBooks(bibleId) {
  try {
    const resp = await fetch(`/api/bibles/${bibleId}`);
    const data = await resp.json();
    currentBooks = data.books || [];

    const select = document.getElementById('book-select');
    select.innerHTML = '<option value="">Select book...</option>';
    currentBooks.forEach(book => {
      const opt = document.createElement('option');
      opt.value = book.id;
      opt.textContent = book.name;
      opt.dataset.chapters = book.chapter_count;
      select.appendChild(opt);
    });
  } catch (e) {
    console.error('Failed to load books:', e);
  }
}

function updateChapters() {
  const bookSelect = document.getElementById('book-select');
  const chapterSelect = document.getElementById('chapter-select');
  const opt = bookSelect.options[bookSelect.selectedIndex];

  chapterSelect.innerHTML = '<option value="">Chapter</option>';
  if (opt && opt.dataset.chapters) {
    const count = parseInt(opt.dataset.chapters);
    for (let i = 1; i <= count; i++) {
      const o = document.createElement('option');
      o.value = i;
      o.textContent = i;
      chapterSelect.appendChild(o);
    }
  }
}

async function updateVerses() {
  const bookId = document.getElementById('book-select').value;
  const chapter = document.getElementById('chapter-select').value;

  if (!bookId || !chapter) return;

  const grid = document.getElementById('verse-grid');
  grid.innerHTML = '';

  const selected = getSelectedBibles();
  if (selected.length === 0) return;

  try {
    const resp = await fetch(`/api/bibles/${selected[0]}/${bookId}/${chapter}`);
    const data = await resp.json();
    currentVerses = data.verses || [];

    const allBtn = document.createElement('button');
    allBtn.textContent = 'All';
    allBtn.className = 'secondary';
    allBtn.addEventListener('click', () => loadChapter());
    grid.appendChild(allBtn);

    currentVerses.forEach(v => {
      const btn = document.createElement('button');
      btn.textContent = v.number;
      btn.className = 'outline';
      btn.addEventListener('click', () => loadVerse(v.number));
      grid.appendChild(btn);
    });
  } catch (e) {
    console.error('Failed to load verses:', e);
  }
}

function getSelectedBibles() {
  const checkboxes = document.querySelectorAll('input[name="bible"]:checked');
  return Array.from(checkboxes).map(cb => cb.value);
}

async function loadChapter() {
  const bookId = document.getElementById('book-select').value;
  const chapter = document.getElementById('chapter-select').value;
  const selected = getSelectedBibles();

  if (!bookId || !chapter || selected.length === 0) {
    document.getElementById('comparison-area').innerHTML = '<p class="meta">Select translations and a passage.</p>';
    return;
  }

  const area = document.getElementById('comparison-area');
  area.innerHTML = '<p>Loading...</p>';

  try {
    const promises = selected.map(id =>
      fetch(`/api/bibles/${id}/${bookId}/${chapter}`).then(r => r.json())
    );
    const results = await Promise.all(promises);

    let html = '';
    const maxVerses = Math.max(...results.map(r => (r.verses || []).length));

    for (let i = 0; i < maxVerses; i++) {
      const verseNum = i + 1;
      const book = currentBooks.find(b => b.id === bookId);
      const bookName = book ? book.name : bookId;

      html += `<div class="verse-group" id="v${verseNum}">`;
      html += `<div class="verse-ref">${bookName} ${chapter}:${verseNum}</div>`;

      results.forEach((r, idx) => {
        const verse = (r.verses || []).find(v => v.number === verseNum);
        const bible = bibles.find(b => b.id === selected[idx]);
        const abbrev = bible ? bible.abbrev : selected[idx];
        const text = verse ? verse.text : '<em>Verse not available</em>';

        html += `<div class="translation-row"><span class="translation-abbrev">${abbrev}:</span> <span class="verse-text">${text}</span></div>`;
      });

      html += '</div>';
    }

    area.innerHTML = html;
    updateCompareURL(bookId, chapter);

    if (document.getElementById('highlight-diff').checked) {
      highlightDifferences();
    }
  } catch (e) {
    area.innerHTML = `<p class="alert alert-error">Error loading: ${e.message}</p>`;
  }
}

async function loadVerse(verseNum) {
  const bookId = document.getElementById('book-select').value;
  const chapter = document.getElementById('chapter-select').value;
  const selected = getSelectedBibles();

  if (!bookId || !chapter || selected.length === 0) return;

  const area = document.getElementById('comparison-area');
  area.innerHTML = '<p>Loading...</p>';

  try {
    const promises = selected.map(id =>
      fetch(`/api/bibles/${id}/${bookId}/${chapter}`).then(r => r.json())
    );
    const results = await Promise.all(promises);

    const book = currentBooks.find(b => b.id === bookId);
    const bookName = book ? book.name : bookId;

    let html = `<div class="verse-group">`;
    html += `<div class="verse-ref">${bookName} ${chapter}:${verseNum}</div>`;

    results.forEach((r, idx) => {
      const verse = (r.verses || []).find(v => v.number === verseNum);
      const bible = bibles.find(b => b.id === selected[idx]);
      const abbrev = bible ? bible.abbrev : selected[idx];
      const text = verse ? verse.text : '<em>Verse not available</em>';

      html += `<div class="translation-row"><span class="translation-abbrev">${abbrev}:</span> <span class="verse-text">${text}</span></div>`;
    });

    html += '</div>';
    area.innerHTML = html;

    updateCompareURL(bookId, chapter, verseNum);

    if (document.getElementById('highlight-diff').checked) {
      highlightDifferences();
    }
  } catch (e) {
    area.innerHTML = `<p class="alert alert-error">Error: ${e.message}</p>`;
  }
}

function updateComparison() {
  const chapter = document.getElementById('chapter-select').value;
  if (chapter) {
    loadChapter();
  }
}

function updateCompareURL(book, chapter, verse) {
  const selected = getSelectedBibles();
  const ref = verse ? `${book}.${chapter}.${verse}` : `${book}.${chapter}`;
  const url = new URL(window.location);
  url.searchParams.set('tab', 'compare');
  url.searchParams.set('ref', ref);
  url.searchParams.set('bibles', selected.join(','));
  history.replaceState({}, '', url);
}

function parseAndLoadRef(ref) {
  const parts = ref.split('.');
  if (parts.length >= 2) {
    const bookId = parts[0];
    const chapter = parts[1];

    setTimeout(() => {
      const bookSelect = document.getElementById('book-select');
      for (let i = 0; i < bookSelect.options.length; i++) {
        if (bookSelect.options[i].value.toLowerCase() === bookId.toLowerCase()) {
          bookSelect.selectedIndex = i;
          updateChapters();

          const chapterSelect = document.getElementById('chapter-select');
          chapterSelect.value = chapter;
          updateVerses();

          setTimeout(() => {
            if (parts.length >= 3) {
              loadVerse(parseInt(parts[2]));
            } else {
              loadChapter();
            }
          }, 500);
          break;
        }
      }
    }, 500);
  }
}

function toggleHighlight() {
  if (document.getElementById('highlight-diff').checked) {
    highlightDifferences();
  } else {
    document.querySelectorAll('.highlight').forEach(el => {
      el.classList.remove('highlight');
    });
  }
}

function highlightDifferences() {
  const groups = document.querySelectorAll('.verse-group');
  groups.forEach(group => {
    const rows = group.querySelectorAll('.translation-row');
    if (rows.length < 2) return;

    const allWords = new Set();
    const rowWords = [];

    rows.forEach(row => {
      const textEl = row.querySelector('.verse-text');
      if (!textEl) return;
      const words = textEl.textContent.toLowerCase().split(/\s+/).filter(w => w.length > 2);
      rowWords.push(new Set(words));
      words.forEach(w => allWords.add(w));
    });

    rows.forEach((row, idx) => {
      const textEl = row.querySelector('.verse-text');
      if (!textEl || !rowWords[idx]) return;

      let html = textEl.innerHTML;
      rowWords[idx].forEach(word => {
        const isUnique = rowWords.every((ws, i) => i === idx || !ws.has(word));
        if (isUnique && word.length > 2) {
          const regex = new RegExp(`\\b(${word})\\b`, 'gi');
          html = html.replace(regex, '<span class="highlight">$1</span>');
        }
      });
      textEl.innerHTML = html;
    });
  });
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', function() {
  // Load bibles data from data attribute if available
  loadBiblesFromDataAttribute();

  // Attach event listeners for bible compare page
  const bibleCheckboxes = document.querySelectorAll('.bible-checkbox');
  bibleCheckboxes.forEach(checkbox => {
    checkbox.addEventListener('change', updateComparison);
  });

  const bookSelect = document.getElementById('book-select');
  if (bookSelect) {
    bookSelect.addEventListener('change', updateChapters);
  }

  const chapterSelect = document.getElementById('chapter-select');
  if (chapterSelect) {
    chapterSelect.addEventListener('change', updateVerses);
  }

  const loadChapterBtn = document.getElementById('load-chapter-btn');
  if (loadChapterBtn) {
    loadChapterBtn.addEventListener('click', loadChapter);
  }

  const highlightDiff = document.getElementById('highlight-diff');
  if (highlightDiff) {
    highlightDiff.addEventListener('change', toggleHighlight);
  }

  const params = new URLSearchParams(window.location.search);
  if (params.get('tab') === 'compare') {
    initCompare();
  }
});

// Also init if tab link is clicked
const tabCompareLink = document.getElementById('tab-compare-link');
if (tabCompareLink) {
  tabCompareLink.addEventListener('click', function(e) {
    // Allow navigation, but init on page load
  });
}
