"use client";

import type { ConnectionState } from "@/lib/types";

interface Props {
  name: string | undefined;
  deviceId: string | undefined;
  connectionState: ConnectionState;
  paired: boolean;
}

export function StatusBar({ name, deviceId, connectionState, paired }: Props) {
  const stateColor = {
    connected: "bg-green-500",
    connecting: "bg-yellow-500 animate-pulse",
    disconnected: "bg-red-500",
  }[connectionState];

  return (
    <header className="bg-gray-900 border-b border-gray-800 px-6 py-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <div className="text-2xl">📺</div>
          <div>
            <h1 className="text-xl font-semibold text-white">
              {name || "AirPlay Server"}
            </h1>
            <p className="text-sm text-gray-400">{deviceId || "—"}</p>
          </div>
        </div>
        <div className="flex items-center gap-4">
          {paired && (
            <span className="px-3 py-1 bg-blue-600 text-white text-xs rounded-full">
              Paired
            </span>
          )}
          <div className="flex items-center gap-2">
            <div className={`w-2.5 h-2.5 rounded-full ${stateColor}`} />
            <span className="text-sm text-gray-300 capitalize">
              {connectionState}
            </span>
          </div>
        </div>
      </div>
    </header>
  );
}
