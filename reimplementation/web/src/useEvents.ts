import { useCallback, useEffect, useRef, useState } from "react";
import { useAuth } from "./auth";
import type { AuditEvent, EventEnvelope } from "./types";

export function useEvents() {
  const { request, identity } = useAuth();
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [status, setStatus] = useState("connecting");
  const reconnect = useRef(0);

  const merge = useCallback((incoming: AuditEvent[]) => {
    setEvents((current) => {
      const byID = new Map(current.map((event) => [event.id, event]));
      incoming.forEach((event) => byID.set(event.id, event));
      const next = [...byID.values()].sort((a, b) => b.id - a.id).slice(0, 300);
      if (next[0]) localStorage.setItem("specter.lastEventId", String(next[0].id));
      return next;
    });
  }, []);

  useEffect(() => {
    if (!identity) return;
    let cancelled = false;
    let socket: WebSocket | undefined;
    let timer: number | undefined;

    request<AuditEvent[]>("/api/events/latest").then(merge).catch(() => undefined);

    async function connect() {
      try {
        setStatus("connecting");
        const { ticket } = await request<{ ticket: string }>("/api/events/ticket", { method: "POST" });
        if (cancelled) return;
        const scheme = location.protocol === "https:" ? "wss" : "ws";
        const after = localStorage.getItem("specter.lastEventId") ?? "0";
        socket = new WebSocket(`${scheme}://${location.host}/api/events?ticket=${encodeURIComponent(ticket)}&after=${after}`);
        socket.onopen = () => {
          reconnect.current = 0;
          setStatus("live");
        };
        socket.onmessage = (message) => {
          const envelope = JSON.parse(String(message.data)) as EventEnvelope;
          merge([envelope.payload]);
        };
        socket.onclose = () => {
          if (cancelled) return;
          setStatus("reconnecting");
          reconnect.current += 1;
          timer = window.setTimeout(connect, Math.min(1000 * 2 ** reconnect.current, 15000));
        };
      } catch {
        if (!cancelled) {
          setStatus("reconnecting");
          timer = window.setTimeout(connect, 3000);
        }
      }
    }
    connect();
    return () => {
      cancelled = true;
      if (timer) window.clearTimeout(timer);
      socket?.close();
    };
  }, [identity, merge, request]);

  return { events, status };
}
