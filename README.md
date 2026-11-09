# ДЕЛО · тестовая версия

CRM художественной студии: Go API + отдельный worker, React/TypeScript, PostgreSQL, Helm, Docker Desktop Kubernetes. Это частичная реализация ТЗ, не готовый production. Подробный [статус каждого требования](docs/implementation-status.md) отделяет реализованные сценарии от планов.

## Запуск на Windows

Требуются Docker Desktop с включённым Kubernetes (`docker-desktop`), kubectl, Helm 3, Node.js 24+, Go 1.27.3 для разработки. Локально скачанные Go/Helm лежат в игнорируемом `.tools/`; при переносе установите их самостоятельно.

```powershell
./scripts/dev.ps1 -Build -Bootstrap
```

Скрипт создаёт namespace `delo-local`, случайный пароль БД в Kubernetes Secret, PostgreSQL с PVC, выполняет миграции, запускает API/web/worker. Пароль владельца `owner@demo.local` вводится интерактивно. Повторный bootstrap существующего владельца завершится ошибкой, данные не перезапишет. Следующие запуски: `./scripts/dev.ps1 -Build`. Откройте [локальный интерфейс](http://localhost:8088); port-forward должен оставаться запущенным. Секреты не сохраняются в Git.

Для разработки frontend: `cd frontend; npm ci; npm run dev`. Vite проксирует API на localhost:8080; запустите API с `DATABASE_URL`, `APP_ORIGIN=http://localhost:5173`, `COOKIE_SECURE=false` по `.env.example`. Сервер не читает `.env` автоматически.

## Проверки

```powershell
docker run -d --name delo-integration -e POSTGRES_PASSWORD=integration-only -p 127.0.0.1:55432:5432 postgres:17.6-alpine
$env:TEST_DATABASE_URL='postgres://postgres:integration-only@localhost:55432/postgres?sslmode=disable'
./scripts/check.ps1
```

Каждый интеграционный прогон создаёт отдельную случайную схему и удаляет только её. Не используйте production URL. CI выполняет Go race/vet, frontend тесты/build/audit, Helm lint/render и сборки образов. Release workflow публикует образы в GHCR после успешного verify того же commit; фактический GitHub запуск требует push ветки и доступности release workflow на default branch. Локальная поставка выполняется `dev.ps1`. Для ручного CD есть `deploy-local.yml`: нужен отдельно зарегистрированный Windows self-hosted runner с меткой `delo-dev`, Helm/Docker/kubectl, защищённый environment `local-dev` и инициализированный namespace. Workflow проверяет успешный CI для точного commit. Runner в этом сеансе не регистрировался, GitHub workflows не запускались.

## Возможности тестовой версии

Вход, выход, серверные сессии, организация и роли; клиенты/представители; занятия и конкурентная запись; тарифы и снимки абонементов; резерв последнего посещения; ручной лист ожидания; отметка присутствия; ручные платежи/начисления/распределения/возвраты; картины и выставки; подтверждение основания экспонирования и индивидуальная отправка/приём через API; аудит и внутренние уведомления через транзакционный outbox. Все команды требуют `Idempotency-Key` и `Origin`; организация передаётся в `X-Organization-ID` и проверяется по членству.

Расписание пока отображается таблицей. Для связанных объектов в формах используются UUID из таблиц. Часть команд доступна только в API. UI не заменяет серверную проверку прав. Суммы — целые минимальные денежные единицы. Начисления и платежи раздельны, распределение требует одинакового клиента и валюты. Возврат распределённой суммы блокируется до реализации сторно распределения.

Реальные деньги, онлайн-провайдеры, файлы/S3, PDF и production отключены/не реализованы. HTTP и cookie без Secure разрешены исключительно локально. Неизвестные политики отмен/списаний не заменяются выдуманными правилами. Проектные документы остаются источником целевых требований, а не доказательством их реализации.
