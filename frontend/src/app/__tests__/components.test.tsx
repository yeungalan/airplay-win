import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { StatusBar } from "@/components/StatusBar";
import { VideoPlayer } from "@/components/VideoPlayer";
import { PhotoViewer } from "@/components/PhotoViewer";
import { MirrorView } from "@/components/MirrorView";
import { PairingPanel } from "@/components/PairingPanel";
import { AudioPlayer } from "@/components/AudioPlayer";
import { IdleScreen } from "@/components/IdleScreen";

// Mock @iconify/react
jest.mock("@iconify/react", () => ({
  Icon: ({ icon, className }: { icon: string; className?: string }) => (
    <span data-testid={`icon-${icon}`} className={className} />
  ),
}));

describe("StatusBar", () => {
  it("renders name when connected", () => {
    render(<StatusBar name="Living Room" connectionState="connected" />);
    expect(screen.getByText("Living Room")).toBeInTheDocument();
  });

  it("renders nothing when disconnected", () => {
    const { container } = render(
      <StatusBar name="Test" connectionState="disconnected" />
    );
    expect(container.innerHTML).toBe("");
  });

  it("shows default name when undefined", () => {
    render(<StatusBar name={undefined} connectionState="connected" />);
    expect(screen.getByText("AirPlay")).toBeInTheDocument();
  });
});

describe("VideoPlayer", () => {
  it("renders nothing when not playing", () => {
    const { container } = render(
      <VideoPlayer playing={false} url="" position={0} duration={0} rate={0} />
    );
    expect(container.innerHTML).toBe("");
  });

  it("shows time when playing", () => {
    render(
      <VideoPlayer
        playing={true}
        url="http://example.com/video.mp4"
        position={65}
        duration={120}
        rate={1}
      />
    );
    expect(screen.getByText("1:05")).toBeInTheDocument();
    expect(screen.getByText("-0:55")).toBeInTheDocument();
  });

  it("shows pause icon when rate is 0", () => {
    render(
      <VideoPlayer
        playing={true}
        url="http://example.com/v.mp4"
        position={0}
        duration={100}
        rate={0}
      />
    );
    expect(screen.getByTestId("icon-mdi:pause")).toBeInTheDocument();
  });

  it("shows play icon when rate is 1", () => {
    render(
      <VideoPlayer
        playing={true}
        url="http://example.com/v.mp4"
        position={0}
        duration={100}
        rate={1}
      />
    );
    expect(screen.getByTestId("icon-mdi:play")).toBeInTheDocument();
  });
});

describe("PhotoViewer", () => {
  it("renders nothing when no photo", () => {
    const { container } = render(<PhotoViewer photoData={null} />);
    expect(container.innerHTML).toBe("");
  });

  it("renders image when photo data provided", () => {
    render(<PhotoViewer photoData="data:image/jpeg;base64,/9j/4AAQ" />);
    const img = screen.getByRole("img", { name: "AirPlay Photo" });
    expect(img).toHaveAttribute("src", "data:image/jpeg;base64,/9j/4AAQ");
  });
});

describe("MirrorView", () => {
  it("renders nothing when inactive", () => {
    const { container } = render(
      <MirrorView active={false} width={1920} height={1080} />
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders canvas when active", () => {
    render(<MirrorView active={true} width={1920} height={1080} />);
    expect(screen.getByTestId("icon-mdi:cast-connected")).toBeInTheDocument();
  });
});

describe("PairingPanel", () => {
  it("renders nothing when paired", () => {
    const { container } = render(
      <PairingPanel paired={true} pin="3939" />
    );
    expect(container.innerHTML).toBe("");
  });

  it("shows PIN digits when not paired", () => {
    render(<PairingPanel paired={false} pin="3939" />);
    expect(screen.getByText("Enter the code on your device")).toBeInTheDocument();
    const digits = screen.getAllByText(/^[0-9]$/);
    expect(digits).toHaveLength(4);
  });
});

describe("AudioPlayer", () => {
  it("renders nothing when not playing", () => {
    const { container } = render(
      <AudioPlayer status={null} volume={75} />
    );
    expect(container.innerHTML).toBe("");
  });

  it("shows audio streaming when playing", () => {
    render(
      <AudioPlayer
        status={{
          name: "Test", deviceId: "AA:BB:CC:DD:EE:FF", model: "AppleTV6,2",
          paired: false, playing: true, url: "", position: 0, duration: 0,
          rate: 1, width: 1920, height: 1080,
        }}
        volume={75}
      />
    );
    expect(screen.getByText("Audio Streaming")).toBeInTheDocument();
  });
});

describe("IdleScreen", () => {
  it("shows AirPlay prompt when connected", () => {
    render(<IdleScreen name="My TV" connectionState="connected" />);
    expect(screen.getByText("My TV")).toBeInTheDocument();
    expect(screen.getByText("Use AirPlay to stream to this display")).toBeInTheDocument();
  });

  it("shows connecting state", () => {
    render(<IdleScreen name="My TV" connectionState="connecting" />);
    expect(screen.getByText("Connecting...")).toBeInTheDocument();
  });

  it("shows disconnected state", () => {
    render(<IdleScreen name="My TV" connectionState="disconnected" />);
    expect(screen.getByText("Disconnected")).toBeInTheDocument();
  });
});
