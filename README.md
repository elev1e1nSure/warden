# warden

автономный cli-агент по компу. строгий, лаконичный, делает что говорят.

---

## стек

| слой | технология |
|---|---|
| tui | go + bubbletea |
| llm | ollama (qwen2.5:7b или qwen3:8b) |
| computer use | pyautogui, pillow (скрины) |
| браузер | playwright |
| терминал/файлы | subprocess, pathlib, shutil |
| llm клиент | ollama python sdk |

---

## архитектура

```
пользователь
    ↓ промт
tui (bubbletea)          ← go, только рендер
    ↓ http
backend (aiohttp)        ← python, llm + тулзы
    ↓
ollama
    ↓
[screenshot] [mouse/keyboard] [browser] [terminal] [filesystem]
    ↓
результат → стриминг токенов → tui
```

tui и backend общаются по http: ndjson стрим. tui не знает про ollama, backend не знает про ui.

---

## структура проекта

```
warden/
├── go/
│   ├── go.mod           # модули go
│   ├── main.go          # точка входа
│   ├── model.go         # bubbletea model
│   ├── client.go        # http клиент к бэкенду
│   └── styles.go        # lipgloss стили
├── agent/
│   ├── server.py        # aiohttp бэкенд
│   ├── ollama_client.py # подключение к ollama
│   └── chat.py          # сессия и стриминг
├── requirements.txt     # python зависимости
├── CLAUDE.md            # инструкции для claude code
├── AGENTS.md            # инструкции для агентов
└── README.md            # этот файл
```

---

## визуальный стиль

- тёмный фон, никаких обводок и панелей с фоном
- минимализм: текст, цвет, жирность — и всё
- цвета: белый для обычного текста, cyan для варден, жёлтый для тулзов, красный для ошибок, серый для метаинфо
- управление: стрелки, enter, esc, ctrl+c
- никаких кнопок, никаких мышиных кликов в самом tui

---

## системный промт

```
You are Warden — a computer control agent running in a terminal.
You are strict, minimal, and efficient. No small talk.
You have tools to control the computer. Use them to complete tasks.
Think step by step. After each tool call, evaluate the result and decide next action.
When done, report briefly what was accomplished.
Respond in the same language the user writes in.
```

---

## тулзы mvp

### screenshot
```json
{
  "name": "screenshot",
  "description": "Take a screenshot and describe what's visible on screen",
  "parameters": {}
}
```
возвращает: base64 изображение + текстовое описание от модели

### run_command
```json
{
  "name": "run_command",
  "description": "Run a shell command and return output",
  "parameters": {
    "command": "string — команда для выполнения",
    "timeout": "int — таймаут в секундах (default: 10)"
  }
}
```
возвращает: stdout, stderr, exit code

### move_click
```json
{
  "name": "move_click",
  "description": "Move mouse to coordinates and click",
  "parameters": {
    "x": "int",
    "y": "int",
    "button": "string — left/right/double (default: left)"
  }
}
```

### type_text
```json
{
  "name": "type_text",
  "description": "Type text using keyboard",
  "parameters": {
    "text": "string",
    "interval": "float — задержка между символами (default: 0.05)"
  }
}
```

### key_press
```json
{
  "name": "key_press",
  "description": "Press a key or key combination",
  "parameters": {
    "key": "string — например enter, ctrl+c, alt+tab"
  }
}
```

---

## агентский луп

```
1. получить промт от пользователя
2. добавить в историю
3. отправить в ollama со стримингом
4. если модель вызывает тул:
   a. показать в tui: [tool_name] параметры...
   b. выполнить тул
   c. показать результат
   d. добавить в историю как tool_result
   e. вернуться к шагу 3
5. если модель отвечает текстом — стримить в tui
6. ждать следующего промта
```

---

## формат вывода в tui

```
▸ screenshot                    ← жёлтый, тул вызван
  [описание что на экране]      ← dim серый, результат

▸ run_command: ls -la           ← жёлтый
  total 42                      ← dim серый
  drwxr-xr-x ...

warden: задача выполнена.       ← cyan, финальный ответ
```

---

## ограничения безопасности

- run_command: не выполнять rm -rf / и подобное без явного подтверждения
- move_click: предупреждать если координаты за пределами экрана
- в будущем: режим подтверждения для деструктивных действий

---

## роадмап

### mvp v2 — go tui + python backend (в работе)

- [x] go tui: bubbletea, лог + input, стриминг, цвета
- [x] python backend: aiohttp, ndjson стрим, ollama, chat
- [x] протокол: http ndjson между go и python
- [ ] тул: screenshot
- [ ] тул: run_command
- [ ] тул: type_text / move_click

### mvp v1 — чат + tui + ollama (готово, python)

- [x] скелет проекта
- [x] базовый tui
- [x] автоподнятие и закрытие ollama
- [x] чат с llm
- [x] полировка

---

## модель

**рекомендуется:** qwen3:8b (лучше тулзы, быстрее)

запуск: `ollama run qwen3:8b`
