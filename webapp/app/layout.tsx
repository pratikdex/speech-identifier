import type { Metadata } from "next";
import "./globals.css";
import { Header } from "@/components/Header";
import RecorderProvider from "@/components/RecorderContext";

export const metadata: Metadata = {
  title: "Interspeech Demo",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="h-screen w-screen flex flex-col">
          <RecorderProvider>
            <Header />
            <div className="p-5 flex-1">{children}</div>
          </RecorderProvider>
      </body>
    </html>
  );
}
