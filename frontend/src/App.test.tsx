// @vitest-environment jsdom
import React from "react";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import App from "./App";
import { api } from "./api";
vi.mock("./api", () => ({ api: vi.fn() }));
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});

it("never shows a delayed response from the previous organization", async () => {
  let resolveOld!: (value: unknown) => void;
  let delay = false;
  vi.mocked(api).mockImplementation(async (path, org) => {
    if (path === "me")
      return [
        { id: "org-a", name: "Studio A", role: "owner" },
        { id: "org-b", name: "Studio B", role: "owner" },
      ];
    if (path === "persons" && delay && org === "org-a")
      return new Promise((resolve) => {
        resolveOld = resolve;
      });
    if (path === "persons" && org === "org-b")
      return [{ id: "b", name: "B private", contact: "" }];
    return [];
  });
  render(<App />);
  const people = await screen.findByRole("button", {
    name: "Люди",
  });
  delay = true;
  fireEvent.click(people);
  fireEvent.change(screen.getByRole("combobox", { name: "Организация" }), {
    target: { value: "org-b" },
  });
  expect(await screen.findByText("B private")).toBeTruthy();
  await act(async () => {
    resolveOld([{ id: "a", name: "A private", contact: "" }]);
  });
  expect(screen.queryByText("A private")).toBeNull();
  expect(screen.getByText("B private")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Люди" }));
  expect(screen.getByText("B private")).toBeTruthy();
});

it("submits the demo email and opens its studio after login", async () => {
  let loggedIn = false;
  vi.mocked(api).mockImplementation(async (path) => {
    if (path === "login") {
      loggedIn = true;
      return { status: "ok" };
    }
    if (path === "me") {
      if (!loggedIn) throw new Error("Войдите в систему");
      return [{ id: "demo", name: "Демо-студия", role: "owner" }];
    }
    return [];
  });
  render(<App />);
  const login = screen.getByLabelText("Логин или электронная почта") as HTMLInputElement;
  expect(login.value).toBe("admin@admin.com");
  fireEvent.change(screen.getByLabelText("Пароль"), { target: { value: "admin" } });
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "Войти в ДЕЛО" })); });
  expect(api).toHaveBeenCalledWith("login", "", { login: "admin@admin.com", password: "admin" });
  expect(await screen.findByRole("combobox", { name: "Организация" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Войти в ДЕЛО" })).toBeNull();
});

it("keeps the login form and displays an incorrect-password error", async () => {
  vi.mocked(api).mockImplementation(async (path) => {
    throw new Error(path === "login" ? "Неверные данные входа" : "Войдите в систему");
  });
  render(<App />);
  fireEvent.change(screen.getByLabelText("Пароль"), { target: { value: "wrong" } });
  await act(async () => { fireEvent.click(screen.getByRole("button", { name: "Войти в ДЕЛО" })); });
  expect(await screen.findByRole("alert")).toHaveProperty("textContent", "Неверные данные входа");
  expect(screen.queryByRole("combobox", { name: "Организация" })).toBeNull();
});
