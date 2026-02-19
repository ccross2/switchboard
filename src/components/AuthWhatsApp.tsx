import { useEffect, useState } from "react";
import QRCode from "qrcode";
import { useAppStore } from "../stores/appStore";
import type { ServiceID, Envelope } from "../types/protocol";

interface Props {
  sendToBridge: (service: ServiceID, envelope: Envelope) => Promise<void>;
}

export default function AuthWhatsApp({ sendToBridge }: Props) {
  const authState = useAppStore((s) => s.services.whatsapp.authState);
  const [qrDataUrl, setQrDataUrl] = useState<string | null>(null);
  const [qrError, setQrError] = useState<string | null>(null);

  // Generate QR code image from code string
  useEffect(() => {
    if (authState.step !== "qr") {
      setQrDataUrl(null);
      return;
    }

    QRCode.toDataURL(authState.code, {
      width: 256,
      margin: 2,
      color: { dark: "#000000", light: "#ffffff" },
    })
      .then((url) => {
        setQrDataUrl(url);
        setQrError(null);
      })
      .catch((err: Error) => {
        setQrError(err.message);
      });
  }, [authState]);

  const handleStartAuth = () => {
    sendToBridge("whatsapp", { type: "auth.start" }).catch(console.error);
  };

  return (
    <div className="flex h-full flex-col items-center justify-center gap-6 bg-[#1a1a2e] px-8">
      {/* WhatsApp branding */}
      <div className="flex flex-col items-center gap-2">
        <div
          className="flex h-16 w-16 items-center justify-center rounded-2xl"
          style={{ backgroundColor: "#25D366" }}
        >
          <span className="text-3xl">💬</span>
        </div>
        <h2 className="text-xl font-bold text-gray-100">Connect WhatsApp</h2>
      </div>

      {authState.step === "idle" && (
        <div className="flex flex-col items-center gap-4">
          <p className="text-sm text-gray-400">
            Scan a QR code with your phone to connect WhatsApp.
          </p>
          <button
            onClick={handleStartAuth}
            className="rounded-lg bg-[#25D366] px-6 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-[#20b858]"
          >
            Start Auth
          </button>
        </div>
      )}

      {authState.step === "qr" && (
        <div className="flex flex-col items-center gap-4">
          <p className="text-sm text-gray-400">
            Open WhatsApp on your phone and scan this QR code.
          </p>
          {qrError && (
            <p className="text-sm text-red-400">QR error: {qrError}</p>
          )}
          {qrDataUrl && (
            <div className="rounded-xl border-4 border-white p-2 shadow-xl">
              <img
                src={qrDataUrl}
                alt="WhatsApp QR Code"
                className="h-56 w-56"
              />
            </div>
          )}
          {!qrDataUrl && !qrError && (
            <div className="flex h-56 w-56 items-center justify-center rounded-xl bg-[#16213e] text-gray-500">
              Generating QR…
            </div>
          )}
          <p className="text-xs text-gray-500">
            QR code expires in ~60 seconds. Click below to refresh.
          </p>
          <button
            onClick={handleStartAuth}
            className="text-sm text-gray-400 underline underline-offset-2 hover:text-gray-200"
          >
            Refresh QR code
          </button>
        </div>
      )}

      {authState.step === "authenticated" && (
        <div className="flex flex-col items-center gap-2 text-center">
          <span className="text-4xl">✓</span>
          <p className="text-base font-semibold text-[#25D366]">
            Connected as {authState.user}
          </p>
        </div>
      )}
    </div>
  );
}
