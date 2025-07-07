# Terraform Provider for Redis

A Terraform provider for managing Redis keys. Useful for when Redis is used as a store for infrastructure or application configuration.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.19

## Installation

### Using Terraform CLI

```bash
terraform init
```

### Manual Installation

1. Download the latest release for your platform
2. Extract the binary to your Terraform plugins directory:
   - Linux/macOS: `~/.terraform.d/plugins/registry.terraform.io/rdeavila94/redis/`
   - Windows: `%APPDATA%\terraform.d\plugins\registry.terraform.io\rdeavila94\redis\`

## Usage

### Provider Configuration

```hcl
terraform {
  required_providers {
    redis = {
      source  = "rdeavila94/redis"
      version = "~> 0.0"
    }
  }
}

provider "redis" {
  redis_url = "redis://localhost:6379/0"
}
```

### Resources

#### `redis_string`

Manages a Redis string key-value pair.

```hcl
resource "redis_string" "example" {
  key   = "my_key"
  value = "my_value"
}
```

**Arguments:**

- `key` (Required) - The Redis key to manage
- `value` (Required) - The string value to store
- `overridable` (Optional) - If true, allows overriding existing Redis keys. If false, creation will fail if the key already exists. Defaults to `false`.

**Attributes:**

- `id` - The Redis key
- `key` - The Redis key
- `value` - The stored value
- `overridable` - Whether the key can override existing values

#### `redis_user`

Manages a Redis user with ACL permissions (requires Redis 6.0+).

```hcl
resource "redis_user" "example" {
  username       = "app_user"
  password       = "secure_password"
  enabled        = true
  keys           = ["app:*", "cache:*"]
  commands       = ["+@read", "+@write", "-@dangerous"]
  channels       = ["news:*", "events:*"]
  reset_keys     = false
  reset_channels = false
  reset_commands = false
}
```

**Arguments:**

- `username` (Required) - The name of the Redis user
- `password` (Optional) - The password for the user. Set to empty string for passwordless authentication
- `enabled` (Optional) - Whether the user is enabled. Defaults to `true`
- `keys` (Optional) - List of key patterns the user can access (e.g., "cache:*")
- `commands` (Optional) - List of commands or command categories the user can execute (e.g., "+@read", "+get", "-@dangerous")
- `channels` (Optional) - List of Pub/Sub channel patterns the user can access
- `reset_keys` (Optional) - Whether to reset key permissions before applying new ones. Defaults to `false`
- `reset_channels` (Optional) - Whether to reset channel permissions before applying new ones. Defaults to `false`
- `reset_commands` (Optional) - Whether to reset command permissions before applying new ones. Defaults to `false`

**Attributes:**

- `id` - The Redis username
- `username` - The Redis username
- All other arguments are also exported as attributes

### Data Sources

Currently, this provider does not include data sources.

## Examples

### Basic Usage

```hcl
terraform {
  required_providers {
    redis = {
      source  = "rdeavila94/redis"
      version = "~> 0.0"
    }
  }
}

provider "redis" {
  redis_url = "redis://localhost:6379/0"
}

resource "redis_string" "app_config" {
  key   = "app:config:version"
  value = "1.0.0"
}

resource "redis_string" "user_session" {
  key   = "user:session:12345"
  value = "active"
}
```

### Overriding Existing Keys

```hcl
resource "redis_string" "existing_key" {
  key         = "existing:key"
  value       = "new_value"
  overridable = true
}
```

### Redis User with ACL Permissions

```hcl
resource "redis_user" "readonly_user" {
  username  = "readonly"
  password  = "password123"
  enabled   = true
  keys      = ["app:*", "cache:*"]
  commands  = ["+@read", "-@write", "-@admin"]
  channels  = ["notifications:*"]
}

resource "redis_user" "admin_user" {
  username  = "admin"
  password  = "secure_admin_password"
  enabled   = true
  commands  = ["+@all"]
  keys      = ["*"]
}
```

## Development

### Building from Source

```bash
git clone https://github.com/rdeavila94/terraform-provider-redis
cd terraform-provider-redis
make build
```

### Running Tests

```bash
go test ./...
```

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

For support, please open an issue on GitHub.
