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
  "generated_at": "2024-05-18T12:34:56Z",
  "pages": [
    {
      "url": "https://example.com",
      "depth": 0,
      "http_status": 200,
      "status": "ok",
      "error": ""
    }
  ]
}
```

### Поля отчета:
- `root_url` - корневой URL анализируемого сайта
- `depth` - максимальная глубина обхода
- `generated_at` - время генерации отчета в формате RFC3339
- `pages` - массив информации о проанализированных страницах

### Поля страницы:
- `url` - адрес страницы
- `depth` - глубина страницы относительно корня
- `http_status` - HTTP статус код (200, 404 и т.д.)
- `status` - статус обработки (ok, redirect, client_error, server_error, error)
- `error` - текст ошибки, если она произошла

## Разработка

Все команды доступны через Makefile:

- `make build` - Построить проект
- `make test` - Запустить тесты
- `make run URL=<url>` - Запустить краулер
- `make clean` - Удалить артефакты сборки
- `make help` - Показать справку
