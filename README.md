# PDF Studio

Web application for creating and managing PDF documents with a page-by-page WYSIWYG editor.

> This project was generated with the assistance of AI (Claude, Anthropic).
> See [LICENSE](LICENSE) for terms of use and disclaimer.

## Features

- **Page editor** — WYSIWYG editor with A4 page layout, rich text formatting, tables, images
- **PDF generation** — Gotenberg (headless Chromium) converts pages to PDF with embedded images
- **Version control** — each document tracks versions; create new versions, browse history
- **File management** — upload assets (images, files), download generated PDFs
- **Document management** — rename documents, publish/unpublish, search by title
- **Role-based access** — Admin, Editor, Viewer roles with strict ownership enforcement
- **Auto-save** — editor auto-saves every 3 seconds; also saves on page switch and Ctrl+S
- **Admin protection** — admin cannot deactivate themselves or remove their own admin role
- **Secure sessions** — HttpOnly cookies, CSRF protection, session regeneration

## Prerequisites

- Docker & Docker Compose

## Quick Start

```bash
# 1. Configure
cp .env.example .env
# Edit .env if needed (defaults work for local dev)

# 2. Start everything
docker compose up -d --build

# 3. Open in browser
open http://localhost:8080
```

To rebuild after code changes:
```bash
docker compose up -d --build
```

To reset the admin user if locked out:
```bash
docker compose exec postgres psql -U pdfstudio -d pdfstudio \
  -c "UPDATE users SET active = true, role = 'admin' WHERE email = 'admin@pdfstudio.local';"
```

## Default Admin Login

| Field    | Value                   |
|----------|-------------------------|
| Email    | `admin@pdfstudio.local` |
| Password | `Admin123!`             |

## Usage

### Create a user (as Admin)

1. Log in as admin
2. Click **Admin Panel** in the navbar
3. Click **+ Create User**
4. Set email, password, and role (editor/viewer)

### Create a document (as Editor)

1. Log in as editor (or admin)
2. Click **+ New Document** on the dashboard
3. Type a title and confirm
4. You're taken to the editor

### Rename a document

- Click the **document title** in the editor navbar — a prompt will appear to enter a new name

### Edit pages

- Use the **toolbar** at the bottom for formatting (Bold, Italic, Headings, Lists, Tables, Images)
- **Add Page** button in the sidebar creates new pages
- Click pages in the sidebar to switch between them — the previous page is saved automatically
- **Ctrl+S** / **Cmd+S** saves the current page manually
- Auto-save runs every 3 seconds when there are unsaved changes
- Upload images via **Upload File** button, then insert into the page

### Generate PDF

1. In the editor, click **Generate PDF**
2. Wait for Gotenberg to convert (usually 2-5 seconds)
3. Images are embedded into the PDF as base64 — no broken links
4. The PDF appears in the **Files** sidebar section and in the document details on the dashboard
5. Click the PDF link to download

### Publish / Unpublish

- Click **Publish** (green button) to set document status to `published`
- Click **To Draft** to return it to `draft` status

### Create a new version

- Click **New Version** in the editor — copies all pages from the current version into a new revision

## Architecture

```
pdf-studio/
├── cmd/server/          # Entry point, routing
├── internal/
│   ├── config/          # Environment config
│   ├── database/        # PostgreSQL connection & migrations
│   ├── handlers/        # HTTP handlers (auth, admin, docs, pages, files, views)
│   ├── middleware/       # Auth, CSRF, logging, security headers
│   ├── models/          # Data models (User, Document, Page, File, Session)
│   └── services/        # Business logic (auth, storage fs/db, PDF with image inlining)
├── migrations/          # SQL migrations (auto-applied on startup)
├── web/
│   ├── templates/       # HTML templates (login, dashboard, editor, admin)
│   └── static/css/      # Stylesheets
├── Dockerfile           # Multi-stage Go build
├── docker-compose.yml   # app + postgres + gotenberg
└── .env.example
```

## API Routes

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/auth/login` | No | Login |
| POST | `/auth/logout` | Yes | Logout |
| GET | `/me` | Yes | Current user info |
| GET | `/admin/users` | Admin | List users |
| POST | `/admin/users` | Admin | Create user |
| GET | `/admin/users/:id` | Admin | Get user |
| PUT | `/admin/users/:id` | Admin | Update user (with self-protection) |
| GET | `/api/documents` | Yes | List own documents (?q= for search) |
| POST | `/api/documents` | Editor+ | Create document |
| GET | `/api/documents/:id` | Yes | Get document with versions and files |
| PUT | `/api/documents/:id` | Editor+ | Update title / status |
| DELETE | `/api/documents/:id` | Editor+ | Delete document |
| POST | `/api/documents/:id/versions` | Editor+ | Create new version |
| POST | `/api/documents/:id/generate-pdf` | Editor+ | Generate PDF |
| GET | `/api/documents/:id/files` | Yes | List document files |
| GET | `/api/versions/:vid/pages` | Yes | List pages |
| POST | `/api/versions/:vid/pages` | Editor+ | Add page |
| PUT | `/api/versions/:vid/pages/reorder` | Editor+ | Reorder pages |
| GET | `/api/pages/:pid` | Yes | Get page |
| PUT | `/api/pages/:pid` | Editor+ | Update page content |
| DELETE | `/api/pages/:pid` | Editor+ | Delete page |
| POST | `/api/files/upload` | Editor+ | Upload file |
| GET | `/api/files/:id/download` | Yes | Download file |

## Security Notes

- **No self-registration** — only admins create users
- **Ownership enforcement** — all document/file access checks `owner_id` (IDOR protection)
- **Admin self-protection** — cannot deactivate yourself or remove your own admin role
- **Secure cookies** — HttpOnly, SameSite=Lax, Secure in production
- **CSRF protection** — token-based for forms, origin check for JSON API
- **Gotenberg isolated** — not exposed externally (internal Docker network only)
- **Chromium JS disabled** — Gotenberg runs with `--chromium-disable-javascript`
- **Security headers** — X-Content-Type-Options, X-Frame-Options, X-XSS-Protection
- **Session invalidation** — sessions destroyed on role change, deactivation, and logout
- **Images in PDF** — inlined as base64 data URIs, Gotenberg has no access to app server
- **Rate limiting** — not included; add a reverse proxy (nginx, Caddy) in production

## Environment Variables

See [.env.example](.env.example) for all available settings.

Key variables:
| Variable | Default | Description |
|----------|---------|-------------|
| `APP_PORT` | `8080` | Server port |
| `APP_SECRET` | — | Session secret (change in production!) |
| `APP_ENV` | `development` | Set to `production` for Secure cookies |
| `STORAGE_MODE` | `fs` | `fs` for filesystem, `db` for database (bytea) |
| `MAX_UPLOAD_SIZE` | `10485760` | Max upload size in bytes (10 MB) |
| `ADMIN_EMAIL` | `admin@pdfstudio.local` | Seed admin email |
| `ADMIN_PASSWORD` | `Admin123!` | Seed admin password |

## AI-Generated Code

This project was generated with the assistance of **AI** (Claude, Anthropic).
The code may contain errors or security vulnerabilities. Review and test
thoroughly before any use. Use at your own risk.

## License

[MIT](LICENSE) — free for any use, including commercial. See [LICENSE](LICENSE) for full terms.
