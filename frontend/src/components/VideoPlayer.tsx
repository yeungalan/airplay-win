"use client";

interface Props {
  playing: boolean;
  url: string;
  position: number;
  duration: number;
  rate: number;
}

function formatTime(seconds: number): string {
  if (!seconds || !isFinite(seconds)) return "0:00";
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, "0")}`;
}

export function VideoPlayer({ playing, url, position, duration, rate }: Props) {
  const progress = duration > 0 ? (position / duration) * 100 : 0;

  if (!playing && !url) {
    return (
      <div className="flex items-center justify-center h-full bg-black rounded-xl">
        <div className="text-center text-gray-500">
          <div className="text-6xl mb-4">📺</div>
          <p className="text-lg">Waiting for AirPlay content...</p>
          <p className="text-sm mt-2">
            Send a video from your iOS device or iTunes
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-black rounded-xl overflow-hidden">
      <div className="aspect-video bg-gray-950 flex items-center justify-center relative">
        {url ? (
          <video
            src={url}
            autoPlay={playing}
            className="w-full h-full object-contain"
            controls={false}
          />
        ) : (
          <div className="text-white text-lg">Loading video...</div>
        )}

        {/* Playback overlay */}
        <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/80 to-transparent p-4">
          <div className="flex items-center gap-3">
            <span className="text-white text-sm">
              {rate === 0 ? "⏸" : "▶"}
            </span>
            <div className="flex-1">
              <div className="w-full bg-gray-700 rounded-full h-1.5">
                <div
                  className="bg-blue-500 h-1.5 rounded-full transition-all"
                  style={{ width: `${progress}%` }}
                />
              </div>
            </div>
            <span className="text-white text-xs font-mono">
              {formatTime(position)} / {formatTime(duration)}
            </span>
          </div>
          {url && (
            <p className="text-gray-400 text-xs mt-2 truncate">{url}</p>
          )}
        </div>
      </div>
    </div>
  );
}
