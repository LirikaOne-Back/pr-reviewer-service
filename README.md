# PR Reviewer Assignment Service

Микросервис для автоматического назначения ревьюеров на Pull Request'ы.

Сервис будет доступен на `http://localhost:8080`

> **Примечание**: Создайте файл `.env` на основе `.env.example` перед запуском.

## Стек технологий

- **Go 1.25** — основной язык
- **PostgreSQL 15** — база данных
- **Docker & Docker Compose** — контейнеризация

## Архитектура

Проект следует принципам чистой архитектуры с разделением на слои:

```
cmd/                 # Точка входа, маршрутизация
internal/
  ├── model/         # Модели данных
  ├── storage/       # Работа с БД
  ├── service/       # Бизнес-логика
  └── handler/       # HTTP handlers
migrations/          # SQL миграции
tests/               # Интеграционные тесты
```

## API Endpoints

### Teams
- `POST /team/add` — Создать команду с участниками
- `GET /team/get?team_name=<name>` — Получить команду
- `POST /team/deactivate` — Массовая деактивация команды

### Users
- `POST /users/setIsActive` — Изменить статус активности пользователя
- `GET /users/getReview?user_id=<id>` — Получить PR'ы назначенные на ревьюера

### Pull Requests
- `POST /pullRequest/create` — Создать PR с автоназначением ревьюеров
- `POST /pullRequest/merge` — Пометить PR как MERGED 
- `POST /pullRequest/reassign` — Переназначить ревьюера

### Statistics
- `GET /statistics` — Статистика по PR и ревьюерам *

\* — дополнительные эндпоинты

## Бизнес-логика

1. **Автоназначение ревьюеров**: При создании PR автоматически назначаются до 2 активных ревьюеров из команды автора (исключая самого автора)
2. **Переназначение**: Заменяет ревьюера на случайного активного участника из команды заменяемого ревьюера
3. **Ограничения**: После merge PR изменение ревьюеров запрещено
4. **Идемпотентность**: Повторный вызов merge возвращает актуальное состояние без ошибки
5. **Активность**: Пользователи с `is_active = false` не назначаются на ревью

## Примеры использования

### Создание команды

```
POST /team/add
{
  "team_name": "backend",
  "members": [
    {"user_id": "u1", "username": "Alice", "is_active": true},
    {"user_id": "u2", "username": "Bob", "is_active": true}
  ]
}
```

### Получение команды

```
GET /team/get?team_name=backend
```

### Массовая деактивация команды

```
POST /team/deactivate
{
  "team_name": "backend"
}
```

### Изменение статуса пользователя

```
POST /users/setIsActive
{
  "user_id": "u2",
  "is_active": false
}
```

### Получение PR для ревьюера

```
GET /users/getReview?user_id=u2
```

### Создание PR

```
POST /pullRequest/create
{
  "pull_request_id": "pr-1001",
  "pull_request_name": "Add feature",
  "author_id": "u1"
}
```

### Merge PR

```
POST /pullRequest/merge
{
  "pull_request_id": "pr-1001"
}
```

### Переназначение ревьюера

```
POST /pullRequest/reassign
{
  "pull_request_id": "pr-1001",
  "old_user_id": "u2"
}
```

### Получение статистики

```
GET /statistics
```

## Тестирование

Интеграционные тесты требуют запущенного сервиса:

```
docker-compose up
make test
```

## Дополнительные задания

Реализованы 4 из 5 дополнительных заданий:

**1. Эндпоинт статистики** — `GET /statistics` возвращает количество PR по статусам и топ-10 ревьюеров

**3. Массовая деактивация** — `POST /team/deactivate` деактивирует всех пользователей команды и переназначает открытые PR. Производительность: ~26ms для 5 пользователей + 5 PR (цель <100ms)

**4. Интеграционное тестирование** — `tests/integration_test.go` содержит полный набор тестов для проверки всех сценариев

**5. Конфигурация линтера** — `.golangci.yml` с настроенными линтерами (govet, errcheck, staticcheck, ineffassign, unused)

## Makefile команды

```
make build   # Сборка Docker образов
make run     # Запуск сервиса
make clean   # Остановка и удаление volumes
make test    # Запуск тестов
```

## Структура БД

Используются 4 таблицы:

- **teams** — команды (team_name)
- **users** — пользователи с привязкой к команде и флагом активности
- **pull_requests** — PR'ы со статусом OPEN/MERGED
- **pr_reviewers** — связь многие-ко-многим для назначенных ревьюеров

Индексы созданы на `team_name`, `is_active`, `status` для быстрых выборок.

