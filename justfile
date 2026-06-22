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

# ── go ──

VERSION := env_var_or_default("TAG", "dev")

build:
    cd go && go build -ldflags="-s -w -X 'warden.wardenVersion={{VERSION}}'" -o ../warden.exe ./cmd/warden

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
      run_backend.py

release: build build-backend
    @echo "warden.exe + dist/warden-backend.exe ready"
