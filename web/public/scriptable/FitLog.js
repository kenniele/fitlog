// Variables used by Scriptable.
// icon-color: deep-green; icon-glyph: heartbeat;
// FitLog: numeric widgets. Download this file only from your FitLog or its repository.
// First run: enter your HTTPS server origin and the widget-only key from Summary.
// No dashboard token, browser cookie, analytics or third-party requests are used.

/* global Request, ListWidget, Color, Font, Size, Alert, Keychain, Script, config, args */

const CREDENTIAL_KEY = "fitlog.widget.connection.v1";
const MISSING = "—";
const COLORS = {
  background: "0d1110",
  text: "e9efeb",
  muted: "9baaa2",
  accent: "89cbb0",
};

function knownError(message) {
  const error = new Error(message);
  error.safe = true;
  return error;
}

function originURL(input) {
  const match =
    /^https:\/\/([a-z0-9](?:[a-z0-9.-]*[a-z0-9])?)(?::([0-9]{1,5}))?\/?$/i.exec(
      String(input).trim(),
    );
  if (
    !match ||
    (match[2] && (Number(match[2]) < 1 || Number(match[2]) > 65535))
  ) {
    throw knownError(
      "Укажите HTTPS-адрес сервера без пути, например https://fitlog.example.com",
    );
  }
  return "https://" + match[1].toLowerCase() + (match[2] ? ":" + match[2] : "");
}

function connection(input) {
  const origin = originURL(input.origin);
  const token = String(input.token || "").trim();
  if (token.length > 512 || !/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/.test(token)) {
    throw knownError(
      "Вставьте ключ виджета со страницы «Сводка», а не токен входа в FitLog.",
    );
  }
  return { origin, token };
}

function number(value, unit, digits) {
  if (typeof value !== "number" || !Number.isFinite(value)) return MISSING;
  return (
    value.toLocaleString("ru-RU", {
      maximumFractionDigits: digits == null ? 1 : digits,
    }) + (unit || "")
  );
}

function duration(value) {
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0)
    return MISSING;
  const minutes = Math.round(value / 60);
  return Math.floor(minutes / 60) + "ч " + (minutes % 60) + "м";
}

function day(value) {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(String(value || ""));
  return match ? match[3] + "." + match[2] : MISSING;
}

function localDate(value, zone, withTime) {
  if (!value) return MISSING;
  try {
    const date = new Date(value);
    if (!Number.isFinite(date.getTime())) return MISSING;
    const options = {
      timeZone: zone,
      day: "2-digit",
      month: "2-digit",
      year: "2-digit",
    };
    if (withTime) {
      options.hour = "2-digit";
      options.minute = "2-digit";
      options.hourCycle = "h23";
    }
    return new Intl.DateTimeFormat("ru-RU", options).format(date);
  } catch {
    return MISSING;
  }
}

async function snapshot(settings) {
  const request = new Request(settings.origin + "/api/v1/widget");
  request.headers = {
    Authorization: "Bearer " + settings.token,
    Accept: "application/json",
    "Cache-Control": "no-cache",
  };
  request.timeoutInterval = 15;
  request.onRedirect = () => null;
  const raw = await request.loadString();
  const status = request.response ? request.response.statusCode : 0;
  if (status === 401 || status === 403) {
    const error = knownError(
      "Создайте новый ключ в «Сводке» и подключите Scriptable заново.",
    );
    error.authentication = true;
    throw error;
  }
  if (status !== 200)
    throw knownError("Сервер недоступен. Проверьте адрес и интернет.");
  let payload;
  try {
    payload = JSON.parse(raw);
  } catch {
    throw knownError("Сервер вернул неверный ответ. Проверьте адрес FitLog.");
  }
  const data = payload && payload.data;
  if (
    !data ||
    data.version !== 1 ||
    !data.training ||
    !/^\d{4}-\d{2}-\d{2}$/.test(data.date) ||
    !Number.isFinite(new Date(data.generated_at).getTime())
  ) {
    throw knownError("Обновите скрипт и сервер FitLog.");
  }
  return data;
}

function metrics(data) {
  const r = data.recovery || {},
    s = data.sleep || {},
    n = data.nutrition || {},
    b = data.body || {},
    t = data.training;
  const recovery = [
    ["Восст.", number(r.recovery_score, "%", 0)],
    ["HRV", number(r.hrv_ms, " мс", 0)],
    ["Пульс", number(r.resting_heart_rate_bpm, " уд", 0)],
    ["Нагрузка", number(r.daily_strain)],
    ["Дыхание", number(r.respiratory_rate, "/мин")],
    ["SpO₂", number(r.spo2_percent, "%")],
    ["Кожа", number(r.skin_temperature_c, "°C")],
  ];
  const sleep = [
    ["Сон", duration(s.sleep_seconds)],
    ["Норма сна", number(s.sleep_performance_percent, "%", 0)],
    ["Эфф. сна", number(s.sleep_efficiency_percent, "%", 0)],
    ["REM", duration(s.rem_seconds)],
    ["Глубокий", duration(s.deep_seconds)],
  ];
  const training = [
    ["Трен. / 7д", number(t.sessions, "", 0)],
    ["Подходы", number(t.working_sets, "", 0)],
    ["Тоннаж", number(t.volume_kg, " кг", 0)],
    ["Повторы", number(t.repetitions, "", 0)],
    ["RIR", number(t.average_rir)],
  ];
  const nutrition = [
    ["Ккал", number(n.calories_kcal, "", 0)],
    ["Белки", number(n.protein_g, " г")],
    ["Жиры", number(n.fat_g, " г")],
    ["Углеводы", number(n.carbohydrates_g, " г")],
  ];
  const body = [
    ["Вес", number(b.weight_kg, " кг")],
    ["Жир", number(b.body_fat_percent, "%")],
    ["Мышцы", number(b.skeletal_muscle_mass_kg, " кг")],
  ];
  // All 24 metrics in six compact rows. Smaller widgets show a useful subset.
  const all = [
    recovery[0],
    sleep[0],
    training[0],
    nutrition[0],
    recovery[1],
    recovery[2],
    recovery[3],
    sleep[1],
    sleep[2],
    sleep[3],
    sleep[4],
    recovery[4],
    recovery[5],
    recovery[6],
    body[0],
    body[1],
    body[2],
    nutrition[1],
    nutrition[2],
    nutrition[3],
    training[1],
    training[2],
    training[3],
    training[4],
  ];
  return { all, recovery, sleep, training, nutrition, body };
}

function text(stack, value, size, color, bold) {
  const item = stack.addText(String(value));
  item.font = bold ? Font.semiboldSystemFont(size) : Font.systemFont(size);
  item.textColor = new Color(color || COLORS.text);
  item.lineLimit = 1;
  item.minimumScaleFactor = 0.65;
  return item;
}

function baseWidget() {
  const widget = new ListWidget();
  widget.backgroundColor = new Color(COLORS.background);
  widget.setPadding(12, 12, 12, 12);
  widget.refreshAfterDate = new Date(Date.now() + 30 * 60 * 1000);
  return widget;
}

function draw(data, settings, family, section) {
  const widget = baseWidget();
  widget.url = settings.origin + "/dashboard/summary";
  const groups = metrics(data);
  const titles = {
    all: "FITLOG",
    recovery: "ВОССТАНОВЛЕНИЕ",
    sleep: "СОН",
    training: "ТРЕНИРОВКИ · 7Д",
    nutrition: "ПИТАНИЕ",
    body: "СОСТАВ ТЕЛА",
  };
  const selected = Object.prototype.hasOwnProperty.call(groups, section)
    ? section
    : "all";
  const header = widget.addStack();
  text(header, titles[selected], 10, COLORS.accent, true);
  header.addSpacer();
  text(header, day(data.date), 9, COLORS.muted);
  widget.addSpacer(7);
  const limit = family === "small" ? 4 : family === "medium" ? 8 : 24;
  const items = groups[selected].slice(0, limit);
  const columns =
    family === "small" || (family === "large" && selected !== "all") ? 2 : 4;
  const compact = selected === "all" && family === "large";
  for (let index = 0; index < items.length; index += columns) {
    const row = widget.addStack();
    row.layoutHorizontally();
    for (let column = 0; column < columns; column++) {
      const cell = row.addStack();
      cell.layoutVertically();
      const width = family === "small" ? 54 : columns === 4 ? 67 : 130;
      cell.size = new Size(width, 0);
      const metric = items[index + column];
      if (metric) {
        text(cell, metric[0], compact ? 8 : 9, COLORS.muted);
        text(cell, metric[1], compact ? 13 : 16, COLORS.text, true);
      }
      if (column + 1 < columns) row.addSpacer();
    }
    widget.addSpacer(compact ? 4 : 7);
  }
  widget.addSpacer();
  if (selected === "all") {
    text(
      widget,
      "В " +
        day(data.recovery && data.recovery.date) +
        " · С " +
        day(data.sleep && data.sleep.date),
      8,
      COLORS.muted,
    );
    if (family === "large") {
      text(
        widget,
        "П " +
          day(data.nutrition && data.nutrition.date) +
          " · Тело " +
          localDate(data.body && data.body.measured_at, data.timezone, false),
        8,
        COLORS.muted,
      );
    } else {
      text(
        widget,
        "П " + day(data.nutrition && data.nutrition.date),
        8,
        COLORS.muted,
      );
    }
  } else {
    const observed =
      selected === "body"
        ? localDate(data.body && data.body.measured_at, data.timezone, false)
        : selected === "training"
          ? "7 дней по " + day(data.date)
          : day(data[selected] && data[selected].date);
    text(widget, "Данные: " + observed, 8, COLORS.muted);
  }
  if (family === "large" && data.next_workout) {
    text(
      widget,
      data.next_workout.title +
        " · " +
        localDate(data.next_workout.scheduled_at, data.timezone, true),
      9,
      COLORS.accent,
    );
  }
  text(
    widget,
    "Обновлено " + localDate(data.generated_at, data.timezone, true),
    8,
    COLORS.muted,
  );
  return widget;
}

function errorWidget(message) {
  const widget = baseWidget();
  text(widget, "FITLOG", 11, COLORS.accent, true);
  widget.addSpacer(10);
  const item = widget.addText(message);
  item.font = Font.systemFont(12);
  item.textColor = new Color(COLORS.muted);
  item.lineLimit = 5;
  widget.addSpacer();
  return widget;
}

async function setup(previous) {
  const alert = new Alert();
  alert.title = "Подключить FitLog";
  alert.message =
    "HTTPS-адрес сервера без /dashboard и ключ виджета со страницы «Сводка».";
  alert.addTextField(
    "https://fitlog.example.com",
    previous ? previous.origin : "",
  );
  alert.addSecureTextField("Ключ виджета", "");
  alert.addAction("Подключить");
  alert.addCancelAction("Отмена");
  if ((await alert.presentAlert()) < 0) return null;
  const settings = connection({
    origin: alert.textFieldValue(0),
    token: alert.textFieldValue(1),
  });
  const data = await snapshot(settings);
  // Save only after a successful authenticated request; never save the login token.
  Keychain.set(CREDENTIAL_KEY, JSON.stringify(settings));
  return { settings, data };
}

async function main() {
  const family =
    ["small", "medium", "large"].indexOf(config.widgetFamily) >= 0
      ? config.widgetFamily
      : "large";
  const section = String(args.widgetParameter || "all")
    .trim()
    .toLowerCase();
  let settings = null,
    data = null;
  if (Keychain.contains(CREDENTIAL_KEY)) {
    try {
      settings = connection(JSON.parse(Keychain.get(CREDENTIAL_KEY)));
    } catch {
      Keychain.remove(CREDENTIAL_KEY);
    }
  }
  if (config.runsInApp) {
    if (settings) {
      const menu = new Alert();
      menu.title = "FitLog";
      menu.addAction("Обновить и показать");
      menu.addAction("Подключить заново");
      menu.addDestructiveAction("Отключить");
      menu.addCancelAction("Отмена");
      const action = await menu.presentSheet();
      if (action < 0) return;
      if (action === 2) {
        Keychain.remove(CREDENTIAL_KEY);
        return;
      }
      if (action === 1) {
        const result = await setup(settings);
        if (!result) return;
        settings = result.settings;
        data = result.data;
      }
    } else {
      const result = await setup(null);
      if (!result) return;
      settings = result.settings;
      data = result.data;
    }
  }
  if (!settings) {
    Script.setWidget(
      errorWidget(
        "Откройте Scriptable и запустите FitLog, чтобы подключить сервер.",
      ),
    );
    return;
  }
  try {
    data = data || (await snapshot(settings));
  } catch (error) {
    if (error.authentication) Keychain.remove(CREDENTIAL_KEY);
    throw error;
  }
  const widget = draw(data, settings, family, section);
  Script.setWidget(widget);
  if (config.runsInApp) await widget.presentLarge();
}

(async () => {
  try {
    await main();
  } catch (error) {
    const message = error.authentication
      ? "Ключ истёк. Подключите FitLog заново."
      : "Нет данных. Проверьте подключение в Scriptable.";
    Script.setWidget(errorWidget(message));
    if (config.runsInApp) {
      const alert = new Alert();
      alert.title = "FitLog";
      alert.message = error.safe ? error.message : message;
      alert.addAction("OK");
      await alert.presentAlert();
    }
  }
  Script.complete();
})();
