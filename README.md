# warden

CLI-агент управления компьютером. Go TUI + Python backend + Ollama.

## стек

| слой | технология |
|---|---|
| frontend | go 1.21+, bubbletea, lipgloss |
| backend | python 3.11+, aiohttp |
| llm | ollama (qwen3:8b) |
| computer use | pyautogui, pillow |
| браузер | playwright |
| поиск | duckduckgo-search |

## архитектура

```
go tui (bubbletea)
    ↓ HTTP NDJSON
python backend (aiohttp, localhost:8765)
    ↓
ollama
    ↓
[bash] [filesystem] [screenshot] [mouse/keyboard] [browser] [search]
```

frontend и backend разделены: TUI не знает про Ollama, backend не знает про UI.

## структура

```
warden/
├── go/
│   ├── main.go          # точка входа
│   ├── model.go         # bubbletea model
│   ├── client.go        # http клиент
│   ├── styles.go        # lipgloss стили
│   └── logger.go        # логи frontend
├── agent/
│   ├── server.py        # aiohttp backend
│   ├── chat.py          # сессия и стриминг
│   ├── ollama_client.py # управление ollama
│   ├── tools.py         # инструменты агента
│   └── logger.py        # цветные логи backend
├── requirements.txt
├── README.md
└── CLAUDE.md
```

## запуск

```bash
# 1. backend
python agent/server.py

# 2. frontend (другой терминал)
cd go && go run .
# или
./go/warden.exe
```

backend поднимается на `localhost:8765`, сам запускает ollama и качает модель при необходимости.

## тулзы

| имя | описание |
|---|---|
| `bash` | PowerShell команды |
| `file_read` | чтение файла |
| `file_write` | запись файла (создаёт папки) |
| `file_delete` | удаление файла, только внутри cwd |
| `file_list` | список файлов и папок |
| `clipboard` | чтение / запись буфера обмена |
| `screenshot` | скриншот экрана |
| `mouse` | move, click, right_click, double_click, scroll |
| `keyboard` | type, press (hotkey) |
| `browser_open` | открыть URL в браузере пользователя |
| `browser_read` | прочитать текст страницы |
| `browser_screenshot` | скриншот страницы |
| `youtube_search` | поиск видео на YouTube |
| `google_search` | поиск в интернете (DuckDuckGo) |

## slash-команды

Вводятся в поле сообщения:

| команда | действие |
|---|---|
| `/auto` | авторежим — опасные команды без подтверждения |
| `/safe` | безопасный режим — подтверждение на опасные |
| `/reset` | сбросить сессию |
| `/thinking` | вкл/выкл размышления модели |

## безопасность

- `bash`: опасные паттерны (`rm -rf`, `format`, `rmdir` и др.) требуют подтверждения в safe-режиме
- `file_delete`: только внутри cwd, всегда требует подтверждения
- `file_write`: за пределами cwd требует подтверждения
- в TUI: `y` + Enter — подтвердить, Esc — отмена

## модель

Рекомендуется: `qwen3:8b`

```bash
ollama run qwen3:8b
```
