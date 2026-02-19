import { useState } from "react";
import { useAppStore } from "../stores/appStore";
import type { ServiceID, Envelope } from "../types/protocol";

interface Props {
  sendToBridge: (service: ServiceID, envelope: Envelope) => Promise<void>;
}

export default function AuthTelegram({ sendToBridge }: Props) {
  const authState = useAppStore((s) => s.services.telegram.authState);
  const [phone, setPhone] = useState("");
  const [code, setCode] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const handleStartAuth = async () => {
    await sendToBridge("telegram", { type: "auth.start" }).catch(console.error);
  };

  const handlePhoneSubmit = async () => {
    const trimmed = phone.trim();
    if (!trimmed) return;
    setSubmitting(true);
    await sendToBridge("telegram", {
      type: "auth.phone",
      data: { phone: trimmed },
    }).catch(console.error);
    setSubmitting(false);
  };

  const handleCodeSubmit = async () => {
    const trimmed = code.trim();
    if (!trimmed) return;
    setSubmitting(true);
    await sendToBridge("telegram", {
      type: "auth.code",
      data: { code: trimmed },
    }).catch(console.error);
    setCode("");
    setSubmitting(false);
  };

  return (
    <div className="flex h-full flex-col items-center justify-center gap-6 bg-[#1a1a2e] px-8">
      {/* Telegram branding */}
      <div className="flex flex-col items-center gap-2">
        <div
          className="flex h-16 w-16 items-center justify-center rounded-2xl"
          style={{ backgroundColor: "#0088cc" }}
        >
          <span className="text-3xl">✈️</span>
        </div>
        <h2 className="text-xl font-bold text-gray-100">Connect Telegram</h2>
      </div>

      {authState.step === "idle" && (
        <div className="flex flex-col items-center gap-4">
          <p className="text-sm text-gray-400">
            Sign in with your Telegram account to receive messages.
          </p>
          <button
            onClick={handleStartAuth}
            className="rounded-lg px-6 py-2.5 text-sm font-semibold text-white transition-colors"
            style={{ backgroundColor: "#0088cc" }}
          >
            Start Auth
          </button>
        </div>
      )}

      {authState.step === "phone_input" && (
        <div className="flex w-full max-w-sm flex-col gap-4">
          <p className="text-sm text-gray-400">
            Enter your phone number (with country code, e.g. +1 555 000 0000).
          </p>
          <input
            type="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") handlePhoneSubmit();
            }}
            placeholder="+1 555 000 0000"
            className="rounded-lg border border-[#2a2a4a] bg-[#16213e] px-4 py-2.5 text-sm text-gray-100 placeholder-gray-600 outline-none focus:border-[#0088cc] focus:ring-1 focus:ring-[#0088cc]"
            autoFocus
          />
          <button
            onClick={handlePhoneSubmit}
            disabled={submitting || !phone.trim()}
            className="rounded-lg py-2.5 text-sm font-semibold text-white transition-colors disabled:cursor-not-allowed disabled:opacity-50"
            style={{ backgroundColor: "#0088cc" }}
          >
            {submitting ? "Sending…" : "Send Code"}
          </button>
        </div>
      )}

      {authState.step === "code_input" && (
        <div className="flex w-full max-w-sm flex-col gap-4">
          <p className="text-sm text-gray-400">
            A code was sent to{" "}
            <span className="font-medium text-gray-200">
              {authState.phone_hint}
            </span>
            . Enter it below.
          </p>
          <input
            type="text"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleCodeSubmit();
            }}
            placeholder="12345"
            maxLength={10}
            className="rounded-lg border border-[#2a2a4a] bg-[#16213e] px-4 py-2.5 text-center text-lg tracking-widest text-gray-100 placeholder-gray-600 outline-none focus:border-[#0088cc] focus:ring-1 focus:ring-[#0088cc]"
            autoFocus
          />
          <button
            onClick={handleCodeSubmit}
            disabled={submitting || !code.trim()}
            className="rounded-lg py-2.5 text-sm font-semibold text-white transition-colors disabled:cursor-not-allowed disabled:opacity-50"
            style={{ backgroundColor: "#0088cc" }}
          >
            {submitting ? "Verifying…" : "Verify Code"}
          </button>
          <button
            onClick={() => sendToBridge("telegram", { type: "auth.start" }).catch(console.error)}
            className="text-sm text-gray-500 underline underline-offset-2 hover:text-gray-300"
          >
            Wrong number? Start over
          </button>
        </div>
      )}

      {authState.step === "authenticated" && (
        <div className="flex flex-col items-center gap-2 text-center">
          <span className="text-4xl">✓</span>
          <p className="text-base font-semibold" style={{ color: "#0088cc" }}>
            Connected as {authState.user}
          </p>
        </div>
      )}
    </div>
  );
}
