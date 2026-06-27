# warden — project task runner
# https://github.com/casey/just

set positional-arguments := true

default:
    @just --list

# ── install ──

install:
    pip install -r requirements.txt

# ── python ──

test *args:
    pytest {{args}}

test-cov:
    pytest

lint-py:
    ruff check agent/

lint-py-fix:
    ruff check --fix agent/

fmt-py:
    ruff format --check agent/

fmt-py-write:
    ruff format agent/

# ── go ──

VERSION := env_var_or_default("TAG", "dev")

build:
    just build-backend
    cd go && go build -ldflags="-s -w -X 'github.com/elev1e1n/warden.wardenVersion={{VERSION}}'" -o ../warden.exe ./cmd/warden

build-check:
    cd go && go build ./...

test-go *args:
    cd go && go test ./... {{args}}

test-go-cov:
    cd go && go test ./... -coverprofile=cover.out

lint-go:
    cd go && go vet ./...

fmt-go:
    cd go && gofmt -d -l .

fmt-go-check:
    cd go && test -z "$(gofmt -l .)" || (echo "These files are not gofmt-compliant:" && gofmt -l . && exit 1)

fmt-go-write:
    cd go && gofmt -w .

# ── run ──

run:
    python run_backend.py

# ── release ──

build-backend:
    pip install -r requirements.txt pyinstaller
    pyinstaller --onefile --name warden-backend \
      --collect-all agent \
      --collect-all playwright \
      --collect-submodules openai \
      --collect-submodules ollama \
      --hidden-import=aiohttp \
      --hidden-import=PIL \
      --hidden-import=pyautogui \
      --hidden-import=openai \
      --hidden-import=ollama \
      --hidden-import=duckduckgo_search \
      --hidden-import=html2text \
      --hidden-import=certifi \
      --distpath go/cmd/warden \
      run_backend.py

release: build
    @echo "warden.exe ready"
