.PHONY: server migrate-up migrate-down sqlc

MAIN_ENV_FILE ?= .env
WORKTREE_ENV_FILE ?= .env.worktree
ENV_FILE ?= $(if $(wildcard $(MAIN_ENV_FILE)),$(MAIN_ENV_FILE),$(if $(wildcard $(WORKTREE_ENV_FILE)),$(WORKTREE_ENV_FILE),$(MAIN_ENV_FILE)))

-include $(ENV_FILE)
export

# Server

server: ## Run only the Go server for the current checkout
	@bash scripts/ensure-postgres.sh "$(ENV_FILE)"
	cd server && go run ./cmd/server

# Database

migrate-up: ## Create the target DB if needed, then apply database migrations
	@bash scripts/ensure-postgres.sh "$(ENV_FILE)"
	cd server && go run ./cmd/migrate up

migrate-down: ## Create the target DB if needed, then roll back database migrations
	@bash scripts/ensure-postgres.sh "$(ENV_FILE)"
	cd server && go run ./cmd/migrate down

sqlc: ## Regenerate sqlc code
	cd server && sqlc generate
