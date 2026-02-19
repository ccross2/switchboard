import { useEffect, useState } from "react";
import { useAppStore } from "../stores/appStore";
import type { ServiceID, Envelope, Chat } from "../types/protocol";

interface Props {
  sendToBridge: (service: ServiceID, envelope: Envelope) => Promise<void>;
}

type Filter = "all" | "unread" | "groups";

function formatTime(timestamp?: number): string {
  if (!timestamp) return "";
  const date = new Date(timestamp * 1000);
  const now = new Date();
  if (date.toDateString() === now.toDateString()) {
    return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }
  const yesterday = new Date(now);
  yesterday.setDate(yesterday.getDate() - 1);
  if (date.toDateString() === yesterday.toDateString()) return "Yesterday";
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
        isActive ? "bg-[#2a3942]" : "hover:bg-[#2a2a4a]/60"
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
          {chat.last_time ? (
            <span
              className={`flex-shrink-0 text-[11px] ${
                (chat.unread ?? 0) > 0 ? "text-[#00a884]" : "text-gray-500"
              }`}
            >
              {formatTime(chat.last_time)}
            </span>
          ) : null}
        </div>
        <div className="flex items-center justify-between gap-1">
          <span className="truncate text-xs text-gray-400">
            {chat.last_message || ""}
          </span>
          {(chat.unread ?? 0) > 0 && (
            <span className="flex h-5 min-w-5 flex-shrink-0 items-center justify-center rounded-full bg-[#00a884] px-1 text-[10px] font-bold text-white">
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
  const activeStatus = useAppStore((s) =>
    s.activeService ? s.services[s.activeService].status : null,
  );

  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<Filter>("all");

  const allChats: Chat[] = activeService ? chats[activeService] : [];

  const filteredChats = allChats.filter((chat) => {
    const matchesSearch = chat.name
      .toLowerCase()
      .includes(search.toLowerCase());
    const matchesFilter =
      filter === "all" ||
      (filter === "unread" && (chat.unread ?? 0) > 0) ||
      (filter === "groups" && chat.is_group);
    return matchesSearch && matchesFilter;
  });

  const serviceName = activeService
    ? activeService.charAt(0).toUpperCase() + activeService.slice(1)
    : "Select a service";

  // Reset search/filter when service changes
  useEffect(() => {
    setSearch("");
    setFilter("all");
  }, [activeService]);

  // Fetch chats when bridge becomes connected
  useEffect(() => {
    if (!activeService || activeStatus !== "connected") return;
    sendToBridge(activeService, { type: "chats.list" }).catch(console.error);
  }, [activeService, activeStatus, sendToBridge]);

  const filterLabels: { key: Filter; label: string }[] = [
    { key: "all", label: "All" },
    { key: "unread", label: "Unread" },
    { key: "groups", label: "Groups" },
  ];

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex h-14 flex-shrink-0 items-center border-b border-[#2a2a4a] px-4">
        <h2 className="text-base font-semibold text-gray-100">{serviceName}</h2>
      </div>

      {/* Search bar */}
      {activeService && (
        <div className="px-3 py-2">
          <div className="flex items-center gap-2 rounded-lg bg-[#2a2a4a] px-3 py-2">
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              className="h-4 w-4 flex-shrink-0 text-gray-500"
            >
              <circle cx="11" cy="11" r="8" />
              <line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search"
              className="flex-1 bg-transparent text-sm text-gray-200 placeholder-gray-500 outline-none"
            />
            {search && (
              <button
                onClick={() => setSearch("")}
                className="text-gray-500 hover:text-gray-300"
              >
                ×
              </button>
            )}
          </div>
        </div>
      )}

      {/* Filter tabs */}
      {activeService && (
        <div className="flex gap-1 px-3 pb-2">
          {filterLabels.map(({ key, label }) => (
            <button
              key={key}
              onClick={() => setFilter(key)}
              className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
                filter === key
                  ? "bg-[#00a884] text-white"
                  : "bg-[#2a2a4a] text-gray-400 hover:text-gray-200"
              }`}
            >
              {label}
            </button>
          ))}
        </div>
      )}

      {/* Chat list */}
      <div className="flex-1 overflow-y-auto">
        {!activeService && (
          <div className="flex h-full items-center justify-center text-sm text-gray-500">
            Select a service to view chats
          </div>
        )}
        {activeService && filteredChats.length === 0 && (
          <div className="flex h-full items-center justify-center text-sm text-gray-500">
            {search
              ? `No chats matching "${search}"`
              : filter !== "all"
                ? `No ${filter} chats`
                : "No chats yet"}
          </div>
        )}
        {filteredChats.map((chat) => (
          <ChatItem
            key={chat.id}
            chat={chat}
            isActive={activeChat === chat.id}
            onClick={() => setActiveChat(chat.id)}
          />
        ))}
      </div>
    </div>
  );
}
