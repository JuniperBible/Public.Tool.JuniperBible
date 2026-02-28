# Configuration Reference

This is the complete reference for all site configuration. All configuration is centralized in `hugo.toml` with internationalization strings in `i18n/en.toml`.

## Table of Contents

1. [Site Settings](#site-settings)
2. [Site Parameters](#site-parameters)
3. [SEO Configuration](#seo-configuration)
4. [Email Configuration](#email-configuration)
5. [CAPTCHA Configuration](#captcha-configuration)
6. [Resume Section](#resume-section)
7. [Mermaid Diagrams](#mermaid-diagrams)
8. [Carousel Navigation](#carousel-navigation)
9. [PGP Encryption](#pgp-encryption)
10. [Navigation Menus](#navigation-menus)
11. [Module Mounts](#module-mounts)
12. [Output Formats](#output-formats)
13. [Markup Settings](#markup-settings)
14. [Taxonomies](#taxonomies)
15. [Pagination](#pagination)
16. [Environment Variables](#environment-variables)
17. [Internationalization](#internationalization)
18. [Files to Keep in Sync](#files-to-keep-in-sync)

---

## Site Settings

Core Hugo configuration at the top of `hugo.toml`:

```toml
baseURL = 'https://focuswithjustin.com/'
languageCode = 'en-us'
title = 'Focus with Justin'
theme = 'airfold'
buildDrafts = false
enableRobotsTXT = true
```

| Setting | Description |
|---------|-------------|
| `baseURL` | Production URL (used for absolute links, sitemap, RSS) |
| `languageCode` | BCP 47 language tag for HTML lang attribute |
| `title` | Site title (used in browser tab, SEO) |
| `theme` | Active theme directory name |
| `buildDrafts` | Include draft content in builds |
| `enableRobotsTXT` | Generate robots.txt from template |

---

## Site Parameters

All custom parameters under `[params]`:

### Basic Metadata

```toml
[params]
  description = 'Cybersecurity professional, CMMC assessor, and creative. Helping organizations protect their assets while exploring photography and technology.'
  keywords = 'cybersecurity, CMMC, CISSP, security consulting, photography, Justin Weeks'
  heroImage = '/images/hero.png'
  favicon = '/favicon.ico'
  copyrightStartYear = 2024
  footerText = 'Focus with Justin™. All rights reserved...'
  tagline = 'Cybersecurity professional, CMMC assessor, and creative...'
  socialCtaText = 'Explore my work and connect with me through social media.'
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `description` | string | Default meta description for pages without their own |
| `keywords` | string | Default meta keywords (comma-separated) |
| `heroImage` | string | Hero image path for homepage |
| `favicon` | string | Favicon path (ICO format) |
| `copyrightStartYear` | int | First year for copyright range |
| `footerText` | string | Footer disclaimer (HTML allowed) |
| `tagline` | string | Homepage tagline below title |
| `socialCtaText` | string | Call-to-action above social icons |

### Credly Integration

```toml
[params]
  credlyUrl = 'https://www.credly.com/users/68218b7c-6c76-40d1-8bad-25e7c30a8565'
```

Used to link to full certification badge gallery on Credly.

### Tool Categories

```toml
[params]
  toolCategories = ['SIEM & Observability', 'Container', 'Development', 'RMM', 'Threat Informed Defense', 'Virtualization']
```

Categories for filtering tools on the tools list page. Must match categories in `data/tools.json`.

---

## SEO Configuration

### Author Information

```toml
[params.author]
  name = 'Justin'
  email = 'noreply@focuswithjustin.com'
  url = 'https://focuswithjustin.com/about/'
  jobTitle = 'Cybersecurity Professional & Purveyor of Esoterica'
  sameAs = [
    'https://www.linkedin.com/in/focuswithjustin/',
    'https://x.com/FocuswithJustin',
    'https://www.instagram.com/focuswithjustin/',
    'https://github.com/focuswithjustin'
  ]
```

| Parameter | Description |
|-----------|-------------|
| `name` | Author name for Person schema |
| `email` | Contact email (can be noreply) |
| `url` | Link to about page |
| `jobTitle` | Job title for Person schema |
| `sameAs` | Social profile URLs for schema.org sameAs property |

### Organization Information

```toml
[params.organization]
  name = 'Focus with Justin'
  url = 'https://focuswithjustin.com/'
  logo = '/images/hero.png'
```

Used for Organization schema in JSON-LD structured data.

### SEO Settings

```toml
[params.seo]
  defaultImage = '/images/hero.png'
  twitterSite = 'FocuswithJustin'
  twitterCreator = 'FocuswithJustin'
  themeColor = '#7a00b0'
  themeColorDark = '#1a1a1a'
  enableStructuredData = true
  wordsPerMinute = 200
  # googleSiteVerification = 'your-verification-code'
  # bingSiteVerification = 'your-verification-code'
```

| Parameter | Description |
|-----------|-------------|
| `defaultImage` | Fallback Open Graph image |
| `twitterSite` | Twitter handle for site (without @) |
| `twitterCreator` | Twitter handle for content creator |
| `themeColor` | Mobile browser theme color (light mode) |
| `themeColorDark` | Mobile browser theme color (dark mode) |
| `enableStructuredData` | Enable JSON-LD structured data |
| `wordsPerMinute` | Reading speed for estimated read time |
| `googleSiteVerification` | Google Search Console verification |
| `bingSiteVerification` | Bing Webmaster verification |

---

## Email Configuration

Reference values for email worker coordination:

```toml
[params.email]
  domain = 'focuswithjustin.com'
  from = 'noreply@focuswithjustin.com'
  to = 'jmw@focuswithjustin.com'
  senderName = 'Focus with Justin Contact Form'
```

| Parameter | Description |
|-----------|-------------|
| `domain` | Domain for Message-ID headers |
| `from` | Sender email address |
| `to` | Recipient email address |
| `senderName` | Display name for sender |

**Important:** These are reference values. The actual email configuration is in `workers/email-sender/wrangler.toml` and environment variables.

---

## CAPTCHA Configuration

The contact form supports multiple CAPTCHA providers:

```toml
[params.captcha]
  provider = "turnstile"
  # siteKey = "your-site-key-here"      # Or use env variable
  # secretKey = "your-secret-key-here"  # Use env variable (recommended)
```

### Supported Providers

| Provider | Description |
|----------|-------------|
| `turnstile` | Cloudflare Turnstile (recommended for Cloudflare Pages) |
| `recaptcha-v2` | Google reCAPTCHA v2 (checkbox challenge) |
| `recaptcha-v3` | Google reCAPTCHA v3 (invisible, score-based) |
| `hcaptcha` | hCaptcha (privacy-focused) |
| `friendly-captcha` | Friendly Captcha (GDPR compliant) |
| `disabled` | No CAPTCHA protection |

### Environment Variables by Provider

| Provider | Site Key (Build Time) | Secret Key (Runtime) |
|----------|----------------------|---------------------|
| Turnstile | `HUGO_TURNSTILE_SITE_KEY` | `TURNSTILE_SECRET_KEY` |
| reCAPTCHA | `HUGO_RECAPTCHA_SITE_KEY` | `RECAPTCHA_SECRET_KEY` |
| hCaptcha | `HUGO_HCAPTCHA_SITE_KEY` | `HCAPTCHA_SECRET_KEY` |
| Friendly Captcha | `HUGO_FRIENDLY_CAPTCHA_SITE_KEY` | `FRIENDLY_CAPTCHA_SECRET_KEY` |

**Note:** Site keys need the `HUGO_` prefix for Hugo's build-time security policy. Secret keys are used at runtime by Cloudflare Pages Functions.

### Provider Setup Links

- **Turnstile:** https://dash.cloudflare.com/?to=/:account/turnstile
- **reCAPTCHA:** https://www.google.com/recaptcha/admin
- **hCaptcha:** https://dashboard.hcaptcha.com/
- **Friendly Captcha:** https://friendlycaptcha.com/

---

## Resume Section

Section titles for the resume page:

```toml
[params.resume]
  securityPrograms = 'Security Programs'
  compliance = 'Compliance'
  technology = 'Technology'
  toolsSkills = 'Tools, Technologies & Skills'
  certifications = 'Certifications'
  experience = 'Experience'
  education = 'Education'
```

These titles appear as headings on the resume page layout.

---

## Mermaid Diagrams

Configuration for Mermaid.js diagram support:

```toml
[params.mermaid]
  selfHosted = true
```

When `selfHosted = true`, Mermaid is loaded from `/js/mermaid.min.js` instead of CDN.

---

## Carousel Navigation

Configure the homepage carousel arrow and dot navigation styles:

```toml
[params.carousel]
  theatreMode = true
  arrowStyle = "triangle"
  arrowPosition = "inside"
  dotPosition = "outside"
  tabRounded = true
  bgOpacity = 89
  autoAdvance = true
  autoAdvanceDelay = 8000
  peekMode = false
```

### Theatre Mode

When `theatreMode = true`, all settings are overridden with cinematic defaults:

- Tab-style arrows with rounded corners
- Arrows positioned outside on left/right sides
- Contained peek mode (partial adjacent slides visible within container)
- Auto-advance enabled
- Dots positioned outside

This provides a polished, presentation-style experience. Set to `false` to use the individual settings below.

### Arrow Style

| Value | Description |
|-------|-------------|
| `triangle` | Filled triangle arrows (default) |
| `tab` | Rectangular tab buttons with smaller triangles |

### Arrow Position

| Value | Description |
|-------|-------------|
| `inside` | Absolute positioned on left/right edges inside carousel (default) |
| `outside` | Placed in the bottom navigation bar with dots |
| `outside-side` | Flanking the carousel on left/right sides, close to container |
| `outside-wide` | Flanking the carousel on left/right sides, far from container |
| `outside-outside` | Fixed at viewport edges (on site background, outside the paper card). Hidden on screens < 1200px |
| `outside-outside-bottom` | At the far edges of the navigation bar |

### Dot Position

| Value | Description |
|-------|-------------|
| `inside` | Centered at bottom inside the carousel |
| `outside` | Placed in the bottom navigation bar (default) |

### Other Options

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `theatreMode` | bool | `false` | Override all settings with cinematic defaults (enables grey/scale slide transitions, opaque arrows) |
| `tabRounded` | bool | `true` | Add rounded corners to tab-style arrows |
| `bgOpacity` | int | `89` | Background opacity percentage (0-100) |
| `autoAdvance` | bool | `true` | Automatically advance slides |
| `autoAdvanceDelay` | int | `8000` | Delay between slides in milliseconds |
| `peekMode` | string/bool | `false` | `false`, `"overflow"` (extends outside), or `"contained"` (within bounds) |

---

## PGP Encryption

Optional client-side PGP encryption for contact form messages:

```toml
[params]
  pgpPublicKey = '''
-----BEGIN PGP PUBLIC KEY BLOCK-----
...your public key here...
-----END PGP PUBLIC KEY BLOCK-----
'''
```

When set, messages are encrypted with OpenPGP.js before submission.

---

## Navigation Menus

### Menu Item Structure

```toml
[menus]
  [[menus.main]]
    name = 'Home'
    pageRef = '/'
    weight = 10
    [menus.main.params]
      display = 3
```

| Field | Description |
|-------|-------------|
| `name` | Display text |
| `pageRef` | Page path (relative to content/) |
| `weight` | Sort order (lower = first) |
| `display` | Display mode (see below) |

### Display Modes

| Value | Meaning |
|-------|---------|
| `1` | Header only |
| `2` | Footer only |
| `3` | Header and footer (default) |
| `4` | Disabled (hidden) |

### Current Menu Configuration

```toml
[menus]
  [[menus.main]]
    name = 'Home'
    pageRef = '/'
    weight = 10
    [menus.main.params]
      display = 3

  [[menus.main]]
    name = 'About'
    pageRef = '/about'
    weight = 20
    [menus.main.params]
      display = 3

  [[menus.main]]
    name = 'Projects'
    pageRef = '/projects'
    weight = 30
    [menus.main.params]
      display = 4  # Disabled

  [[menus.main]]
    name = 'Religion'
    pageRef = '/religion'
    weight = 50
    [menus.main.params]
      display = 3

  [[menus.main]]
    name = 'Esoterica'
    pageRef = '/esoterica'
    weight = 52
    [menus.main.params]
      display = 3

  [[menus.main]]
    name = 'Résumé'
    pageRef = '/resume'
    weight = 55
    [menus.main.params]
      display = 2  # Footer only

  [[menus.main]]
    name = 'Contact'
    pageRef = '/contact'
    weight = 60
    [menus.main.params]
      display = 3
```

---

## Module Mounts

Module mounts map theme extension content to site URLs:

```toml
[module]
  # Site content (required)
  [[module.mounts]]
    source = 'content'
    target = 'content'

  # Certifications under /resume/certifications/
  [[module.mounts]]
    source = 'themes/airfold/extensions/certifications/content/certifications'
    target = 'content/resume/certifications'

  # Skills under /resume/skills/
  [[module.mounts]]
    source = 'themes/airfold/extensions/certifications/content/skills'
    target = 'content/resume/skills'

  # Tools under /resume/tools/
  [[module.mounts]]
    source = 'themes/airfold/extensions/tools/content/tools'
    target = 'content/resume/tools'

  # Projects under /projects/
  [[module.mounts]]
    source = 'themes/airfold/extensions/portfolio/content/projects'
    target = 'content/projects'

  # Bibles under /religion/bibles/
  [[module.mounts]]
    source = 'themes/airfold/extensions/religion/content/bibles'
    target = 'content/religion/bibles'
```

### How Mounts Work

1. Hugo merges content from mounted source directories
2. `_content.gotmpl` templates in source directories generate pages from JSON
3. Generated pages appear at the target URL path
4. Site content takes precedence over theme content

---

## Output Formats

Custom output formats for RSS and XSL stylesheets:

```toml
# Home page outputs
[outputs]
  home = ['HTML', 'RSS', 'SitemapXSL', 'RSSXSL']
  section = ['HTML', 'RSS']

# Custom media types
[mediaTypes."application/xslt+xml"]
  suffixes = ["xsl"]

# RSS feed configuration
[outputFormats.RSS]
  mediatype = 'application/rss+xml'
  baseName = 'feed'

# XSL stylesheet for sitemap
[outputFormats.SitemapXSL]
  mediaType = 'application/xslt+xml'
  baseName = 'sitemap-style'
  isPlainText = true
  notAlternative = true
  path = ''

# XSL stylesheet for RSS
[outputFormats.RSSXSL]
  mediaType = 'application/xslt+xml'
  baseName = 'rss-style'
  isPlainText = true
  notAlternative = true
  path = ''
```

### Generated Files

| Output | File | Purpose |
|--------|------|---------|
| RSS | `/feed.xml` | RSS feed for blog posts |
| SitemapXSL | `/sitemap-style.xsl` | Stylesheet for sitemap |
| RSSXSL | `/rss-style.xsl` | Stylesheet for RSS feed |
| Sitemap | `/sitemap.xml` | XML sitemap for search engines |

---

## Markup Settings

Configure markdown rendering:

```toml
[markup]
  [markup.goldmark]
    [markup.goldmark.renderer]
      unsafe = true  # Allow raw HTML in markdown
```

**Why `unsafe = true`?**

- Needed for custom HTML in markdown (chapter grids, special formatting)
- Content is trusted (no user-generated content)

### Syntax Highlighting

```toml
[markup.highlight]
  style = 'monokai'
  lineNos = true
  tabWidth = 2
```

---

## Taxonomies

Enable tag taxonomy for content categorization:

```toml
[taxonomies]
  tag = 'tags'
```

Usage in frontmatter:
```yaml
tags: ["security", "CMMC", "compliance"]
```

---

## Pagination

Configure list page pagination:

```toml
[pagination]
  pagerSize = 3
```

Small page size for better UX on blog listings.

---

## Environment Variables

### Hugo Build Time

Variables with `HUGO_` prefix are available during Hugo build:

| Variable | Purpose |
|----------|---------|
| `HUGO_TURNSTILE_SITE_KEY` | Turnstile CAPTCHA site key |
| `HUGO_RECAPTCHA_SITE_KEY` | reCAPTCHA site key |
| `HUGO_HCAPTCHA_SITE_KEY` | hCaptcha site key |
| `HUGO_FRIENDLY_CAPTCHA_SITE_KEY` | Friendly Captcha site key |
| `HUGO_ENV` | Build environment (`production` or not) |

### Cloudflare Pages Functions

Set in Cloudflare Pages dashboard → Settings → Environment Variables:

| Variable | Purpose |
|----------|---------|
| `TURNSTILE_SECRET_KEY` | CAPTCHA token verification |
| `RECAPTCHA_SECRET_KEY` | reCAPTCHA verification |
| `HCAPTCHA_SECRET_KEY` | hCaptcha verification |
| `FRIENDLY_CAPTCHA_SECRET_KEY` | Friendly Captcha verification |
| `ALLOWED_ORIGINS` | Comma-separated CORS origins |
| `ERROR_MISSING_CAPTCHA` | Custom error message |
| `ERROR_INVALID_CAPTCHA` | Custom error message |
| `ERROR_RATE_LIMITED` | Custom error message |

### Email Worker

Set in `workers/email-sender/` via `wrangler secret`:

| Variable | Purpose |
|----------|---------|
| `TURNSTILE_SECRET_KEY` | HMAC verification (must match Pages) |
| `EMAIL_FROM` | Sender email address |
| `EMAIL_TO` | Recipient email address |
| `EMAIL_SENDER_NAME` | Sender display name |
| `EMAIL_DOMAIN` | Domain for Message-ID |

---

## Internationalization

UI strings are stored in `i18n/en.toml` for translation support.

### String Categories

| Category | Examples |
|----------|----------|
| Accessibility | `skipToContent` |
| Navigation | `readMore`, `previous`, `next`, `backToHome` |
| Tags | `allTags`, `exploreTopics` |
| Reading | `minRead` |
| Resume | `backToResume`, `verifyOnCredly` |
| Contact | `nameLabel`, `sendMessage`, `encryptingStatus` |
| Religion | `bibles`, `backToBible`, `compareTranslations` |
| Search | `searchBible`, `noResults` |
| Projects | `projectFilters`, `noProjectsMatch` |
| 404 | `notFoundTitle`, `notFoundMessage` |

### Usage in Templates

```html
{{ i18n "readMore" }}
<!-- Output: Read more -->

{{ i18n "pageOf" 1 10 }}
<!-- Output: Page 1 of 10 -->

{{ i18n "articles" 1 }}
<!-- Output: article -->

{{ i18n "articles" 5 }}
<!-- Output: articles (plural form) -->
```

### Adding New Strings

1. Add to `i18n/en.toml`:

```toml
[newString]
other = "Your new string"
```

2. For plurals:

```toml
[items]
one = "item"
other = "items"
```

3. Use in templates:

```html
{{ i18n "newString" }}
{{ i18n "items" .Count }}
```

### Complete i18n Reference

```toml
# Accessibility
[skipToContent]
other = "Skip to content"

# General navigation
[readMore]
other = "Read more"

[previous]
other = "Previous"

[next]
other = "Next"

[pageOf]
other = "Page %d of %d"

[backToHome]
other = "Back to Home"

[viewAll]
other = "View All"

[viewDetails]
other = "View Details"

[all]
other = "All"

# Tags
[allTags]
other = "All Tags"

[tagsAZ]
other = "Tags A-Z"

[exploreTopics]
other = "Explore Other Topics"

[exploreProjects]
other = "Explore Other Projects"

[articles]
one = "article"
other = "articles"

[tags]
one = "tag"
other = "tags"

# Reading time
[minRead]
other = "min read"

# Resume section
[backToResume]
other = "Back to Resume"

[allCertifications]
other = "All Certifications"

[allTools]
other = "All Tools"

[allSkills]
other = "All Skills"

[relatedCertifications]
other = "Related Certifications"

[viewResume]
other = "View Resume"

[viewProject]
other = "View Project"

[verifyOnCredly]
other = "Verify on Credly"

[viewAllBadgesOnCredly]
other = "View All Badges on Credly"

[noArticlesWithTag]
other = "No articles with this tag."

[skills]
other = "Skills"

[expires]
other = "Expires"

[issued]
other = "Issued"

[officialWebsite]
other = "Official Website"

[filterByTag]
other = "Filter by Tag"

[noImage]
other = "No image"

[noProjectsYet]
other = "No projects yet."

[noProjectsMatch]
other = "No projects match this filter."

[recentArticles]
other = "Recent Articles"

[noArticlesYet]
other = "No articles yet."

# 404 Page
[notFoundTitle]
other = "404"

[notFoundHeading]
other = "Page not found"

[notFoundMessage]
other = "The page you're looking for doesn't exist or has been moved."

# Contact form
[encryptedPlaceholder]
other = "[Encrypted - see encrypted_message field]"

[nameLabel]
other = "Name"

[emailLabel]
other = "Email"

[subjectLabel]
other = "Subject"

[messageLabel]
other = "Message"

[sendMessage]
other = "Send Message"

[pgpNotice]
other = "Your message will be encrypted with PGP"

[orConnectElsewhere]
other = "Or connect with me elsewhere"

[encryptingStatus]
other = "Encrypting..."

[sendingStatus]
other = "Sending..."

[encryptionFailed]
other = "Failed to encrypt message. Please try again."

[clientLabel]
other = "Client"

# Religion/Bible navigation
[bibleNavigation]
other = "Bible Navigation"

[bibles]
other = "Bible Translations"

[biblesDescription]
other = "Browse historic Bible translations"

[backToReligion]
other = "Back to Religion"

[backToBook]
other = "Back to Book"

[backToBible]
other = "Back to Bible"

[allBibles]
other = "All Bibles"

[allChapters]
other = "All Chapters"

[selectBible]
other = "Select Bible"

[selectBook]
other = "Select Book"

[selectChapter]
other = "Select Chapter"

[chapter]
other = "Ch"

[previousChapter]
other = "Previous Chapter"

[nextChapter]
other = "Next Chapter"

[unavailable]
other = "unavailable"

# Compare translations
[compareTranslations]
other = "Compare Translations"

[selectTranslations]
other = "Select Translations"

[selectTranslationsHint]
other = "Select 2-4 translations to compare side-by-side."

[selectPassage]
other = "Select Passage"

[compare]
other = "Compare"

[selectPassagePrompt]
other = "Select translations and a passage to begin comparing."

[maxTranslationsWarning]
other = "Maximum 4 translations can be compared at once."

# Bible search
[searchBible]
other = "Search Bible"

[searchQuery]
other = "Search for"

[searchPlaceholder]
other = "Enter words or phrase..."

[search]
other = "Search"

[caseSensitive]
other = "Case sensitive"

[wholeWord]
other = "Whole word"

[searching]
other = "Searching..."

[searchPrompt]
other = "Enter a search term to find verses across the Bible."

[searchResults]
other = "Search Results"

[noResults]
other = "No results found"

# PGP encryption status
[keyExpired]
other = "The encryption key has expired. Message cannot be encrypted."

[keyInvalid]
other = "Invalid encryption key. Message cannot be encrypted."

# Project filters
[projectFilters]
other = "Project filters"

[showAllProjects]
other = "Show all projects"

[filterByTagLabel]
other = "Filter by tag"

[projectScreenshot]
other = "Screenshot of"

[certificationBadge]
other = "certification badge"
```

---

## Files to Keep in Sync

These files must be kept synchronized:

| hugo.toml Section | External File | Notes |
|-------------------|---------------|-------|
| `[params.email]` | `workers/email-sender/wrangler.toml` | Email addresses |
| `[params.captcha]` | Pages environment variables | Secret keys |
| `[params.captcha]` | `workers/email-sender/` secrets | Same secret for HMAC |

---

## Related Documentation

- [Architecture Guide](architecture.md) - System design and patterns
- [Development Guide](development.md) - Setup and workflows
- [Contact Form Guide](contact-form.md) - CAPTCHA and email setup
- [Deployment Guide](deployment.md) - Cloudflare Pages configuration
