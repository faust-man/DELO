import React, { useEffect, useState, useRef } from "react";
import {
  Palette,
  Users,
  CalendarDays,
  Image,
  Landmark,
  LayoutDashboard,
  Bell,
  Search,
  Plus,
  ArrowUpRight,
  LogOut,
  ShieldCheck,
  Menu,
} from "lucide-react";
import { api } from "./api";
import "./styles.css";
type Row = Record<string, string | number | boolean>;
type Org = { id: string; name: string; role: string };
const sections = [
  ["dashboard", "Обзор", LayoutDashboard],
  ["persons", "Люди", Users],
  ["branches", "Филиалы", LayoutDashboard],
  ["rooms", "Помещения", LayoutDashboard],
  ["groups", "Группы", Users],
  ["classes", "Расписание", CalendarDays],
  ["class_changes", "Переносы занятий", CalendarDays],
  ["operational_tasks", "Задачи", Bell],
  ["bookings", "Записи", CalendarDays],
  ["cancellation_events", "Отмены записей", CalendarDays],
  ["organization_policies", "Тестовые правила", ShieldCheck],
  ["tariffs", "Тарифы", Landmark],
  ["subscriptions", "Абонементы", CalendarDays],
  ["waitlist", "Лист ожидания", Users],
  ["attendance_events", "Посещения", CalendarDays],
  ["relationships", "Представители", Users],
  ["artworks", "Коллекция", Image],
  ["exhibitions", "Выставки", Palette],
  ["rights_confirmations", "Права на экспонирование", ShieldCheck],
  ["movements", "Перемещения", ArrowUpRight],
  ["artwork_receipts", "Приём картин", Image],
  ["payments", "Платежи", Landmark],
  ["refunds", "Возвраты", Landmark],
  ["charges", "Начисления", Landmark],
  ["allocations", "Распределения", Landmark],
  ["notifications", "Уведомления", Bell],
  ["finance_summary", "Финансовая сводка", Landmark],
  ["audit", "Журнал действий", ShieldCheck],
] as const;
const labels: Record<string, string> = {
  representative_id: "Представитель (ID)",
  child_id: "Ребёнок (ID)",
  visits: "Число посещений",
  price: "Цена в копейках",
  tariff_id: "Тариф (ID)",
  expires: "Срок действия до",
  booking_id: "Запись (ID)",
  status: "Статус (present)",
  charge_id: "Начисление (ID)",
  received: "Получено",
  refunded: "Возвращено",
  charged: "Начислено",
  allocated: "Распределено",
  artwork_id: "Картина (ID)",
  movement_id: "Отправка (ID)",
  evidence: "Документальное основание",
  destination: "Место назначения",
  policy_version: "Версия правила",
  late: "Поздняя отмена",
  cancel_hours: "Часов до отмены",
  demo: "Тестовая политика",
  address: "Адрес",
  timezone: "Часовой пояс IANA",
  branch_id: "Филиал (ID)",
  room_id: "Помещение (ID, необязательно)",
  teacher_id: "Преподаватель (ID, необязательно)",
  group_id: "Группа (ID, необязательно)",
  subscription_id: "Абонемент (ID, необязательно)",
  id: "ID",
  name: "Название / имя",
  contact: "Контакт",
  starts: "Начало",
  ends: "Окончание",
  capacity: "Мест",
  state: "Статус",
  number: "Инв. номер",
  author: "Автор",
  owner_name: "Собственник",
  location: "Местонахождение",
  condition: "Состояние",
  rights_confirmed: "Права подтверждены",
  person_id: "Клиент (ID)",
  class_id: "Занятие (ID)",
  amount: "Сумма в копейках",
  currency: "Валюта",
  reason: "Основание",
  payment_id: "Платёж (ID)",
  actor: "Автор",
  action: "Действие",
  object_id: "Объект",
  created_at: "Дата",
  message: "Событие",
};
const forms: Record<string, string[]> = {
  branches: ["name", "address", "timezone"],
  rooms: ["branch_id", "name", "capacity"],
  groups: ["name"],
  class_changes: ["class_id", "starts", "ends", "reason"],
  cancellation_events: ["booking_id", "reason"],
  movements: ["artwork_id", "destination", "reason"],
  rights_confirmations: ["artwork_id", "evidence"],
  artwork_receipts: ["movement_id", "condition"],
  relationships: ["representative_id", "child_id", "reason"],
  tariffs: ["name", "visits", "price", "currency"],
  subscriptions: ["person_id", "tariff_id", "expires"],
  waitlist: ["class_id", "person_id"],
  attendance_events: ["booking_id", "status", "reason"],
  charges: ["person_id", "amount", "currency", "reason"],
  allocations: ["payment_id", "charge_id", "amount"],
  persons: ["name", "contact"],
  classes: [
    "name",
    "starts",
    "ends",
    "capacity",
    "room_id",
    "teacher_id",
    "group_id",
  ],
  artworks: ["number", "name", "author", "owner_name", "location", "condition"],
  exhibitions: ["name", "starts", "ends", "expires"],
  bookings: ["class_id", "person_id", "subscription_id"],
  payments: ["person_id", "amount", "currency", "reason"],
  refunds: ["payment_id", "amount", "reason"],
};
export default function App() {
  const saveKey = useRef(crypto.randomUUID());
  const loadGeneration = useRef(0);
  const currentView = useRef({ org: "", page: "dashboard" });
  const [orgs, setOrgs] = useState<Org[]>([]),
    [org, setOrgState] = useState(""),
    [page, setPageState] = useState("dashboard"),
    [rows, setRows] = useState<Row[]>([]),
    [error, setError] = useState(""),
    [loading, setLoading] = useState(false),
    [modal, setModal] = useState(false),
    [query, setQuery] = useState(""),
    [menu, setMenu] = useState(false),
    [stats, setStats] = useState<Record<string, number>>({});
  function setOrg(next: string) {
    loadGeneration.current++;
    currentView.current.org = next;
    setRows([]);
    setStats({});
    setError("");
    setModal(false);
    setOrgState(next);
  }
  function setPage(next: string) {
    loadGeneration.current++;
    currentView.current.page = next;
    setRows([]);
    setStats({});
    setError("");
    setModal(false);
    setPageState(next);
  }
  async function init() {
    try {
      const result = await api("me", "");
      setOrgs(result);
      setOrg(result[0]?.id || "");
    } catch {
      setOrgs([]);
    }
  }
  useEffect(() => {
    init();
  }, []);
  async function load() {
    if (
      !org ||
      currentView.current.org !== org ||
      currentView.current.page !== page
    )
      return;
    const generation = ++loadGeneration.current;
    setRows([]);
    setError("");
    setLoading(true);
    try {
      if (page === "dashboard") {
        const values: Record<string, number> = {};
        for (const key of ["persons", "classes", "artworks", "exhibitions"]) {
          try {
            values[key] = (await api(key, org)).length;
          } catch {
            values[key] = -1;
          }
        }
        if (generation === loadGeneration.current) setStats(values);
      } else {
        const result = await api(page, org);
        if (generation === loadGeneration.current) setRows(result);
      }
    } catch (e) {
      if (generation === loadGeneration.current) setError((e as Error).message);
    } finally {
      if (generation === loadGeneration.current) setLoading(false);
    }
  }
  useEffect(() => {
    load();
  }, [org, page]);
  if (!orgs.length)
    return (
      <main className="login">
        <div className="login-art">
          <Palette size={56} />
          <h1>
            У искусства есть
            <br />
            <em>своё ДЕЛО.</em>
          </h1>
          <p>
            Люди, занятия и коллекции.
            <br />В одном пространстве.
          </p>
          <span>ТЕСТОВАЯ ВЕРСИЯ · 0.1</span>
        </div>
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            setError("");
            const f = new FormData(e.currentTarget);
            try {
              await api("login", "", {
                login: f.get("login"),
                password: f.get("password"),
              });
              await init();
            } catch (e) {
              setError((e as Error).message);
            }
          }}
        >
          <span className="eyebrow">РАБОЧЕЕ ПРОСТРАНСТВО</span>
          <h2>Рады видеть вас</h2>
          <p>Войдите в пространство вашей студии</p>
          <label>
            Электронная почта
            <input
              name="login"
              type="email"
              defaultValue="owner@demo.local"
              required
              autoComplete="username"
            />
          </label>
          <label>
            Пароль
            <input
              name="password"
              type="password"
              required
              autoComplete="current-password"
            />
          </label>
          {error && (
            <p role="alert" className="error">
              {error}
            </p>
          )}
          <button>
            Войти в ДЕЛО <ArrowUpRight size={18} />
          </button>
          <small>
            Только синтетические данные. Реальные платежи отключены.
          </small>
        </form>
      </main>
    );
  const active = sections.find((x) => x[0] === page);
  const visible = rows.filter((r) =>
    Object.values(r).join(" ").toLowerCase().includes(query.toLowerCase()),
  );
  return (
    <div className="shell">
      <aside className={menu ? "open" : ""}>
        <a className="brand" href="#" onClick={() => setPage("dashboard")}>
          <Palette /> ДЕЛО<span>studio workspace</span>
        </a>
        <div className="org">
          <span className="avatar">Д</span>
          <div>
            <strong>{orgs.find((o) => o.id === org)?.name}</strong>
            <small>Тестовое пространство</small>
          </div>
        </div>
        <span className="nav-label">ВАША СТУДИЯ</span>
        <nav>
          {sections.map(([key, title, Icon]) => (
            <button
              key={key}
              className={page === key ? "selected" : ""}
              onClick={() => {
                setPage(key);
                setModal(false);
                setMenu(false);
                setQuery("");
              }}
            >
              <Icon size={19} />
              {title}
            </button>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <span className="dot" /> Локальная dev-среда
          <small>v0.1 · правила требуют согласования</small>
        </div>
      </aside>
      <div className="workspace">
        <header>
          <button
            className="mobile-menu"
            onClick={() => setMenu(!menu)}
            aria-label="Меню"
          >
            <Menu />
          </button>
          <span>
            Пространство студии <b>/ {active?.[1]}</b>
          </span>
          <div>
            <select
              aria-label="Организация"
              value={org}
              onChange={(e) => {
                setRows([]);
                setOrg(e.target.value);
              }}
            >
              {orgs.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name}
                </option>
              ))}
            </select>
            <button
              className="icon"
              aria-label="Выйти"
              onClick={async () => {
                await api("logout", org, {});
                setRows([]);
                setOrgs([]);
                setOrg("");
              }}
            >
              <LogOut size={18} />
            </button>
          </div>
        </header>
        <main>
          <div className="page-heading">
            <div>
              <span className="eyebrow">
                ДЕЛО / {page === "dashboard" ? "КАЖДЫЙ ДЕНЬ" : "УЧЁТ"}
              </span>
              <h1>
                {page === "dashboard" ? "Всё начинается с идеи." : active?.[1]}
              </h1>
              <p>
                {page === "dashboard"
                  ? "А порядок помогает ей стать чем-то большим."
                  : "Актуальные данные вашей организации"}
              </p>
            </div>
            {forms[page] && (
              <button
                onClick={() => {
                  saveKey.current = crypto.randomUUID();
                  setModal(true);
                }}
              >
                <Plus size={18} />
                Добавить
              </button>
            )}
          </div>
          {error && (
            <p role="alert" className="error">
              {error}
            </p>
          )}
          {loading ? (
            <p role="status">Загружаем данные…</p>
          ) : page === "dashboard" ? (
            <>
              <section className="hero">
                <div>
                  <span className="eyebrow">ПРОСТРАНСТВО ДЛЯ ТВОРЧЕСТВА</span>
                  <h2>
                    Больше искусства.
                    <br />
                    Меньше рутины.
                  </h2>
                  <p>
                    Планируйте занятия, знакомьтесь с людьми
                    <br />и заботьтесь о каждой работе.
                  </p>
                  <button onClick={() => setPage("classes")}>
                    Открыть расписание <ArrowUpRight size={18} />
                  </button>
                </div>
                <div className="abstract" aria-hidden="true">
                  <i />
                  <i />
                  <i />
                  <span>Д.</span>
                </div>
              </section>
              <section className="stats">
                {[
                  ["persons", "Люди в студии"],
                  ["classes", "Занятия"],
                  ["artworks", "Работы в коллекции"],
                  ["exhibitions", "Выставки"],
                ].map(([key, title]) => (
                  <button key={key} onClick={() => setPage(key)}>
                    <span>
                      {title}
                      <ArrowUpRight size={16} />
                    </span>
                    <strong>
                      {stats[key] === -1 ? "—" : (stats[key] ?? "…")}
                    </strong>
                    <small>Открыть раздел</small>
                  </button>
                ))}
              </section>
              <section className="intro">
                <div>
                  <span className="eyebrow">С ЧЕГО НАЧАТЬ</span>
                  <h2>У каждого дела своё место</h2>
                  <p>
                    Добавьте первых клиентов и занятия.
                    <br />
                    Картины живут в отдельном каталоге.
                  </p>
                </div>
                <button
                  className="secondary"
                  onClick={() => setPage("artworks")}
                >
                  Перейти к коллекции <Image size={18} />
                </button>
              </section>
              <div className="notice">
                <ShieldCheck size={20} />
                <p>
                  <b>Тестовая среда</b> · Неизвестные правила денег, отмен и
                  передачи картин блокируют автоматические операции. Полный
                  статус реализации — в документации проекта.
                </p>
              </div>
            </>
          ) : (
            <section className="data-panel">
              <div className="toolbar">
                <b>
                  {active?.[1]} <span className="count">{rows.length}</span>
                </b>
                <label className="search">
                  <Search size={17} />
                  <input
                    aria-label="Поиск"
                    placeholder="Найти в списке…"
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                  />
                </label>
              </div>
              {visible.length ? (
                <div className="table-scroll">
                  <table>
                    <thead>
                      <tr>
                        {Object.keys(visible[0]).map((k) => (
                          <th key={k}>{labels[k] || k}</th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {visible.map((r, i) => (
                        <tr key={i}>
                          {Object.entries(r).map(([k, v]) => (
                            <td key={k} title={String(v)}>
                              {typeof v === "boolean"
                                ? v
                                  ? "Да"
                                  : "Нет"
                                : String(v ?? "—")}
                            </td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="empty">
                  <Palette size={36} />
                  <h2>
                    {query ? "Ничего не найдено" : "Здесь начинается ваше дело"}
                  </h2>
                  <p>
                    {query
                      ? "Попробуйте другой запрос."
                      : "Записей пока нет. Добавьте первую через кнопку выше."}
                  </p>
                </div>
              )}
              <small className="table-note">
                Показаны первые 500 записей. Все суммы — целое число минимальных
                денежных единиц.
              </small>
            </section>
          )}
        </main>
        <footer>
          ДЕЛО <span>Создано для тех, кто создаёт.</span>
          <small>DEV · синтетические данные</small>
        </footer>
      </div>
      {modal && (
        <div className="overlay">
          <form
            className="dialog"
            role="dialog"
            aria-modal="true"
            aria-label="Новая запись"
            onSubmit={async (e) => {
              e.preventDefault();
              setError("");
              const f = new FormData(e.currentTarget);
              const body: Record<string, unknown> = {};
              forms[page].forEach((k) => {
                let value: unknown = f.get(k);
                if (["amount", "capacity", "visits", "price"].includes(k))
                  value = Number(value);
                if (["starts", "ends", "expires"].includes(k))
                  value = new Date(String(value)).toISOString();
                body[k] = value;
              });
              setLoading(true);
              try {
                await api(
                  (
                    {
                      bookings: "book",
                      payments: "payment",
                      refunds: "refund",
                      class_changes: "move-class",
                      cancellation_events: "cancel-booking",
                      movements: "dispatch-artwork",
                      rights_confirmations: "confirm-rights",
                      artwork_receipts: "receive-artwork",
                      attendance_events: "attendance",
                      allocations: "allocate",
                    } as Record<string, string>
                  )[page] || page,
                  org,
                  body,
                  saveKey.current,
                );
                setModal(false);
                await load();
              } catch (e) {
                setError((e as Error).message);
              } finally {
                setLoading(false);
              }
            }}
          >
            <span className="eyebrow">НОВАЯ ЗАПИСЬ</span>
            <h2>{active?.[1]}</h2>
            {forms[page].map((k) => (
              <label key={k}>
                {labels[k]}
                <input
                  autoFocus={k === forms[page][0]}
                  name={k}
                  required={
                    ![
                      "contact",
                      "room_id",
                      "teacher_id",
                      "group_id",
                      "subscription_id",
                    ].includes(k)
                  }
                  type={
                    ["starts", "ends", "expires"].includes(k)
                      ? "datetime-local"
                      : ["amount", "capacity", "visits", "price"].includes(k)
                        ? "number"
                        : "text"
                  }
                  min={
                    ["amount", "capacity", "visits", "price"].includes(k)
                      ? 1
                      : undefined
                  }
                  defaultValue={k === "currency" ? "RUB" : undefined}
                />
              </label>
            ))}
            {error && (
              <p role="alert" className="error">
                {error}
              </p>
            )}
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setModal(false)}
              >
                Отмена
              </button>
              <button disabled={loading}>Сохранить</button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}
