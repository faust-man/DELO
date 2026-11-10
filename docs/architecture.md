> Проверка 10 ноября 2026: [Docker-сборки, SQL и чистый Kubernetes-запуск прошли](build-verification.md); границы production-приёмки указаны в протоколе.

> Уточнение Linux-поставки от 10 ноября 2026: подготовлены отдельные entrypoint и Docker-образы сервисов, прямой kubectl apply -k с migration Job и ожиданием схемы. [Действующий порядок и ограничения](microservices.md). Бизнес-библиотека и БД пока общие; полная автономность данных не реализована.

> Изменение от 10 ноября 2026: по указанию заказчика подготовлена [сервисная топология и запуск через kubectl](microservices.md). Этот документ уточняет прежние предложения монолита и Helm. Реализовано разделение процессов и HTTP-маршрутов; общая БД и общий пакет бизнес-правил пока сохраняются. Кластерная приёмка не выполнена.

# Архитектура и структура проекта

> Обновление реализации, 10 ноября 2026: [статус всех требований и отметки +](implementation-status.md), [тестовые настройки](demo-settings.md), [запуск](../README.md). Ниже описана целевая система; выполненные проверки отделены от планов.

Версия 1.1, 7 ноября 2026 года. Дополнение к [ТЗ](technical-specification.md). Это структура будущей реализации; полный перечень исходников ещё не реализован; текущий тестовый срез указан в статусе. Backend — Go, frontend — React, платформа развёртывания — Kubernetes по прямому требованию заказчика. Остальной стек — проектное предложение; неизвестные решения отмечены `___`. Детальная инфраструктурная спецификация: [Kubernetes](kubernetes.md).

## 1. Схема приложения

Предлагается модульный монолит с горизонтально масштабируемыми Go API и worker в отдельных Kubernetes Deployments, React SPA в Deployment веб-сервера, HA PostgreSQL и закрытым отказоустойчивым S3-совместимым хранилищем. Единый код backend не означает одну реплику. Модульные границы позволяют впоследствии выделять сервисы по измеренным требованиям, сохраняя транзакционные инварианты. Kubernetes обязателен для dev, staging и production; локально используется облегчённый кластер.

```text
Браузер / HTTPS
       │
Балансировщик → Gateway API / gateway-controller
       │                     Kubernetes cluster
       ├── /       → Service → React static web Deployment (≥ 2 prod-реплик)
       └── /api/v1 → Service → Go API Deployment (≥ 2 prod-реплик, HPA)
                                  │
                       HA PostgreSQL + закрытый S3
                                  │
                       Go worker Deployment (≥ 2 prod-реплик)
                                  └── уведомления, документы, интеграции
Job миграции / CronJob обслуживания; мониторинг; внешнее хранилище секретов
```

Провайдеры, хостинг, домен, SMTP, объектное хранилище и сервис сканирования: ___. Redis, брокер сообщений и поисковый кластер не обязательны в базовой поставке; вводятся при измеренной необходимости. Для первого поиска достаточно PostgreSQL, нормализованных колонок и индексов. Публичный каталог, если понадобится, получает отдельную разрешённую проекцию, а не доступ к внутренним карточкам.

Предлагаемый frontend: TypeScript, Vite, React Router, TanStack Query для серверного состояния, React Hook Form и schema-валидация для форм. UI-библиотека ___; библиотека календаря и условия лицензии ___; генератор API-клиента ___; конкретные версии ___. Это не обязательность именно этих библиотек, а согласуемый базовый набор. Правила денег, прав и переходов всегда исполняются на сервере.

Предлагаемый backend: net/http с роутером ___, PostgreSQL-драйвер pgx, типизированные SQL-запросы sqlc либо ручной repository; окончательный выбор ___. Миграционный инструмент ___; формат — версионный SQL. ORM не является требованием. API-контракт — OpenAPI; версия спецификации ___, фиксируется инструментарием. Приложение выдаёт JSON, ошибки — единый формат.

## 2. Полная целевая структура репозитория

```text
DELO/
├── README.md                         # назначение, быстрый старт, ссылки
├── AGENTS.md                         # правила работы в проекте
├── .editorconfig
├── .gitignore
├── .env.example                      # имена настроек, без секретов
├── Makefile                          # единые задачи; на Windows через согласованный runner
├── skaffold.yaml                     # локальная сборка/обновление в Kubernetes
├── docs/
│   ├── technical-specification.md
│   ├── architecture.md
│   ├── kubernetes.md                 # топология, HA, масштабирование, DR, GitOps
│   ├── acceptance-and-decisions.md
│   ├── api/openapi.yaml              # источник истины HTTP-контрактов
│   ├── data/erd.md                   # связи и ограничения
│   ├── data/dictionary.md            # поля, единицы, nullable, PII
│   ├── permissions.md               # матрица действий и областей
│   ├── state-machines.md             # допустимые переходы и компенсации
│   ├── adr/                         # принятые архитектурные решения
│   ├── runbooks/                    # запуск, аварии, копии, восстановление
│   ├── user-guide/                  # инструкции по ролям
│   └── releases/                    # изменения, миграции, ограничения
├── backend/
│   ├── go.mod
│   ├── go.sum
│   ├── Dockerfile
│   ├── cmd/
│   │   ├── api/main.go
│   │   ├── worker/main.go
│   │   └── admin/main.go             # bootstrap владельца и сервисные операции
│   ├── internal/
│   │   ├── app/                     # сборка зависимостей и graceful shutdown
│   │   │   ├── api.go
│   │   │   ├── worker.go
│   │   │   └── dependencies.go
│   │   ├── platform/                # инфраструктура, без бизнес-правил
│   │   │   ├── config/
│   │   │   ├── database/            # pool, tx, миграционная совместимость
│   │   │   ├── httpserver/          # middleware, JSON, ошибки, request ID
│   │   │   ├── session/
│   │   │   ├── crypto/
│   │   │   ├── objectstore/
│   │   │   ├── observability/
│   │   │   ├── jobs/
│   │   │   ├── idempotency/
│   │   │   ├── clock/
│   │   │   └── validation/
│   │   └── modules/
│   │       ├── identity/            # аккаунты, вход, сессии, приглашения
│   │       ├── organizations/       # филиалы, помещения, настройки
│   │       ├── access/              # роли, разрешения, области доступа
│   │       ├── contacts/            # люди, представители, согласия, merge
│   │       ├── education/           # услуги, группы, расписание
│   │       ├── bookings/            # запись, лист ожидания, отметки
│   │       ├── subscriptions/       # тарифы, права, резервы, заморозки
│   │       ├── finance/             # начисления, платежи, возвраты, сверка
│   │       ├── artworks/            # работы, авторство, собственность
│   │       ├── inventory/           # локации, движения, инвентаризация
│   │       ├── exhibitions/         # планы выставок и участие
│   │       ├── sales/               # продажи, комиссии, расчёты с авторами
│   │       ├── documents/           # шаблоны, снимки, выпуск
│   │       ├── files/               # метаданные, карантин, превью, права
│   │       ├── notifications/       # правила, шаблоны, доставка
│   │       ├── reports/             # проекции и определения показателей
│   │       ├── imports/             # задания, строки, ошибки, ввод остатков
│   │       └── audit/               # бизнес-аудит и просмотр
│   ├── db/
│   │   ├── migrations/              # NNNN_name.up.sql / down.sql при безопасности
│   │   ├── queries/                 # явные SQL, разбивка по модулям
│   │   └── generated/               # sqlc, если выбран; вручную не править
│   ├── templates/                   # документные/почтовые шаблоны с версиями
│   ├── tests/
│   │   ├── integration/
│   │   ├── contracts/
│   │   ├── fixtures/                # синтетические данные
│   │   └── testutil/
│   └── tools/                       # конфигурация генераторов и линтеров
├── frontend/
│   ├── package.json
│   ├── lockfile                     # реальное имя после выбора package manager
│   ├── index.html
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── eslint.config.js
│   ├── Dockerfile
│   ├── public/                      # только публичные ресурсы приложения
│   ├── src/
│   │   ├── app/
│   │   │   ├── main.tsx
│   │   │   ├── router.tsx
│   │   │   ├── providers.tsx
│   │   │   ├── layouts/
│   │   │   └── styles/
│   │   ├── pages/
│   │   │   ├── auth/
│   │   │   ├── dashboard/
│   │   │   ├── contacts/
│   │   │   ├── calendar/
│   │   │   ├── groups/
│   │   │   ├── attendance/
│   │   │   ├── subscriptions/
│   │   │   ├── finance/
│   │   │   ├── artists/
│   │   │   ├── artworks/
│   │   │   ├── exhibitions/
│   │   │   ├── movements/
│   │   │   ├── documents/
│   │   │   ├── reports/
│   │   │   ├── settings/
│   │   │   ├── portal/              # клиент и художник при включении этапа
│   │   │   └── errors/
│   │   ├── features/                # пользовательские действия
│   │   │   ├── book-session/
│   │   │   ├── mark-attendance/
│   │   │   ├── sell-subscription/
│   │   │   ├── register-payment/
│   │   │   ├── refund-payment/
│   │   │   ├── dispatch-batch/
│   │   │   ├── receive-artwork/
│   │   │   ├── return-artworks/
│   │   │   ├── merge-contacts/
│   │   │   └── import-data/
│   │   ├── entities/                # типы, query keys, карточки и таблицы сущностей
│   │   └── shared/
│   │       ├── api/                 # клиент, generated types, обработка ошибок
│   │       ├── ui/                  # таблицы, формы, диалоги, skeleton, empty state
│   │       ├── auth/                # состояние сессии и отображение прав
│   │       ├── lib/                 # даты/валюты/форматирование без бизнес-списаний
│   │       ├── config/
│   │       └── assets/
│   └── tests/
│       ├── components/
│       └── e2e/
├── infra/
│   ├── local/kind.yaml              # локальный кластер; runtime-провайдер ___
│   ├── terraform/                   # сеть, кластеры, registry, state storage
│   ├── helm/delo/                   # chart приложения
│   │   ├── Chart.yaml
│   │   ├── values.yaml              # безопасная база; не готовый prod-профиль
│   │   ├── values.schema.json
│   │   └── templates/               # Deployments, Services, HPA, PDB, RBAC, jobs
│   ├── environments/                # values dev/staging/prod без секретов
│   ├── gitops/                      # Argo CD, порядок и разрешения синхронизации
│   ├── platform/                    # gateway, CNI, CSI, secrets, certificates
│   ├── data-services/               # операторы БД/S3 при размещении внутри k8s
│   ├── monitoring/                  # метрики, логи, трассы, правила оповещений
│   ├── policies/                    # Pod Security, NetworkPolicy, admission
│   └── backup/                      # политика копий и восстановление
├── scripts/
│   ├── dev.ps1                      # Windows bootstrap
│   ├── dev.sh                       # окружение Linux/macOS при необходимости
│   ├── backup/
│   ├── restore/
│   └── seed/                        # безопасный повторяемый demo seed
├── .github/workflows/               # если выбран GitHub; CI-провайдер ___
│   ├── backend.yml
│   ├── frontend.yml
│   └── release.yml
└── work_notes/
    ├── project_context/
    └── technical_specification/
```

### Устройство бизнес-модуля Go

```text
bookings/
├── domain/       # сущности, состояния, чистые инварианты и ошибки
├── application/  # команды/запросы, сценарии, границы транзакций, порты
├── postgres/     # реализации repository, блокировки и SQL-маппинг
├── transport/    # HTTP handlers, DTO, преобразование ошибок
└── module.go     # регистрация маршрутов и зависимостей
```

Для небольших справочников допустимо меньше файлов при сохранении разделения ответственности. HTTP-handler не выполняет SQL и не вычисляет стоимость возврата. Repository не принимает бизнес-решения о допустимости пропуска. Cross-module сценарии координируются application-сервисом через явные интерфейсы и общую транзакцию; циклические зависимости модулей запрещены. Транзакционные операции не разносятся в асинхронные подписчики, если это нарушает инвариант.

### Устройство frontend

Направление зависимостей: app → pages → features → entities → shared. Shared не импортирует бизнес-страницы. Формы используют API DTO и собственную модель ввода, не копируют внутреннюю структуру таблиц БД. В shared/api хранятся сгенерированные типы и единый transport; серверное состояние кешируется по организации и области доступа, сбрасывается при выходе/смене организации. Необходимые данные одной организации не остаются видимыми при переключении.

## 3. Данные и связи

Имена таблиц предварительные. Для бизнес-таблиц обязательны `id UUID`, `organization_id`, `created_at`, `updated_at`, при изменяемости `version`; по необходимости `created_by`, `archived_at`. Время событий — `timestamptz`, локальные правила расписания — отдельно timezone и wall-clock поля. Справочники и историю нельзя принудительно помещать в одну универсальную JSON-таблицу. JSON допустим для версионных снимков, внешних payload с ограничением данных и безопасных метаданных.

| Область | Таблицы и ключевые поля/связи |
|---|---|
| Аккаунты | users (нормализованный login, password_hash, status), sessions (hash токена, user, expiry, revoked), invitations, recovery_tokens; users глобальны, доступ через memberships |
| Доступ | memberships (org, user, status), roles, permissions, role_permissions, membership_roles, scope_assignments; активное членство обязательно для каждой операции |
| Организация | organizations, branches (timezone), rooms (branch, capacity), organization_settings_versions (effective_from, settings) |
| Люди | persons, person_roles, person_contacts (type, normalized_value, verified), person_relationships (from, to, type, permissions, valid_from/to), staff_profiles, artist_profiles |
| Согласия | consents (person, representative, purpose, text_version, granted_at, revoked_at, evidence_file), contact_merge_events, restricted_notes |
| Обучение | services, service_versions, groups, group_memberships (интервал), group_teachers, schedule_rules (timezone, recurrence, generation_cursor), schedule_exceptions |
| Занятия | class_sessions (group/service, start/end, room, capacity, state, version), class_session_teachers, class_session_changes (snapshot, reason) |
| Записи | bookings (session, person, state, funding_kind), attendance (booking unique, state, marked_by), waitlist_entries, seat_offers (expiry, state), booking_policy_snapshots |
| Абонементы | tariff_versions, subscriptions (owner, beneficiary, terms_snapshot, dates, state), subscription_freezes, entitlement_reservations (booking unique active), entitlement_ledger (delta, kind, source, reversed_entry) |
| Начисления | charges (debtor, beneficiary, currency, state), charge_lines (service/source, qty, price_minor, discount_minor), charge_adjustments, credit_notes |
| Деньги | payments (payer, amount_minor, currency, method, state, external_id), payment_allocations (payment, charge, amount_minor), refunds, refund_allocations (исходные payment allocations), finance_ledger, reconciliations, reconciliation_items |
| Картины | artworks (inventory_number, title, dimensions, condition, commercial_state), artwork_authors, ownership_periods (owner, share, basis, from/to), artwork_media, artwork_rights, condition_reports (photos, event), artwork_custody_state (artwork unique, location, custodian, version) |
| Локации | locations (type, parent, branch, external_partner), counterparties, custody_agreements |
| Выставки | exhibitions (dates, state, curator, partner), exhibition_artworks (plan, period, deadline, state), exhibition_participants |
| Движения | movement_batches (from/to, state, direction, carrier, dates), movement_items (artwork, source_position, per_item_state), movement_events (from/to, time, actor), custody_acceptances, incidents |
| Инвентаризация | inventory_sessions (scope, date, state), inventory_items (expected/actual, condition, resolution), inventory_adjustments |
| Продажи | sale_orders, sale_items (artwork unique при активной продаже), commission_terms_versions, owner_settlements, owner_payouts; связь с charges/payments и movement_items |
| Документы | document_templates, documents (number, state, immutable_snapshot, issued_at), document_links, document_signatures (method, evidence, time) |
| Файлы | files (key, size, mime, hash, scan_state, storage_version), file_links (типизированные допустимые связи с проверкой org), file_variants, upload_sessions |
| Уведомления | notification_rules, notification_templates, notification_preferences, notifications (event_key, recipient, channel, state), notification_attempts |
| Фоновые операции | outbox_events (event_id, type, version, payload, available_at), job_runs (attempts, locked_until), processed_events, import_jobs, import_rows, export_jobs |
| Контроль | audit_events, idempotency_keys (org, actor, route, key, request_hash, response, expires_at), webhook_events (provider, event_id, signature_result, state) |

Схема уточняется ERD до реализации миграций. Обязательные детали:

- Бизнес-внешние ключи составные `(organization_id, target_id)`, с уникальностью пары у цели; одного UUID недостаточно для защиты от ошибочной межорганизационной ссылки. Для глобального users используется обычный FK и membership-проверка.
- Уникальны: номер картины в организации; внешний ID платежа в пределах аккаунта провайдера; event_id webhook; активная запись ученика на занятие; источник списания/сторно; позиция принятия одной передачи. Для nullable ключей семантика NULL задаётся явно.
- Денежные значения `bigint` в минимальных единицах + ISO-код валюты, CHECK amount ≥ 0 там, где значение не является подписанным движением журнала. В ledger разрешён знак согласно типу движения, сумма не меняется после проведения. Лимиты и переполнение проверяются.
- У занятия start < end и capacity > 0; фактическая вместимость не уменьшается ниже числа confirmed без разрешения конфликта. Пересечения проверяются транзакционно; для помещений/преподавателей предлагаются booking-таблицы ресурсов с exclusion constraint на диапазон времени либо эквивалентная блокировка.
- Текущее местонахождение — единственная строка custody_state. Проверка версии и блокировка работы защищают от двойной отправки; перемещение подтверждено журналом. Нельзя поддерживать несколько независимых изменяемых колонок «текущая выставка/склад» как источники истины.
- Для долевой собственности доли действующего периода суммируются в единицу; необходимость долевого владения ___. Если первый выпуск поддерживает только одного собственника, это ограничение явно включается в интерфейс и импорт, а соавторство сохраняется отдельно.
- Индексы начинаются с organization_id для типовых списков; дополнительно session.start, booking.session/person, charge.debtor/state, movement.deadline/state, artwork.inventory_number, contacts.normalized_value. Финальный набор — по EXPLAIN и нагрузочным запросам.
- Удаление users не разрушает историю: бизнес-инициатор сохраняется допустимым обезличенным reference. Политика удаления и архивирования определяет конкретные FK RESTRICT/SET NULL; каскад на денежные/складские события запрещён.

## 4. Модель транзакций и конкуренции

### 4.1 Запись на последнее место

API проверяет членство и разрешение → начинает транзакцию → блокирует занятие и выбранный абонемент/его баланс → проверяет состояние, свободные места и срок → создаёт/переиспользует запись → создаёт резерв посещения либо согласованное начисление → пишет аудит и outbox → commit. Число мест учитывает confirmed и действующие seat_offers; offer конвертируется без двойного счёта. Одновременные операции берут блокировки в одинаковом порядке. Конфликт возвращает 409 с причиной и актуальными доступными данными.

### 4.2 Посещаемость и корректировка

Блокируются запись и текущие движения абонемента. Для present резерв расходуется ровно один раз; для смены отметки записывается сторно исходного движения, затем новое по сохранённой политике. Финансовые коррекции входят в ту же транзакцию, внешний денежный возврат создаётся как отдельный workflow. Уникальность source_event и optimistic version защищает повтор.

### 4.3 Возврат денег

Локальная транзакция резервирует сумму возврата и создаёт refund с уникальным ключом. Worker вызывает провайдера с его idempotency key; ответ и webhook согласуются через external ID. При timeout сначала выясняется результат, а не создаётся новая выплата. Завершённый возврат атомарно меняет финансовый журнал и распределения; компенсация начисления и прав посещения выполняется по явному основанию. Для ручного возврата уполномоченный сотрудник подтверждает факт и документ.

### 4.4 Передача партии

В транзакции все работы блокируются по отсортированным ID, сверяются исходные локации/права/резервы, фиксируются immutable-состав и событие dispatch, custody становится «в пути», создаются аудит/outbox. Приём каждой позиции отдельной транзакцией проверяет ожидаемую передачу и меняет custody на целевую локацию. Статус партии вычисляется по позициям. После сбоя сервер отвечает исходом всей транзакции; «половина картины переехала» недопустима.

### 4.5 Outbox и фоновая обработка

Бизнес-событие и outbox записываются одним commit. Worker забирает задания с блокировкой/lease, выполняет, фиксирует результат; после аварии истёкший lease позволяет повтор. Семантика доставки — как минимум один раз, потребитель использует event_id и уникальный business key. Обещания exactly-once для внешних каналов нет. Неудачные задания после лимита попадают в доступный оператору список с безопасным повтором.

Периодические задачи: генерация занятий; истечение предложений мест; активация/истечение абонементов; обработка заморозок; напоминания; сверка processing-платежей/возвратов; контроль приёма/возврата картин; карантин и превью файлов; генерация документов; импорт/экспорт; очистка истёкших сессий и временных файлов. Повторные запуски и несколько worker не дублируют результат. Конкретные интервалы и лимиты ___.

## 5. HTTP API

Префикс `/api/v1`. Ресурсы организации: `/organizations/{orgId}/...`, исключая глобальные auth/me и проверяемые webhook. Списки — `items` + cursor/has_more; пагинация и стабильная сортировка обязательны, максимальный limit ___. Фильтры дат задают полуоткрытый период; даты без времени и абсолютные timestamps имеют разные типы. Форматы — ISO 8601/RFC 3339 по назначению, деньги `{amountMinor, currency}`; bigint передаётся строкой, если может выйти за безопасный integer JavaScript.

| Группа | Основные ресурсы и команды |
|---|---|
| Вход | POST /auth/login, /logout, /recovery/request, /recovery/confirm; GET /me; GET/DELETE /me/sessions; POST /invitations/{token}/accept |
| Организация | GET/POST /organizations; GET/PATCH /organizations/{orgId}; /branches, /rooms, /settings; /memberships, /roles, /permissions, /invitations |
| Люди | GET/POST /persons; GET/PATCH /persons/{id}; POST /{id}/archive, /{id}/merge-preview, /{id}/merge; /{id}/contacts, /relationships, /consents, /timeline |
| Обучение | /services, /groups, /schedule-rules; POST /schedule-rules/{id}/preview, /generate; /class-sessions; POST /class-sessions/{id}/reschedule, /cancel, /complete, /reopen |
| Записи | GET/POST /class-sessions/{id}/bookings; POST /bookings/{id}/cancel; PUT /bookings/{id}/attendance; /class-sessions/{id}/waitlist; POST /seat-offers/{id}/accept, /decline |
| Абонементы | /tariffs, /tariff-versions, /subscriptions; POST /subscriptions/{id}/activate, /freeze-preview, /freeze, /unfreeze, /cancel-preview, /cancel; GET /{id}/ledger |
| Финансы | /charges, /payments; POST /payments/{id}/confirm, /allocate; /refunds; POST /refunds/{id}/approve, /reject, /confirm-manual; /reconciliations; GET /persons/{id}/balance |
| Картины | /artists, /artworks; /artworks/{id}/authors, /ownership, /condition-reports, /history; POST /artworks/{id}/intake, /archive, /labels |
| Выставки | /exhibitions; /exhibitions/{id}/artworks; POST /{id}/plan, /open, /close-preview, /close, /cancel |
| Перемещения | /locations, /movement-batches; POST /movement-batches/{id}/dispatch, /cancel; POST /movement-items/{id}/receive, /report-discrepancy; POST /movement-batches/{id}/return-draft; /incidents |
| Инвентаризация | /inventory-sessions; POST /{id}/scan, /complete, /resolve; GET /{id}/discrepancies |
| Продажи | /sale-orders; POST /{id}/reserve, /confirm, /fulfill, /cancel, /return-preview, /return; /owner-settlements, /owner-payouts |
| Документы | /document-templates, /documents; POST /documents/{id}/issue, /register-signature, /void; GET /documents/{id}/download |
| Файлы | POST /uploads; POST /uploads/{id}/complete; GET /files/{id}/download; DELETE /files/{id} с проверкой связей и retention |
| Уведомления | /notifications, /notification-preferences, /notification-rules; POST /notifications/{id}/retry с разрешением |
| Отчёты | GET /reports/attendance, /occupancy, /receivables, /cash-flow, /subscriptions, /artwork-locations, /overdue-returns, /owner-settlements |
| Обмен | POST /imports, /imports/{id}/validate, /commit; GET /imports/{id}/rows; POST /exports; GET /exports/{id} |
| Аудит | GET /audit-events, GET /jobs/{id} для разрешённого статуса задачи |
| Интеграции | POST /webhooks/{provider}; GET /health/live, /health/ready; /metrics закрыт инфраструктурой |

В таблице пути в пределах организации сокращены: перед ними добавляется `/organizations/{orgId}`. Для коллекций явно разрешаются GET/POST, для карточек GET/PATCH, если состояние допускает изменение. Произвольный PATCH state не заменяет перечисленные бизнес-команды. DELETE применяется только к безопасным черновикам без истории; иначе archive/cancel/void с причиной. Точные DTO и разрешения каждого метода фиксируются в OpenAPI до реализации. Глобальное создание организации доступно только по разрешённому сценарию регистрации/оператору ___, а не любому вошедшему пользователю.

### Общие контракты

- 200 — результат, 201 — создан ресурс с Location, 202 — принято фоновое задание с jobId/statusUrl, 204 — нет тела.
- 400 — неверный формат, 401 — нет сессии, 403 — запрещённое действие, 404 — объект не найден в доступной области, 409 — бизнес-конфликт или конфликт версии, 413 — размер, 422 — ошибка полей/правил, 429 — лимит, 500/503 — внутренняя ошибка/временная недоступность. Существование чужой карточки не раскрывается через разницу 403/404.
- Ошибка: `{code, message, fieldErrors, requestId, details}`; code стабилен, message пригоден пользователю, details не содержат SQL/секретов. Примеры code: `CLASS_FULL`, `SUBSCRIPTION_EXPIRED`, `VERSION_CONFLICT`, `ARTWORK_LOCATION_CHANGED`, `REFUND_LIMIT_EXCEEDED`, `POLICY_NOT_CONFIGURED`.
- На создании платежей, возвратов, записей, проведении движений и импорте обязателен `Idempotency-Key`. Ключ связан с org, actor, операцией и hash payload. Тот же ключ + другое тело → 409; тот же запрос возвращает прежний результат. Retention ___ не меньше окна допустимых повторов; долговременная уникальность операции обеспечивается также business constraint, а не только временным кешем.
- Изменяемая карточка передаёт version/If-Match. Старая версия → 409/согласованный код ___, обновление не затирает чужие изменения. Все API выбирают одну согласованную схему конфликтов до реализации.
- Preview финансовых/складских последствий возвращает snapshotVersion и итог. Commit заново валидирует данные; изменившиеся условия требуют нового preview. Preview не резервирует ресурс, если это явно не указано.
- Webhook проверяет подпись и привязку к аккаунту провайдера, хранит уникальный event ID, допускает повтор и нарушение порядка. Успех клиентской страницы оплаты не подтверждает платёж. Для входящего события сначала надёжная фиксация, затем обработка; provider-specific ответ ___ устанавливается контрактом интеграции.

Пример предлагаемого запроса записи:

```json
{
  "personId": "uuid",
  "funding": {"type": "subscription", "subscriptionId": "uuid"},
  "expectedSessionVersion": 7
}
```

Ответ содержит booking ID/state, резерв и новую версию. Данные цены и прав из браузера сервер не принимает как источник истины. Комментарий и причина проверяются по длине ___ и видимости.

## 6. Навигация frontend

```text
/login, /recover, /accept-invite
/o/:orgId/dashboard
/o/:orgId/contacts, /contacts/:personId
/o/:orgId/calendar, /classes/:sessionId, /groups/:groupId
/o/:orgId/subscriptions, /subscriptions/:id
/o/:orgId/finance/{charges,payments,refunds,reconciliation}
/o/:orgId/artists/:personId
/o/:orgId/artworks, /artworks/:id
/o/:orgId/exhibitions, /exhibitions/:id
/o/:orgId/movements, /movements/:id
/o/:orgId/inventory, /inventory/:id
/o/:orgId/sales, /settlements
/o/:orgId/documents, /reports, /imports, /notifications
/o/:orgId/settings/{organization,branches,staff,roles,tariffs,policies,integrations,audit}
/portal/:orgId/{schedule,bookings,subscriptions,payments,artworks,documents}
```

Маршруты расширений существуют только при включённом модуле и разрешении. Форма новой/изменяемой записи может быть страницей или диалогом; прямой URL карточки стабилен. Страница ошибки содержит request ID и безопасный путь назад. Инвалидация кеша после отметки посещения обновляет запись, занятие, абонемент и баланс, а не только закрывает диалог.

## 7. Конфигурация и развёртывание

Единственная целевая платформа — Kubernetes. Dockerfile описывает сборку OCI-образа, а не отдельный способ production-запуска. Docker Compose не входит в целевую структуру. Локальный цикл: kind → платформа разработки → Helm dev values → Skaffold; инструменты и версии фиксируются до реализации. Production отделён от non-production кластером, staging/dev разделены namespace и политиками. Конкретные провайдер, мощности, регионы, домены и стоимость ___.

Предлагаемый способ поставки — Helm + Argo CD GitOps; infra/terraform управляет внешними ресурсами. CI публикует неизменяемые образы по digest, проверяет chart и создаёт изменение конфигурации среды. GitOps приводит кластер к зафиксированному состоянию с контролируемым production-процессом. Доступ CI к cluster-admin не требуется. Ресурсы, реплики, HPA/PDB, шлюз, БД/S3, CronJobs, политики безопасности и DR определены в [спецификации Kubernetes](kubernetes.md).

Минимальные переменные: APP_ENV, HTTP_ADDR, PUBLIC_BASE_URL, DATABASE_URL, SESSION_KEY_REFERENCE, OBJECT_STORAGE_ENDPOINT/BUCKET/REGION, STORAGE_CREDENTIAL_REFERENCE, SMTP/NOTIFICATION_PROVIDER_REFERENCE, PAYMENT_PROVIDER_REFERENCE, LOG_LEVEL, OTEL_ENDPOINT при подключении. Секреты передаются через защищённую конфигурацию; реальные значения не пишутся в env.example. Локальный режим использует тестовые провайдеры, фальшивые деньги и почтовую песочницу.

Backend и worker собираются из одного commit. Контейнеры выполняются без root, имеют ограниченные ресурсы, healthchecks и graceful shutdown. Worker завершает lease корректно либо оставляет его для безопасного подбора. Статические frontend-ресурсы версионируются, index.html получает подходящую политику кеша; API никогда не кешируется публично с персональными данными.

CI: format/lint/typecheck → unit → integration на настоящем PostgreSQL → API contract → frontend build → E2E критических сценариев → dependency/container scan → сборка OCI-образов, SBOM, подпись → Helm lint/render и проверка политик → Kubernetes staging и smoke → обновление GitOps-конфигурации production. Реестр образов ___; процесс согласования production ___; ответственный ___. Секреты CI минимальны и разделены по средам. Публичный выпуск и подключение реальных платежей выполняются только в согласованной среде.

Релиз совместим с текущей и следующей схемой в период раскатки. Порядок: проверка актуальной восстанавливаемой копии/WAL → однократный Job совместимой миграции с блокировкой БД → backend/worker RollingUpdate → frontend → smoke → мониторинг. Неуспешная миграция блокирует rollout. Удаление старых колонок отдельным релизом. Откат образов и GitOps-ревизии допустим только при совместимости схемы; иначе forward-fix по runbook. Обычный Helm rollback не восстанавливает БД и файлы.

## 8. Требования к реализации тестов

Unit — денежные формулы, границы дат/сроков, переходы состояний, списание и сторно. Integration — PostgreSQL constraints, транзакции, конкурентная запись/отправка, изоляция организаций, worker после сбоя, повторы webhook. Contract — соответствие OpenAPI и сгенерированных TS-типов. E2E — пользовательские цепочки на синтетических данных и с ролями. Производительность и восстановление проверяются по профилю ТЗ. Конкретные инструменты тестирования frontend/E2E ___; предлагаются Vitest, Testing Library и Playwright после согласования.

Не требуются тесты, повторяющие каждую строку простого CRUD. Обязательны проверки рисков: чужой orgId, чужой fileId, одновременное последнее место/посещение, повтор платежа/возврата, повтор приёма картины, частичная партия и сбой между commit/outbox-доставкой. Полный перечень приёмки приведён отдельно.
