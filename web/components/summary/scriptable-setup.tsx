"use client";

import { useState } from "react";
import { apiFetch } from "@/lib/api";
import { formatDate } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/field";
import { InlineError } from "@/components/ui/states";

type Grant = { access_token: string; expires_at: string; scope: string };

export function ScriptableSetup() {
  const [grant, setGrant] = useState<Grant | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [copied, setCopied] = useState("");

  async function createKey() {
    setBusy(true);
    setError(null);
    setCopied("");
    setGrant(null);
    try {
      setGrant(await apiFetch<Grant>("/auth/widget-token", { method: "POST" }));
    } catch (error) {
      setError(error);
    } finally {
      setBusy(false);
    }
  }

  async function copyScript() {
    setError(null);
    setCopied("");
    try {
      const response = await fetch("/scriptable/FitLog.js", {
        cache: "no-store",
      });
      if (!response.ok) throw new Error("Не удалось скачать скрипт.");
      await navigator.clipboard.writeText(await response.text());
      setCopied("Скрипт скопирован. Вставьте его в новый скрипт в Scriptable.");
    } catch {
      setError(
        new Error(
          "Не удалось скопировать. Скачайте файл и скопируйте его содержимое вручную.",
        ),
      );
    }
  }

  async function copyKey() {
    if (!grant) return;
    setError(null);
    setCopied("");
    try {
      await navigator.clipboard.writeText(grant.access_token);
      setCopied(
        "Ключ скопирован. Вставьте его в окно подключения FitLog в Scriptable.",
      );
    } catch {
      setError(
        new Error(
          "Не удалось скопировать ключ. Выделите поле и скопируйте вручную.",
        ),
      );
    }
  }

  return (
    <details className="mt-5 rounded-control border border-line px-4 py-1 text-sm">
      <summary className="cursor-pointer py-3 font-medium">
        Виджет через Scriptable
      </summary>
      <div className="space-y-4 pb-4">
        <p className="text-muted">
          Цифры на главном экране iPhone. Для всех 24 показателей выберите
          большой виджет; маленький и средний показывают основные.
        </p>
        <ol className="list-decimal space-y-3 pl-5 text-muted">
          <li>
            Установите{" "}
            <a
              href="https://apps.apple.com/app/scriptable/id1405459188"
              target="_blank"
              rel="noreferrer"
              className="text-accent"
            >
              Scriptable
            </a>{" "}
            и откройте приложение один раз.
          </li>
          <li>
            Скопируйте скрипт кнопкой ниже. В Scriptable нажмите «+», вставьте
            код и назовите скрипт{" "}
            <span className="font-medium text-ink">FitLog</span>. Можно также
            скачать файл и перенести его в iCloud Drive → Scriptable.
          </li>
          <li>
            Создайте ключ виджета кнопкой ниже. Запустите FitLog в Scriptable:
            укажите адрес этого сайта без пути /dashboard и вставьте ключ.
            Основной токен входа не нужен.
          </li>
          <li>
            На главном экране iPhone удерживайте пустое место → добавьте виджет
            → Scriptable → большой размер. Удерживайте виджет → «Изменить
            виджет» → Script → FitLog. Parameter оставьте пустым.
          </li>
        </ol>
        <div className="flex flex-wrap gap-2">
          <Button size="sm" onClick={() => void copyScript()}>
            Скопировать скрипт
          </Button>
          <a
            href="/scriptable/FitLog.js"
            download="FitLog.js"
            className="inline-flex min-h-11 items-center px-3 text-accent"
          >
            Скачать FitLog.js
          </a>
          <Button size="sm" loading={busy} onClick={() => void createKey()}>
            Создать ключ виджета
          </Button>
        </div>
        {grant && (
          <div className="space-y-2 rounded-control bg-ink/[.03] p-3">
            <label className="grid gap-2">
              <span className="font-medium">Ключ только для чтения</span>
              <Input
                aria-label="Ключ виджета"
                type="text"
                readOnly
                autoComplete="off"
                spellCheck={false}
                value={grant.access_token}
                onFocus={(event) => event.target.select()}
              />
            </label>
            <div className="flex flex-wrap items-center gap-3">
              <Button size="sm" onClick={() => void copyKey()}>
                Скопировать ключ
              </Button>
              <p className="text-xs text-muted">
                Действует до {formatDate(grant.expires_at)}.
              </p>
            </div>
            <p className="text-xs leading-5 text-muted">
              Ключ открывает только цифры сводки. Храните его как пароль.
              Создание нового ключа не отключает предыдущие; смена токена входа
              в FitLog отключит все ключи.
            </p>
          </div>
        )}
        <InlineError error={error} />
        <p role="status" aria-live="polite" className="text-xs text-accent">
          {copied}
        </p>
        <p className="text-xs leading-5 text-muted">
          Для отдельного блока в Parameter можно указать recovery, sleep,
          training, nutrition или body. Обновлением управляет iOS; нажатие на
          виджет открывает веб-сводку.
        </p>
        <p className="text-xs leading-5 text-muted">
          На Mac: включите «Использовать виджеты iPhone» в настройках «Рабочий
          стол и Dock» и найдите Scriptable в галерее виджетов. Нужны один Apple
          Account, iOS 17+, macOS Sonoma+ и iPhone рядом либо в той же Wi-Fi
          сети.
        </p>
      </div>
    </details>
  );
}
