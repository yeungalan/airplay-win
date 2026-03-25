"use client";

interface Props {
  photoData: string | null;
}

export function PhotoViewer({ photoData }: Props) {
  if (!photoData) {
    return (
      <div className="flex items-center justify-center h-64 bg-gray-900 rounded-xl border border-gray-800">
        <div className="text-center text-gray-500">
          <div className="text-4xl mb-2">🖼️</div>
          <p>No photo received</p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gray-900 rounded-xl overflow-hidden border border-gray-800">
      <div className="aspect-video flex items-center justify-center bg-black">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={photoData}
          alt="AirPlay Photo"
          className="max-w-full max-h-full object-contain"
        />
      </div>
    </div>
  );
}
