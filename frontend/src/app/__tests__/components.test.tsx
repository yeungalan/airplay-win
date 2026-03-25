import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { StatusBar } from "@/components/StatusBar";
import { VideoPlayer } from "@/components/VideoPlayer";
import { PhotoViewer } from "@/components/PhotoViewer";
import { MirrorView } from "@/components/MirrorView";
import { EventLog } from "@/components/EventLog";
import { PairingPanel } from "@/components/PairingPanel";
import { AudioPlayer } from "@/components/AudioPlayer";

describe("StatusBar", () => {
  it("renders server name", () => {
    render(
      <StatusBar
        name="Test Server"
        deviceId="AA:BB:CC:DD:EE:FF"
        connectionState="connected"
        paired={false}
      />
    );
    expect(screen.getByText("Test Server")).toBeInTheDocument();
    expect(screen.getByText("AA:BB:CC:DD:EE:FF")).toBeInTheDocument();
    expect(screen.getByText("connected")).toBeInTheDocument();
  });

  it("shows paired badge when paired", () => {
    render(
      <StatusBar
        name="Test"
        deviceId="AA:BB:CC:DD:EE:FF"
        connectionState="connected"
        paired={true}
      />
    );
    expect(screen.getByText("Paired")).toBeInTheDocument();
  });

  it("shows default name when undefined", () => {
    render(
      <StatusBar
        name={undefined}
        deviceId={undefined}
        connectionState="disconnected"
        paired={false}
      />
    );
    expect(screen.getByText("AirPlay Server")).toBeInTheDocument();
    expect(screen.getByText("—")).toBeInTheDocument();
  });
});

describe("VideoPlayer", () => {
  it("shows waiting state when not playing", () => {
    render(
      <VideoPlayer
        playing={false}
        url=""
        position={0}
        duration={0}
        rate={0}
      />
    );
    expect(
      screen.getByText("Waiting for AirPlay content...")
    ).toBeInTheDocument();
  });

  it("shows video when playing", () => {
    render(
      <VideoPlayer
        playing={true}
        url="http://example.com/video.mp4"
        position={30}
        duration={120}
        rate={1}
      />
    );
    expect(
      screen.getByText("http://example.com/video.mp4")
    ).toBeInTheDocument();
    expect(screen.getByText("0:30 / 2:00")).toBeInTheDocument();
  });

  it("shows pause icon when rate is 0", () => {
    render(
      <VideoPlayer
        playing={true}
        url="http://example.com/video.mp4"
        position={0}
        duration={100}
        rate={0}
      />
    );
    expect(screen.getByText("⏸")).toBeInTheDocument();
  });

  it("shows play icon when rate is 1", () => {
    render(
      <VideoPlayer
        playing={true}
        url="http://example.com/video.mp4"
        position={0}
        duration={100}
        rate={1}
      />
    );
    expect(screen.getByText("▶")).toBeInTheDocument();
  });
});

describe("PhotoViewer", () => {
  it("shows placeholder when no photo", () => {
    render(<PhotoViewer photoData={null} />);
    expect(screen.getByText("No photo received")).toBeInTheDocument();
  });

  it("renders image when photo data provided", () => {
    render(<PhotoViewer photoData="data:image/jpeg;base64,/9j/4AAQ" />);
    const img = screen.getByRole("img", { name: "AirPlay Photo" });
    expect(img).toBeInTheDocument();
    expect(img).toHaveAttribute("src", "data:image/jpeg;base64,/9j/4AAQ");
  });
});

describe("MirrorView", () => {
  it("shows inactive state", () => {
    render(<MirrorView active={false} width={1920} height={1080} />);
    expect(screen.getByText("Screen mirroring inactive")).toBeInTheDocument();
  });

  it("shows active mirroring with resolution", () => {
    render(<MirrorView active={true} width={1920} height={1080} />);
    expect(screen.getByText("MIRRORING 1920×1080")).toBeInTheDocument();
  });
});

describe("EventLog", () => {
  it("shows empty state", () => {
    render(<EventLog events={[]} />);
    expect(screen.getByText("No events yet")).toBeInTheDocument();
    expect(screen.getByText("Event Log (0)")).toBeInTheDocument();
  });

  it("renders events", () => {
    const events = [
      { type: "play", data: { url: "http://example.com/video.mp4" } },
      { type: "photo", data: { size: 1024 } },
    ];
    render(<EventLog events={events} />);
    expect(screen.getByText("Event Log (2)")).toBeInTheDocument();
    expect(screen.getByText("play")).toBeInTheDocument();
    expect(screen.getByText("photo")).toBeInTheDocument();
  });
});

describe("PairingPanel", () => {
  it("shows PIN when not paired", () => {
    render(<PairingPanel paired={false} pin="3939" />);
    expect(screen.getByText("Waiting for pairing...")).toBeInTheDocument();
    const digits = screen.getAllByText(/^[0-9]$/);
    expect(digits).toHaveLength(4);
  });

  it("shows paired state", () => {
    render(<PairingPanel paired={true} pin="3939" />);
    expect(screen.getByText("Device paired")).toBeInTheDocument();
  });
});

describe("AudioPlayer", () => {
  it("shows no audio when not playing", () => {
    render(<AudioPlayer status={null} />);
    expect(screen.getByText("No audio")).toBeInTheDocument();
  });

  it("shows streaming when playing", () => {
    render(
      <AudioPlayer
        status={{
          name: "Test",
          deviceId: "AA:BB:CC:DD:EE:FF",
          model: "AppleTV6,2",
          paired: false,
          playing: true,
          url: "",
          position: 0,
          duration: 0,
          rate: 1,
          width: 1920,
          height: 1080,
        }}
      />
    );
    expect(screen.getByText("Audio streaming")).toBeInTheDocument();
  });
});
