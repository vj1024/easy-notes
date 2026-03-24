# Easy Notes

A simple and secure web-based note editor built with Go and Gin framework. Features JWT authentication, file/folder management, and an intuitive web interface.

## Features

- **JWT Authentication** - Secure login with bcrypt password hashing
- **File Management** - Browse, create, upload, and edit files
- **Tree View** - Visual folder/file tree using jsTree format
- **Web Editor** - Clean browser-based editor interface
- **Embedded Assets** - All web assets embedded in binary

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

The server will start on `http://localhost:8089`

## API Endpoints

### Public Routes
- `GET /` - Redirect to login
- `GET /login` - Login page
- `GET /editor` - Editor page (requires auth)
- `POST /api/login` - Authenticate and get JWT token
- `GET /api/check-auth` - Check authentication status

### Authenticated Routes (requires JWT token)
- `GET /api/files?list=true` - List files in tree format
- `GET /api/files/*path` - Get file content or directory listing
- `PUT /api/files/*path` - Upload/update file
- `POST /api/files/*path` - Upload file (multipart/form-data)
- `POST /api/logout` - Logout

### Authentication

Include JWT token in requests:
- Header: `Authorization: Bearer <token>`
- Query param: `?token=<token>`

## Project Structure

```
easy-notes/
├── main.go      # Main application with routes and handlers
├── jstree.go    # Tree structure generation for file browser
├── web/         # Embedded web assets
│   ├── login.html
│   ├── editor.html
│   ├── favicon.ico
│   └── embed.go
├── data/        # Storage directory (created automatically)
├── start.sh     # Startup script
├── go.mod
└── go.sum
```

## License

MIT License - see [LICENSE](LICENSE) file for details

## Author

VJ (c) 2025
