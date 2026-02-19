import { useState, type KeyboardEvent } from "react";
import type { ServiceID, Envelope } from "../types/protocol";

interface Props {
  activeService: ServiceID;
  activeChat: string;
  sendToBridge: (service: ServiceID, envelope: Envelope) => Promise<void>;
}

function SendIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="h-5 w-5"
    >
      <line x1="22" y1="2" x2="11" y2="13" />
      <polygon points="22 2 15 22 11 13 2 9 22 2" />
    </svg>
  );
}

export default function MessageInput({
  activeService,
  activeChat,
  sendToBridge,
}: Props) {
  const [text, setText] = useState("");

  const handleSend = async () => {
    const trimmed = text.trim();
    if (!trimmed) return;

    setText("");

    await sendToBridge(activeService, {
      type: "message.send",
      data: {
        chat_id: activeChat,
        text: trimmed,
      },
    }).catch(console.error);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="flex-shrink-0 border-t border-[#2a2a4a] bg-[#202c33] px-4 py-3">
      <div className="flex items-end gap-2 rounded-xl bg-[#2a3942] px-3 py-2">
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Type a message…"
          rows={1}
          className="max-h-32 flex-1 resize-none bg-transparent text-sm text-[#e9edef] placeholder-[#8696a0] outline-none"
          style={{ scrollbarWidth: "thin" }}
        />
        <button
          onClick={handleSend}
          disabled={!text.trim()}
          className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full transition-colors disabled:cursor-not-allowed disabled:opacity-40"
          style={{
            backgroundColor: text.trim() ? "#00a884" : "#2a3942",
            color: text.trim() ? "#fff" : "#8696a0",
          }}
        >
          <SendIcon />
        </button>
      </div>
    </div>
  );
}
