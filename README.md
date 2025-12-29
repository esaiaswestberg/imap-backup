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
