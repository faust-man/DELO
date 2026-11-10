# ДЕЛО · тестовая версия

CRM художественной студии: 13 профильных Go API, gateway, worker, React/TypeScript и PostgreSQL. [Сервисная архитектура, границы и kubectl](docs/microservices.md). Это частичная реализация ТЗ, не готовый production. Подробный [статус каждого требования](docs/implementation-status.md) отделяет реализованные сценарии от планов.

## Linux: запуск через kubectl

После [подготовки отдельных Docker-образов, registry и секретов](docs/microservices.md):

```bash
kubectl apply -n delo-micro-local -k infra/k8s/local
kubectl rollout status deployment -n delo-micro-local --timeout=300s
kubectl port-forward -n delo-micro-local service/delo-web 8088:8080
```

Образы: identity, contacts, organizations, education, bookings, subscriptions, finance, artworks, inventory, exhibitions, reports, audit, notifications, gateway, worker, web и служебный admin. Каждый Go-сервис имеет свой entrypoint и образ; переключение его роли через переменную окружения не поддерживается. Gateway не получает доступ к БД. Общие бизнес-библиотека и схема PostgreSQL пока сохраняются: независимость данных сервисов ещё не реализована.

Манифесты включают migration Job и ожидание схемы перед запуском API/worker. PostgreSQL использует PVC. Первого владельца создаёт отдельный bootstrap Job — [порядок установки и обновления](docs/microservices.md). Интерфейс доступен по [localhost:8088](http://localhost:8088), пока работает port-forward. Kubernetes-профиль не зависит от Docker Desktop и Helm. Для разработки на Windows сохранён scripts/dev.ps1; основной способ Linux-поставки — команды выше.

Для разработки frontend: `cd frontend; npm ci; npm run dev`. Vite проксирует API на localhost:8080; запустите API с `DATABASE_URL`, `APP_ORIGIN=http://localhost:5173`, `COOKIE_SECURE=false` по `.env.example`. Сервер не читает `.env` автоматически.

## Проверки

[Протокол проверки 17 Docker-сборок, SQL-тестов и чистого Kubernetes-запуска](docs/build-verification.md).

```powershell
docker run -d --name delo-integration -e POSTGRES_PASSWORD=integration-only -p 127.0.0.1:55432:5432 postgres:17.6-alpine
$env:TEST_DATABASE_URL='postgres://postgres:integration-only@localhost:55432/postgres?sslmode=disable'
./scripts/check.ps1
```

Каждый интеграционный прогон создаёт отдельную случайную схему и удаляет только её. Не используйте production URL. CI выполняет Go race/vet, frontend тесты/build/audit, legacy Helm render, Kustomize render и проверки ресурсов. Все 17 Docker-образов собираются в отдельных Linux matrix jobs. Release workflow настроен на публикацию отдельных GHCR-образов после успешного verify того же commit; изменённый workflow в этом сеансе не запускался. Прежний Windows deploy-local workflow относится к совместимой Helm-установке и не является Linux-поставкой.

## Возможности тестовой версии

Вход, выход, серверные сессии, организация и роли; клиенты/представители; занятия и конкурентная запись; тарифы и снимки абонементов; резерв последнего посещения; ручной лист ожидания; отметка присутствия; ручные платежи/начисления/распределения/возвраты; картины и выставки; подтверждение основания экспонирования и индивидуальная отправка/приём через API; аудит и внутренние уведомления через транзакционный outbox. Все команды требуют `Idempotency-Key` и `Origin`; организация передаётся в `X-Organization-ID` и проверяется по членству.

Расписание пока отображается таблицей. Для связанных объектов в формах используются UUID из таблиц. Часть команд доступна только в API. UI не заменяет серверную проверку прав. Суммы — целые минимальные денежные единицы. Начисления и платежи раздельны, распределение требует одинакового клиента и валюты. Возврат распределённой суммы блокируется до реализации сторно распределения.

Реальные деньги, онлайн-провайдеры, файлы/S3, PDF и production отключены/не реализованы. HTTP и cookie без Secure разрешены исключительно локально. Неизвестные политики отмен/списаний не заменяются выдуманными правилами. Проектные документы остаются источником целевых требований, а не доказательством их реализации.
