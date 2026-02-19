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

function MessageBubble({ message }: { message: Message }) {
  const isMe = message.from_me;

  return (
    <div className={`flex ${isMe ? "justify-end" : "justify-start"} mb-2`}>
      <div
        className={`max-w-[70%] rounded-2xl px-4 py-2 ${
          isMe
            ? "rounded-br-sm bg-[#0f3460] text-gray-100"
            : "rounded-bl-sm bg-[#16213e] text-gray-100"
        }`}
      >
        {/* Sender name for group messages */}
        {!isMe && message.from && (
          <div className="mb-1 text-xs font-semibold text-[#0088cc]">
            {message.from}
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

        {/* Text */}
        {message.text && (
          <p className="whitespace-pre-wrap break-words text-sm">{message.text}</p>
        )}

        {/* Timestamp */}
        <div
          className={`mt-1 text-[11px] ${
            isMe ? "text-right text-blue-300/60" : "text-gray-500"
          }`}
        >
          {formatTimestamp(message.timestamp)}
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
      <div className="flex h-14 flex-shrink-0 items-center border-b border-[#2a2a4a] bg-[#16213e] px-4">
        <h3 className="text-base font-semibold text-gray-100">{chatName}</h3>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-4 py-4">
        {currentMessages.length === 0 && (
          <div className="flex h-full items-center justify-center text-sm text-gray-500">
            No messages
          </div>
        )}
        {currentMessages.map((msg) => (
          <MessageBubble key={msg.id} message={msg} />
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
