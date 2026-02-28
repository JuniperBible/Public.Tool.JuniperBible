# STEPBible-Style Interface Implementation Charter

## Project Overview

This charter outlines the implementation of a comprehensive Bible study interface inspired by STEPBible.org, adapted to the Focus with Justin website's paper aesthetic theme (AirFold). The goal is to create an advanced, yet accessible Bible study tool that rivals professional Biblical software while maintaining the site's unique handwritten, paper-craft visual identity.

**Project Start Date:** 2025-12-30
**Status:** Phases 1-3 Complete, Phase 4+ In Planning
**Primary Goal:** Transform the existing basic Bible reader into a full-featured study interface with parallel translations, interlinear view, Strong's integration, cross-references, and search capabilities.

## Background

### Current State
The Focus with Justin website currently has:

- 8 Bible translations (KJV, Douay-Rheims, Geneva 1599, Vulgate, Tyndale, LXX, OSMHB, SBLGNT)
- Parallel translation comparison view (up to 4 translations)
- Bible search with Strong's number and phrase search
- Verse sharing with social media integration
- Strong's number detection and linking (via strongs.js)
- Book/chapter navigation dropdowns
- Versification support (KJV, Vulgate, LXX systems)
- Data-driven architecture using Hugo templates and JSON

### STEPBible Inspiration
STEPBible.org (developed by Tyndale House, Cambridge) is considered one of the cleanest and most powerful free Bible study tools available. Key strengths:

- Intuitive parallel and comparison interface
- Interlinear Hebrew/Greek with customizable word order
- Integrated Strong's numbers with occurrence counts
- Built-in cross-referencing system
- Gospel harmony and parallel passage reports
- Accessible directly in browser with no download required

**References:**

- [Bible Study Essentials: STEPBible Review](https://www.patheos.com/blogs/leadaquietlife/2025/04/bible-study-essentials-a-look-at-the-step-bible/)
- [STEPBible.org](https://www.stepbible.org/)
- [STEP User Guide FAQ](https://stepweb.atlassian.net/wiki/spaces/SUG/pages/8323078/Frequently+Asked+Questions)

## Project Goals

### Primary Objectives

1. **Parallel Translation View**: Display 2-4 Bible translations side-by-side for verse-by-verse comparison
2. **Interlinear Mode**: Show original Hebrew/Greek text aligned with English translation word-by-word
3. **Enhanced Strong's Integration**: Expand beyond links to show full definitions, occurrence counts, and related verses
4. **Cross-Reference System**: Display biblical cross-references and thematic connections
5. **Advanced Search**: Search across all translations with filters for original language, Strong's numbers, and morphology
6. **Verse Comparison Tool**: Deep-dive comparison of a single verse across all available translations
7. **Paper Aesthetic Compliance**: Ensure all new features maintain the handwritten, organic visual style

### Secondary Objectives

- Bookmark and highlight system for personal study notes
- Share verse feature with formatted citations
- Print-optimized layouts for study guides
- Mobile-responsive design with touch-friendly controls
- Offline-capable progressive web app (PWA) functionality

## Key Features to Implement

### Phase 1: Parallel Translation View (Foundation)

#### 1.1 Side-by-Side Translation Display
**Description:** Display multiple Bible translations simultaneously for easy comparison.

**Technical Approach:**

- Create new Hugo partial: `partials/bible-parallel.html`
- CSS Grid layout with 2-4 columns (responsive breakpoints)
- JavaScript component for dynamic translation selection
- URL parameter support: `?translations=kjv,drc,geneva1599`
- Synchronized scrolling across all translation columns

**UI/UX Considerations:**

- Wavy borders between translation columns (matching paper aesthetic)
- Handwritten font (Neucha/Patrick Hand) for all text
- Translation selector with checkboxes (max 4 selections)
- Verse numbers aligned across columns with accent color (#7a00b0)
- Mobile: Stack translations vertically with clear headers

**Data Requirements:**

- Leverage existing `bibles_auxiliary.json` structure
- Add parallel view metadata to `bibles.json`
- Create verse-aligned JSON structure for efficient loading

#### 1.2 Verse-Level Comparison Highlighting
**Description:** Highlight textual differences between translations.

**Technical Approach:**

- JavaScript text diff algorithm (word-level comparison)
- CSS classes for additions/omissions/variations
- Optional toggle to enable/disable highlighting
- Color-coded differences with paper-friendly palette

**UI/UX Considerations:**

- Soft pastel highlighting (not harsh yellow)
- Dotted underlines for variations
- Tooltip on hover explaining difference type
- Toggle button styled as paper checkbox

### Phase 2: Interlinear Hebrew/Greek View

#### 2.1 Interlinear Text Display
**Description:** Show original language text with word-by-word English gloss.

**Technical Approach:**

- Extend SWORD converter to extract Hebrew/Greek text with morphology
- Create interlinear data structure in JSON:
  ```json
  {
    "verse": "Gen.1.1",
    "words": [
      {
        "original": "בְּרֵאשִׁית",
        "transliteration": "bərēšîṯ",
        "strongs": "H7225",
        "morphology": "Prep-b | Noun-fs",
        "gloss": "In-beginning",
        "english": "In the beginning"
      }
    ]
  }
  ```

- Hugo partial: `partials/bible-interlinear.html`
- Two display modes:
  - Original language order (default)
  - English order (reverse interlinear)

**UI/UX Considerations:**

- Vertical word stacks: [Original] over [Transliteration] over [Gloss]
- Handwritten font for transliteration
- Hebrew/Greek fonts with good RTL support
- Toggle between Hebrew-first and English-first word order
- Color-coding for parts of speech (nouns, verbs, etc.) with paper palette

#### 2.2 Morphological Parsing Display
**Description:** Show grammatical information for each word.

**Technical Approach:**

- Parse morphology codes into human-readable labels
- Tooltip system for detailed grammar explanations
- Filter options: show only verbs, nouns, etc.

**UI/UX Considerations:**

- Compact morphology codes in subscript
- Expandable tooltips with full parsing (tense, voice, mood, etc.)
- Paper-style tooltip boxes with wavy borders
- Educational hover states for learning grammar

### Phase 3: Enhanced Strong's Numbers Integration

#### 3.1 Inline Strong's Definitions
**Description:** Expand current Strong's linking to show full definitions inline.

**Technical Approach:**

- Create `data/strongs_lexicon.json` with:
  - Hebrew (H1-H8674) and Greek (G1-G5624) definitions
  - Short gloss + full definition
  - Root words and cognates
  - Transliteration and pronunciation
- Enhance `strongs.js` to fetch from local JSON instead of external links
- Fallback to Blue Letter Bible for detailed study

**Data Sources:**

- Open Scriptures Hebrew/Greek Lexicon (CC BY-SA 4.0)
- Strong's original public domain content
- Cross-reference with STEPBible's open data

**UI/UX Considerations:**

- Improved tooltip design with paper aesthetic
- Two-level system: hover for quick gloss, click for full definition
- Definition modal with wavy border frame
- Related verses section showing other uses

#### 3.2 Strong's Concordance View
**Description:** Show all occurrences of a Strong's number across Scripture.

**Technical Approach:**

- Pre-index all Strong's numbers during build
- Create concordance JSON: `data/strongs_concordance.json`
- Search page: `/religion/concordance/?strongs=H430`
- Grouped by Testament, then by book

**UI/UX Considerations:**

- List view with verse snippets
- Highlight the Strong's word in each verse
- Filter by book, translation, or Testament
- Occurrence count badge (paper-style pill)

### Phase 4: Cross-Reference System

#### 4.1 Treasury of Scripture Knowledge Integration
**Description:** Display biblical cross-references for each verse.

**Technical Approach:**

- Integrate Treasury of Scripture Knowledge (TSK) data (public domain)
- Create `data/cross_references.json`:
  ```json
  {
    "Gen.1.1": [
      {
        "verse": "John.1.1",
        "type": "parallel",
        "note": "Creation parallel in Gospel"
      },
      {
        "verse": "Heb.11.3",
        "type": "commentary",
        "note": "Faith and creation"
      }
    ]
  }
  ```

- Display in sidebar or expandable section per verse

**UI/UX Considerations:**

- Cross-reference icon (⨁) next to verses with references
- Expandable panel styled as paper note card
- Click to navigate to reference verse
- Categorize: direct quotes, allusions, thematic parallels, commentary

#### 4.2 Parallel Passage Reports
**Description:** Gospel harmony and OT/NT parallel passage viewing.

**Technical Approach:**

- Pre-compile parallel passage data:
  - Synoptic Gospels (Matthew, Mark, Luke parallel events)
  - Samuel/Kings vs Chronicles parallels
  - Psalms quoted in NT
- Special layout: `layouts/religion/parallel-report.html`
- URL: `/religion/reports/gospel-harmony/`

**UI/UX Considerations:**

- Multi-column layout for synoptic comparison
- Event-based navigation (not chapter-based)
- Highlight variations between accounts
- Paper-style section dividers for different events

### Phase 5: Advanced Search Functionality

#### 5.1 Multi-Translation Search
**Description:** Search across all Bible translations with advanced filters.

**Technical Approach:**

- Lunr.js or Pagefind for client-side search indexing
- Build search index during Hugo build
- Search page: `/religion/search/`
- Query parameters:
  - `q` - search term
  - `translations` - comma-separated translation IDs
  - `testament` - OT, NT, or both
  - `books` - comma-separated book IDs
  - `strongs` - Strong's number filter
  - `exact` - exact phrase vs fuzzy match

**UI/UX Considerations:**

- Clean search interface with paper-style input field
- Advanced options in expandable accordion
- Results grouped by book or by translation
- Verse snippet with keyword highlighting
- Result count and search time display

#### 5.2 Original Language Search
**Description:** Search Hebrew/Greek text and Strong's numbers.

**Technical Approach:**

- Separate search index for original language words
- Transliteration search support (e.g., "logos" finds λόγος)
- Strong's number wildcard search (H* for all Hebrew)

**UI/UX Considerations:**

- Virtual Greek/Hebrew keyboard for input
- Transliteration auto-conversion
- Morphology filter sidebar (tense, mood, voice, etc.)
- Results show original text + translation

### Phase 6: Verse Comparison Tool

#### 6.1 Single Verse Deep Comparison
**Description:** Compare one verse across all available translations.

**Technical Approach:**

- Dedicated page: `/religion/compare/?verse=John.3.16`
- Display all translations in a table or card grid
- Highlight textual variants
- Show translation philosophy notes (formal vs dynamic equivalence)

**UI/UX Considerations:**

- Visual hierarchy: most literal → most dynamic
- Translation year and tradition badges (Protestant, Catholic, Orthodox)
- Expandable notes on translation decisions
- Copy verse button for each translation

#### 6.2 Word Study Integration
**Description:** Link verse comparison to Strong's word study.

**Technical Approach:**

- Click on any word in any translation
- Show original Greek/Hebrew word
- Display all translation choices for that word
- Link to full concordance

**UI/UX Considerations:**

- Word-level click handlers
- Highlight all instances of the same original word in the verse
- Side panel for word study details
- Compare translation footnotes

### Phase 7: Paper Aesthetic UI Components

#### 7.1 Design System Components
Create reusable components matching the AirFold theme:

**Components to Build:**

- `bible-toolbar.html` - Translation selector, view mode toggle, search
- `bible-verse-card.html` - Single verse display card
- `bible-word-study-modal.html` - Strong's popup with paper frame
- `bible-cross-ref-panel.html` - Cross-reference sidebar
- `bible-parallel-grid.html` - Responsive parallel columns
- `bible-interlinear-stack.html` - Original + gloss word stack

**CSS Classes (extend main.css):**
```css
/* Bible Study Interface Components */
.bible-toolbar {
  /* Paper-style toolbar with wavy border */
}

.bible-parallel-column {
  /* Column for parallel view */
}

.bible-interlinear-word {
  /* Original language word stack */
}

.bible-strongs-tooltip {
  /* Enhanced Strong's popup */
}

.bible-cross-ref {
  /* Cross-reference link styling */
}

.bible-search-result {
  /* Search result card */
}

.bible-comparison-table {
  /* Verse comparison table */
}
```

#### 7.2 Typography and Color Palette
**Fonts:**

- Primary: Neucha (handwritten, informal)
- Secondary: Patrick Hand (handwritten, slightly more formal)
- Original Language: SBL Hebrew, SBL Greek (biblical scholarship standard)

**Colors (from hugo.toml):**

- Accent: `#7a00b0` (purple) - for Strong's numbers, links, highlights
- Paper White: `#b5a48e` (cream background)
- Paper Bright: `#e6ddc0` (content background)
- Paper Black: `#2c2416` (text)
- Paper Gray: `#5c5347` (secondary text)
- Paper Border: `#3d3428` (borders, dividers)

**Dark Mode:** Full support per existing color variables in main.css

#### 7.3 Animation and Interaction
**Subtle Paper Effects:**

- Wavy border animations on hover
- Gentle shadow offsets for depth
- Smooth transitions (200ms ease-out)
- Touch-friendly targets (44px minimum)

**Accessibility:**

- ARIA labels for all interactive elements
- Keyboard navigation (Tab, Enter, Escape)
- Screen reader announcements for dynamic content
- Focus indicators with paper aesthetic

### Phase 8: Mobile Optimization

#### 8.1 Responsive Layouts
**Breakpoints (Tailwind):**

- Mobile: < 640px (stack all columns, simplified toolbar)
- Tablet: 640px - 1024px (2-column parallel, compact interlinear)
- Desktop: > 1024px (3-4 column parallel, full interlinear)

**Mobile-Specific Features:**

- Swipe gestures for chapter navigation (via Hammer.js)
- Bottom navigation bar for common actions
- Collapsible sections for cross-references
- Simplified Strong's tooltips

#### 8.2 Progressive Web App (PWA)
**Offline Capability:**

- Service worker for caching Bible data
- Offline-first architecture
- Background sync for bookmarks
- Install prompt for home screen

**Performance:**

- Lazy load Bible chapters (load on scroll)
- Compress JSON with gzip/brotli
- Code-split JavaScript by feature
- Image optimization for translation logos

## Technical Architecture

### Data Layer

#### JSON Data Structure
```
data/
├── bibles.json                      # Translation metadata (existing)
├── bibles_auxiliary.json            # Chapter/verse content (existing)
├── bibles_interlinear.json          # Hebrew/Greek with morphology (NEW)
├── strongs_lexicon.json             # Strong's definitions (NEW)
├── strongs_concordance.json         # Strong's verse index (NEW)
├── cross_references.json            # TSK cross-references (NEW)
├── parallel_passages.json           # Gospel harmony, etc. (NEW)
└── translation_notes.json           # Translation philosophy notes (NEW)
```

#### Hugo Template Layer
```
layouts/religion/
├── parallel.html                    # Parallel translation view (NEW)
├── interlinear.html                 # Interlinear view (NEW)
├── search.html                      # Search results page (NEW)
├── compare.html                     # Verse comparison (NEW)
├── concordance.html                 # Strong's concordance (NEW)
└── parallel-report.html             # Gospel harmony (NEW)

partials/
├── bible-toolbar.html               # Main navigation toolbar (NEW)
├── bible-parallel-grid.html         # Parallel columns component (NEW)
├── bible-interlinear-stack.html     # Interlinear word display (NEW)
├── bible-strongs-enhanced.html      # Enhanced Strong's popup (NEW)
├── bible-cross-ref-panel.html       # Cross-reference sidebar (NEW)
└── bible-search-form.html           # Search interface (NEW)
```

#### JavaScript Modules
```
assets/js/
├── strongs.js                       # Strong's tooltips (existing)
├── bible-parallel.js                # Parallel view controller (NEW)
├── bible-interlinear.js             # Interlinear display logic (NEW)
├── bible-search.js                  # Search functionality (NEW)
├── bible-comparison.js              # Verse comparison (NEW)
├── bible-sync-scroll.js             # Synchronized scrolling (NEW)
└── bible-bookmarks.js               # Bookmark system (NEW)
```

### Build Process Integration

#### SWORD Converter Enhancements
Extend `tools/juniper/` to extract:

1. Hebrew/Greek text with Strong's numbers
2. Morphological parsing codes
3. Interlinear word alignment data
4. Cross-reference markers from modules

**New Converter Commands:**
```bash
# Extract interlinear data
./juniper interlinear --modules KJV,OSHB,SBLGNT --output ../../data/

# Extract Strong's lexicon
./juniper lexicon --modules StrongsHebrew,StrongsGreek --output ../../data/

# Extract cross-references
./juniper cross-refs --modules TSK --output ../../data/
```

#### Hugo Build Pipeline
```bash
npm run build
├─ npm run sword:interlinear   # Extract interlinear data (NEW)
├─ npm run sword:lexicon       # Extract Strong's lexicon (NEW)
├─ npm run sword:cross-refs    # Extract cross-references (NEW)
├─ npm run search:index        # Build search index (NEW)
└─ hugo build                  # Generate site
```

### Performance Considerations

#### Data Optimization

- **Chunking:** Split large JSON files by book (66 files vs 1 giant file)
- **Compression:** Use Brotli compression for JSON (reduce 30MB → 3MB)
- **Caching:** Service worker cache strategy (cache-first for Bible data)
- **Lazy Loading:** Load chapters on demand, not entire Bible upfront

#### Rendering Optimization

- **Virtual Scrolling:** Only render visible verses (windowing)
- **Debounced Search:** Wait 300ms after typing before searching
- **Web Workers:** Run search indexing in background thread
- **Code Splitting:** Load parallel/interlinear modules on-demand

## Implementation Phases

### Phase 1: Foundation ✓ COMPLETE (2025-12-30)
**Goal:** Establish parallel translation view infrastructure

**Completed:**

- [x] Design parallel view layout (mobile + desktop wireframes)
- [x] Create `layouts/religion/compare.html` with responsive CSS Grid
- [x] Implement translation selector UI (checkboxes, max 4)
- [x] Add URL parameter support (?bibles=kjv,drc&ref=Gen.1)
- [x] Build synchronized scrolling JavaScript (`parallel.js`)
- [x] Test with 8 translations (KJV, DRC, Geneva, Vulgate, Tyndale, LXX, OSMHB, SBLGNT)
- [x] Mobile optimization and touch gestures

**Deliverable:** Working 2-4 column parallel Bible reader at `/religion/bibles/compare/`

### Phase 2: Search Functionality ✓ COMPLETE (2025-12-30)
**Goal:** Multi-translation search with Strong's support

**Completed:**

- [x] Create `layouts/religion/search.html` search page
- [x] Create `assets/js/bible-search.js` with on-demand chapter fetching
- [x] Implement search with case-sensitive and whole-word options
- [x] Add Strong's number search (H#### or G####)
- [x] Add phrase search with quotation marks
- [x] URL parameter persistence (?q=word&bible=kjv)
- [x] Highlighted matches in results
- [x] Search link from Bibles list page
- [x] i18n strings for search UI

**Deliverable:** Full-featured Bible search at `/religion/bibles/search/`

### Phase 3: Share Verse Feature ✓ COMPLETE (2025-12-30)
**Goal:** Social sharing and verse linking

**Completed:**

- [x] Create share button component (`share.js`)
- [x] Generate shareable URLs (`/religion/bibles/kjv/gen/1/?v=1`)
- [x] Copy to clipboard functionality with visual feedback
- [x] Social media share links (Twitter/X, Facebook)
- [x] Scroll to verse when URL has ?v= parameter
- [x] CSS for share buttons and menu

**Deliverable:** Working verse sharing with social integration

### Phase 4: Interlinear View (Future)
**Goal:** Display Hebrew/Greek with word-by-word gloss

**Subtasks:**

1. Extend SWORD converter for Hebrew/Greek extraction
2. Extract OSHB (Open Scriptures Hebrew Bible) module data
3. Extract SBLGNT (SBL Greek New Testament) module data
4. Create `bibles_interlinear.json` data structure
5. Build `bible-interlinear-stack.html` component
6. Implement word order toggle (original vs English)
7. Add RTL support for Hebrew text
8. Design and style morphology tooltips
9. Test with multiple chapters, verify Unicode handling

**Deliverable:** Functional interlinear view for OT and NT

### Phase 5: Enhanced Strong's Numbers (Future)
**Goal:** Full Strong's dictionary with concordance

**Subtasks:**

1. Source Strong's lexicon data (Open Scriptures, public domain)
2. Create `strongs_lexicon.json` (definitions, transliterations)
3. Build concordance index (`strongs_concordance.json`)
4. Enhance `strongs.js` for local lexicon lookup
5. Design improved tooltip with paper aesthetic
6. Create Strong's concordance page layout
7. Add occurrence count badges
8. Implement related verses section
9. Test with high-frequency words (H430 "God", G2316 "theos")

**Deliverable:** Complete Strong's integration with local definitions

### Phase 6: Cross-References (Future)
**Goal:** Display biblical cross-references using TSK

**Subtasks:**

1. Obtain Treasury of Scripture Knowledge (TSK) data
2. Parse TSK into `cross_references.json` format
3. Create `bible-cross-ref-panel.html` component
4. Add cross-reference icons to verse display
5. Implement expandable reference cards
6. Categorize references (quote, allusion, parallel, commentary)
7. Build parallel passage reports (Gospel harmony)
8. Create synoptic Gospel comparison layout
9. Test with heavily cross-referenced passages (Isa 53, John 1)

**Deliverable:** Working cross-reference system with parallel passage reports

### Phase 7: Advanced Search (Future)
**Goal:** Multi-translation search with filters

**Subtasks:**

1. Evaluate search libraries (Lunr.js vs Pagefind vs Fuse.js)
2. Build search index during Hugo build
3. Create `/religion/search/` page layout
4. Implement search form with paper aesthetic
5. Add advanced filter UI (translation, book, testament)
6. Build search results display (grouped by book/translation)
7. Implement keyword highlighting in results
8. Add original language search capability
9. Create transliteration auto-conversion
10. Test search performance with large result sets

**Deliverable:** Full-featured Bible search engine

### Phase 8: Critical Comparison Highlighter (Future)
**Goal:** Deep-dive single verse comparison tool

**Subtasks:**

1. Design comparison page layout (`/religion/compare/`)
2. Build verse comparison table/grid
3. Implement textual variant highlighting
4. Add translation philosophy badges
5. Create translation notes display
6. Implement word-level click handlers
7. Link comparison to Strong's word study
8. Add copy verse buttons (all formats)
9. Create print-optimized layout
10. Test with complex verses (John 1:1, Rom 3:23)

**Deliverable:** Comprehensive verse comparison tool

### Phase 9: Polish & Optimization (Future)
**Goal:** Mobile optimization, PWA, accessibility

**Subtasks:**

1. Comprehensive mobile testing (iOS, Android)
2. Implement touch gestures (swipe navigation)
3. Build service worker for offline capability
4. Create PWA manifest and install prompt
5. Optimize JSON compression and chunking
6. Implement virtual scrolling for long chapters
7. Code-split JavaScript modules
8. ARIA label audit and screen reader testing
9. Keyboard navigation testing (Tab, Enter, Esc)
10. Performance audit (Lighthouse score 95+)

**Deliverable:** Production-ready, accessible, performant interface

### Phase 10: Documentation & Launch (Future)
**Goal:** User documentation and public release

**Subtasks:**

1. Create user guide (`/religion/help/`)
2. Write tutorial articles (blog posts on usage)
3. Record demo videos (parallel view, interlinear, search)
4. Create JSON schema files for new data structures
5. Write developer documentation for future maintainers
6. Soft launch to beta testers
7. Collect feedback and address bugs
8. Public announcement (blog post, social media)
9. Monitor analytics and user feedback

**Deliverable:** Fully documented, publicly launched Bible study interface

## UI/UX Design Principles

### 1. Paper Aesthetic Consistency
**Maintain the handwritten, organic feel throughout:**

- Wavy borders on all containers (using SVG clip-path)
- Offset box shadows (4px 4px) for depth, not floating blur shadows
- Handwritten fonts (Neucha, Patrick Hand) for all UI text
- Cream/brown color palette (avoid harsh white or black)
- Subtle texture overlays on backgrounds

**Avoid:**

- Sharp rectangular boxes
- Modern flat design aesthetics
- High-contrast colors
- Sans-serif fonts
- Gradients or glowing effects

### 2. Hierarchical Information Display
**Progressive disclosure of complexity:**

- Level 1: Simple Bible reading (current functionality)
- Level 2: Parallel translation comparison (visible option)
- Level 3: Interlinear and Strong's (one click away)
- Level 4: Cross-references and word study (expandable panels)
- Level 5: Advanced search and reports (dedicated pages)

**User can choose their depth:** Beginners see clean text, scholars see rich tools.

### 3. Contextual Tooltips and Help
**Educate users without overwhelming:**

- Hover tooltips explain features on first use
- "?" help icons with paper-style help cards
- Inline examples in search filters ("e.g., John 3:16")
- Tutorial mode: highlight features in sequence
- Dismissible tips that don't return after acknowledged

### 4. Mobile-First, Desktop-Enhanced
**Design for mobile, enhance for desktop:**

- Mobile: Single column, bottom toolbar, swipe gestures
- Tablet: Two columns, side toolbar, touch + mouse
- Desktop: 3-4 columns, full toolbar, keyboard shortcuts

**Touch targets:** Minimum 44px × 44px for fingers

### 5. Accessibility as Core Feature
**Not an afterthought:**

- ARIA labels on all interactive elements
- Keyboard navigation paths clearly defined
- Screen reader announcements for dynamic content
- High contrast mode support (dark mode compliance)
- Focus indicators styled with paper aesthetic (not default outline)

## Success Metrics

### User Engagement

- **Time on Page:** Increase from 2 min → 10 min average
- **Pages per Session:** Increase from 1.5 → 5+ pages
- **Return Visitors:** 40%+ within 30 days
- **Feature Adoption:** 30%+ use parallel view, 20%+ use interlinear

### Technical Performance

- **Lighthouse Score:** 95+ (Performance, Accessibility, Best Practices, SEO)
- **First Contentful Paint:** < 1.5s
- **Time to Interactive:** < 3.5s
- **Total Page Weight:** < 2MB initial load, < 500KB per chapter

### Quality Metrics

- **Accessibility:** WCAG 2.1 AA compliant (AAA where possible)
- **Browser Support:** 95%+ of users (Chrome, Firefox, Safari, Edge)
- **Mobile Responsiveness:** Works on 320px width screens
- **Test Coverage:** 90%+ for JavaScript modules

## Risk Assessment

### Technical Risks

**Risk 1: Data Volume**
*Issue:* Full interlinear Bible with Strong's data could exceed 100MB JSON.
*Mitigation:* Chunk by book, compress with Brotli, lazy load chapters.
*Likelihood:* High | *Impact:* High

**Risk 2: Search Performance**
*Issue:* Client-side search on 31,000+ verses may be slow on mobile.
*Mitigation:* Use Web Workers, implement pagination, cache results.
*Likelihood:* Medium | *Impact:* Medium

**Risk 3: Browser Compatibility**
*Issue:* Advanced features (service workers, CSS Grid) not on old browsers.
*Mitigation:* Progressive enhancement, feature detection, graceful fallbacks.
*Likelihood:* Low | *Impact:* Medium

### Content Risks

**Risk 4: Copyright/Licensing**
*Issue:* Some Strong's lexicons or cross-references may have unclear licensing.
*Mitigation:* Use only public domain or CC BY-SA sources, document provenance.
*Likelihood:* Low | *Impact:* High

**Risk 5: Data Accuracy**
*Issue:* Errors in morphology parsing or Strong's number alignment.
*Mitigation:* Validate against multiple sources, implement user reporting.
*Likelihood:* Medium | *Impact:* Medium

### UX Risks

**Risk 6: Feature Overload**
*Issue:* Too many features overwhelm casual users.
*Mitigation:* Progressive disclosure, clear UI hierarchy, tutorial mode.
*Likelihood:* Medium | *Impact:* Medium

**Risk 7: Mobile Complexity**
*Issue:* Advanced features don't translate well to small screens.
*Mitigation:* Mobile-specific simplified views, prioritize core features.
*Likelihood:* High | *Impact:* Medium

## Dependencies

### External Data Sources

- **Open Scriptures Hebrew Bible (OSHB):** CC BY 4.0 license
- **SBL Greek New Testament (SBLGNT):** Free for non-commercial use
- **Strong's Lexicon:** Public domain (original 1890 edition)
- **Treasury of Scripture Knowledge (TSK):** Public domain
- **Open Scriptures Greek Lexicon:** CC BY-SA 4.0

### Software Libraries

- **Hugo:** Static site generator (v0.120+)
- **Tailwind CSS v4:** Styling framework (already in use)
- **Lunr.js / Pagefind:** Client-side search indexing
- **Hammer.js:** Touch gesture library (already in use)
- **Font: SBL BibLit:** Hebrew/Greek fonts (Open Font License)

### Development Tools

- **Go:** For SWORD converter enhancements (v1.21+)
- **Python:** For data processing scripts (v3.9+)
- **SQLite:** For SWORD module database parsing
- **Node.js:** For build scripts and npm (v18+)

## Open Questions

1. **Search Strategy:** Should we pre-build search index (larger download) or index on-demand (slower first search)?
   - *Recommendation:* Pre-build for popular translations (KJV, ESV), on-demand for others

2. **Interlinear Scope:** Include every word or only key theological terms?
   - *Recommendation:* Full OT/NT interlinear, but make it toggleable per verse

3. **Mobile Interlinear:** How to display complex word stacks on 320px screens?
   - *Recommendation:* Horizontal scroll per verse, tap word for popup details

4. **Offline Data:** Download entire Bible or selected books only?
   - *Recommendation:* Smart caching: download viewed chapters + user's favorite translation

5. **Community Features:** Should we add shared bookmarks or study notes?
   - *Recommendation:* Phase 9 consideration, requires backend infrastructure

## Future Expansion (Beyond Charter Scope)

### Additional Bible Translations

- Modern translations (ESV, NIV, NASB) - requires licensing negotiations
- Non-English translations (Spanish, French, German, Chinese)
- Apocryphal/Deuterocanonical books (Catholic canon support)

### Advanced Study Tools

- Topical Bible index (Nave's Topical Bible)
- Bible dictionaries (ISBE, Easton's)
- Commentaries (Matthew Henry, Gill's Exposition)
- Maps and timelines (biblical geography, chronology)

### Collaboration Features

- User accounts for synced bookmarks across devices
- Public study notes and highlighting
- Group Bible study rooms (real-time collaboration)
- Sermon prep workspace

### AI Integration

- Semantic search ("verses about faith and works")
- Passage summarization
- Greek/Hebrew word study assistant
- Translation comparison explanations

## Conclusion

This charter outlines an ambitious but achievable roadmap to transform the Focus with Justin Bible section from a basic reader into a world-class study tool. By drawing inspiration from STEPBible's clean interface and powerful features, while maintaining the unique paper aesthetic of the AirFold theme, we can create a distinctive Bible study experience that serves both casual readers and serious students of Scripture.

The phased approach allows for incremental delivery of value, with each phase building on the previous foundation. The emphasis on data-driven architecture, progressive enhancement, and accessibility ensures the tool will be usable and performant for the widest possible audience.

Success will be measured not just in technical metrics, but in the depth and quality of Bible engagement enabled by these tools. The goal is to make serious biblical scholarship accessible and delightful, wrapped in the warm, inviting aesthetic of a handwritten journal.

---

**Charter Approved By:** Justin
**Last Updated:** 2026-01-01
**Next Review Date:** After Phase 4 completion (Interlinear View)
