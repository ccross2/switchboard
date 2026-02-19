import { useEffect, useRef } from "react";
import { convertFileSrc } from "@tauri-apps/api/core";
import { useAppStore } from "../stores/appStore";
import type { ServiceID, Envelope, Message } from "../types/protocol";
import MessageInput from "./MessageInput";

interface Props {
  sendToBridge: (service: ServiceID, envelope: Envelope) => Promise<void>;
}

function formatTimestamp(timestamp: number): string {
  const date = new Date(timestamp * 1000);
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function MessageBubble({
  message,
  isGroup,
}: {
  message: Message;
  isGroup: boolean;
}) {
  const isMe = message.from_me;
  // Strip JID domain (e.g. "16785594751@s.whatsapp.net" → "16785594751")
  const senderDisplay = message.from ? message.from.split("@")[0] : "";

  return (
    <div className={`flex ${isMe ? "justify-end" : "justify-start"} mb-1.5`}>
      <div
        className={`max-w-[72%] rounded-2xl px-3 py-2 shadow-sm ${
          isMe
            ? "rounded-br-sm bg-[#005c4b] text-[#e9edef]"
            : "rounded-bl-sm bg-[#202c33] text-[#e9edef]"
        }`}
      >
        {/* Sender name only in group chats */}
        {!isMe && isGroup && senderDisplay && (
          <div className="mb-0.5 text-xs font-semibold text-[#00a884]">
            {senderDisplay}
          </div>
        )}

        {/* Image */}
        {message.image_path && (
          <img
            src={convertFileSrc(message.image_path)}
            alt="Media"
            className="mb-2 max-h-64 w-full rounded-lg object-contain"
            loading="lazy"
          />
        )}

        {/* Text + timestamp inline */}
        <div className="flex items-end gap-2">
          {message.text && (
            <p className="flex-1 whitespace-pre-wrap break-words text-sm leading-relaxed">
              {message.text}
            </p>
          )}
          <span
            className={`flex-shrink-0 self-end text-[10px] ${
              isMe ? "text-[#8daf9e]" : "text-[#8696a0]"
            }`}
          >
            {formatTimestamp(message.timestamp)}
          </span>
        </div>
      </div>
    </div>
  );
}

export default function MessageView({ sendToBridge }: Props) {
  const activeService = useAppStore((s) => s.activeService);
  const activeChat = useAppStore((s) => s.activeChat);
  const chats = useAppStore((s) => s.chats);
  const messages = useAppStore((s) => s.messages);

  const messagesEndRef = useRef<HTMLDivElement>(null);

  const messageKey =
    activeService && activeChat ? `${activeService}:${activeChat}` : null;
  const currentMessages: Message[] = messageKey ? (messages[messageKey] ?? []) : [];

  const activeChat_ = activeService && activeChat
    ? chats[activeService].find((c) => c.id === activeChat)
    : null;
  const chatName = activeChat_?.name ?? activeChat ?? "";
  const isGroup = activeChat_?.is_group ?? false;

  // Fetch messages when chat selection changes
  useEffect(() => {
    if (!activeService || !activeChat) return;
    sendToBridge(activeService, {
      type: "chat.messages",
      data: { chat_id: activeChat },
    }).catch(console.error);
  }, [activeService, activeChat, sendToBridge]);

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [currentMessages.length]);

  if (!activeService || !activeChat) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-gray-500">
        Select a chat to start messaging
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex h-14 flex-shrink-0 items-center gap-3 border-b border-[#2a2a4a] bg-[#202c33] px-4">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full bg-[#2a3942] text-sm font-semibold text-gray-200">
          {chatName.charAt(0).toUpperCase() || "?"}
        </div>
        <h3 className="text-base font-semibold text-[#e9edef]">{chatName}</h3>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto bg-[#0b141a] px-4 py-4">
        {currentMessages.length === 0 && (
          <div className="flex h-full flex-col items-center justify-center gap-2 text-center">
            <span className="text-3xl">💬</span>
            <p className="text-sm font-medium text-gray-400">No messages yet</p>
            <p className="max-w-xs text-xs text-gray-600">
              New messages will appear here. History isn't available for this
              session — only messages received while connected show up.
            </p>
          </div>
        )}
        {currentMessages.map((msg) => (
          <MessageBubble key={msg.id} message={msg} isGroup={isGroup} />
        ))}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <MessageInput
        activeService={activeService}
        activeChat={activeChat}
        sendToBridge={sendToBridge}
      />
    </div>
  );
}
