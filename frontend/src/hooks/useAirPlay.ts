"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import type { AirPlayEvent, ServerStatus, ConnectionState } from "@/lib/types";

function getWsUrl() {
  if (typeof window === "undefined") return "";
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}/api/ws`;
}

export function useAirPlay() {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<NodeJS.Timeout>(undefined);
  const [connectionState, setConnectionState] = useState<ConnectionState>("disconnected");
  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [events, setEvents] = useState<AirPlayEvent[]>([]);
  const [photoData, setPhotoData] = useState<string | null>(null);
  const [mirroring, setMirroring] = useState(false);
  const [volume, setVolume] = useState(75);
  const [showPairing, setShowPairing] = useState(false);

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    setConnectionState("connecting");
    const ws = new WebSocket(getWsUrl());
    ws.binaryType = 'arraybuffer';
    wsRef.current = ws;

    ws.onopen = () => {
      setConnectionState("connected");
      ws.send(JSON.stringify({ action: "get_status" }));
    };

    ws.onmessage = (e) => {
      if (e.data instanceof ArrayBuffer) {
        window.dispatchEvent(new CustomEvent('mirrorData', { detail: e.data }));
        return;
      }
      try {
        const msg: AirPlayEvent = JSON.parse(e.data);
        if (msg.type === "status") {
          setStatus(msg.data as unknown as ServerStatus);
          return;
        }

        setEvents((prev) => [msg, ...prev].slice(0, 50));

        switch (msg.type) {
          case "photo":
            if (msg.data?.dataBase64) {
              setPhotoData(`data:image/jpeg;base64,${msg.data.dataBase64}`);
            }
            break;
          case "play":
            setStatus((p) => p ? { ...p, playing: true, url: msg.data?.url as string } : p);
            setPhotoData(null);
            break;
          case "stop":
            setStatus((p) => p ? { ...p, playing: false, url: "", rate: 0 } : p);
            setPhotoData(null);
            break;
          case "mirror_start":
            setMirroring(true);
            break;
          case "mirror_stop":
            setMirroring(false);
            break;
          case "rate":
            setStatus((p) => p ? { ...p, rate: msg.data?.value as number, playing: (msg.data?.value as number) > 0 } : p);
            break;
          case "scrub":
            setStatus((p) => p ? { ...p, position: msg.data?.position as number } : p);
            break;
          case "volume":
            setVolume(Math.max(0, Math.min(100, ((msg.data?.volume as number) + 30) * (100 / 30))));
            break;
          case "pairing":
            setShowPairing(true);
            if (msg.data?.paired) {
              setStatus((p) => p ? { ...p, paired: true } : p);
              setTimeout(() => setShowPairing(false), 1500);
            }
            break;
        }
      } catch {
        // ignore
      }
    };

    ws.onclose = () => {
      setConnectionState("disconnected");
      reconnectTimer.current = setTimeout(connect, 3000);
    };

    ws.onerror = () => ws.close();
  }, []);

  useEffect(() => {
    connect();
    return () => {
      clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);

  return { connectionState, status, events, photoData, mirroring, volume, showPairing };
}
