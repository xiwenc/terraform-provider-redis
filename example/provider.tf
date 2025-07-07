terraform {
    required_providers {
        redis = {
            source = "local/xiwenc/redis"
            version = "~> 0.0"
        }
    }
}

provider "redis" {
    redis_url = "redis://localhost:6379/0"
}