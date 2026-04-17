# 🧩 Архитектура backend-приложения

```txt
📦 groupeak-main/
├── backend-app/
│   ├── cmd/
│   │   └── api/
│   │       └── main.go
│   ├── docs/
│   │   ├── docs.go
│   │   ├── swagger.json
│   │   └── swagger.yaml
│   ├── internal/
│   │   ├── apperror/
│   │   │   └── error.go
│   │   ├── config/
│   │   │   └── config.go
│   │   ├── db/
│   │   │   └── postgres.go
│   │   ├── dto/
│   │   │   ├── event_dto.go
│   │   │   ├── notification_dto.go
│   │   │   ├── project_dto.go
│   │   │   ├── task_dto.go
│   │   │   └── user_dto.go
│   │   ├── eventbus/
│   │   │   └── bus.go
│   │   ├── handlers/
│   │   │   ├── auth_handlers.go
│   │   │   ├── event_handlers.go
│   │   │   ├── helper_handlers.go
│   │   │   ├── notification_handlers.go
│   │   │   ├── proj_mem_handlers.go
│   │   │   ├── project_handlers.go
│   │   │   ├── task_handlers.go
│   │   │   ├── task_work_handlers.go
│   │   │   └── user_handlers.go
│   │   ├── middleware/
│   │   │   └── auth_middleware.go
│   │   ├── models/
│   │   │   ├── event.go
│   │   │   ├── notification.go
│   │   │   ├── project.go
│   │   │   ├── task.go
│   │   │   └── user.go
│   │   ├── repository/
│   │   │   ├── event_repo.go
│   │   │   ├── notification_repo.go
│   │   │   ├── project_repo.go
│   │   │   ├── repository.go
│   │   │   ├── task_repo.go
│   │   │   └── user_repo.go
│   │   ├── router/
│   │   │   └── router.go
│   │   ├── services/
│   │   │   ├── auth_service.go
│   │   │   ├── event_service.go
│   │   │   ├── notification_service.go
│   │   │   ├── project_invites.go
│   │   │   ├── project_service.go
│   │   │   ├── task_service.go
│   │   │   ├── task_workflow.go
│   │   │   ├── user_service.go
│   │   │   └── validate_service.go
│   │   └── worker/
│   │       └── deadline_worker.go
│   ├── migrations/
│   │   ├── 001_init_schema.sql
│   │   ├── 002_add_project_events.sql
│   │   ├── 003_upgrade_notifications.sql
│   │   └── 004_fix_notifications_logic.sql
│   ├── .env
│   ├── go.mod
│   └── go.sum
├── .gitignore
├── .pre-commit-config.yaml
├── go.work
├── go.work.sum
├── README.md
└── ~
```

---

Проект построен на **Слоистой архитектуре (Layered Architecture)** с четким разделением зон ответственности. Это позволяет легко тестировать код, менять базу данных или протоколы общения без переписывания всей бизнес-логики.

## Основные слои и директории

### 1. Точка входа и конфигурация
* **`cmd/api/main.go`** – сердце приложения. Собирает зависимости (Dependency Injection), инициализирует БД, запускает воркеры, накатывает миграции, поднимает HTTP-сервер.
* **`internal/config/`** – парсинг конфигурации (переменные окружения, `.env`).
* **`internal/db/`** – управление подключениями к инфраструктуре (PostgreSQL, Redis пулы).

### 2. Транспортный слой (Presentation Layer)
Отвечает за прием запросов от клиента и отдачу ответов. Ничего не знает про базу данных.
* **`internal/router/`** – регистрация маршрутов и привязка их к хэндлерам.
* **`internal/middleware/`** – перехватчики запросов (авторизация по JWT, логирование, защита от паник).
* **`internal/handlers/`** – HTTP-контроллеры. Парсят входящий JSON, вызывают нужный сервис и формируют HTTP-ответ.
* **`docs/`** – автосгенерированная документация Swagger (OpenAPI).

### 3. Слой бизнес-логики (Business Layer)
Все правила приложения. Этот слой не знает про HTTP-запросы и SQL-запросы.
* **`internal/services/`** – ядро системы. Содержит логику создания задач, проверки прав, распределения ролей в проекте и т.д.
* **`internal/eventbus/`** – внутренняя шина событий. Позволяет сервисам общаться асинхронно (например, сервис задач кидает событие "задача создана", а сервис уведомлений его ловит).
* **`internal/worker/`** – фоновые процессы. Например, `deadline_worker.go` может по крону проверять просроченные задачи и генерировать события.

### 4. Слой доступа к данным (Data Access Layer)
Изолирует работу с хранилищами (SQL, кэш).
* **`internal/repository/`** – реализация паттерна Repository. Только здесь пишутся SQL-запросы. Сервисы общаются с базой строго через интерфейсы репозиториев.

### 5. Доменные сущности и контракты
* **`internal/models/`** – чистые доменные структуры данных, описывающие сущности (User, Task, Project). Мапятся на таблицы в БД.
* **`internal/dto/`** – Data Transfer Objects. Структуры, описывающие то, что приходит от клиента (requests) и уходит клиенту (responses). Защищают доменные модели от прямого изменения извне.
* **`internal/apperror/`** – централизованная система кастомных ошибок приложения для единообразной отдачи статусов клиенту.

---

## 🔄 Жизненный цикл запроса

Общение между слоями происходит строго **сверху вниз** (или от внешнего круга к внутреннему). 

1.  **Запрос (Router ➡️ Middleware ➡️ Handler):** Пользователь дергает эндпоинт (например, `POST /tasks`). Запрос проходит через `auth_middleware`, который достает `userID`. Далее управление переходит в `task_handlers.go`.
2.  **Валидация (Handler ➡️ DTO):** Хэндлер парсит JSON-тело в структуру `TaskCreateDTO` и валидирует базовые поля.
3.  **Бизнес-логика (Handler ➡️ Service):** Хэндлер вызывает метод сервиса `task_service.CreateTask(dto, userID)`. Сервис проверяет сложные правила (например, есть ли у пользователя права в этом проекте, обращаясь к `project_repo`).
4.  **Работа с БД (Service ➡️ Repository):** Если всё ок, сервис передает доменную модель в `task_repo.Create(task)`. Репозиторий выполняет `INSERT` в PostgreSQL и возвращает готовый объект с присвоенным ID.
5.  **Асинхронные события (Service ➡️ EventBus):** Сервис задач отправляет в `bus.go` событие `TaskCreatedEvent`. В фоне `notification_service` ловит это событие и создает уведомление для исполнителя. Главный поток не блокируется.
6.  **Ответ (Service ➡️ Handler ➡️ Ответ):** Сервис возвращает готовую задачу в хэндлер. Хэндлер мапит модель в `TaskResponseDTO` и отдает клиенту JSON со статусом `201 Created`.

---

## 🗄️ Миграции БД (goose) и База Данных

В директории `migrations/` лежат SQL-файлы, которые автоматически применяются при старте `cmd/api`. 
* `001_init_schema.sql` – базовые таблицы (users, projects, tasks).
* `002...` до `004...` – эволюция БД (добавление логики событий и фиксы структуры уведомлений).

**Ключевые таблицы:**
1.  **`users`**: пользователи системы.
2.  **`projects`**: проекты (связь с создателем).
3.  **`tasks`**: задачи внутри проектов (связь с `projects` и исполнителем из `users`).
4.  **`events` / `project_calendar`**: встречи и дедлайны.
5.  **`notifications`**: система оповещений (связь с юзером, проектом или задачей).

