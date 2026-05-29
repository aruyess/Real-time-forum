# Real-Time Forum

SPA-форум на Go + JS со SQLite и WebSocket-чатом в реальном времени.

## Возможности

**Регистрация и сессии**
- Регистрация с никнеймом, email, возрастом, полом, именем и фамилией
- Вход по никнейму или email
- Одна активная сессия на пользователя (вход в новом браузере вытесняет старую)
- Редактирование собственного профиля, кроме email

**Посты**
- Создание с одним или несколькими категориями
- Редактирование и удаление только своих постов
- Лайки/дизлайки

**Комментарии**
- Добавление, редактирование, удаление
- Лайки/дизлайки

**Личные сообщения**
- Чат 1-к-1 с моментальной доставкой через WebSocket
- Прикрепление изображений (JPEG/PNG/GIF/WebP до 5 МБ)
- Подгрузка истории по 10 сообщений при скролле наверх
- Сортировка контактов в стиле Discord: с кем переписывался — выше, остальные — по алфавиту

**Реальное время**
- Индикатор онлайн/оффлайн в боковой панели
- Уведомление о новом сообщении
- Авто-переподключение WebSocket с экспоненциальным backoff

**UI**
- Одна HTML-страница, hash-router в JS
- Светлая и тёмная темы
- Sidebar с тремя секциями: навигация, категории-тогглы, пользователи

## Структура проекта

```
real-time-forum/
├── cmd/
│   ├── server/main.go            ← точка входа, читает FORUM_DB и FORUM_ADDR
│   └── seed/main.go              ← демо-данные для аудиторов
├── internal/
│   ├── db/                       ← SQLite-слой
│   │   ├── schema.sql            ← embed-схема, применяется при старте
│   │   ├── db.go                 ← Open + миграции
│   │   ├── users.go, posts.go, comments.go,
│   │   ├── messages.go, reactions.go, categories.go
│   ├── handlers/                 ← HTTP-хендлеры (auth, posts, users,
│   │                                messages, reactions, uploads, ws)
│   ├── models/                   ← структуры User, Post, Comment, Message
│   ├── routes/routes.go          ← регистрация всех маршрутов на ServeMux
│   ├── session/session.go        ← cookie-сессии (HttpOnly + SameSite=Lax)
│   └── ws/                       ← WebSocket: Hub + Client (gorilla)
├── web/
│   ├── index.html                ← оболочка SPA + inline theme-init
│   ├── css/style.css             ← все стили (тёмная и светлая темы)
│   └── js/
│       ├── app.js                ← bootstrap: theme, /api/me, WS, роутер
│       ├── api.js                ← fetch-обёртка, get/post/put/del/upload
│       ├── router.js             ← hash-роутинг с regex-паттернами
│       ├── state.js              ← state.user — единственный источник правды
│       ├── layout.js             ← topbar + sidebar shell, idempotent
│       ├── sidebar.js            ← навигация + категории + пользователи
│       ├── reactions.js          ← общий обработчик лайков/дизлайков
│       ├── unread.js             ← счётчик непрочитанных + pub-sub
│       ├── ws.js                 ← клиент WebSocket с auto-reconnect
│       ├── theme.js              ← переключатель темы
│       ├── utils.js              ← escapeHTML, timeAgo, throttle, debounce
│       └── views/                ← страницы:
│           login.js, register.js, feed.js, post.js, post-card.js,
│           chat.js, profile.js
├── Dockerfile                    ← multi-stage build (alpine, ~26 MB)
├── docker-compose.yml            ← запуск с volume для forum.db
├── go.mod / go.sum
└── uploads/                      ← runtime: картинки из чата (в .gitignore)
```

## Запуск

```bash
git clone https://01.tomorrow-school.ai/git/azzz/real-time-forum.git
cd real-time-forum
go run ./cmd/server
```

Открыть **http://localhost:8080**.

Демо-данные для аудита:
```bash
go run ./cmd/seed
```

Сидер создаёт пользователей **Ada**, **Grace**, **Lori** и 7 постов.
Пароль для всех демо-пользователей: `password123`.
Команду можно запускать повторно — дубликаты не создаются.

Через Docker:
```bash
docker compose up --build
docker compose exec forum /app/seed
```

## Авторы

- **Arua Yess** — `#ayess`
- **Saken Nabu** — `#azzz`
