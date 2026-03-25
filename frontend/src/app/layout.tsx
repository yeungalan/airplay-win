import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "AirPlay Receiver",
  description: "AirPlay server display - receives photos, videos, audio, and screen mirroring",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="antialiased">{children}</body>
    </html>
  );
}
