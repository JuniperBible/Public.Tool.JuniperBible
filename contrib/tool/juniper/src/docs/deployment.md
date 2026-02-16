# Deployment Guide

The site deploys automatically via Cloudflare Pages on push to main. This guide covers the complete deployment architecture and setup.

## Architecture Overview

```
GitHub Repository
    │
    │ Push to main
    ▼
Cloudflare Pages Build
    │
    │ Hugo build with Tailwind
    ▼
Cloudflare Pages CDN ────────┬─────────────────┐
    │                        │                 │
    │ Static HTML/CSS/JS     │ Service         │ Email
    │                        │ Binding         │ Routing
    ▼                        ▼                 ▼
Browser ◄─── /api/* ──► Pages Function ──► Email Worker ──► Inbox
```

## Prerequisites

- Cloudflare account with Pages enabled
- GitHub repository connected to Cloudflare Pages
- Email Routing enabled for your domain

## Environment Setup

### Cloudflare Pages Settings

Navigate to your project in Cloudflare Pages dashboard.

#### Build Settings

| Setting | Value |
|---------|-------|
| Framework preset | None |
| Build command | `npm run build` |
| Build output directory | `public` |
| Node.js version | 22 |

#### Environment Variables

**Production and Preview:**

| Variable | Purpose | Required |
|----------|---------|----------|
| `HUGO_TURNSTILE_SITE_KEY` | CAPTCHA widget site key | Yes |
| `TURNSTILE_SECRET_KEY` | CAPTCHA verification secret | Yes |
| `ALLOWED_ORIGINS` | Comma-separated CORS origins | No |

### Email Worker Setup

The email worker handles contact form submissions via Cloudflare Email Routing.

#### 1. Deploy Worker

```bash
cd workers/email-sender
npm install
npx wrangler deploy
```

#### 2. Configure Secrets

```bash
# Must match the secret in Pages
npx wrangler secret put TURNSTILE_SECRET_KEY
```

#### 3. Verify Configuration

Check `wrangler.toml` matches your domain:

```toml
name = "focuswithjustin-email-sender"
main = "src/index.js"
compatibility_date = "2024-09-23"

send_email = [
  { name = "EMAIL", destination_address = "you@example.com" }
]

[vars]
EMAIL_DOMAIN = "yoursite.com"
EMAIL_FROM = "noreply@yoursite.com"
EMAIL_TO = "you@yoursite.com"
EMAIL_SENDER_NAME = "Contact Form"
```

### Service Binding

Connect the Pages Function to the Email Worker:

1. Go to **Pages → Your Project → Settings → Functions**
2. Add **Service Binding**:
   - Variable name: `EMAIL_WORKER`
   - Service: Select your email worker

### Email Routing

1. Go to **Cloudflare Dashboard → Email → Email Routing**
2. Enable Email Routing for your domain
3. Add and verify destination email address

## Deployment Process

### Automatic Deployment

Every push to `main` triggers:

1. Cloudflare pulls latest code
2. Runs `npm run build` (Hugo + Tailwind)
3. Deploys `public/` to CDN
4. Updates Functions if changed

### Manual Deployment

Build locally and verify:

```bash
npm run build
ls -la public/
```

Trigger deployment:
```bash
git push origin main
```

Or retry from Cloudflare Dashboard.

### Preview Deployments

Push to any non-main branch for a preview URL:

- Feature branches get unique URLs
- PRs automatically get preview deployments
- Preview URLs expire after 30 days

## Post-Deployment Verification

### Checklist

1. **Site loads** - Visit homepage, check console for errors
2. **Navigation works** - Test all menu links
3. **Contact form** - Submit test message
4. **Bible section** - Check `/religion/bibles/`
5. **SSL** - Verify HTTPS and certificate

### Verify Contact Form

```bash
# Test CAPTCHA endpoint
curl -X POST https://yoursite.com/api/contact \
  -H "Content-Type: application/json" \
  -d '{"name":"Test","email":"test@test.com","message":"Test"}'

# Should return error about missing CAPTCHA token
```

### Check Worker Logs

```bash
npx wrangler tail focuswithjustin-email-sender
```

## Rollback

### Revert to Previous Deploy

1. Go to **Pages → Your Project → Deployments**
2. Find previous successful deployment
3. Click **...** → **Rollback to this deployment**

### Git Revert

```bash
git revert HEAD
git push origin main
```

## Troubleshooting

### Build Failures

**Hugo errors:**
```bash
# Check locally
npm run build
```

**Missing environment variables:**

- Verify `HUGO_TURNSTILE_SITE_KEY` is set in Pages settings
- Check variable is set for correct environment (Production/Preview)

### Contact Form Issues

**"Failed to send email":**

1. Verify service binding exists
2. Check worker is deployed: `npx wrangler deployments list`
3. Verify Email Routing is enabled
4. Check destination email is verified

**CAPTCHA errors:**

1. Verify site key matches domain
2. Check secret key matches between Pages and Worker
3. Ensure Turnstile widget is rendering (browser console)

### 404 Errors

**Missing pages:**

- Check Hugo build output includes expected files
- Verify module mounts in `hugo.toml`
- Check content files have `draft: false`

**Functions not found:**

- Verify `functions/` directory structure
- Check function exports the correct handler

## Custom Domain

### Add Domain

1. Go to **Pages → Your Project → Custom Domains**
2. Add your domain (e.g., `focuswithjustin.com`)
3. Add DNS records as instructed

### DNS Configuration

| Type | Name | Target |
|------|------|--------|
| CNAME | `@` | `your-project.pages.dev` |
| CNAME | `www` | `your-project.pages.dev` |

### SSL Certificate

Cloudflare automatically provisions SSL:

- Certificate issued within minutes
- Automatic renewal
- Full (strict) SSL mode recommended

## Performance Optimization

### Build Cache

Cloudflare caches:

- `node_modules/` - npm dependencies
- `.cache/` - Hugo build cache

### CDN Settings

Recommended settings in Cloudflare:

- **Auto Minify** - HTML, CSS, JS
- **Brotli** - Enabled
- **Early Hints** - Enabled
- **HTTP/3** - Enabled

### Cache Rules

Static assets are automatically cached:

- Images: 1 year
- CSS/JS: 1 year (fingerprinted)
- HTML: No cache (always fresh)

## Security Headers

Pages Functions can add security headers. Current headers:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: strict-origin-when-cross-origin`

See `functions/_middleware.js` if implemented.

## Related Documentation

- [Architecture Guide](architecture.md)
- [Contact Form Guide](contact-form.md)
- [Configuration Reference](configuration.md)
