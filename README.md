# Concerts REST API (Go + PostgreSQL)

## Структура проекта

```
concerts-api/
├── main.go                      # Точка входа, запуск сервера
├── go.mod                       # Go-модуль и зависимости
├── router/
│   └── router.go                # Все маршруты (chi router)
└── internal/
    ├── db/
    │   └── db.go                # Подключение к базе данных
    ├── models/
    │   └── models.go            # Структуры данных (request/response)
    └── handlers/
        ├── helpers.go           # Общие утилиты (JSON, токены)
        ├── concerts.go          # GET /concerts, GET /concerts/{id}
        ├── seating.go           # GET /seating
        ├── reservation.go       # POST /reservation
        ├── booking.go           # POST /booking
        └── tickets.go           # POST /tickets, POST /tickets/{id}/cancel
```

## Эндпоинты

| Метод  | URL                                                              | Описание                         |
|--------|------------------------------------------------------------------|----------------------------------|
| GET    | /api/v1/concerts                                                 | Список всех концертов            |
| GET    | /api/v1/concerts/{concert-id}                                    | Один концерт по ID               |
| GET    | /api/v1/concerts/{concert-id}/shows/{show-id}/seating            | Информация о местах              |
| POST   | /api/v1/concerts/{concert-id}/shows/{show-id}/reservation        | Забронировать место              |
| POST   | /api/v1/concerts/{concert-id}/shows/{show-id}/booking            | Оформить билет из брони          |
| POST   | /api/v1/tickets                                                  | Получить билеты (code + name)    |
| POST   | /api/v1/tickets/{ticket-id}/cancel                               | Отменить билет                   |

## Установка и запуск

### 1. Установить зависимости

```bash
cd concerts-api
go mod tidy
```

### 2. Убедиться, что БД запущена

Убедитесь, что PostgreSQL запущен и база данных `concerts_db` создана с паролем `12345`.

Если нужно применить дамп:
```bash
psql -U postgres concerts_db < database1.sql
```

### 3. Запустить сервер

```bash
go run main.go
```

Сервер запустится на `http://localhost:8080`

## Примеры запросов (curl)

### Получить все концерты
```bash
curl http://localhost:8080/api/v1/concerts
```

### Получить один концерт
```bash
curl http://localhost:8080/api/v1/concerts/1
```

### Информация о местах
```bash
curl http://localhost:8080/api/v1/concerts/1/shows/1/seating
```

### Забронировать места (новая бронь)
```bash
curl -X POST http://localhost:8080/api/v1/concerts/1/shows/1/reservation \
  -H "Content-Type: application/json" \
  -d '{
    "reservations": [{"row": 1, "seat": 1}],
    "duration": 300
  }'
```

### Заменить бронь (используя токен)
```bash
curl -X POST http://localhost:8080/api/v1/concerts/1/shows/1/reservation \
  -H "Content-Type: application/json" \
  -d '{
    "reservation_token": "ВАШ_ТОКЕН",
    "reservations": [{"row": 1, "seat": 2}],
    "duration": 120
  }'
```

### Оформить билет из брони
```bash
curl -X POST http://localhost:8080/api/v1/concerts/1/shows/1/booking \
  -H "Content-Type: application/json" \
  -d '{
    "reservation_token": "ВАШ_ТОКЕН",
    "name": "John Doe",
    "address": "Bahnhofstrasse 15",
    "city": "Graz",
    "zip": "8010",
    "country": "Austria"
  }'
```

### Получить билеты по коду и имени
```bash
curl -X POST http://localhost:8080/api/v1/tickets \
  -H "Content-Type: application/json" \
  -d '{"code": "QVLJTWK4Y7", "name": "John Doe"}'
```

### Отменить билет
```bash
curl -X POST http://localhost:8080/api/v1/tickets/1/cancel \
  -H "Content-Type: application/json" \
  -d '{"code": "QVLJTWK4Y7", "name": "John Doe"}'
```

## Конфигурация БД

Параметры подключения находятся в `main.go`:

```go
db.Connect(db.Config{
    Host:     "localhost",
    Port:     5432,
    User:     "postgres",
    Password: "12345",
    DBName:   "concerts_db",
    SSLMode:  "disable",
})
```
