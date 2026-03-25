"use client";

import { useAirPlay } from "@/hooks/useAirPlay";
import { StatusBar } from "@/components/StatusBar";
import { VideoPlayer } from "@/components/VideoPlayer";
import { PhotoViewer } from "@/components/PhotoViewer";
import { MirrorView } from "@/components/MirrorView";
import { EventLog } from "@/components/EventLog";
import { PairingPanel } from "@/components/PairingPanel";
import { AudioPlayer } from "@/components/AudioPlayer";

export default function Home() {
  const { connectionState, status, events, photoData, mirroring } =
    useAirPlay();

  return (
    <div className="min-h-screen bg-gray-950 text-white">
      <StatusBar
        name={status?.name}
        deviceId={status?.deviceId}
        connectionState={connectionState}
        paired={status?.paired ?? false}
      />

      <main className="max-w-7xl mx-auto px-4 py-6">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Main display area */}
          <div className="lg:col-span-2 space-y-6">
            {mirroring ? (
              <MirrorView
                active={mirroring}
                width={status?.width ?? 1920}
                height={status?.height ?? 1080}
              />
            ) : photoData ? (
              <PhotoViewer photoData={photoData} />
            ) : (
              <VideoPlayer
                playing={status?.playing ?? false}
                url={status?.url ?? ""}
                position={status?.position ?? 0}
                duration={status?.duration ?? 0}
                rate={status?.rate ?? 0}
              />
            )}

            <AudioPlayer status={status} />
          </div>

          {/* Sidebar */}
          <div className="space-y-6">
            <PairingPanel paired={status?.paired ?? false} pin="3939" />
            <EventLog events={events} />
          </div>
        </div>
      </main>
    </div>
  );
}
