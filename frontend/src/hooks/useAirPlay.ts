"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import type { AirPlayEvent, ServerStatus, ConnectionState } from "@/lib/types";

const WS_URL = process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:7000/api/ws";

export function useAirPlay() {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<NodeJS.Timeout>(undefined);
  const [connectionState, setConnectionState] = useState<ConnectionState>("disconnected");
  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [events, setEvents] = useState<AirPlayEvent[]>([]);
  const [photoData, setPhotoData] = useState<string | null>(null);
  const [mirroring, setMirroring] = useState(false);

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    setConnectionState("connecting");
    const ws = new WebSocket(WS_URL);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnectionState("connected");
      ws.send(JSON.stringify({ action: "get_status" }));
    };

    ws.onmessage = (e) => {
      try {
        const msg: AirPlayEvent = JSON.parse(e.data);
        if (msg.type === "status") {
          setStatus(msg.data as unknown as ServerStatus);
        } else {
          setEvents((prev) => [msg, ...prev].slice(0, 100));

          if (msg.type === "photo" && msg.data?.dataBase64) {
            setPhotoData(`data:image/jpeg;base64,${msg.data.dataBase64}`);
          }
          if (msg.type === "play") {
            setStatus((prev) =>
              prev ? { ...prev, playing: true, url: msg.data?.url as string } : prev
            );
          }
          if (msg.type === "stop") {
            setStatus((prev) =>
              prev ? { ...prev, playing: false, url: "" } : prev
            );
            setPhotoData(null);
          }
          if (msg.type === "mirror_start") {
            setMirroring(true);
          }
          if (msg.type === "mirror_stop") {
            setMirroring(false);
          }
          if (msg.type === "rate") {
            const rate = msg.data?.value as number;
            setStatus((prev) =>
              prev ? { ...prev, rate, playing: rate > 0 } : prev
            );
          }
          if (msg.type === "scrub") {
            setStatus((prev) =>
              prev ? { ...prev, position: msg.data?.position as number } : prev
            );
          }
          if (msg.type === "pairing") {
            if (msg.data?.paired) {
              setStatus((prev) => (prev ? { ...prev, paired: true } : prev));
            }
          }
        }
      } catch {
        // ignore parse errors
      }
    };

    ws.onclose = () => {
      setConnectionState("disconnected");
      reconnectTimer.current = setTimeout(connect, 3000);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, []);

  useEffect(() => {
    connect();
    return () => {
      clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);

  const sendCommand = useCallback(
    (action: string, data?: Record<string, unknown>) => {
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(JSON.stringify({ action, data }));
      }
    },
    []
  );

  return {
    connectionState,
    status,
    events,
    photoData,
    mirroring,
    sendCommand,
  };
}
