export interface AirPlayEvent {
  type: string;
  data: Record<string, unknown>;
}

export interface ServerStatus {
  name: string;
  deviceId: string;
  model: string;
  paired: boolean;
  playing: boolean;
  url: string;
  position: number;
  duration: number;
  rate: number;
  width: number;
  height: number;
  pin?: string;
}

export type ConnectionState = "connecting" | "connected" | "disconnected";
