# Development Guide

This comprehensive guide covers setting up a development environment, common workflows, content management, and troubleshooting.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Quick Start](#quick-start)
3. [Development Environment](#development-environment)
4. [Available Commands](#available-commands)
5. [Environment Variables](#environment-variables)
6. [Project Organization](#project-organization)
7. [Development Process](#development-process)
8. [Content Management](#content-management)
9. [Styling and CSS](#styling-and-css)
10. [JavaScript Development](#javascript-development)
11. [Testing](#testing)
12. [Code Style](#code-style)
13. [Git Workflow](#git-workflow)
14. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Required Software

| Software | Version | Purpose |
|----------|---------|---------|
| [Nix](https://nixos.org/) | Latest | Reproducible development environment (recommended) |
| [Hugo](https://gohugo.io/) | 0.128.0+ extended | Static site generator |
| [Node.js](https://nodejs.org/) | 22+ | JavaScript runtime, npm |
| [Go](https://go.dev/) | 1.22+ | SWORD converter tool |

### Using Nix (Recommended)

Nix provides a reproducible development environment with all dependencies:

```bash
# Install Nix (if not already installed)
sh <(curl -L https://nixos.org/nix/install) --daemon

# Enter the development environment
nix-shell
```

The `shell.nix` provides:

- Hugo (extended version)
- Node.js 22
- Go toolchain
- Image tools (libwebp, ImageMagick)
- SQLite (for e-Sword support)
- Python 3 with PyYAML
- SWORD library and tools

### Without Nix

If not using Nix, install manually:

```bash
# macOS with Homebrew
brew install hugo node go sqlite imagemagick webp

# Ubuntu/Debian
sudo apt install hugo nodejs npm golang sqlite3 imagemagick webp

# Verify installations
hugo version   # Should show "extended"
node --version # Should be 22+
go version     # Should be 1.22+
```

---

## Quick Start

```bash
# 1. Clone the repository
git clone https://github.com/cyanitol/Website.Public.FocuswithJustin.com.git
cd Website.Public.FocuswithJustin.com

# 2. Enter Nix development environment
nix-shell

# 3. Install Node dependencies
npm install

# 4. Start development server
npm run dev
```

The site will be available at `http://localhost:1313/`.

### What Happens on Start

1. `npm run dev` triggers:
   - `decompress:data` - Extracts Bible data from compressed archive
   - `dev:hugo` - Starts Hugo server with live reload
   - `dev:css` - Starts Tailwind CSS watch mode

2. Hugo serves the site with hot reload
3. Tailwind watches for CSS changes and recompiles

---

## Development Environment

### Nix Shell Details

When you run `nix-shell`, you get:

```
Focus with Justin - Development Environment
============================================

Commands:
  npm run dev    - Start development server
  npm run build  - Build for production

Image tools:
  cwebp          - Convert images to WebP
  convert        - Resize/process images (ImageMagick)

SWORD converter (tools/juniper/):
  go build ./cmd/juniper  - Build converter
  go test ./...                   - Run tests

Bible extraction:
  python3 tools/juniper/extract_scriptures.py data/
  diatheke -b KJV -k Gen 1:1      - Test SWORD access
```

### Directory Structure for Development

```
.
├── content/              # Markdown content (esoterica, projects)
├── data/                 # JSON data files
│   ├── bibles.json       # Bible metadata
│   ├── bibles_auxiliary/ # Full Bible text (~30MB per translation)
│   ├── certifications.json
│   ├── skills.json
│   └── tools.json
├── docs/                 # Project documentation (you are here)
├── functions/            # Cloudflare Pages Functions
│   └── api/
│       └── contact.js    # Contact form handler
├── i18n/                 # Internationalization
│   └── en.toml           # English UI strings
├── layouts/              # Site-specific layout overrides
├── static/               # Static assets
│   ├── css/main.css      # Compiled Tailwind CSS
│   ├── images/           # Site images
│   └── js/               # JavaScript files
├── themes/airfold/       # AirFold theme
│   ├── assets/css/       # Tailwind source CSS
│   ├── extensions/       # Reusable content type extensions
│   └── layouts/          # Theme layouts
├── tools/                # Development tools
│   └── juniper/  # Go Bible extraction tool
└── workers/              # Cloudflare Workers
    └── email-sender/     # Email routing worker
```

---

## Available Commands

### npm Scripts

| Command | Description |
|---------|-------------|
| `npm run dev` | Start Hugo + Tailwind in parallel watch mode |
| `npm run build` | Build for production (CSS minified, Hugo minified) |
| `npm run build:css` | Build CSS only (minified) |
| `npm run build:hugo` | Build Hugo only (minified) |
| `npm run decompress:data` | Extract Bible data from compressed archive |
| `npm run test` | Run all tests |
| `npm run test:sword` | Run SWORD converter tests |
| `npm run test:security` | Run security tests against production |
| `npm run test:security:local` | Run security tests against local dev |
| `npm run sword:build` | Build the Go converter |
| `npm run sword:migrate` | Run migration tool |
| `npm run sword:convert` | Convert SWORD modules to JSON |

### Direct Hugo Commands

```bash
# Development server
hugo server                         # Basic dev server
hugo server --disableFastRender     # Dev server with full rebuilds
hugo server --bind 0.0.0.0          # Accessible from other devices
hugo server -p 1314                 # Use different port

# Building
hugo                                # Build site
hugo --minify                       # Build minified for production
hugo --verbose                      # Build with detailed output
hugo --templateMetrics              # Show template performance

# Content
hugo new esoterica/my-post.md       # Create new blog post
hugo new projects/my-project.md     # Create new project page
```

### SWORD Converter Commands

```bash
cd tools/juniper

# Build
go build ./cmd/juniper
go build ./cmd/extract

# Run converter
./juniper convert --output ../../data/
./juniper list              # List available modules
./juniper info KJV          # Show module info

# Repository management (native installmgr replacement)
./juniper repo list-sources        # List remote sources
./juniper repo list CrossWire      # List available modules
./juniper repo install CrossWire KJV  # Install a module
./juniper repo installed           # List installed modules
./juniper repo verify              # Verify all module integrity
./juniper repo verify KJV          # Verify single module
./juniper repo uninstall KJV       # Remove a module

# Testing
go test ./...                       # All tests
go test -v ./pkg/sword/...          # Verbose, sword package only
go test -run Integration ./...      # Integration tests only
go test -cover ./...                # With coverage report
go test -coverprofile=cover.out ./... && go tool cover -html=cover.out
```

### Just Commands (Recommended)

Use `just` for common tasks:

```bash
# Bible management
just bible-sources                  # List remote sources
just bible-list                     # List installed modules
just bible-available                # List available from CrossWire
just bible-available "eBible.org"   # List from other sources
just bible-download KJV             # Download single module
just bible-remove KJV               # Uninstall module
just bible-verify                   # Verify all module integrity
just bible-verify KJV               # Verify single module
just bible-convert                  # Convert to Hugo JSON
just bible-add KJV                  # Full workflow: download + convert

# Bulk downloads
just bible-download-all             # Download all from CrossWire
just bible-download-all "eBible.org"  # Download all from source
just bible-download-mega            # Download from ALL sources
just bible-download-all-verify      # Verify CrossWire complete
just bible-download-mega-verify     # Verify all sources complete

# Testing
just test                           # Run comprehensive tests
just test-quick                     # Quick pre-deploy tests
just test-extensive                 # Full validation with fuzz tests
just coverage                       # Generate coverage report

# Development
just dev                            # Start dev server
just build                          # Production build
just sword-build                    # Build juniper
```

### Image Processing Commands

```bash
# Convert to WebP (lossy)
cwebp -q 80 input.png -o output.webp

# Convert to WebP (lossless)
cwebp -lossless input.png -o output.webp

# Resize image
convert input.jpg -resize 800x600 output.jpg

# Create thumbnail
convert input.jpg -thumbnail 200x200^ -gravity center -extent 200x200 thumb.jpg
```

---

## Environment Variables

### Required for Development

| Variable | Value | Purpose |
|----------|-------|---------|
| `HUGO_TURNSTILE_SITE_KEY` | `test` | CAPTCHA site key (use "test" for local dev) |

### Optional Development Variables

| Variable | Purpose |
|----------|---------|
| `HUGO_ENV` | Set to `production` for production-like builds |
| `DEBUG` | Enable debug output in scripts |
| `SWORD_PATH` | Custom path to SWORD modules (default: `~/.sword`) |
| `UPDATE_GOLDEN` | Set to `1` to update golden test files |

### Production Variables (Cloudflare)

| Variable | Purpose |
|----------|---------|
| `HUGO_TURNSTILE_SITE_KEY` | Turnstile public site key |
| `TURNSTILE_SECRET_KEY` | Turnstile secret (runtime, Pages + Worker) |
| `ALLOWED_ORIGINS` | Comma-separated allowed origins for CORS |

See [Configuration Guide](configuration.md#environment-variables) for complete reference.

---

## Project Organization

### Content Structure

```
content/
├── _index.md            # Homepage content
├── about.md             # About page
├── contact.md           # Contact page
├── privacy.md           # Privacy policy
├── terms.md             # Terms of use
├── esoterica/           # Blog posts
│   ├── _index.md        # Blog listing page
│   └── *.md             # Individual posts
├── projects/            # Project pages
│   ├── _index.md        # Projects listing
│   └── *.md             # Individual projects
├── resume/              # Resume sections (generated from JSON)
│   ├── _index.md        # Resume main page
│   ├── certifications/  # Generated from data/certifications.json
│   ├── skills/          # Generated from data/skills.json
│   └── tools/           # Generated from data/tools.json
└── religion/            # Religion section
    ├── _index.md        # Religion main page
    └── bibles/          # Generated from data/bibles.json
```

### Data Files

```
data/
├── bibles.json              # Bible translation metadata
├── bibles_auxiliary/        # Full Bible text (one file per translation)
│   ├── kjv.json             # King James Version (~5MB)
│   └── ...
├── book_mappings.json       # Canonical status per tradition
├── certifications.json      # Certification metadata
├── certifications_auxiliary.json  # Full certification content
├── skills.json              # Skills metadata
├── skills_auxiliary.json    # Full skills content
├── tools.json               # Tools metadata
├── tools_auxiliary.json     # Full tools content
└── social.yaml              # Social media links
```

### Theme Structure

```
themes/airfold/
├── assets/
│   └── css/
│       └── main.css         # Tailwind source CSS
├── extensions/              # Reusable content types
│   ├── certifications/      # Professional certifications
│   ├── portfolio/           # Project galleries
│   ├── religion/            # Bible translations
│   └── resume/              # CV/resume layouts
├── i18n/                    # Theme translations
├── layouts/
│   ├── _default/            # Default layouts
│   ├── partials/            # Reusable components
│   └── shortcodes/          # Custom shortcodes
└── static/
    ├── fonts/               # Web fonts
    └── js/                  # JavaScript libraries
```

---

## Development Process

Follow this process for all changes:

```

1. Document Behavior  →  Describe what the feature should do in docs/
2. Write Tests        →  Create tests that verify the documented behavior
3. Write Code         →  Implement the feature to pass the tests
4. Test               →  Run all tests to confirm nothing broke
```

### Example: Adding a New Feature

```bash
# 1. Document the feature
# Edit docs/architecture.md or create feature-specific doc

# 2. Write tests first (if applicable)
cd tools/juniper
# Create test in pkg/feature/feature_test.go

# 3. Implement the feature
# Edit the code

# 4. Run tests
npm run test:sword

# 5. Verify build
npm run build

# 6. Commit
git add .
git commit -m "Add feature: description"
```

---

## Content Management

### Adding a Blog Post

```bash
# Create new post
hugo new esoterica/my-new-post.md

# Edit the file
# content/esoterica/my-new-post.md
```

Frontmatter options:

```yaml
---
title: "My New Post"
date: 2024-01-15
draft: true                    # Set to false to publish
description: "SEO description"
tags: ["security", "tutorial"]
noindex: false                 # Exclude from search engines
nofollow: false                # Prevent link following
showSocialBanner: true         # Show social links section
---

Post content here...
```

### Adding a Project

```bash
hugo new projects/my-project.md
```

Frontmatter:

```yaml
---
title: "Project Name"
date: 2024-01-15
draft: true
description: "What this project does"
tags: ["python", "security"]
image: "/images/projects/my-project.png"
---

Project description...
```

### Adding a Certification

1. Edit `data/certifications.json`:

```json
{
  "id": "my-cert",
  "title": "My Certification",
  "description": "Short description for SEO",
  "issuer": "Certifying Organization",
  "issued": "2024-01-15",
  "expires": "2027-01-15",
  "credly_url": "https://www.credly.com/badges/...",
  "logo": "/images/certifications/my-cert.png",
  "tags": ["security", "compliance"],
  "weight": 10
}
```

2. Edit `data/certifications_auxiliary.json`:

```json
{
  "certifications": {
    "my-cert": {
      "content": "Introduction paragraph about the certification.",
      "sections": [
        {
          "heading": "About",
          "content": "Detailed description..."
        },
        {
          "heading": "Requirements",
          "list": ["Requirement 1", "Requirement 2"]
        },
        {
          "heading": "Links",
          "links": [
            { "text": "Official Website", "url": "https://..." }
          ]
        }
      ]
    }
  }
}
```

3. Rebuild: `npm run build`

### Adding a Bible Translation

See [Religion Section Guide](religion-section.md#adding-a-new-translation) for detailed instructions.

Quick steps:

```bash
# Option 1: Using just (recommended)
just bible-add KJV                  # Download + convert + register

# Option 2: Step by step
just bible-download KJV             # Download from CrossWire
just bible-verify KJV               # Verify installation
just bible-convert                  # Convert to Hugo JSON

# Option 3: Direct commands
cd tools/juniper
./juniper repo install CrossWire KJV
./juniper repo verify KJV
# Then convert and register in bibles.json
```

---

## Styling and CSS

### Tailwind CSS v4

The site uses Tailwind CSS v4 with a paper aesthetic. Source CSS is in:
`themes/airfold/assets/css/main.css`

```css
/* Custom CSS uses Tailwind v4 syntax */
@theme {
  --color-accent: #7a00b0;
  --color-paper: #fdf5e6;
  --font-family-heading: "Neucha", cursive;
  --font-family-body: "Patrick Hand", cursive;
}
```

### Modifying Styles

```bash
# 1. Edit the source CSS
vim themes/airfold/assets/css/main.css

# 2. In dev mode, changes compile automatically
# In production, manually build:
npm run build:css
```

### Paper Aesthetic Classes

```html
<!-- Paper-like card -->
<div class="paper-card">Content</div>

<!-- Paper button -->
<button class="paper-btn">Click me</button>

<!-- Paper input -->
<input class="paper-input" type="text">
```

### Adding Custom Styles

Add to `themes/airfold/assets/css/main.css`:

```css
/* Custom component */
.my-component {
  @apply bg-paper rounded-lg shadow-md p-4;
  border: 2px solid var(--color-accent);
}
```

---

## JavaScript Development

### Self-Hosted Libraries

All JavaScript is self-hosted (no CDN dependencies):

| Library | Location | Purpose |
|---------|----------|---------|
| OpenPGP.js | `static/js/openpgp.min.js` | PGP encryption |
| Hammer.js | `static/js/hammer.min.js` | Touch gestures |
| Mermaid | `static/js/mermaid.min.js` | Diagrams |
| Lightbox | `themes/airfold/static/js/lightbox.js` | Image zoom |

### Adding JavaScript

1. Add script to `static/js/` or `themes/airfold/static/js/`
2. Include in template:

```html
{{ $script := resources.Get "js/myscript.js" | minify | fingerprint }}
<script src="{{ $script.Permalink }}"></script>
```

### Lightbox Feature

Tables and Mermaid diagrams get automatic lightbox support:

- Hover shows expand arrow
- Click opens fullscreen with zoom/rotate
- Touch gestures: pinch-to-zoom, two-finger rotate
- Keyboard: Escape, +/-, R/Shift+R

To disable:
```html
<table class="no-lightbox">...</table>
```

---

## Testing

### Quick Test Commands

```bash
# Build test (catches template errors)
npm run build

# SWORD converter tests
npm run test:sword

# Security tests (requires running server)
npm run dev &
npm run test:security:local
```

### Test Categories

| Test Type | Command | Purpose |
|-----------|---------|---------|
| Build | `npm run build` | Hugo compilation |
| SWORD | `npm run test:sword` | Go converter tests |
| Security | `npm run test:security` | Contact form security |

### Running Specific Tests

```bash
cd tools/juniper

# All tests
go test ./...

# Specific package
go test -v ./pkg/sword/...

# Single test
go test -v -run TestParseOSIS ./pkg/markup/

# Integration tests
go test -run Integration ./...

# Coverage
go test -coverprofile=cover.out ./...
go tool cover -html=cover.out
```

See [Testing Guide](testing.md) for comprehensive test documentation.

---

## Code Style

### Go

- Standard `gofmt` formatting
- Run `go fmt ./...` before committing
- Use meaningful package names
- Document exported functions

### JavaScript

- Vanilla JS (no frameworks)
- Use `const` and `let` (no `var`)
- Document with JSDoc comments

### CSS

- Tailwind utility classes preferred
- Custom CSS uses Tailwind v4 `@theme` syntax
- Follow paper aesthetic design

### Templates

- Hugo's Go template syntax
- Use partials for reusable components
- Keep templates focused and small

### General

- No trailing whitespace
- Files end with newline
- Use descriptive variable names
- Keep functions small and focused

---

## Git Workflow

### Branching

```bash
# Feature branch
git checkout -b feature/my-feature

# Bug fix
git checkout -b fix/bug-description

# Documentation
git checkout -b docs/update-readme
```

### Commit Messages

Follow conventional commits:

```
type: subject

body (optional)

footer (optional)
```

Types:

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `style`: Formatting
- `refactor`: Code restructure
- `test`: Tests
- `chore`: Maintenance

### Pre-Commit Checks

Before committing:

```bash
# Build must succeed
npm run build

# Tests must pass (if applicable)
npm run test:sword

# Format Go code
cd tools/juniper && go fmt ./...
```

---

## Troubleshooting

### Common Issues

#### "CAPTCHA site key not set"

```bash
export HUGO_TURNSTILE_SITE_KEY=test
```

#### Styles not updating

```bash
# Force recompile CSS
npx @tailwindcss/cli -i ./themes/airfold/assets/css/main.css -o ./static/css/main.css
```

#### Hugo build errors

```bash
# Verbose output
hugo --verbose

# Check specific template
hugo --templateMetrics
```

#### Go module issues

```bash
cd tools/juniper
go mod tidy
go mod download
```

#### Bible data not loading

```bash
# Ensure data is decompressed
npm run decompress:data

# Check file exists
ls -la data/bibles_auxiliary/

# Verify JSON is valid
python3 -m json.tool data/bibles.json > /dev/null
```

#### Port already in use

```bash
# Use different port
hugo server -p 1314

# Or kill existing process
lsof -ti :1313 | xargs kill
```

### Debug Mode

```bash
# Enable Hugo debug
hugo server --debug

# Enable verbose logging
hugo server --verbose --debug

# Template metrics
hugo server --templateMetrics --templateMetricsHints
```

### Performance Issues

If the dev server is slow:

```bash
# Disable fast render (more reliable)
hugo server --disableFastRender

# Build only specific content
hugo server --ignoreCache
```

### SWORD Module Issues

```bash
# List installed modules
just bible-list

# Verify module integrity
just bible-verify KJV

# Verify all modules
just bible-verify

# Check module files directly
ls ~/.sword/mods.d/

# Test module access (if diatheke available)
diatheke -b KJV -k Gen 1:1

# Reinstall a corrupted module
just bible-remove KJV
just bible-download KJV
just bible-verify KJV
```

---

## Related Documentation

- [Architecture Guide](architecture.md) - System design and patterns
- [Configuration Reference](configuration.md) - Complete hugo.toml reference
- [Testing Guide](testing.md) - Test strategy and coverage
- [Deployment Guide](deployment.md) - Production deployment
- [Religion Section Guide](religion-section.md) - Bible features
- [Contact Form Guide](contact-form.md) - CAPTCHA and email setup
