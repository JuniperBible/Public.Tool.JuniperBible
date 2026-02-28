# Contact Form Guide

The contact form at `/contact/` uses a secure architecture with CAPTCHA verification, HMAC authentication, and optional PGP encryption.

## Architecture

```
Browser (contact form)
    │
    │ POST /api/contact
    ▼
Cloudflare Pages Function (functions/api/contact.js)
    │
    │ CAPTCHA verification
    │ HMAC signature
    │
    │ Service Binding
    ▼
Email Worker (workers/email-sender/)
    │
    │ Signature verification
    │ Format email
    │
    │ Email Routing API
    ▼
Destination Inbox
```

## Security Features

### CAPTCHA Protection

Multiple CAPTCHA providers are supported:

| Provider | Config Value | Best For |
|----------|--------------|----------|
| Cloudflare Turnstile | `turnstile` | Cloudflare Pages (recommended) |
| Google reCAPTCHA v2 | `recaptcha-v2` | Checkbox challenge |
| Google reCAPTCHA v3 | `recaptcha-v3` | Invisible, score-based |
| hCaptcha | `hcaptcha` | Privacy-focused |
| Friendly Captcha | `friendly-captcha` | GDPR compliant |
| Disabled | `disabled` | No CAPTCHA |

### HMAC Authentication

The Pages Function signs requests to the email worker:

```
signature = HMAC-SHA256(secret, timestamp + payload)
```

The worker verifies:

1. Signature matches
2. Timestamp within 5-minute window (replay protection)

### PGP Encryption (Optional)

When configured, messages are encrypted client-side using OpenPGP.js before submission.

## Setup

### 1. Get CAPTCHA Keys

**Turnstile (Cloudflare):**

- Go to https://dash.cloudflare.com/ → Turnstile
- Create a site widget
- Copy Site Key and Secret Key

**Other providers:**

- reCAPTCHA: https://www.google.com/recaptcha/admin
- hCaptcha: https://dashboard.hcaptcha.com/
- Friendly Captcha: https://friendlycaptcha.com/

### 2. Configure hugo.toml

```toml
[params.captcha]
  provider = "turnstile"
  siteKey = "0x4AAAA..."
  # secretKey - use environment variable instead
```

### 3. Set Environment Variables

**Cloudflare Pages Settings → Environment Variables:**

| Variable | Purpose |
|----------|---------|
| `HUGO_TURNSTILE_SITE_KEY` | Site key for Hugo build |
| `TURNSTILE_SECRET_KEY` | Secret for API verification |

Note: Site keys need `HUGO_` prefix for Hugo access.

### 4. Deploy Email Worker

```bash
cd workers/email-sender
npm install
npx wrangler secret put TURNSTILE_SECRET_KEY  # Enter same secret
npx wrangler deploy
```

### 5. Add Service Binding

**Cloudflare Pages Settings → Functions → Service Bindings:**

| Variable name | Service |
|---------------|---------|
| `EMAIL_WORKER` | `focuswithjustin-email-sender` |

### 6. Enable Email Routing

**Cloudflare Dashboard → Email → Email Routing:**

1. Enable Email Routing
2. Add destination email address
3. Verify destination via email link

### 7. Redeploy

Push a commit or retry deployment to apply changes.

## Configuration Reference

### Pages Function (functions/api/contact.js)

| Variable | Required | Description |
|----------|----------|-------------|
| `TURNSTILE_SECRET_KEY` | One of these | Cloudflare Turnstile secret |
| `RECAPTCHA_SECRET_KEY` | | Google reCAPTCHA secret |
| `HCAPTCHA_SECRET_KEY` | | hCaptcha secret |
| `FRIENDLY_CAPTCHA_SECRET_KEY` | | Friendly Captcha secret |
| `ALLOWED_ORIGINS` | No | Comma-separated origins |
| `ERROR_*` | No | Custom error messages |

### Email Worker (workers/email-sender/)

| Variable | Required | Description |
|----------|----------|-------------|
| `TURNSTILE_SECRET_KEY` | Yes | Must match Pages Function |
| `EMAIL_FROM` | No | Sender address |
| `EMAIL_TO` | No | Recipient address |
| `EMAIL_SENDER_NAME` | No | Sender display name |
| `EMAIL_DOMAIN` | No | Domain for message IDs |

### wrangler.toml

```toml
name = "focuswithjustin-email-sender"
main = "src/index.js"
compatibility_date = "2024-01-01"

[vars]
EMAIL_FROM = "noreply@focuswithjustin.com"
EMAIL_TO = "contact@focuswithjustin.com"
EMAIL_SENDER_NAME = "Focus with Justin Contact Form"
EMAIL_DOMAIN = "focuswithjustin.com"
```

## PGP Encryption Setup

### 1. Generate Key Pair

```bash
gpg --full-generate-key
# Choose RSA and RSA, 4096 bits
```

### 2. Export Public Key

```bash
gpg --armor --export your@email.com
```

### 3. Add to hugo.toml

```toml
[params]
  pgpPublicKey = '''
-----BEGIN PGP PUBLIC KEY BLOCK-----
...your key...
-----END PGP PUBLIC KEY BLOCK-----
'''
```

### 4. Decrypt Messages

```bash
gpg --decrypt message.asc
```

## Security Testing

Run the security test suite:

```bash
# Test production
npm run test:security

# Test local dev
npm run test:security:local
```

### What It Tests

1. CAPTCHA widget renders on page
2. Submit button disabled until CAPTCHA complete
3. API rejects requests without token
4. API rejects fake tokens
5. API requires provider field
6. Worker rejects unsigned requests
7. Worker rejects bad signatures
8. Worker rejects expired timestamps

## Troubleshooting

### "Failed to send email"

1. Check worker deployed: `npx wrangler deployments list`
2. Verify service binding (not environment variable)
3. Check Email Routing enabled and verified
4. View function logs in Cloudflare Dashboard

### CAPTCHA not appearing

1. Check `HUGO_TURNSTILE_SITE_KEY` set
2. Verify provider matches key type
3. Check browser console for errors

### Worker signature errors

1. Ensure same secret in Pages and Worker
2. Check system clocks synchronized
3. Verify secret set via `wrangler secret`

## Related Documentation

- [Deployment Guide](deployment.md)
- [Configuration Reference](configuration.md)
- [Architecture Guide](architecture.md)
