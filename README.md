### Hexlet tests and linter status:
[![Actions Status](https://github.com/motokazmin/go-test-project-316/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/motokazmin/go-test-project-316/actions)

# Hexlet Go Crawler

Парсер для анализа структуры веб-сайтов с поддержкой многопоточности, управления запросами и детерминированного тестирования.

### Построение проекта

```bash
make build
```

Команда создаст исполняемый файл `bin/hexlet-go-crawler`.

### Запуск тестов

```bash
make test
```

### Запуск краулера

```bash
make run URL=https://example.com
```

Или напрямую через CLI:

```bash
bin/hexlet-go-crawler https://example.com
```

## Справка утилиты

```
hexlet-go-crawler --help
NAME:
   hexlet-go-crawler - analyze a website structure

USAGE:
   hexlet-go-crawler [global options] command [command options] <url>

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --depth value       crawl depth (default: 10)
   --retries value     number of retries for failed requests (default: 1)
   --delay value       delay between requests (example: 200ms, 1s) (default: 0s)
   --timeout value     per-request timeout (default: 15s)
   --rps value         limit requests per second (overrides delay) (default: 0)
   --user-agent value  custom user agent
   --workers value     number of concurrent workers (default: 4)
   --help, -h          show help
```

## Примеры использования

Анализ сайта с глубиной обхода 2:

```bash
make run URL=https://example.com --depth 2
```

Анализ с ограничением на 10 запросов в секунду:

```bash
bin/hexlet-go-crawler https://example.com --rps 10
```

Анализ с пользовательским User-Agent и 2 секундной задержкой:

```bash
bin/hexlet-go-crawler https://example.com --user-agent "MyBot/1.0" --delay 2s
```

## Формат вывода

Краулер выводит JSON-отчет со следующей структурой:

```json
{
  "root_url": "https://example.com",
  "depth": 1,
  "generated_at": "2024-06-01T12:34:56Z",
  "pages": [
    {
      "url": "https://example.com",
      "depth": 0,
      "http_status": 200,
      "status": "ok",
      "error": "",
      "seo": {
        "has_title": true,
        "title": "Example title",
        "has_description": true,
        "description": "Example description",
        "has_h1": true
      },
      "broken_links": [
        {
          "url": "https://example.com/missing",
          "status_code": 404,
          "error": "Not Found"
        }
      ],
      "assets": [
        {
          "url": "https://example.com/static/logo.png",
          "type": "image",
          "status_code": 200,
          "size_bytes": 12345,
          "error": ""
        }
      ],
      "discovered_at": "2024-06-01T12:34:56Z"
    }
  ]
}
```

### Поля корневого отчета

- **`root_url`** (string) - Корневой URL анализируемого сайта
- **`depth`** (integer) - Максимальная глубина обхода (0-based)
- **`generated_at`** (string) - Время генерации отчета в формате RFC3339 (ISO 8601)
- **`pages`** (array) - Массив объектов Page с информацией о проанализированных страницах

### Поля страницы (Page)

- **`url`** (string) - Полный адрес страницы
- **`depth`** (integer) - Глубина страницы относительно корня (0 = корневая)
- **`http_status`** (integer) - HTTP статус код (200, 301, 404, 500 и т.д.)
- **`status`** (string) - Статус обработки: `ok`, `redirect`, `client_error`, `server_error`, `error`
- **`error`** (string) - Текст ошибки (если она произошла), пусто при успехе
- **`seo`** (object) - SEO параметры страницы (см. Поля SEO)
- **`broken_links`** (array) - Массив обнаруженных битых ссылок (см. Поля BrokenLink)
- **`assets`** (array) - Массив статических ресурсов (см. Поля Asset)
- **`discovered_at`** (string) - Время обнаружения страницы в формате RFC3339

### Поля SEO

- **`has_title`** (boolean) - Наличие тега `<title>` на странице
- **`title`** (string or null) - Содержимое тега `<title>` (null если отсутствует)
- **`has_description`** (boolean) - Наличие мета-тега `description`
- **`description`** (string or null) - Содержимое атрибута `content` мета-тега `description` (null если отсутствует)
- **`has_h1`** (boolean) - Наличие заголовка `<h1>` на странице

### Поля BrokenLink (битой ссылки)

- **`url`** (string) - URL битой ссылки
- **`status_code`** (integer) - HTTP статус код ошибки (4xx или 5xx), опционально
- **`error`** (string) - Текст ошибки сети или timeout, опционально

### Поля Asset (статического ресурса)

- **`url`** (string) - URL ресурса
- **`type`** (string) - Тип ресурса: `image`, `script`, `stylesheet`, `other`
- **`status_code`** (integer) - HTTP статус код (200 при успехе)
- **`size_bytes`** (integer) - Размер ресурса в байтах
- **`error`** (string) - Текст ошибки (если произошла), пусто при успехе

### Значения статуса страницы

- **`ok`** - успешно обработана (2xx статус)
- **`redirect`** - переадресация (3xx статус)
- **`client_error`** - ошибка клиента (4xx статус)
- **`server_error`** - ошибка сервера (5xx статус)
- **`error`** - ошибка при обработке (сеть, таймаут и т.д.)
