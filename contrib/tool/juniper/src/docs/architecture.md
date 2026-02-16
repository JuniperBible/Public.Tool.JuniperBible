# Architecture Guide

This document provides a comprehensive overview of the Focus with Justin website architecture, including system design, data patterns, theme structure, and technical implementation details.

## Table of Contents

1. [System Overview](#system-overview)
2. [Directory Structure](#directory-structure)
3. [Theme Architecture](#theme-architecture)
4. [Data-Driven Content](#data-driven-content)
5. [Module Mounts](#module-mounts)
6. [Extension System](#extension-system)
7. [Security Architecture](#security-architecture)
8. [Build Process](#build-process)
9. [JavaScript Components](#javascript-components)
10. [Self-Hosting Strategy](#self-hosting-strategy)
11. [Internationalization](#internationalization)
12. [Performance Optimization](#performance-optimization)

---

## System Overview

The Focus with Justin website is a static site built with Hugo and deployed on Cloudflare Pages. It uses a custom theme (AirFold) with a paper aesthetic design, Tailwind CSS v4 for styling, and a data-driven content architecture for structured content types.

### Technology Stack

| Layer | Technology | Purpose |
|-------|------------|---------|
| Static Site Generator | Hugo (extended) v0.128.0+ | Page generation, templating |
| CSS Framework | Tailwind CSS v4 | Utility-first styling |
| Fonts | Neucha, Patrick Hand | Handwritten paper aesthetic |
| Hosting | Cloudflare Pages | CDN, edge deployment |
| Functions | Cloudflare Pages Functions | Contact form API |
| Email | Cloudflare Workers + Email Routing | Email delivery |
| CAPTCHA | Cloudflare Turnstile | Bot protection |
| Touch Gestures | Hammer.js | Mobile interactions |
| Encryption | OpenPGP.js | Contact form encryption |

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                           User's Browser                             │
├─────────────────────────────────────────────────────────────────────┤
│  Static Assets (HTML/CSS/JS)  │  API Requests  │  Email Submission  │
└───────────────┬───────────────┴───────┬────────┴─────────┬──────────┘
                │                       │                  │
                ▼                       ▼                  ▼
┌───────────────────────┐  ┌────────────────────┐  ┌──────────────────┐
│   Cloudflare CDN      │  │   Pages Function   │  │   Email Worker   │
│   (static hosting)    │  │   /api/contact     │  │   (via binding)  │
└───────────────────────┘  └────────────────────┘  └──────────────────┘
                                    │                       │
                                    │ CAPTCHA verify        │ Email Routing
                                    ▼                       ▼
                           ┌────────────────┐      ┌────────────────┐
                           │   Turnstile    │      │   Inbox        │
                           └────────────────┘      └────────────────┘
```

---

## Directory Structure

```
.
├── archetypes/                    # Content templates for hugo new
│   ├── certifications.md          # Certification page template
│   ├── default.md                 # Default page template
│   ├── esoterica.md               # Blog post template
│   ├── projects.md                # Project page template
│   ├── skills.md                  # Skill page template
│   └── tools.md                   # Tool page template
├── assets/
│   └── css/main.css               # Tailwind CSS source (site override)
├── content/                       # Markdown content files
│   ├── about.md                   # About page
│   ├── contact/                   # Contact section
│   │   ├── _index.md              # Contact page
│   │   └── thank-you.md           # Thank you page
│   ├── esoterica/                 # Blog posts and articles
│   │   ├── _index.md              # Blog index
│   │   └── *.md                   # Individual posts
│   ├── projects/                  # Portfolio projects
│   │   ├── _index.md              # Projects index
│   │   └── *.md                   # Individual projects
│   ├── religion/                  # Religion section
│   │   └── _index.md              # Religion index
│   └── resume.md                  # Resume page
├── data/                          # JSON data files
│   ├── bibles.json                # Bible translation metadata
│   ├── bibles_auxiliary/          # Bible content (per-translation)
│   │   ├── kjv.json               # King James Version (~5MB)
│   │   ├── drc.json               # Douay-Rheims (~5MB)
│   │   ├── geneva1599.json        # Geneva Bible (~5MB)
│   │   ├── vulgate.json           # Latin Vulgate (~5MB)
│   │   └── tyndale.json           # Tyndale Bible (~5MB)
│   ├── book_mappings.json         # Canonical status per tradition
│   ├── certifications.json        # Certification metadata
│   ├── certifications_auxiliary.json
│   ├── resume.json                # Resume data (JSON Resume v1.0.0)
│   ├── skills.json                # Skills metadata
│   ├── skills_auxiliary.json
│   ├── social.yaml                # Social media links
│   ├── tools.json                 # Tools metadata
│   └── tools_auxiliary.json
├── docs/                          # Project documentation
│   ├── README.md                  # Documentation index
│   ├── architecture.md            # This file
│   ├── configuration.md           # hugo.toml reference
│   ├── contact-form.md            # Contact form guide
│   ├── current-project-charter.md # Project status and backlog
│   ├── data_structures.md         # SWORD binary format
│   ├── deployment.md              # Deployment guide
│   ├── development.md             # Development guide
│   ├── religion-section.md        # Religion section guide
│   ├── stepbible-interface-charter.md # Bible UI roadmap
│   └── testing.md                 # Testing guide
├── functions/                     # Cloudflare Pages Functions
│   └── api/
│       └── contact.js             # Contact form handler
├── i18n/                          # Internationalization
│   └── en.toml                    # English UI strings (80+ keys)
├── layouts/                       # Site-specific layout overrides
│   ├── _default/
│   │   └── resume.html            # Resume page layout
│   ├── esoterica/                 # Blog layouts
│   │   ├── list.html
│   │   └── single.html
│   ├── partials/                  # Site-specific partials
│   │   ├── bible-nav.html
│   │   ├── canonical-comparison.html
│   │   ├── translation-selector.html
│   │   └── ...
│   ├── projects/                  # Project layouts
│   ├── religion/                  # Religion section layouts
│   │   ├── compare.html           # Parallel translation view
│   │   ├── list.html              # Bible list
│   │   ├── search.html            # Bible search
│   │   └── single.html            # Chapter view
│   └── resume/                    # Resume sub-sections
│       ├── certifications/
│       ├── skills/
│       └── tools/
├── static/                        # Static assets
│   ├── css/                       # Compiled CSS
│   ├── fonts/                     # Self-hosted fonts
│   │   ├── Neucha/
│   │   └── PatrickHand/
│   ├── images/                    # Images and logos
│   ├── js/                        # JavaScript files
│   │   ├── site.js                # Theme toggle, mobile menu
│   │   ├── lightbox.js            # Table/chart enlargement
│   │   ├── strongs.js             # Strong's number linking
│   │   ├── parallel.js            # Parallel translation
│   │   ├── bible-search.js        # Bible search
│   │   ├── share.js               # Verse sharing
│   │   ├── openpgp.min.js         # PGP encryption
│   │   ├── hammer.min.js          # Touch gestures
│   │   └── mermaid.min.js         # Diagrams
│   └── schemas/                   # JSON schema files
│       ├── bibles.schema.json
│       └── bibles-auxiliary.schema.json
├── themes/airfold/                # AirFold theme
│   ├── assets/css/main.css        # Theme CSS source
│   ├── extensions/                # Optional feature extensions
│   │   ├── certifications/
│   │   ├── portfolio/
│   │   ├── religion/
│   │   └── resume/
│   ├── layouts/                   # Theme layouts
│   │   ├── _default/
│   │   ├── partials/
│   │   ├── about.html
│   │   ├── contact.html
│   │   └── term.html
│   └── static/                    # Theme static assets
├── tools/                         # Development tools
│   └── juniper/           # Go CLI for Bible extraction
│       ├── cmd/                   # CLI commands
│       ├── pkg/                   # Go packages
│       │   ├── config/            # YAML configuration
│       │   ├── cgo/               # libsword bindings (test only)
│       │   ├── esword/            # e-Sword SQLite parsers
│       │   ├── markup/            # OSIS/ThML/GBF/TEI converters
│       │   ├── migrate/           # Module migration
│       │   ├── output/            # JSON generation
│       │   ├── sword/             # SWORD binary parsers
│       │   └── testing/           # Test framework
│       ├── versification/         # Canon YAML files
│       └── README.md
├── workers/                       # Cloudflare Workers
│   └── email-sender/              # Email delivery worker
│       ├── src/index.js
│       └── wrangler.toml
├── attic/                         # Archived/baseline code
│   └── baseline/tools/            # Original Python scripts
├── CONTRIBUTING.md                # Contribution guidelines
├── README.md                      # Project readme
├── THIRD-PARTY-LICENSES.md        # Third-party attribution
├── TODO.txt                       # Task tracking
├── hugo.toml                      # Hugo configuration
├── package.json                   # Node.js dependencies
├── shell.nix                      # Nix development environment
└── tailwind.config.js             # Tailwind configuration
```

---

## Theme Architecture

The site uses the **AirFold** theme, a custom Hugo theme with a paper aesthetic featuring handwritten fonts, wavy borders, and cream/brown color palette.

### Theme Provides vs Site Overrides

**Theme Provides (`themes/airfold/`):**

- Base layouts: `baseof.html`, `single.html`, `list.html`, `index.html`
- Page layouts: `about.html`, `contact.html`, `term.html`
- Core partials: `seo.html`, `header.html`, `footer.html`, `social-icon.html`
- CSS: Tailwind v4 with paper aesthetic components
- Self-hosted fonts: Neucha, Patrick Hand
- JavaScript: OpenPGP.js, Hammer.js, Mermaid.js

**Site Overrides (`layouts/`):**

- `esoterica/` - Blog/articles section
- `projects/` - Portfolio projects
- `resume/` - Resume page and sub-sections
- `religion/` - Bible reading interface
- Custom partials: `bible-nav.html`, `canonical-comparison.html`, etc.

### Design System

**Fonts:**
| Font | Usage | Weight |
|------|-------|--------|
| Neucha | Primary headings, body text | 400 |
| Patrick Hand | Secondary text, accents | 400 |
| SBL Hebrew/Greek | Biblical languages | (future) |

**Color Palette (CSS Variables):**
```css
/* Light Mode */
--color-paper-white: #b5a48e;      /* Page background */
--color-paper-bright: #e6ddc0;     /* Content background */
--color-paper-black: #2c2416;      /* Primary text */
--color-paper-gray: #5c5347;       /* Secondary text */
--color-paper-light: #d4cbb4;      /* Light accents */
--color-paper-border: #3d3428;     /* Borders */
--color-accent: #7a00b0;           /* Purple accent */

/* Dark Mode - Automatic via Tailwind dark: prefix */
```

**Visual Elements:**

- Wavy borders using SVG clip-path
- Offset box shadows (4px 4px) for depth
- No harsh white or black colors
- Subtle paper texture overlays
- Handwritten aesthetic throughout

---

## Data-Driven Content

The site uses a **data-driven architecture** where structured content types are generated from JSON files rather than individual markdown files.

### Pattern Overview

```

1. Metadata File (data/*.json)
   ├── List of items with core fields (id, title, tags, weight)
   └── Used for list pages, filtering, sorting

2. Auxiliary File (data/*_auxiliary.json or data/*_auxiliary/)
   ├── Full content for each item
   └── Sections, descriptions, links, rich content

3. Content Template (extensions/.../content/*/_content.gotmpl)
   ├── Reads JSON data at build time
   ├── Generates Hugo pages via $.AddPage
   └── Outputs markdown content from JSON structure

4. Layout Templates (layouts/*/)
   ├── Renders generated pages with appropriate styling
   └── Uses data for navigation, metadata, SEO
```

### Data File Structure Examples

**Metadata File (`data/certifications.json`):**
```json
{
  "$schema": "https://focuswithjustin.com/schemas/certifications.schema.json",
  "certifications": [
    {
      "id": "cissp",
      "title": "CISSP",
      "description": "Certified Information Systems Security Professional",
      "issuer": "ISC2",
      "issued": "2020-01-15",
      "expires": "2026-01-15",
      "credly_url": "https://credly.com/badges/...",
      "logo": "/images/certifications/cissp.svg",
      "tags": ["security", "governance", "risk"],
      "weight": 10
    }
  ],
  "meta": {
    "version": "1.0.0",
    "lastModified": "2025-12-30"
  }
}
```

**Auxiliary File (`data/certifications_auxiliary.json`):**
```json
{
  "certifications": {
    "cissp": {
      "content": "The CISSP is a globally recognized certification...",
      "sections": [
        {
          "heading": "About This Certification",
          "content": "Detailed description of the certification..."
        },
        {
          "heading": "Requirements",
          "list": [
            "5 years cumulative paid work experience",
            "Pass the 6-hour exam (100-150 questions)",
            "Endorsement by ISC2 member"
          ]
        },
        {
          "heading": "Official Resources",
          "links": [
            {"text": "ISC2 Official Site", "url": "https://isc2.org/cissp"},
            {"text": "Exam Outline", "url": "https://isc2.org/cissp/exam"}
          ]
        }
      ]
    }
  }
}
```

**Trademark Data (`data/trademarks.json`):**
```json
{
  "trademarks": [
    {"name": "Focus with Justin", "abbrev": null},
    {"name": "Side by Side Scripture", "abbrev": "SSS"},
    {"name": "Christian Canon Compared", "abbrev": "CCC"}
  ],
  "symbols": [{
    "name": "description of symbol",
    "svg": "<svg viewBox=\"0 0 24 24\">...</svg>"
  }],
  "owner": "Justin Weeks"
}
```

**Bible Content (`data/bibles_auxiliary/kjv.json`):**
```json
{
  "books": [
    {
      "id": "Gen",
      "name": "Genesis",
      "abbrev": "Gen",
      "testament": "OT",
      "chapters": [
        {
          "number": 1,
          "verses": [
            {
              "number": 1,
              "text": "In the beginning God created the heaven and the earth."
            }
          ]
        }
      ]
    }
  ]
}
```

### Sections Using Data-Driven Pattern

| Section | Metadata | Auxiliary | URL Pattern | Generated Pages |
|---------|----------|-----------|-------------|-----------------|
| Certifications | `certifications.json` | `certifications_auxiliary.json` | `/resume/certifications/{id}/` | ~20 |
| Skills | `skills.json` | `skills_auxiliary.json` | `/resume/skills/{id}/` | ~30 |
| Tools | `tools.json` | `tools_auxiliary.json` | `/resume/tools/{id}/` | ~50 |
| Bibles | `bibles.json` | `bibles_auxiliary/{id}.json` | `/religion/bibles/{id}/{book}/{chapter}/` | ~6500 |

---

## Module Mounts

Hugo module mounts map content and layout directories to URL paths. This enables the extension system and clean URL structures.

### Configuration in hugo.toml

```toml
[module]
  # Static assets (theme and site)
  [[module.mounts]]
    source = 'static'
    target = 'static'
  [[module.mounts]]
    source = 'themes/airfold/static'
    target = 'static'

  # Assets (CSS, JS)
  [[module.mounts]]
    source = 'assets'
    target = 'assets'
  [[module.mounts]]
    source = 'themes/airfold/assets'
    target = 'assets'

  # Content mounts (extensions → site URLs)
  [[module.mounts]]
    source = 'content'
    target = 'content'
  [[module.mounts]]
    source = 'themes/airfold/extensions/certifications/content/certifications'
    target = 'content/resume/certifications'
  [[module.mounts]]
    source = 'themes/airfold/extensions/tools/content/tools'
    target = 'content/resume/tools'
  [[module.mounts]]
    source = 'themes/airfold/extensions/religion/content/bibles'
    target = 'content/religion/bibles'

  # Layout mounts (site overrides theme)
  [[module.mounts]]
    source = 'layouts'
    target = 'layouts'
  [[module.mounts]]
    source = 'themes/airfold/layouts'
    target = 'layouts'
```

### How Mounts Work

1. **Source** - The actual directory in the file system
2. **Target** - The virtual path Hugo sees during build
3. **Priority** - Later mounts override earlier ones for conflicts

**Example Flow:**
```
themes/airfold/extensions/religion/content/bibles/_content.gotmpl
    │
    │ Module mount maps source → target
    ▼
content/religion/bibles/_content.gotmpl (virtual path)
    │
    │ Hugo processes _content.gotmpl, reads data/*.json
    ▼
Generated pages: /religion/bibles/kjv/, /religion/bibles/kjv/gen/1/, etc.
```

---

## Extension System

Extensions are feature modules in `themes/airfold/extensions/`. Each extension provides content generation templates that read from JSON data files. Layouts are consolidated in the main theme (`themes/airfold/layouts/`).

### Extension Structure

```
themes/airfold/extensions/{extension}/
├── content/{section}/
│   ├── _content.gotmpl          # Page generator (reads JSON, creates pages)
│   └── _index.md                # Section index page (optional)
└── README.md                    # Extension documentation
```

> **Note:** Layout templates are in `themes/airfold/layouts/`, not in extensions. This DRY consolidation ensures layouts are maintained in one location.

### Available Extensions

| Extension | Purpose | Data Files Required | Layouts Location |
|-----------|---------|---------------------|------------------|
| `certifications/` | Professional certifications | `data/certifications*.json` | `layouts/resume/certifications/` |
| `portfolio/` | Project galleries | Markdown in `content/projects/` | `layouts/projects/` |
| `religion/` | Bible translations | `data/bibles*.json` | `layouts/religion/bibles/` |
| `resume/` | CV/resume layouts | `data/resume.json` | `layouts/_default/resume.html` |
| `tools/` | Professional tools | `data/tools*.json` | `layouts/resume/tools/` |

### Creating a New Extension

1. **Create extension directory:**
   ```bash
   mkdir -p themes/airfold/extensions/{name}/content/{section}
   ```

2. **Create `_content.gotmpl`:**
   ```go-html-template
   {{- $metadata := .Site.Data.{name} -}}
   {{- $auxiliary := .Site.Data.{name}_auxiliary -}}
   {{- range $metadata.items -}}
     {{- $aux := index $auxiliary.items .id -}}
     {{- $page := dict
       "path" .id
       "title" .title
       "kind" "page"
       "params" (dict "id" .id ...)
       "content" (dict "mediaType" "text/markdown" "value" $aux.content)
     -}}
     {{- $.AddPage $page -}}
   {{- end -}}
   ```

3. **Create layouts in main theme:**
   ```bash
   mkdir -p themes/airfold/layouts/{section}
   # Create list.html and single.html
   ```

4. **Add module mount to `hugo.toml`:**
   ```toml
   [[module.mounts]]
     source = 'themes/airfold/extensions/{name}/content/{section}'
     target = 'content/{url-path}'
   ```

5. **Create data files:**
   - `data/{name}.json` - Metadata
   - `data/{name}_auxiliary.json` - Content

6. **Document in extension README.md**

---

## Security Architecture

### Contact Form Security

The contact form implements defense-in-depth with multiple security layers:

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Browser                                    │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────────────┐│
│  │ Turnstile      │  │ PGP Encrypt    │  │ Form Validation        ││
│  │ Widget         │  │ (if enabled)   │  │ (client-side)          ││
│  └───────┬────────┘  └───────┬────────┘  └───────────┬────────────┘│
└──────────┼───────────────────┼───────────────────────┼──────────────┘
           │                   │                       │
           ▼                   ▼                       ▼
     ┌─────────────────────────────────────────────────────────────┐
     │                  POST /api/contact                           │
     │  {name, email, message, cf-turnstile-response, encrypted?}  │
     └─────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
     ┌─────────────────────────────────────────────────────────────┐
     │               Pages Function (contact.js)                    │
     │  1. Validate request origin (CORS check)                    │
     │  2. Verify Turnstile token with Cloudflare API              │
     │  3. Validate input fields (length, format, required)        │
     │  4. Generate HMAC-SHA256 signature (timestamp + payload)    │
     │  5. Call email worker via service binding                   │
     └─────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
     ┌─────────────────────────────────────────────────────────────┐
     │                    Email Worker                              │
     │  1. Verify HMAC signature (same secret as Pages)            │
     │  2. Check timestamp freshness (2-minute window)             │
     │  3. Format email with proper headers                        │
     │  4. Send via Cloudflare Email Routing API                   │
     └─────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
                        ┌─────────────────┐
                        │  Destination    │
                        │  Inbox          │
                        └─────────────────┘
```

**Security Features:**

| Feature | Purpose | Implementation |
|---------|---------|----------------|
| CAPTCHA | Prevent automated spam | Cloudflare Turnstile widget |
| HMAC Auth | Authenticate worker requests | SHA-256 signature with shared secret |
| Replay Protection | Prevent request reuse | 2-minute timestamp window |
| Origin Validation | Block cross-origin abuse | CORS header checking |
| Input Validation | Prevent injection | Length limits, format checks |
| PGP Encryption | Protect message content | OpenPGP.js client-side encryption |

### Service Binding vs Environment Variable

The email worker uses a **service binding** (not an environment variable):

- Direct RPC between Pages Function and Worker
- No public network exposure of worker endpoint
- Automatic request authentication
- Configured in Cloudflare dashboard, not code

---

## Build Process

### Development Build

```bash
nix-shell                              # Enter Nix environment
npm run dev                            # Start dev server
```

**What `npm run dev` does:**

1. Starts Tailwind CSS watcher (recompiles on CSS changes)
2. Starts Hugo server with live reload
3. Serves at http://localhost:1313/

### Production Build

```bash
npm run build
```

**Build Pipeline:**
```
npm run build
    │
    ├── npm run build:css
    │   └── npx @tailwindcss/cli -i ./assets/css/main.css -o ./static/css/main.css
    │
    └── hugo --minify
        ├── Process _content.gotmpl templates
        ├── Read data/*.json files
        ├── Generate ~6700 pages
        └── Output to public/
```

### Cloudflare Pages Deployment

```
git push origin main
    │
    ▼
Cloudflare Pages Build
    ├── npm install
    ├── npm run build
    └── Deploy public/ to CDN
```

**Build Settings:**
| Setting | Value |
|---------|-------|
| Framework preset | None |
| Build command | `npm run build` |
| Build output directory | `public` |
| Node.js version | 22 |

---

## JavaScript Components

All JavaScript is vanilla (no framework) and self-hosted.

### Core Scripts

| Script | Purpose | Loaded On | Size |
|--------|---------|-----------|------|
| `site.js` | Theme toggle, mobile menu | All pages | ~5KB |
| `lightbox.js` | Table/chart enlargement | Pages with tables/mermaid | ~8KB |
| `strongs.js` | Strong's number linking | Bible pages | ~3KB |
| `parallel.js` | Parallel translation view | Compare page | ~6KB |
| `bible-search.js` | Full-text Bible search | Search page | ~4KB |
| `share.js` | Verse sharing (URL, social) | Bible chapter pages | ~2KB |
| `trademarks.js` | Auto-detect and style trademarks | All pages | ~2KB |

### Third-Party Libraries (Self-Hosted)

| Library | Version | Purpose | Size |
|---------|---------|---------|------|
| OpenPGP.js | 5.x | PGP encryption | ~200KB |
| Hammer.js | 2.0.8 | Touch gestures | ~8KB |
| Mermaid.js | 11.x | Diagram rendering | ~300KB |

### Lightbox Feature Details

Tables and Mermaid diagrams automatically get lightbox functionality:

**Trigger:**

- Hover shows ">" arrow with "click to enlarge" hint
- Playful bounce animation on arrow

**Controls:**

- Zoom: `+` / `-` keys or buttons
- Rotate: `R` / `Shift+R` keys or buttons
- Close: `Escape` key or click outside

**Touch (via Hammer.js):**

- Pinch-to-zoom
- Two-finger rotate

**Opt-out:**

- Add `no-lightbox` class to disable on specific elements

---

## Self-Hosting Strategy

All external dependencies are self-hosted to eliminate CDN dependencies and ensure privacy.

### Self-Hosted Assets

| Asset | Location | Original Source |
|-------|----------|-----------------|
| Neucha font | `static/fonts/Neucha/` | Google Fonts |
| Patrick Hand font | `static/fonts/PatrickHand/` | Google Fonts |
| OpenPGP.js | `static/js/openpgp.min.js` | npm |
| Hammer.js | `static/js/hammer.min.js` | npm |
| Mermaid.js | `static/js/mermaid.min.js` | npm |
| Tailwind CSS | `static/css/main.css` | Compiled locally |

### Benefits

1. **Privacy** - No third-party tracking or analytics
2. **Performance** - No external DNS lookups or connections
3. **Reliability** - No dependency on CDN availability
4. **Security** - No supply chain risk from CDN compromise
5. **GDPR Compliance** - No data sent to third parties

### Verification

The site makes zero external requests for assets. All resources are served from the same domain.

---

## Internationalization

UI strings are stored in `i18n/en.toml` and accessed via Hugo's i18n function.

### Adding UI Strings

**In `i18n/en.toml`:**
```toml
[keyName]
other = "Display text"

[greeting]
other = "Hello, {{ .Name }}!"
```

**In templates:**
```html
{{ i18n "keyName" }}
{{ i18n "greeting" (dict "Name" .Params.author) }}
```

### Current String Categories

The i18n file contains 80+ strings organized by:

- Navigation (home, about, back, next, previous)
- Content (read more, min read, tags)
- Resume (certifications, skills, tools)
- Contact form (labels, validation, status)
- Religion section (bibles, chapters, verses, compare)
- States (loading, error, empty)
- Accessibility (skip to content, ARIA labels)

### Adding a New Language

1. Create `i18n/{lang}.toml` (e.g., `i18n/es.toml`)
2. Copy all keys from `en.toml`
3. Translate values
4. Hugo automatically uses based on `languageCode` in `hugo.toml`

---

## Performance Optimization

### Static Generation

- All ~6700 pages pre-built at deploy time
- No server-side rendering
- Instant page loads from CDN cache

### Asset Optimization

| Technique | Implementation |
|-----------|----------------|
| CSS minification | Hugo `--minify` flag |
| HTML minification | Hugo `--minify` flag |
| Brotli compression | Cloudflare automatic |
| Image optimization | WebP format where possible |
| Font subsetting | Full character sets for now |

### Bible Data Optimization

| Technique | Before | After |
|-----------|--------|-------|
| Split auxiliary files | 32MB single file | 5MB × 5 files |
| Brotli compression | 32MB | 6.1MB |
| Lazy chapter loading | Load entire Bible | Load on navigation |
| Search caching | Rebuild index each search | Cache search results |

### Lighthouse Targets

| Metric | Target | Current |
|--------|--------|---------|
| Performance | 95+ | 90+ |
| Accessibility | 100 | 95+ |
| Best Practices | 100 | 100 |
| SEO | 100 | 100 |

---

## Related Documentation

- [Development Guide](development.md) - Setup and workflows
- [Configuration Reference](configuration.md) - hugo.toml details
- [Contact Form Guide](contact-form.md) - Security implementation
- [Religion Section Guide](religion-section.md) - Bible features
- [Testing Guide](testing.md) - Test strategy and coverage
- [Deployment Guide](deployment.md) - Cloudflare Pages setup
- [STEPBible Charter](stepbible-interface-charter.md) - Future Bible UI roadmap
