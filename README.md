# Easy Notes

A simple and secure web-based note editor built with Go and Gin framework. Features JWT authentication, file/folder management, and an intuitive web interface.

## Features

- **JWT Authentication** - Secure login with bcrypt password hashing
- **File Management** - Browse, create, upload, edit, and delete files/folders
- **Tree View** - Visual folder/file tree using jsTree format
- **Dual Web Editors** - Vditor for Markdown and Ace for other text formats
- **Safe Storage** - Symlink-safe paths, atomic saves, and a 50 MB request limit
- **Offline Embedded Assets** - Application and third-party assets are embedded in the binary

Embedded editor dependencies are pinned for reproducible offline builds:

- Vditor `3.11.2` (current official npm `latest`)
- Ace `1.32.6`
- jsTree `3.3.12`
- jQuery `3.6.0`

## Tech Stack

- Go 1.25.3
- Gin Web Framework
- JWT (golang-jwt/jwt/v5)
- bcrypt (golang.org/x/crypto)

## Installation

### Prerequisites
- Go 1.25 or higher

### Build from source

```bash
git clone https://github.com/vj1024/easy-notes.git
cd easy-notes
go mod download
go build
```

## Configuration

Configure via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `JWT_SECRET` | Secret key for JWT signing | `your-default-secret-key-change-in-production` |
| `ADMIN_USERNAME` | Admin username | `admin` |
| `ADMIN_PASSWORD` | Admin password (plaintext, for development) | (required) |
| `ADMIN_PASSWORD_HASH` | Admin password hash (bcrypt, for production) | (optional) |

**Note:** For production, use `ADMIN_PASSWORD_HASH` instead of `ADMIN_PASSWORD`. Generate hash with:
```bash
htpasswd -bnBC 10 "" your-password | tr -d ':\n'
```

## Usage

### Quick Start (using provided script)

```bash
chmod +x start.sh
./start.sh
```

### Manual Start

```bash
export JWT_SECRET="your-secret-key"
export ADMIN_USERNAME="admin"
export ADMIN_PASSWORD="your-password"
./easy-notes
```

The server will start on `http://localhost:8089`. The editor does not require
Internet access: jQuery, jsTree, Vditor, Ace, themes, modes, and supporting
assets are served from the embedded filesystem.

Embedded pages and assets use strong content-based `ETag` values. Browsers keep
the local copy and revalidate it with the server; unchanged files receive a
bodyless HTTP `304 Not Modified`, while changed files get a new ETag and are
downloaded normally.

## API Endpoints

### Public Routes
- `GET /` - Redirect to login
- `GET /login` - Login page
- `GET /editor` - Editor shell; its file APIs require authentication
- `GET /assets/*path` - Embedded CSS, JavaScript, themes, and editor resources
- `POST /api/login` - Authenticate and get JWT token
- `GET /api/check-auth` - Check authentication status

### Authenticated Routes (requires JWT token)
- `GET /api/files?list=true` - List files in tree format
- `GET /api/files?search=keyword` - Search file names only
- `GET /api/files?search=/keyword` - Search file names and supported text-file contents
- `GET /api/files/*path` - Get file content or directory listing
- `PUT /api/files/*path` - Create or replace a file from the request body
- `POST /api/files/*path` - Create or replace a raw-body or multipart file
- `DELETE /api/files/*path` - Delete a file or non-root folder recursively
- `POST /api/mkdir` - Create a folder (`{"path":"folder"}`)
- `POST /api/create-file` - Create a file (`{"path":"note.txt","content":""}`)
- `POST /api/logout` - Logout

All authenticated request bodies are limited to 50 MB. Oversized bodies return
HTTP `413 Request Entity Too Large`. File replacements are atomic: incomplete
requests do not truncate the existing file. Paths containing symbolic links are
rejected.

### Authentication

Include JWT token in requests:
- Header: `Authorization: Bearer <token>`
- Query param: `?token=<token>`

## Project Structure

```
easy-notes/
├── main.go              # Configuration, server setup, auth and middleware
├── handlers_files.go    # File API handlers and search
├── storage.go           # Safe path resolution and atomic file writes
├── jstree.go    # Tree structure generation for file browser
├── web/         # Embedded web assets
│   ├── login.html
│   ├── editor.html
│   ├── assets/
│   │   ├── css/         # Page styles
│   │   ├── js/          # Page behavior
│   │   └── vendor/      # Pinned offline third-party dependencies
│   ├── favicon.ico
│   └── embed.go
├── storage_test.go      # Storage boundary and request-limit tests
├── data/        # Storage directory (created automatically)
├── start.sh     # Startup script
├── go.mod
└── go.sum
```

## License

MIT License - see [LICENSE](LICENSE) file for details

## Author

VJ (c) 2025
