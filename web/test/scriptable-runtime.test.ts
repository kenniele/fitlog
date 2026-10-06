// @vitest-environment node
import { readFileSync } from "node:fs";
import vm from "node:vm";
import { describe, expect, it } from "vitest";

const source = readFileSync("public/scriptable/FitLog.js", "utf8");
const key = "fitlog.widget.connection.v1";
const token = "signed.widget-key";
const data = {
  version: 1,
  date: "2026-10-06",
  timezone: "Europe/Moscow",
  generated_at: "2026-10-06T10:00:00Z",
  recovery: { date: "2026-10-06", recovery_score: 82, hrv_ms: null },
  sleep: { date: "2026-10-05", sleep_seconds: 27900 },
  nutrition: { date: "2026-10-04", calories_kcal: 0 },
  body: { measured_at: "2026-08-01T23:00:00Z", weight_kg: 80 },
  training: {
    sessions: 0,
    working_sets: 0,
    repetitions: 0,
    volume_kg: 0,
    average_rir: null,
  },
  next_workout: { title: "Upper", scheduled_at: "2026-10-07T16:00:00Z" },
};

class Node {
  children: Node[] = [];
  value = "";
  font: unknown;
  textColor: unknown;
  lineLimit = 0;
  minimumScaleFactor = 1;
  addText(value: string) {
    const child = new Node();
    child.value = value;
    this.children.push(child);
    return child;
  }
  addStack() {
    const child = new Node();
    this.children.push(child);
    return child;
  }
  addSpacer() {}
  layoutVertically() {}
  layoutHorizontally() {}
  setPadding() {}
  async presentLarge() {}
  texts(): string[] {
    return this.children
      .flatMap((child) => [child.value, ...child.texts()])
      .filter(Boolean);
  }
  metricCount() {
    return this.children
      .filter((row) => row.children.some((cell) => cell.children.length === 2))
      .reduce(
        (count, row) =>
          count +
          row.children.filter((cell) => cell.children.length === 2).length,
        0,
      );
  }
}

async function run(
  options: {
    family?: string;
    section?: string;
    status?: number;
    saved?: string | null;
    networkError?: boolean;
  } = {},
) {
  const requests: {
    url: string;
    headers: Record<string, string>;
    onRedirect: () => null;
    timeoutInterval: number;
  }[] = [];
  const store = new Map<string, string>();
  if (options.saved !== null)
    store.set(
      key,
      options.saved ??
        JSON.stringify({ origin: "https://fitlog.example.com", token }),
    );
  let widget: Node | null = null,
    completed = false;
  class Request {
    headers: Record<string, string> = {};
    timeoutInterval = 0;
    onRedirect = () => null;
    response = { statusCode: options.status ?? 200 };
    constructor(public url: string) {}
    async loadString() {
      requests.push(this);
      if (options.networkError)
        throw new Error("private implementation detail");
      return JSON.stringify({ data });
    }
  }
  const context = {
    config: { runsInApp: false, widgetFamily: options.family ?? "large" },
    args: { widgetParameter: options.section ?? "" },
    Request,
    ListWidget: Node,
    Color: class {
      constructor(public hex: string) {}
    },
    Size: class {
      constructor(
        public width: number,
        public height: number,
      ) {}
    },
    Font: {
      systemFont: (size: number) => size,
      semiboldSystemFont: (size: number) => size,
    },
    Keychain: {
      contains: (key: string) => store.has(key),
      get: (key: string) => store.get(key),
      set: (key: string, value: string) => store.set(key, value),
      remove: (key: string) => store.delete(key),
    },
    Script: {
      setWidget: (value: Node) => {
        widget = value;
      },
      complete: () => {
        completed = true;
      },
    },
  };
  await vm.runInNewContext(source, context, { timeout: 1000 });
  return { widget: widget as Node | null, requests, store, completed };
}

describe("distributed Scriptable file", () => {
  it.each([
    ["small", 4],
    ["medium", 8],
    ["large", 24],
  ])(
    "renders %s with %i numeric fields and only the authorized API request",
    async (family, count) => {
      const result = await run({ family: String(family) });
      expect(result.completed).toBe(true);
      expect(result.widget?.metricCount()).toBe(count);
      expect(result.requests).toHaveLength(1);
      const request = result.requests[0];
      expect(request.url).toBe("https://fitlog.example.com/api/v1/widget");
      expect(request.headers.Authorization).toBe(`Bearer ${token}`);
      expect(request.onRedirect()).toBeNull();
      expect(request.timeoutInterval).toBe(15);
      expect(result.widget?.texts()).toContain("0");
      expect(result.widget?.texts()).toContain("В 06.10 · С 05.10");
      expect(result.widget?.texts()).not.toContain(token);
    },
  );

  it("preserves nulls, zero, old body date and the latest nutrition date in the large layout", async () => {
    const result = await run();
    const texts = result.widget?.texts() ?? [];
    expect(texts).toContain("—");
    expect(texts).toContain("0 кг");
    expect(
      texts.some(
        (text) => text.includes("П 04.10") && text.includes("02.08.26"),
      ),
    ).toBe(true);
    expect(
      texts.some((text) => text.includes("Upper") && text.includes("19:00")),
    ).toBe(true);
  });

  it("supports a separate sleep block selected with the widget parameter", async () => {
    const result = await run({ section: "sleep" });
    expect(result.widget?.metricCount()).toBe(5);
    expect(result.widget?.texts()).toContain("СОН");
    expect(result.widget?.texts()).toContain("Данные: 05.10");
    expect(result.widget?.texts()).not.toContain("Ккал");
  });

  it.each([401, 403])(
    "clears expired or rejected credentials on HTTP %i",
    async (status) => {
      const result = await run({ status });
      expect(result.store.has(key)).toBe(false);
      expect(result.widget?.texts()).toContain(
        "Ключ истёк. Подключите FitLog заново.",
      );
      expect(result.widget?.metricCount()).toBe(0);
    },
  );

  it("does not make requests or prompt for credentials while an unconfigured widget runs", async () => {
    const result = await run({ saved: null });
    expect(result.requests).toHaveLength(0);
    expect(result.widget?.texts().join(" ")).toContain("запустите FitLog");
  });

  it("rejects insecure and path-based origins before transmitting a key", async () => {
    for (const origin of [
      "http://fitlog.example.com",
      "https://other.example.com/path",
      "https://user@other.example.com",
      "https://other.example.com?token=secret",
    ]) {
      const result = await run({ saved: JSON.stringify({ origin, token }) });
      expect(result.requests).toHaveLength(0);
      expect(result.store.has(key)).toBe(false);
    }
  });

  it("does not replace an unavailable server with fabricated numbers or expose underlying errors", async () => {
    const result = await run({ networkError: true });
    expect(result.widget?.metricCount()).toBe(0);
    expect(result.widget?.texts().join(" ")).not.toContain(
      "private implementation detail",
    );
    expect(result.store.has(key)).toBe(true);
  });
});
