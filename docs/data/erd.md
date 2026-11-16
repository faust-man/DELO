# Схема тестовой реализации

```mermaid
erDiagram
 organizations ||--o{ memberships : contains
 users ||--o{ memberships : joins
 users ||--o{ sessions : authenticates
 organizations ||--o{ persons : owns
 persons ||--o{ relationships : represents
 organizations ||--o{ branches : contains
 branches ||--o{ rooms : contains
 rooms ||--o{ classes : hosts
 groups ||--o{ classes : schedules
 persons ||--o{ bookings : attends
 classes ||--o{ bookings : contains
 tariffs ||--o{ subscriptions : snapshots
 subscriptions ||--o{ visit_reservations : reserves
 bookings ||--o| visit_reservations : uses
 bookings ||--o| attendance_events : marks
 bookings ||--o| cancellation_events : cancels
 persons ||--o{ payments : pays
 payments ||--o{ refunds : refunds
 payments ||--o{ allocations : allocates
 charges ||--o{ allocations : settles
 artworks ||--o{ movements : moves
 movements ||--o| artwork_receipts : receives
 audit ||--|| outbox : emits
 outbox ||--o| notifications : delivers
```

Бизнес-связи используют `(org,id)` и составные внешние ключи, исключающие межорганизационную связь. Идентификаторы сущностей UUID, событий outbox/audit — bigint. Денежные суммы bigint в минимальных единицах; API ограничивает безопасный целочисленный диапазон JSON. Время — timestamptz; филиал дополнительно хранит IANA timezone. Имена/контакты PII, bcrypt hash/хеш токена — секретные данные; их нет в audit. Организации изолированы запросами application, PostgreSQL RLS пока отсутствует.

Источники схемы — последовательные SQL-файлы backend/db/migrations. В production нужны отдельные DB-роли migrator/runtime и внешняя защита аудита; локальный PostgreSQL пока использует одну техническую роль.
