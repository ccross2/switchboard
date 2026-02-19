import { useBridge } from "./hooks/useBridge";
import { useAppStore } from "./stores/appStore";
import Sidebar from "./components/Sidebar";
import ChatList from "./components/ChatList";
import MessageView from "./components/MessageView";
import AuthWhatsApp from "./components/AuthWhatsApp";
import AuthTelegram from "./components/AuthTelegram";

function App() {
  const { sendToBridge } = useBridge();
  const activeService = useAppStore((s) => s.activeService);
  const services = useAppStore((s) => s.services);

  const activeStatus = activeService ? services[activeService].status : null;
  const activeAuth = activeService ? services[activeService].authState : null;

  const showAuth =
    activeService &&
    (activeStatus === "auth_needed" || activeAuth?.step === "qr" ||
      activeAuth?.step === "phone_input" ||
      activeAuth?.step === "code_input");

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-[#1a1a2e] text-gray-100">
      {/* Sidebar — service switcher */}
      <Sidebar />

      {/* Chat list panel */}
      <div className="w-[300px] flex-shrink-0 border-r border-[#2a2a4a] bg-[#16213e]">
        <ChatList sendToBridge={sendToBridge} />
      </div>

      {/* Main content area */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {showAuth && activeService === "whatsapp" && (
          <AuthWhatsApp sendToBridge={sendToBridge} />
        )}
        {showAuth && activeService === "telegram" && (
          <AuthTelegram sendToBridge={sendToBridge} />
        )}
        {!showAuth && <MessageView sendToBridge={sendToBridge} />}
      </div>
    </div>
  );
}

export default App;
