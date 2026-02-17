// Bible Index and Compare Tab JavaScript
// Handles Browse tab filtering, Search tab highlighting, and Compare tab functionality

// Browse tab functionality
function filterByLanguage(lang) {
  const cards = document.querySelectorAll('.bible-card');
  for (const card of cards) {
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
  for (const el of results) {
    highlightSearchTermInElement(el, searchTerm);
  });
});

function escapeRegex(string) {
  return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// Escape HTML to prevent XSS
function escapeHtml(str) {
  if (str == null) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

// Safely highlights search terms using DOM manipulation instead of innerHTML
function highlightSearchTermInElement(element, searchTerm) {
  // ReDoS mitigation: limit input length
  if (!searchTerm || searchTerm.length > 100) {
    return;
  }

  const walker = document.createTreeWalker(
    element,
    NodeFilter.SHOW_TEXT,
    null,
    false
  );

  const nodesToReplace = [];
  let node = walker.nextNode();

  while (node) {
    nodesToReplace.push(node);
    node = walker.nextNode();
  }

  // ReDoS mitigation: wrap RegExp creation in try-catch
  let regex;
  try {
    regex = new RegExp(`(${escapeRegex(searchTerm)})`, 'gi');
  } catch (e) {
    console.error('Invalid search term for regex:', e);
    return;
  }

  nodesToReplace.forEach(textNode => {
    const text = textNode.nodeValue;

    // ReDoS mitigation: use simple string matching for basic containment check
    if (!text.toLowerCase().includes(searchTerm.toLowerCase())) {
      return;
    }

    const parts = text.split(regex);

    if (parts.length > 1) {
      const fragment = document.createDocumentFragment();
      parts.forEach((part, i) => {
        if (i % 2 === 1) {
          // Matched text - wrap in <mark>
          const mark = document.createElement('mark');
          mark.textContent = part;
          fragment.appendChild(mark);
        } else if (part) {
          // Non-matched text
          fragment.appendChild(document.createTextNode(part));
        }
      });
      textNode.parentNode.replaceChild(fragment, textNode);
    }
  });
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

      // Ensure verseNum is a number for defense-in-depth
      const safeVerseNum = parseInt(verseNum) || 0;

      html += `<div class="verse-group" id="v${safeVerseNum}">`;
      html += `<div class="verse-ref">${escapeHtml(bookName)} ${escapeHtml(chapter)}:${escapeHtml(safeVerseNum)}</div>`;

      results.forEach((r, idx) => {
        const verse = (r.verses || []).find(v => v.number === verseNum);
        const bible = bibles.find(b => b.id === selected[idx]);
        const abbrev = bible ? escapeHtml(bible.abbrev) : escapeHtml(selected[idx]);
        const text = verse ? escapeHtml(verse.text) : '&lt;em&gt;Verse not available&lt;/em&gt;';

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
    area.innerHTML = '<p class="alert alert-error">Error loading: </p>';
    const errorMsg = document.createTextNode(e.message);
    area.querySelector('p').appendChild(errorMsg);
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

    // Ensure verseNum is a number for defense-in-depth
    const safeVerseNum = parseInt(verseNum) || 0;

    let html = `<div class="verse-group">`;
    html += `<div class="verse-ref">${escapeHtml(bookName)} ${escapeHtml(chapter)}:${escapeHtml(safeVerseNum)}</div>`;

    results.forEach((r, idx) => {
      const verse = (r.verses || []).find(v => v.number === verseNum);
      const bible = bibles.find(b => b.id === selected[idx]);
      const abbrev = bible ? escapeHtml(bible.abbrev) : escapeHtml(selected[idx]);
      const text = verse ? escapeHtml(verse.text) : '&lt;em&gt;Verse not available&lt;/em&gt;';

      html += `<div class="translation-row"><span class="translation-abbrev">${abbrev}:</span> <span class="verse-text">${text}</span></div>`;
    });

    html += '</div>';
    area.innerHTML = html;

    updateCompareURL(bookId, chapter, verseNum);

    if (document.getElementById('highlight-diff').checked) {
      highlightDifferences();
    }
  } catch (e) {
    area.innerHTML = '<p class="alert alert-error">Error: </p>';
    const errorMsg = document.createTextNode(e.message);
    area.querySelector('p').appendChild(errorMsg);
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

      // Find unique words for this row
      const uniqueWords = Array.from(rowWords[idx]).filter(word => {
        const isUnique = rowWords.every((ws, i) => i === idx || !ws.has(word));
        return isUnique && word.length > 2;
      });

      if (uniqueWords.length === 0) return;

      // Use TreeWalker to safely highlight unique words in text nodes
      const walker = document.createTreeWalker(
        textEl,
        NodeFilter.SHOW_TEXT,
        null,
        false
      );

      const nodesToReplace = [];
      let node = walker.nextNode();

      while (node) {
        nodesToReplace.push(node);
        node = walker.nextNode();
      }

      nodesToReplace.forEach(textNode => {
        const text = textNode.nodeValue;
        const parts = [];
        let lastIndex = 0;
        let hasMatch = false;

        // ReDoS mitigation: limit word lengths and count
        const safeWords = uniqueWords.filter(w => w.length <= 50).slice(0, 100);
        if (safeWords.length === 0) return;

        // ReDoS mitigation: use simple string matching for basic containment check
        const textLower = text.toLowerCase();
        const wordsToHighlight = safeWords.filter(word => textLower.includes(word));
        if (wordsToHighlight.length === 0) return;

        // Find all matches in the text
        wordsToHighlight.forEach(word => {
          // ReDoS mitigation: wrap RegExp creation in try-catch
          try {
            const regex = new RegExp(`\\b${escapeRegex(word)}\\b`, 'gi');
            let match;
            regex.lastIndex = 0;
            let match = regex.exec(text);
            while (match !== null) {
              hasMatch = true;
              match = regex.exec(text);
            }
          } catch (e) {
            console.error('Invalid word for regex highlighting:', e);
          }
        });

        if (!hasMatch) return;

        // Build regex pattern for all unique words
        // ReDoS mitigation: wrap RegExp creation in try-catch
        let regex;
        try {
          const pattern = wordsToHighlight.map(w => `\\b(${escapeRegex(w)})\\b`).join('|');
          regex = new RegExp(pattern, 'gi');
        } catch (e) {
          console.error('Invalid pattern for regex highlighting:', e);
          return;
        }

        let match;
        const matches = [];

        while ((match = regex.exec(text)) !== null) {
          matches.push({ index: match.index, length: match[0].length, text: match[0] });
        }

        if (matches.length === 0) return;

        // Sort matches by index
        matches.sort((a, b) => a.index - b.index);

        // Build fragment with highlighted matches
        const fragment = document.createDocumentFragment();
        lastIndex = 0;

        matches.forEach(match => {
          // Add text before match
          if (match.index > lastIndex) {
            fragment.appendChild(document.createTextNode(text.substring(lastIndex, match.index)));
          }
          // Add highlighted match
          const span = document.createElement('span');
          span.className = 'highlight';
          span.textContent = match.text;
          fragment.appendChild(span);
          lastIndex = match.index + match.length;
        });

        // Add remaining text
        if (lastIndex < text.length) {
          fragment.appendChild(document.createTextNode(text.substring(lastIndex)));
        }

        textNode.parentNode.replaceChild(fragment, textNode);
      });
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
