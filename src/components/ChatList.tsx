import { useEffect } from "react";
import { useAppStore } from "../stores/appStore";
import type { ServiceID, Envelope, Chat } from "../types/protocol";

interface Props {
  sendToBridge: (service: ServiceID, envelope: Envelope) => Promise<void>;
}

function formatTime(timestamp?: number): string {
  if (!timestamp) return "";
  const date = new Date(timestamp * 1000);
  const now = new Date();
  const isToday = date.toDateString() === now.toDateString();

  if (isToday) {
    return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }
  return date.toLocaleDateString([], { month: "short", day: "numeric" });
}

function ChatItem({
  chat,
  isActive,
  onClick,
}: {
  chat: Chat;
  isActive: boolean;
  onClick: () => void;
}) {
  const initial = chat.name.charAt(0).toUpperCase() || "?";

  return (
    <button
      onClick={onClick}
      className={`flex w-full items-center gap-3 px-3 py-3 text-left transition-colors duration-150 ${
        isActive
          ? "bg-[#0f3460]"
          : "hover:bg-[#0f3460]/40"
      }`}
    >
      {/* Avatar */}
      <div className="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-full bg-[#2a2a4a] text-sm font-semibold text-gray-200">
        {initial}
      </div>

      {/* Chat info */}
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline justify-between gap-1">
          <span className="truncate text-sm font-medium text-gray-100">
            {chat.name}
          </span>
          <span className="flex-shrink-0 text-[11px] text-gray-500">
            {formatTime(chat.last_time)}
          </span>
        </div>
        <div className="flex items-center justify-between gap-1">
          <span className="truncate text-xs text-gray-400">
            {chat.last_message || ""}
          </span>
          {chat.unread > 0 && (
            <span className="flex h-5 min-w-5 flex-shrink-0 items-center justify-center rounded-full bg-[#25D366] px-1 text-[10px] font-bold text-white">
              {chat.unread > 99 ? "99+" : chat.unread}
            </span>
          )}
        </div>
      </div>
    </button>
  );
}

export default function ChatList({ sendToBridge }: Props) {
  const activeService = useAppStore((s) => s.activeService);
  const activeChat = useAppStore((s) => s.activeChat);
  const setActiveChat = useAppStore((s) => s.setActiveChat);
  const chats = useAppStore((s) => s.chats);

  const currentChats: Chat[] = activeService ? chats[activeService] : [];

  const serviceName = activeService
    ? activeService.charAt(0).toUpperCase() + activeService.slice(1)
    : "Select a service";

  // Fetch chats when service changes
  useEffect(() => {
    if (!activeService) return;
    sendToBridge(activeService, { type: "chats.list" }).catch(console.error);
  }, [activeService, sendToBridge]);

  const handleChatClick = (chat: Chat) => {
    setActiveChat(chat.id);
  };

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex h-14 flex-shrink-0 items-center border-b border-[#2a2a4a] px-4">
        <h2 className="text-base font-semibold text-gray-100">{serviceName}</h2>
      </div>

      {/* Chat list */}
      <div className="flex-1 overflow-y-auto">
        {!activeService && (
          <div className="flex h-full items-center justify-center text-sm text-gray-500">
            Select a service to view chats
          </div>
        )}
        {activeService && currentChats.length === 0 && (
          <div className="flex h-full items-center justify-center text-sm text-gray-500">
            No chats yet
          </div>
        )}
        {currentChats.map((chat) => (
          <ChatItem
            key={chat.id}
            chat={chat}
            isActive={activeChat === chat.id}
            onClick={() => handleChatClick(chat)}
          />
        ))}
      </div>
    </div>
  );
}
