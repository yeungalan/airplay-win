import type { ServerStatus, AirPlayEvent, ConnectionState } from "@/lib/types";

describe("Types", () => {
  it("ServerStatus has correct shape", () => {
    const status: ServerStatus = {
      name: "Test",
      deviceId: "AA:BB:CC:DD:EE:FF",
      model: "AppleTV6,2",
      paired: false,
      playing: false,
      url: "",
      position: 0,
      duration: 0,
      rate: 0,
      width: 1920,
      height: 1080,
    };
    expect(status.name).toBe("Test");
    expect(status.width).toBe(1920);
  });

  it("AirPlayEvent has correct shape", () => {
    const event: AirPlayEvent = {
      type: "play",
      data: { url: "http://example.com/video.mp4" },
    };
    expect(event.type).toBe("play");
    expect(event.data.url).toBe("http://example.com/video.mp4");
  });

  it("ConnectionState values", () => {
    const states: ConnectionState[] = [
      "connecting",
      "connected",
      "disconnected",
    ];
    expect(states).toHaveLength(3);
  });
});
