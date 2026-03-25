"use client";

interface Props {
  paired: boolean;
  pin: string;
}

export function PairingPanel({ paired, pin }: Props) {
  return (
    <div className="bg-gray-900 rounded-xl border border-gray-800 p-4">
      <h3 className="text-sm font-medium text-gray-300 mb-3">Pairing</h3>
      {paired ? (
        <div className="flex items-center gap-2">
          <span className="w-3 h-3 bg-green-500 rounded-full" />
          <span className="text-green-400 text-sm">Device paired</span>
        </div>
      ) : (
        <div>
          <div className="flex items-center gap-2 mb-3">
            <span className="w-3 h-3 bg-yellow-500 rounded-full animate-pulse" />
            <span className="text-yellow-400 text-sm">
              Waiting for pairing...
            </span>
          </div>
          <div className="bg-gray-800 rounded-lg p-4 text-center">
            <p className="text-xs text-gray-400 mb-2">Enter this PIN on your device</p>
            <div className="flex justify-center gap-2">
              {pin.split("").map((digit, i) => (
                <span
                  key={i}
                  className="w-12 h-14 bg-gray-700 rounded-lg flex items-center justify-center text-2xl font-bold text-white"
                >
                  {digit}
                </span>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
