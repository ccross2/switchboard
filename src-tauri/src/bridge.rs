use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;

use serde_json::Value;
use tauri::{AppHandle, Emitter};
use tauri_plugin_shell::process::CommandChild;
use tauri_plugin_shell::ShellExt;
use tokio::sync::Mutex;
use tokio::time::sleep;

#[derive(Debug, Clone, PartialEq)]
pub enum BridgeStatus {
    Disconnected,
    Connected,
    AuthNeeded,
}

impl BridgeStatus {
    pub fn as_str(&self) -> &'static str {
        match self {
            BridgeStatus::Disconnected => "disconnected",
            BridgeStatus::Connected => "connected",
            BridgeStatus::AuthNeeded => "auth_needed",
        }
    }
}

struct BridgeProcess {
    child: CommandChild,
    status: BridgeStatus,
}

pub struct BridgeManager {
    bridges: HashMap<String, BridgeProcess>,
}

impl BridgeManager {
    pub fn new() -> Self {
        BridgeManager {
            bridges: HashMap::new(),
        }
    }

    /// Spawn the sidecar for `service` and wire up the stdout reader + auto-restart loop.
    /// The `manager` Arc is passed in so the background task can update status and re-spawn.
    pub fn spawn_bridge(
        manager: Arc<Mutex<BridgeManager>>,
        service: String,
        app: AppHandle,
    ) {
        let binary_name = format!("switchboard-{}", service);

        // Kick off the supervisor task.
        let service_clone = service.clone();
        let app_clone = app.clone();
        let manager_clone = Arc::clone(&manager);

        tauri::async_runtime::spawn(async move {
            Self::supervisor_loop(manager_clone, service_clone, app_clone, binary_name).await;
        });
    }

    /// Inner async loop: spawn → read stdout → emit events → on exit, wait and restart.
    async fn supervisor_loop(
        manager: Arc<Mutex<BridgeManager>>,
        service: String,
        app: AppHandle,
        binary_name: String,
    ) {
        loop {
            // --- Spawn the sidecar ---
            let sidecar_result = app.shell().sidecar(&binary_name);
            let sidecar = match sidecar_result {
                Ok(s) => s,
                Err(e) => {
                    eprintln!("[bridge:{}] Failed to build sidecar command: {}", service, e);
                    sleep(Duration::from_secs(5)).await;
                    continue;
                }
            };

            let spawn_result = sidecar.spawn();
            let (mut rx, child) = match spawn_result {
                Ok(pair) => pair,
                Err(e) => {
                    eprintln!("[bridge:{}] Failed to spawn sidecar: {}", service, e);
                    sleep(Duration::from_secs(5)).await;
                    continue;
                }
            };

            // Store the child process and mark as connected.
            {
                let mut mgr = manager.lock().await;
                mgr.bridges.insert(
                    service.clone(),
                    BridgeProcess {
                        child,
                        status: BridgeStatus::Connected,
                    },
                );
            }

            let event_name = format!("bridge-event-{}", service);

            // --- Read events from the sidecar ---
            loop {
                match rx.recv().await {
                    Some(tauri_plugin_shell::process::CommandEvent::Stdout(line_bytes)) => {
                        let line = String::from_utf8_lossy(&line_bytes);
                        match serde_json::from_str::<Value>(&line) {
                            Ok(payload) => {
                                // Track bridge status from protocol messages.
                                if let Some(msg_type) = payload.get("type").and_then(|t| t.as_str()) {
                                    let new_status = match msg_type {
                                        "auth.success" => Some(BridgeStatus::Connected),
                                        "auth.qr" | "auth.code_needed" | "auth.phone_needed" => {
                                            Some(BridgeStatus::AuthNeeded)
                                        }
                                        "status" => {
                                            // {"type":"status","data":{"status":"auth_needed"|"connected"|...}}
                                            payload
                                                .get("data")
                                                .and_then(|d| d.get("status"))
                                                .and_then(|s| s.as_str())
                                                .and_then(|s| match s {
                                                    "auth_needed" => Some(BridgeStatus::AuthNeeded),
                                                    "connected" => Some(BridgeStatus::Connected),
                                                    "disconnected" => Some(BridgeStatus::Disconnected),
                                                    _ => None,
                                                })
                                        }
                                        _ => None,
                                    };
                                    if let Some(status) = new_status {
                                        let mut mgr = manager.lock().await;
                                        if let Some(bp) = mgr.bridges.get_mut(&service) {
                                            bp.status = status;
                                        }
                                    }
                                }

                                if let Err(e) = app.emit(&event_name, payload) {
                                    eprintln!("[bridge:{}] Failed to emit event: {}", service, e);
                                }
                            }
                            Err(e) => {
                                eprintln!(
                                    "[bridge:{}] Received non-JSON stdout: {} ({})",
                                    service,
                                    line.trim(),
                                    e
                                );
                            }
                        }
                    }
                    Some(tauri_plugin_shell::process::CommandEvent::Stderr(line_bytes)) => {
                        let line = String::from_utf8_lossy(&line_bytes);
                        eprintln!("[bridge:{}] stderr: {}", service, line.trim());
                    }
                    Some(tauri_plugin_shell::process::CommandEvent::Terminated(payload)) => {
                        eprintln!(
                            "[bridge:{}] Process terminated (code: {:?})",
                            service, payload.code
                        );
                        break;
                    }
                    Some(_) => {
                        // Other events (Error, etc.) — ignore.
                    }
                    None => {
                        eprintln!("[bridge:{}] Channel closed", service);
                        break;
                    }
                }
            }

            // Process exited — update status and remove the dead entry.
            {
                let mut mgr = manager.lock().await;
                mgr.bridges.remove(&service);
            }

            // Emit a disconnected event so the frontend can react.
            let _ = app.emit(
                &event_name,
                serde_json::json!({ "status": "disconnected" }),
            );

            eprintln!("[bridge:{}] Restarting in 5 seconds…", service);
            sleep(Duration::from_secs(5)).await;
        }
    }

    /// Write a raw message (newline-terminated) to the bridge's stdin.
    pub async fn send_to_bridge(&mut self, service: &str, message: &str) -> Result<(), String> {
        let bp = self
            .bridges
            .get_mut(service)
            .ok_or_else(|| format!("No bridge running for service '{}'", service))?;

        let mut payload = message.to_string();
        if !payload.ends_with('\n') {
            payload.push('\n');
        }

        bp.child
            .write(payload.as_bytes())
            .map_err(|e| format!("Failed to write to bridge '{}': {}", service, e))
    }

    /// Return the status string for a given service.
    pub fn get_status(&self, service: &str) -> &str {
        self.bridges
            .get(service)
            .map(|bp| bp.status.as_str())
            .unwrap_or("disconnected")
    }
}
