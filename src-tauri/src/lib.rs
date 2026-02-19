mod bridge;

use std::sync::Arc;

use bridge::BridgeManager;
use tauri::{AppHandle, State};
use tokio::sync::Mutex;

/// Newtype wrapper so we can manage it as Tauri state.
pub struct BridgeState(pub Arc<Mutex<BridgeManager>>);

// ---------------------------------------------------------------------------
// Tauri commands
// ---------------------------------------------------------------------------

/// Send a JSON-line message to a running bridge's stdin.
#[tauri::command]
async fn send_to_bridge(
    service: String,
    message: String,
    state: State<'_, BridgeState>,
) -> Result<(), String> {
    let mut mgr = state.0.lock().await;
    mgr.send_to_bridge(&service, &message).await
}

/// Return the current status string for a bridge ("connected" | "disconnected" | "auth_needed").
#[tauri::command]
async fn get_bridge_status(service: String, state: State<'_, BridgeState>) -> Result<String, String> {
    let mgr = state.0.lock().await;
    Ok(mgr.get_status(&service).to_string())
}

/// Spawn the sidecar bridge for `service` if it is not already running.
#[tauri::command]
async fn start_bridge(
    service: String,
    app: AppHandle,
    state: State<'_, BridgeState>,
) -> Result<(), String> {
    // Quick check: already connected?
    {
        let mgr = state.0.lock().await;
        if mgr.get_status(&service) == "connected" {
            return Ok(());
        }
    }

    BridgeManager::spawn_bridge(Arc::clone(&state.0), service, app);
    Ok(())
}

// ---------------------------------------------------------------------------
// App entry point (called from main.rs)
// ---------------------------------------------------------------------------

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let manager = Arc::new(Mutex::new(BridgeManager::new()));

    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_notification::init())
        .plugin(tauri_plugin_opener::init())
        .manage(BridgeState(manager))
        .invoke_handler(tauri::generate_handler![
            send_to_bridge,
            get_bridge_status,
            start_bridge,
        ])
        .run(tauri::generate_context!())
        .expect("error while running Switchboard");
}
