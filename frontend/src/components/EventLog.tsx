"use client";

import type { AirPlayEvent } from "@/lib/types";

interface Props {
  events: AirPlayEvent[];
}

const eventIcons: Record<string, string> = {
  play: "▶️",
  stop: "⏹️",
  scrub: "⏩",
  rate: "⏯️",
  photo: "🖼️",
  slideshow: "🎞️",
  mirror_start: "🪞",
  mirror_stop: "🪞",
  mirror_codec: "📦",
  pairing: "🔐",
  audio_announce: "🔊",
  audio_setup: "🎵",
  audio_start: "🎶",
  audio_stop: "🔇",
  audio_flush: "🔄",
  volume: "🔉",
  command: "⌨️",
};

export function EventLog({ events }: Props) {
  return (
    <div className="bg-gray-900 rounded-xl border border-gray-800 overflow-hidden">
      <div className="px-4 py-3 border-b border-gray-800">
        <h3 className="text-sm font-medium text-gray-300">
          Event Log ({events.length})
        </h3>
      </div>
      <div className="max-h-80 overflow-y-auto">
        {events.length === 0 ? (
          <div className="p-4 text-center text-gray-500 text-sm">
            No events yet
          </div>
        ) : (
          <ul className="divide-y divide-gray-800">
            {events.map((evt, i) => (
              <li key={i} className="px-4 py-2 hover:bg-gray-800/50">
                <div className="flex items-start gap-2">
                  <span className="text-sm">
                    {eventIcons[evt.type] || "📡"}
                  </span>
                  <div className="flex-1 min-w-0">
                    <span className="text-sm font-medium text-blue-400">
                      {evt.type}
                    </span>
                    {evt.data && (
                      <p className="text-xs text-gray-500 truncate mt-0.5">
                        {JSON.stringify(evt.data).slice(0, 120)}
                      </p>
                    )}
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
