"use client";

interface Props {
  active: boolean;
  width: number;
  height: number;
}

export function MirrorView({ active, width, height }: Props) {
  if (!active) {
    return (
      <div className="flex items-center justify-center h-64 bg-gray-900 rounded-xl border border-gray-800">
        <div className="text-center text-gray-500">
          <div className="text-4xl mb-2">🪞</div>
          <p>Screen mirroring inactive</p>
          <p className="text-xs mt-1">
            Start AirPlay Mirroring from your device
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-black rounded-xl overflow-hidden border border-blue-500/50">
      <div className="relative">
        <div className="absolute top-2 right-2 z-10 flex items-center gap-2">
          <span className="w-2 h-2 bg-red-500 rounded-full animate-pulse" />
          <span className="text-xs text-white bg-black/60 px-2 py-0.5 rounded">
            MIRRORING {width}×{height}
          </span>
        </div>
        <div
          className="aspect-video bg-gray-950 flex items-center justify-center"
          style={{ maxWidth: width, maxHeight: height }}
        >
          <canvas
            id="mirror-canvas"
            width={width}
            height={height}
            className="w-full h-full"
          />
        </div>
      </div>
    </div>
  );
}
