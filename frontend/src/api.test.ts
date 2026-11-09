import { afterEach, expect, it, vi } from "vitest";
import { api } from "./api";
afterEach(() => vi.unstubAllGlobals());
it("sends organization and idempotency key on commands", async () => {
  const mock = vi
    .fn()
    .mockResolvedValue({ ok: true, json: async () => ({ id: "one" }) });
  vi.stubGlobal("fetch", mock);
  expect(await api("persons", "org-a", { name: "Тест" })).toEqual({
    id: "one",
  });
  const options = mock.mock.calls[0][1];
  expect(options.headers["X-Organization-ID"]).toBe("org-a");
  expect(options.headers["Idempotency-Key"]).toMatch(/^[0-9a-f-]{36}$/);
  expect(options.credentials).toBe("same-origin");
});
it("does not report failed saves as success", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: false,
      json: async () => ({ error: "Нет мест" }),
    }),
  );
  await expect(api("book", "a", {})).rejects.toThrow("Нет мест");
});
it("propagates network failures", async () => {
  vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")));
  await expect(api("persons", "a")).rejects.toThrow("offline");
});
