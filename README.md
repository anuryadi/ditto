# ditto

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/anuryadi/ditto/workflows/CI/badge.svg)](https://github.com/anuryadi/ditto/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/anuryadi/ditto)](https://goreportcard.com/report/github.com/anuryadi/ditto)

> 🔄 A powerful CLI tool for database synchronization between PostgreSQL and MySQL

<p align="center">
  <img src="https://raw.githubusercontent.com/arinuryadi/ditto/main/assets/demo.gif" alt="ditto demo" width="600">
</p>

## ✨ Features

- **Schema Comparison** - Compare table structures, columns, indexes, and constraints
- **Data Synchronization** - Sync data with full or incremental strategies
- **Data Export** - Export tables to JSON, CSV, or SQL format
- **Cross-Database Support** - PostgreSQL ↔ MySQL/MariaDB
- **Beautiful CLI** - Rich terminal output with colors, progress bars, and tables
- **Configuration File Support** - YAML config for complex setups

## 🚀 Installation

### Using Go Install

```bash
go install github.com/anuryadi/ditto@latest
```

### From Source

```bash
git clone https://github.com/anuryadi/ditto.git
cd ditto
make build
```

### Binary Releases

Download pre-built binaries from the [Releases](https://github.com/anuryadi/ditto/releases) page.

## 📖 Quick Start

### Compare Database Schemas

```bash
# Compare PostgreSQL to MySQL
ditto compare \
  --source "postgres://user:pass@localhost:5432/source_db" \
  --target "mysql://user:pass@localhost:3306/target_db"

# Output to JSON
ditto compare \
  --source "postgres://user:pass@localhost/db1" \
  --target "mysql://user:pass@localhost/db2" \
  --format json \
  --output diff.json
```

### Sync Data Between Databases

```bash
# Full sync
ditto sync \
  --source "postgres://user:pass@localhost/source" \
  --target "mysql://user:pass@localhost/target" \
  --tables users,orders,products

# Dry run (preview without changes)
ditto sync \
  --source "postgres://..." \
  --target "mysql://..." \
  --dry-run
```

### Export Table Data

```bash
# Export to JSON
ditto export \
  --source "postgres://user:pass@localhost/db" \
  --table users \
  --format json \
  --output users.json

# Export to CSV
ditto export \
  --source "mysql://user:pass@localhost/db" \
  --table orders \
  --format csv \
  --output orders.csv
```

## ⚙️ Configuration

Create a `.ditto.yaml` file in your home directory or use `--config` flag:

```yaml
source:
  driver: postgres
  host: localhost
  port: 5432
  database: source_db
  user: postgres
  password: ${DB_PASSWORD}  # Environment variable

target:
  driver: mysql
  host: localhost
  port: 3306
  database: target_db
  user: root
  password: ${DB_PASSWORD}

sync:
  strategy: full  # full, incremental
  batch_size: 1000
  tables:
    - name: users
      primary_key: id
      timestamp_column: updated_at
    - name: orders
      primary_key: id

logging:
  level: info
  format: console
```

## 🔧 Commands

| Command | Description |
|---------|-------------|
| `ditto compare` | Compare schemas between source and target database |
| `ditto sync` | Synchronize data from source to target |
| `ditto export` | Export table data to file |
| `ditto version` | Show version information |

### Global Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to config file (default: `~/.ditto.yaml`) |
| `-v, --verbose` | Enable verbose output |
| `-h, --help` | Show help |

## 🏗️ Architecture

```
ditto/
├── cmd/                    # CLI commands (Cobra)
├── internal/
│   ├── config/            # Configuration management (Viper)
│   ├── database/          # Database drivers (PostgreSQL, MySQL)
│   ├── schema/            # Schema comparison logic
│   ├── sync/              # Data synchronization engine
│   ├── export/            # Export handlers (JSON, CSV, SQL)
│   └── ui/                # Terminal UI components (pterm)
└── pkg/
    └── logger/            # Structured logging (zerolog)
```

## 🧪 Development

```bash
# Install dependencies
go mod download

# Run tests
make test

# Run with coverage
make coverage

# Lint code
make lint

# Build for all platforms
make build-all
```

## 📝 Roadmap

- [ ] SQLite support
- [ ] Oracle Database support
- [ ] Scheduled sync with cron
- [ ] Web dashboard for monitoring
- [ ] CDC (Change Data Capture) support

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [zerolog](https://github.com/rs/zerolog) - Structured logging
- [pterm](https://github.com/pterm/pterm) - Terminal UI

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/arinuryadi">Ari Nuryadi</a>
</p>
