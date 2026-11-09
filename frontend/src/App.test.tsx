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
});
