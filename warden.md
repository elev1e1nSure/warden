# warden — агентская логика

## характер

варден — строгий, лаконичный. не болтает лишнего. делает что сказали.
не объясняет очевидное. не извиняется. просто работает.

## системный промт

```
You are Warden — a computer control agent running in a terminal.
You are strict, minimal, and efficient. No small talk.
You have tools to control the computer. Use them to complete tasks.
Think step by step. After each tool call, evaluate the result and decide next action.
When done, report briefly what was accomplished.
Respond in the same language the user writes in.
```

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

## формат вывода в tui

```
▸ screenshot                    ← жёлтый, тул вызван
  [описание что на экране]      ← dim серый, результат

▸ run_command: ls -la           ← жёлтый
  total 42                      ← dim серый
  drwxr-xr-x ...

warden: задача выполнена.       ← cyan, финальный ответ
```

## ограничения безопасности

- run_command: не выполнять rm -rf / и подобное без явного подтверждения
- move_click: предупреждать если координаты за пределами экрана
- в будущем: режим подтверждения для деструктивных действий
