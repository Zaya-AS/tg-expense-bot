# Развёртывание и обслуживание

## Первый запуск

1. Скопируйте `.env.example` в `.env` в корне проекта. Заполните `TELEGRAM_BOT_TOKEN` и `POSTGRES_PASSWORD`. Не добавляйте `.env` в Git.
2. Выполните `docker compose up -d --build`.
3. Проверьте `docker compose ps` и `docker compose logs --tail=100 bot`.
4. Отправьте боту `/start`, затем `/help`.

Бот применяет SQL-файлы `migrations/*.up.sql` при старте. Выполненные версии хранятся в таблице `schema_migrations`. На сервере, где таблицы уже были созданы через `/docker-entrypoint-initdb.d`, первая миграция будет отмечена как выполненная без повторного создания таблиц. Следующие миграции выполняются автоматически. Не удаляйте том PostgreSQL ради миграций.

Перед обновлением действующей базы создайте резервную копию:

```sh
git pull origin main
sh deploy/backup.sh
docker compose up -d --build bot
docker compose logs --tail=100 bot
```

Если миграция завершится ошибкой, бот не начнёт обработку сообщений. Проверьте логи и исправьте причину, сохранив том БД.

## Команды бота

Бот обрабатывает команды только в личном чате, чтобы история расходов и отчёты не появлялись в группах.

- `/help` — список команд.
- `/timezone Asia/Yekaterinburg` — установить часовой пояс для отчёта и дат в истории. Без аргумента показывает текущий. По умолчанию используется `UTC`.
- `/category Еда` — добавить категорию; `/categories` — показать все категории.
- `/expense 250.50 | Еда | Обед` — записать расход в рублях. Описание после второго `|` необязательно.
- `/last` — показать последние 10 расходов с их номерами.
- `/delete 123` — удалить свой расход с номером `123`. Запись скрывается из истории и отчёта, но сохраняется для защиты от повторной доставки сообщения Telegram.
- `/report` — расходы за текущий месяц в часовом поясе пользователя.

## Резервные копии

Из корня проекта запустите `sh deploy/backup.sh`. Скрипт сохраняет дамп PostgreSQL в `backups/` с правами доступа только владельца. Для другого каталога укажите его первым аргументом: `sh deploy/backup.sh /path/to/backups`. Каталог `backups/` игнорируется Git.

Для ежедневного запуска можно добавить в crontab сервера, указав реальный путь проекта:

```cron
0 3 * * * cd /path/to/tg-expense-bot && /bin/sh deploy/backup.sh >> /path/to/backup.log 2>&1
```

Периодически копируйте дампы за пределы сервера и проверяйте восстановление в отдельной тестовой БД. Дамп содержит данные пользователей и расходов.

Проверка дампа в отдельной базе на том же сервере (замените имя файла):

```sh
docker compose exec -T db sh -c 'PGPASSWORD="$POSTGRES_PASSWORD" createdb -h 127.0.0.1 -U "$POSTGRES_USER" expenses_restore_test'
docker compose exec -T db sh -c 'PGPASSWORD="$POSTGRES_PASSWORD" pg_restore -h 127.0.0.1 -U "$POSTGRES_USER" -d expenses_restore_test --no-owner' < backups/ИМЯ_ФАЙЛА.dump
docker compose exec -T db sh -c 'PGPASSWORD="$POSTGRES_PASSWORD" psql -h 127.0.0.1 -U "$POSTGRES_USER" -d expenses_restore_test -c "SELECT count(*) FROM users"'
```

После проверки удалите только тестовую базу командой `docker compose exec -T db sh -c 'PGPASSWORD="$POSTGRES_PASSWORD" dropdb -h 127.0.0.1 -U "$POSTGRES_USER" expenses_restore_test'`. Для локальных интеграционных тестов задайте `TEST_DATABASE_URL` отдельной тестовой PostgreSQL и запустите `go test ./...`; тесты создают и удаляют собственную схему.

Compose передаёт настройки БД контейнеру бота через `DB_*`. `DATABASE_URL` в `.env` используется только при запуске `go run ./cmd/api` вне Compose. Если пароль содержит `$`, заключите значение в одинарные кавычки в `.env`, чтобы Compose передал его буквально.
