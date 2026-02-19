import { useEffect, useCallback } from "react";
import { listen } from "@tauri-apps/api/event";
import { invoke } from "@tauri-apps/api/core";
import { sendNotification } from "@tauri-apps/plugin-notification";
import { useAppStore } from "../stores/appStore";
import type {
  ServiceID,
  Envelope,
  AuthQR,
  AuthCodeNeeded,
  AuthSuccess,
  ChatListResponse,
  ChatMessagesResponse,
  StatusData,
  Message,
  NotificationData,
} from "../types/protocol";

export function useBridge() {
  const {
    setServiceStatus,
    setAuthState,
    setChats,
    addMessage,
    setMessages,
  } = useAppStore();

  const handleEvent = useCallback(
    (service: ServiceID, envelope: Envelope) => {
      switch (envelope.type) {
        case "auth.qr": {
          const data = envelope.data as AuthQR;
          setAuthState(service, { step: "qr", code: data.code });
          setServiceStatus(service, "auth_needed");
          break;
        }
        case "auth.code_needed": {
          const data = envelope.data as AuthCodeNeeded;
          setAuthState(service, {
            step: "code_input",
            phone_hint: data.phone_hint,
          });
          setServiceStatus(service, "auth_needed");
          break;
        }
        case "auth.phone_needed": {
          setAuthState(service, { step: "phone_input" });
          setServiceStatus(service, "auth_needed");
          break;
        }
        case "auth.success": {
          const data = envelope.data as AuthSuccess;
          setAuthState(service, { step: "authenticated", user: data.user });
          setServiceStatus(service, "connected");
          break;
        }
        case "chats.list": {
          const data = envelope.data as ChatListResponse;
          setChats(service, data.chats);
          break;
        }
        case "chat.messages": {
          const data = envelope.data as ChatMessagesResponse;
          const msgs = data.messages;
          if (msgs.length > 0) {
            const chatId = msgs[0].chat_id;
            setMessages(service, chatId, msgs);
          }
          break;
        }
        case "message.new": {
          const message = envelope.data as Message;
          addMessage(service, message.chat_id, message);
          // OS notification for incoming messages
          if (!message.from_me) {
            const notif: NotificationData = {
              title: `${service.charAt(0).toUpperCase() + service.slice(1)}: ${message.from}`,
              body: message.text || "(image)",
              service,
            };
            // sendNotification is synchronous (void) — no .catch needed
            sendNotification({ title: notif.title, body: notif.body });
          }
          break;
        }
        case "status": {
          const data = envelope.data as StatusData;
          setServiceStatus(service, data.status);
          if (data.status === "auth_needed") {
            setAuthState(service, { step: "idle" });
          }
          break;
        }
        default:
          // Unknown event type — ignore
          break;
      }
    },
    [setServiceStatus, setAuthState, setChats, addMessage, setMessages],
  );

  useEffect(() => {
    const services: ServiceID[] = ["whatsapp", "telegram"];
    const unlisteners: Array<() => void> = [];

    const setup = async () => {
      for (const service of services) {
        const unlisten = await listen<Envelope>(
          `bridge-event-${service}`,
          (event) => {
            handleEvent(service, event.payload);
          },
        );
        unlisteners.push(unlisten);
      }
      // Bridges are started on-demand when the user selects a service.
    };

    setup().catch(console.error);

    return () => {
      for (const unlisten of unlisteners) {
        unlisten();
      }
    };
  }, [handleEvent]);

  const sendToBridge = useCallback(
    async (service: ServiceID, envelope: Envelope): Promise<void> => {
      const message = JSON.stringify(envelope);
      await invoke("send_to_bridge", { service, message });
    },
    [],
  );

  return { sendToBridge };
}
