.PHONY: quality-plan quality-check hooks commit-lint test-commit-lint test vet build lint tidy fmt

test:
	go test ./...

vet:
	go vet ./...

build:
	go build ./...

tidy:
	go mod tidy

fmt:
	gofmt -w .

# 对齐生态门禁规格：vet + gofmt 漂移检查（standards §4）
lint: vet
	@test -z "$$(gofmt -l .)" || (echo "gofmt 漂移：" && gofmt -l . && exit 1)
	$(MAKE) test

# RR-GIT-01: self-contained commit checks; installation is worktree-scoped.
hooks:
	sh scripts/install-git-hooks.sh

commit-lint:
	python3 scripts/commit-policy.py pending

test-commit-lint:
	python3 scripts/test-commit-policy.py

# Local and CI classify the same base..HEAD window; include staged/unstaged/untracked work.
BASE ?= origin/main

quality-plan:
	python3 scripts/quality-gate.py local --kind utils --base "$(BASE)" --plan-only

quality-check:
	python3 scripts/quality-gate.py local --kind utils --base "$(BASE)"
