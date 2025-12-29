# IMAP Backup Tool

A simple tool to download and backup emails from multiple IMAP accounts.

## Configuration

Configuration is managed via environment variables and a YAML file for accounts. You can also use a `.env` file to set environment variables (see `.env.example`).

### Environment Variables

| Variable | Description | Default Value |
|_**BACKUP_OUTPUT_DIR**_| Directory where emails will be stored | `./output` |
|_**BACKUP_INTERVAL_MINUTES**_| Interval between backup runs in minutes | `60` |
|_**BACKUP_ACCOUNTS_FILE**_| Path to the accounts configuration file | `accounts.yaml` |
|_**BACKUP_RUN_ON_INTERVAL**_| Whether to run continuously on an interval | `false` |
|_**BACKUP_IDLE**_| Whether to use IMAP IDLE to listen for new emails (real-time) | `false` |

### Accounts Configuration

Accounts are defined in a YAML file (default: `accounts.yaml`).

Example:

```yaml
- name: "Personal Email"
  credentials:
    username: "user@example.com"
    password: "your-password"
  server:
    host: imap.example.com
    port: 993
    security: "SSL/TLS"
```

## Usage

### Backup

Run the program to start the backup process based on your configuration.

```bash
./imap-backup
```

### Restore

You can restore emails from the backup to the IMAP server using the following CLI arguments:

- `-restore-account`: Restore a specific account by name or use "all" for all accounts.
- `-restore-accounts`: Restore a comma-separated list of accounts.

Examples:

```bash
# Restore all accounts
./imap-backup -restore-account=all

# Restore a specific account
./imap-backup -restore-account="Personal Email"

# Restore multiple specific accounts
./imap-backup -restore-accounts="Personal Email,Work Email"
```

**Note:** The restore process will append messages to the server. It attempts to parse the date from the email header but does not preserve IMAP flags (like Seen/Read status) as they are not currently backed up.