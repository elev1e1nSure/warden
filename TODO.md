# /memory  Память между сессиями

## Описание

Команда `/memory` управляет долговременным контекстом модели через локальное SQLite-хранилище.  
При активном режиме модель накапливает выжимки о пользователе, предпочтениях и контексте работы между перезапусками.  
При выходе из программы происходит финальная агрегация данных в структурированную карту памяти.

## Статус

`[x]` Реализовано

---

## Команды и флаги

| Команда | Действие |
|---------|----------|
| `/memory on` | Включить запись памяти. Модель начинает сохранять выжимки в SQLite. |
| `/memory off` | Отключить запись. Ранее сохранённые данные остаются в БД, но новые не пишутся. |
| `/memory clear` | Полностью очистить таблицу памяти **без изменения** текущего состояния on/off. |
| `/memory status` | Показать текущее состояние (on/off), размер БД и краткую сводку по сохранённым сущностям. |

---

## Архитектура

### Компоненты

- **MemoryStore**  слой работы с SQLite (инициализация, CRUD, миграции).
- **MemoryExtractor**  логика извлечения значимых фактов из диалога.
- **MemoryAggregator**  финальная сборка карты памяти при завершении сессии.
- **MemoryCommand**  парсер и обработчик CLI-команд `/memory ...`.

### Поток данных

```
Диалог  MemoryExtractor  SQLite (инкрементальная запись)
                              
Завершение сессии  MemoryAggregator  итоговая карта памяти
```

---

## Схема SQLite

### Таблица `memory_entries`

| Поле | Тип | Описание |
|------|-----|----------|
| `id` | INTEGER PRIMARY KEY | Автоинкремент |
| `session_id` | TEXT | UUID сессии |
| `timestamp` | DATETIME DEFAULT CURRENT_TIMESTAMP | Время записи |
| `category` | TEXT | Категория: `user`, `project`, `preference`, `tech_stack` |
| `key` | TEXT | Ключ сущности (например, `preferred_language`) |
| `value` | TEXT | Значение / выжимка |
| `confidence` | REAL | Уверенность модели (0.01.0) |
| `source_message_id` | TEXT | Ссылка на сообщение-источник |

### Таблица `memory_state`

| Поле | Тип | Описание |
|------|-----|----------|
| `key` | TEXT PRIMARY KEY | `enabled` |
| `value` | TEXT | `1` или `0` |
| `updated_at` | DATETIME | Время последнего изменения |

---

## Жизненный цикл

1. **Инициализация** при старте программы:
   - Проверить `memory_state.enabled`.
   - Если `1`  активировать MemoryExtractor.

2. **Во время сессии**:
   - MemoryExtractor анализирует входящие/исходящие сообщения.
   - Значимые факты записываются в `memory_entries`.
   - Дубликаты по `(category, key)` обновляются (`confidence`, `value`, `timestamp`).

3. **При завершении сессии**:
   - MemoryAggregator читает все записи текущей сессии.
   - Формирует итоговую карту (структурированный JSON или Markdown).
   - Карта сохраняется в отдельное поле/таблицу `memory_snapshot`.

4. **Очистка** (`/memory clear`):
   - `DELETE FROM memory_entries;` (или `DELETE` с условием сессии).
   - Состояние `enabled` не трогается.

---

## Формат итоговой карты памяти (пример)

```json
{
  "user": {
    "name": null,
    "preferred_language": "ru",
    "coding_style": "terseness_over_verbosity"
  },
  "projects": [
    {
      "name": "warden",
      "tech_stack": ["Python", "Go", "SQLite"],
      "current_task": "/memory feature"
    }
  ],
  "preferences": {
    "commit_style": "conventional_commits",
    "language": "russian"
  },
  "updated_at": "2026-06-13T02:00:00Z"
}
```

---

## Открытые вопросы / TODO

- [x] MemoryExtractor — эвристики (MVP), LLM позже
- [x] Интеграция с существующим MCP memory — отдельное SQLite-хранилище
- [x] UI: `/memory on|off|clear|status` реализовано
- [ ] Размер лимита БД: ротационный механизм (хранить N последних сессий)?
- [ ] Конфиденциальность: шифровать SQLite-файл на диске?
- [ ] UI: подтверждение при `/memory clear`

