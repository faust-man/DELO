# Локальная эксплуатация

Развёртывание: [README](../../README.md), `scripts/dev.ps1`. Namespace `delo-local`, отдельный PostgreSQL PVC, API/web/worker. Обновление образов: `./scripts/dev.ps1 -Build`; Helm pre-upgrade Job выполняет новые миграции под advisory lock. Миграции записываются в `schema_migrations`, выполняются в одной транзакции. Откат DDL не реализован; используйте совместимое исправление вперёд.

Диагностика:

```powershell
kubectl -n delo-local get pods,jobs,pvc
kubectl -n delo-local logs deployment/delo-api
kubectl -n delo-local logs deployment/delo-worker
kubectl -n delo-local describe pod <имя>
```

`/healthz` проверяет процесс API, `/readyz` — подключение к БД. Worker пока не имеет HTTP health/метрик. Namespace содержит только тестовые данные; не удаляйте его для обычного обновления. Пароли смотрите только в защищённом административном контуре Kubernetes, не копируйте в журнал проверки.

## Копирование dev-БД

Согласованный production backup/PITR отсутствует. Для локальной резервной копии используйте `pg_dump -Fc` через административный процесс, храните результат вне Git, проверьте `pg_restore` в новую изолированную БД, затем сравните количества строк и финансовые итоги. Бинарный dump нельзя перенаправлять старым Windows PowerShell без проверки бинарной сохранности. Автоматический backup/restore и испытание восстановления не реализованы. PVC не считается резервной копией.

## Профили сред

local проверяется в Docker Desktop; dev/staging values — шаблоны с недействующими доменами. Production намеренно отклоняется values.schema.json: отсутствуют выбранные HA PostgreSQL/S3, TLS gateway, secret store, мониторинг, резервирование и проверка DR. Не объявлять SLA/RPO/RTO достигнутыми. Политики сети требуют поддерживающего CNI; сам факт применения YAML не доказывает фильтрацию. В dev разрешён трафик между Pod namespace на 8080/5432; это ещё не production least-privilege.
