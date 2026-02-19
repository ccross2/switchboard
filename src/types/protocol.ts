// IPC protocol types matching bridges/protocol/protocol.go

export type ServiceID = "whatsapp" | "telegram";

export interface Envelope {
  type: string;
  id?: string;
  data?: unknown;
}

export interface Chat {
  id: string;
  name: string;
  unread: number;
  last_message?: string;
  last_time?: number;
  is_group: boolean;
}

export interface Message {
  id: string;
  chat_id: string;
  from: string;
  from_me: boolean;
  text: string;
  timestamp: number;
  image_path?: string;
}

export interface AuthQR {
  code: string;
}

export interface AuthCodeNeeded {
  phone_hint: string;
}

export interface AuthSuccess {
  user: string;
  phone?: string;
}

export interface ChatListResponse {
  chats: Chat[];
}

export interface ChatMessagesResponse {
  messages: Message[];
}

export interface NotificationData {
  title: string;
  body: string;
  service: ServiceID;
}

export interface StatusData {
  status: "connected" | "disconnected" | "auth_needed";
}

export type AuthState =
  | { step: "idle" }
  | { step: "qr"; code: string }
  | { step: "phone_input" }
  | { step: "code_input"; phone_hint: string }
  | { step: "authenticated"; user: string };

export type BridgeStatus = "disconnected" | "connecting" | "auth_needed" | "connected";
