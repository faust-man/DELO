# Сервисы и Linux-поставка через kubectl

Обновлено 10 ноября 2026 года. Подготовлены отдельные entrypoint, Docker-образы, Deployment и Service для каждой реализованной предметной области. Управление приложением не требует PowerShell, Helm или Docker Desktop. Основной путь установки — прямые команды kubectl.

## Границы сервисов и образов

| Сервис / суффикс образа | Ответственность |
|---|---|
| identity | Вход, выход, список организаций пользователя |
| contacts | Люди и представители |
| organizations | Филиалы, помещения, группы, политики |
| education | Занятия и изменения расписания |
| bookings | Запись, отмена, очередь, посещаемость |
| subscriptions | Тарифы и абонементы |
| finance | Платежи, начисления, распределения, возвраты |
| artworks | Картины и подтверждения прав |
| inventory | Перемещения и приём картин |
| exhibitions | Выставки |
| reports | Финансовая сводка |
| audit | Аудит и операционные задачи |
| notifications | Чтение уведомлений |
| worker | Доставка существующего транзакционного outbox |
| gateway | Маршрутизация HTTP |
| web | React SPA и nginx |
| admin | Миграции, ожидание схемы, начальная настройка владельца |

Итого 17 образов: 16 постоянно работающих компонентов и отдельный служебный admin. Каждый Go-образ собирается из собственного `backend/cmd/<service>/main.go` и содержит один бинарный файл `/app/service`; migrations и admin tools есть только в admin-образе. Предметная область фиксирована в entrypoint: переменная SERVICE_NAME не может превратить finance в identity. Общая Dockerfile задаёт одинаковую базовую среду, а отдельная сборка выбирает конкретную команду. Gateway не импортирует бизнес-приложение или pgx и не получает секрет БД.

Маршрут запроса: браузер → web → gateway → профильный API → PostgreSQL. Реестр маршрутов — `backend/internal/topology/routes.go`. Каждый API повторно проверяет cookie, организацию и роль; доверенных заголовков, предоставляющих права от имени gateway, нет. Чужие и неизвестные маршруты дают 404. Gateway сохраняет Origin, cookie, X-Organization-ID и Idempotency-Key; сам не повторяет неуспешные POST-запросы.

**Независимость данных пока не реализована:** доменные сервисы используют общий Go-пакет internal/app и транзакционную БД PostgreSQL. Разные образы и отдельные релизы реализованы, но архитектура данных остаётся связанной. Особенно связаны booking/subscription и finance/person. Для полной автономности нужны владельцы таблиц, отдельные credentials и миграции, внутренние контракты, локальные проекции и компенсации распределённых операций. Сохранённые общие транзакции защищают вместимость, посещения и финансовую идемпотентность. Эти гарантии нельзя убрать простой заменой подключения на разные базы. S3/PDF/онлайн-платежи ещё не реализованы и фиктивными сервисами не представлены.

## Подготовка Linux-кластера

Нужны доступный Linux Kubernetes-кластер, kubectl с правильным context, StorageClass по умолчанию для PVC PostgreSQL и CNI с поддержкой NetworkPolicy. Python 3 нужен для подготовки manifests. Docker/BuildKit нужен на машине сборки, а не на сервере с kubectl. Для закрытого registry настройте imagePullSecrets на service account namespace перед rollout. Привязки к имени context docker-desktop нет. Путь `local` означает тестовый профиль с одной БД и доступом через port-forward, он работает и в Linux.

### Сборка и публикация

Укажите доступный узлам registry и неизменяемый тег, например SHA исходного commit:

```bash
export IMAGE_PREFIX='registry.example.org/team/delo' # замените своим registry/repository
export IMAGE_TAG="$(git rev-parse HEAD)"
export PUSH=true
# После docker login к своему registry:
sh scripts/build-images.sh
python3 scripts/generate-k8s.py --image-prefix "$IMAGE_PREFIX" --tag "$IMAGE_TAG"
```

build-images.sh выполняет только сборку/публикацию. Для отдельного сервиса: `sh scripts/build-images.sh finance`. По умолчанию target platform — linux/amd64; для ARM64 задайте PLATFORM=linux/arm64 при наличии подходящего BuildKit. При установленной другой архитектуре узлов образы должны ей соответствовать.

Эквивалентная отдельная сборка без скрипта:

```bash
docker build --platform linux/amd64 --target service --build-arg SERVICE=finance \
  -t "$IMAGE_PREFIX-finance:$IMAGE_TAG" backend
docker push "$IMAGE_PREFIX-finance:$IMAGE_TAG"
```

Служебный образ собирается с `--target admin`, frontend — из каталога frontend. CI проверяет все 17 сборок в отдельных matrix jobs. Release workflow публикует каждый образ отдельно после успешного verify точного commit. Изменённые workflows в текущем сеансе не запускались в GitHub.

### Namespace и секреты для новой установки

Выполните один раз. Для существующей БД используйте её действующие credentials; не генерируйте новый пароль поверх существующего PVC.

```bash
kubectl create namespace delo-micro-local --dry-run=client -o yaml | kubectl apply -f -
DB_PASSWORD=$(openssl rand -hex 24)
printf 'password=%s\n' "$DB_PASSWORD" |
  kubectl create secret generic delo-db -n delo-micro-local --from-env-file=/dev/stdin
printf 'DATABASE_URL=postgres://delo:%s@delo-db:5432/delo?sslmode=disable\n' "$DB_PASSWORD" |
  kubectl create secret generic delo-runtime -n delo-micro-local --from-env-file=/dev/stdin
unset DB_PASSWORD
```

Пароли не записываются в manifests. Новый namespace отделён от старой Helm-установки `delo-local`; переноса старых данных автоматически нет.

## Запуск именно через kubectl

После подготовки образов, references в manifests и секретов:

```bash
kubectl apply -n delo-micro-local -k infra/k8s/local
kubectl rollout status deployment -n delo-micro-local --timeout=300s
kubectl port-forward -n delo-micro-local service/delo-web 8088:8080
```

Откройте http://localhost:8088 на машине с port-forward. Туннель по умолчанию слушает loopback. Манифесты включают namespace, PostgreSQL с PVC, migration Job, gateway и все приложения. API и worker имеют initContainer из admin-образа: он ждёт все миграции из текущего релиза и не выполняет DDL. Миграции выполняются только Job под advisory lock. При чистом apply admin сначала ждёт доступности PostgreSQL; общий timeout команды — 150 секунд, Job — 180 секунд. API не запускается при неготовой схеме. Gateway readiness показывает работоспособность самого gateway; доступность предметных операций проверяется отдельно.

Имя migration Job вычисляется из admin image и SQL-миграций: новый релиз создаёт новый Job без попытки изменить immutable template старого. Существующие Jobs автоматически не удаляются. Для повторного запуска неуспешного Job сначала исправьте причину, затем удалите именно его через `kubectl delete -n delo-micro-local -f infra/k8s/jobs/migrate.json` и повторите apply. После неуспешной миграции проверьте `kubectl logs -n delo-micro-local -f job/<имя-job>`; rollback кода не запускает destructive down.

### Первый владелец

После успешного rollout выполните отдельно (Bash):

```bash
read -r -s -p 'Пароль owner@demo.local (минимум 12 символов): ' BOOTSTRAP_PASSWORD
printf '\n'
printf 'BOOTSTRAP_PASSWORD=%s\n' "$BOOTSTRAP_PASSWORD" |
  kubectl create secret generic delo-bootstrap -n delo-micro-local --from-env-file=/dev/stdin
unset BOOTSTRAP_PASSWORD
kubectl apply -n delo-micro-local -f infra/k8s/jobs/bootstrap.json
kubectl wait -n delo-micro-local --for=condition=complete \
  -f infra/k8s/jobs/bootstrap.json --timeout=180s
kubectl delete secret delo-bootstrap -n delo-micro-local
```

Начальная настройка не перезаписывает существующего владельца. Повторное выполнение Job должно быть осознанным: прежний завершённый Job не запускается повторно при apply. Адрес владельца owner@demo.local — тестовая настройка.

## Независимое обновление сервиса

Опубликуйте новый finance-образ. При подготовке manifests сохраняйте references остальных компонентов текущего релиза:

```bash
python3 scripts/generate-k8s.py --image-prefix "$IMAGE_PREFIX" --tag "$IMAGE_TAG" \
  --image "finance=$IMAGE_PREFIX-finance:$FINANCE_TAG"
kubectl apply -n delo-micro-local -k infra/k8s/local
kubectl rollout status deployment/delo-finance -n delo-micro-local --timeout=300s
```

Опция --image повторяется и принимает тег либо digest. Если admin image и SQL не изменились, migration Job остаётся прежним. При изменении совместимых SQL-миграций поставьте также новый admin-образ и проверьте обратную совместимость работающих сервисов. Общая схема БД всё ещё ограничивает независимость несовместимых релизов.

## Ресурсы и безопасность

Каждый API, gateway, worker, migration/bootstrap Job и initContainer: requests 25m CPU / 32Mi памяти / 16Mi временного диска; limits 250m / 128Mi / 128Mi. Web: те же requests, limits 200m / 96Mi / 128Mi. PostgreSQL: requests 100m / 128Mi, limits 1 CPU / 512Mi, временный диск requests 32Mi / limits 256Mi, PVC 1Gi. Прикладной tmp ограничен 64Mi; пул каждого Go-процесса — максимум 4 соединения. Это стартовые значения без нагрузочной проверки.

Приложения без root, с read-only filesystem, без capabilities и service-account token. HTTP-компоненты имеют startup/readiness/liveness probes. Readiness API проверяет БД; worker сохраняет прежний цикл outbox без health endpoint. NetworkPolicy закрывает входящий трафик по умолчанию и разрешает web → gateway → API, API/worker/Jobs → PostgreSQL. Egress в тестовом профиле не ограничен; действие политик требует соответствующего CNI.

local/Linux dev: один PostgreSQL и HTTP через port-forward. Для новых dev/staging установок используются отдельные namespace и подготовленные service-image manifests; генератор поддерживает --namespace, --origin, --cookie-secure. Совместимый legacy Helm-профиль и Windows workflow сохранены для старой установки, это отдельный путь поставки с прежними API/worker. Production ещё не готов: нужны HA данных, TLS, secrets management, точные image digest, роли БД и capacity/DR-проверки. При внешнем HTTPS-доступе обязательно задайте реальный APP_ORIGIN и COOKIE_SECURE=true. Локальный тестовый профиль с COOKIE_SECURE=false нельзя считать production-профилем.

## Проверки и приёмка

Локально прошли Go unit/contract/topology tests, go vet, кросс-сборка всех Go entrypoint для linux/amd64, kubectl kustomize и статические проверки образов, лимитов и миграционных барьеров. Повторная проверка с доступным Docker: все 17 сборок, SQL integration tests, frontend tests/build/audit и чистый Kubernetes rollout прошли. Найдена и исправлена гонка миграции с запуском PostgreSQL. [Протокол и границы проверки](build-verification.md). Публикация в удалённый registry и production-приёмка не выполнялись.

На Linux-стенде проверить чистый apply и bootstrap; обновление только finance без изменения остальных Pod templates; вход, tenant/role boundaries и 404 чужого маршрута; остановку одного сервиса; запись на последнее место при двух bookings-репликах; повтор Idempotency-Key; обработку outbox двумя worker. Проверить неуспешную миграцию, NetworkPolicy на выбранном CNI, лимиты/OOM и восстановление PVC. Вход, HTTP-маршруты, отказы tenant/анонимного доступа, чистый apply и bootstrap подтверждены в [протоколе](build-verification.md). Остальные кластерные сценарии остаются планом проверки.
