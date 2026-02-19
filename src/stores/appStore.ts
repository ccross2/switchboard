import { create } from "zustand";
import type { ServiceID, Chat, Message, BridgeStatus, AuthState } from "../types/protocol";

interface ServiceState {
  status: BridgeStatus;
  authState: AuthState;
}

interface AppState {
  // Service connection state
  services: Record<ServiceID, ServiceState>;
  // Chat lists per service
  chats: Record<ServiceID, Chat[]>;
  // Messages keyed by `${service}:${chatId}`
  messages: Record<string, Message[]>;
  // Active selections
  activeService: ServiceID | null;
  activeChat: string | null;

  // Actions
  setActiveService: (service: ServiceID | null) => void;
  setActiveChat: (chatId: string | null) => void;
  setServiceStatus: (service: ServiceID, status: BridgeStatus) => void;
  setAuthState: (service: ServiceID, authState: AuthState) => void;
  setChats: (service: ServiceID, chats: Chat[]) => void;
  addMessage: (service: ServiceID, chatId: string, message: Message) => void;
  setMessages: (service: ServiceID, chatId: string, messages: Message[]) => void;
  updateUnreadCount: (service: ServiceID, chatId: string, count: number) => void;
}

const defaultServiceState = (): ServiceState => ({
  status: "disconnected",
  authState: { step: "idle" },
});

export const useAppStore = create<AppState>((set) => ({
  services: {
    whatsapp: defaultServiceState(),
    telegram: defaultServiceState(),
  },
  chats: {
    whatsapp: [],
    telegram: [],
  },
  messages: {},
  activeService: null,
  activeChat: null,

  setActiveService: (service) =>
    set({ activeService: service, activeChat: null }),

  setActiveChat: (chatId) =>
    set({ activeChat: chatId }),

  setServiceStatus: (service, status) =>
    set((state) => ({
      services: {
        ...state.services,
        [service]: { ...state.services[service], status },
      },
    })),

  setAuthState: (service, authState) =>
    set((state) => ({
      services: {
        ...state.services,
        [service]: { ...state.services[service], authState },
      },
    })),

  setChats: (service, chats) =>
    set((state) => ({
      chats: {
        ...state.chats,
        [service]: [...chats].sort((a, b) => (b.last_time ?? 0) - (a.last_time ?? 0)),
      },
    })),

  addMessage: (service, chatId, message) => {
    const key = `${service}:${chatId}`;
    set((state) => {
      const existing = state.messages[key] ?? [];
      // Avoid duplicates by id
      if (existing.some((m) => m.id === message.id)) return state;
      // Update chat's last_message + last_time and re-sort
      const updatedChats = state.chats[service]
        .map((chat) =>
          chat.id === chatId
            ? {
                ...chat,
                last_message: message.text || "(media)",
                last_time: message.timestamp,
              }
            : chat,
        )
        .sort((a, b) => (b.last_time ?? 0) - (a.last_time ?? 0));
      return {
        messages: { ...state.messages, [key]: [...existing, message] },
        chats: { ...state.chats, [service]: updatedChats },
      };
    });
  },

  setMessages: (service, chatId, messages) => {
    const key = `${service}:${chatId}`;
    set((state) => ({
      messages: { ...state.messages, [key]: messages },
    }));
  },

  updateUnreadCount: (service, chatId, count) =>
    set((state) => ({
      chats: {
        ...state.chats,
        [service]: state.chats[service].map((chat) =>
          chat.id === chatId ? { ...chat, unread: count } : chat,
        ),
      },
    })),
}));
